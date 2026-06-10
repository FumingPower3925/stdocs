package stdocs

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/FumingPower3925/stdocs/internal/pattern"
	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/spec"
	"github.com/FumingPower3925/stdocs/internal/spec/yaml"
)

// marshalSpec marshals the spec document as compact JSON (no
// whitespace). The emitters already sort keys, so indentation would
// only inflate the payload without aiding readability (UIs
// pretty-print on the client).
func marshalSpec(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Mux is an *http.ServeMux that also records route metadata for OpenAPI
// generation. Use stdocs.New to construct one. Mux embeds *http.ServeMux,
// so all its methods are available transparently.
type Mux struct {
	*http.ServeMux
	cfg *Config
	reg *registry

	// cached spec bytes, lazy-built on first call.
	specJSON []byte
	specYAML []byte
	specMu   sync.Mutex

	// mounted guards Mount against double registration.
	mounted bool
}

// New returns a *stdocs.Mux ready to register routes on.
func New(opts ...Option) *Mux {
	cfg := applyOptions(opts)
	m := &Mux{
		ServeMux: http.NewServeMux(),
		cfg:      cfg,
		reg:      &registry{},
	}
	return m
}

// HandleFunc registers h for the given pattern. opts are RouteOpts that
// document the route.
//
// The pattern must be a Go 1.22+ ServeMux pattern (e.g. "GET /users/{id}").
// If parsing fails, HandleFunc panics. This matches the stdlib's behavior
// of panicking on invalid patterns.
func (m *Mux) HandleFunc(p string, h func(http.ResponseWriter, *http.Request), opts ...RouteOpt) {
	parsed, err := pattern.ParsePattern(p)
	if err != nil {
		panic("stdocs: " + err.Error())
	}
	// Register on the ServeMux first: it is the authoritative
	// validator, and its panic on conflicts must fire before the
	// route is recorded in the spec registry.
	m.ServeMux.HandleFunc(p, h)
	if m.underDocsPrefix(parsed.Path()) {
		return
	}
	// Capture the function name for default summary inference.
	funcName := ""
	if h != nil {
		funcName = funcNameOf(h)
	}
	m.reg.add(p, funcName, parsed, m.cfg.Version, opts)
}

// Handle registers h for the given pattern. The handler's underlying
// function name cannot be recovered from an http.Handler, so routes
// registered via Handle do not benefit from function-name-based summary
// inference (they get a blank summary unless Summary is provided).
func (m *Mux) Handle(p string, h http.Handler, opts ...RouteOpt) {
	parsed, err := pattern.ParsePattern(p)
	if err != nil {
		panic("stdocs: " + err.Error())
	}
	m.ServeMux.Handle(p, h)
	if m.underDocsPrefix(parsed.Path()) {
		return
	}
	m.reg.add(p, "", parsed, m.cfg.Version, opts)
}

// underDocsPrefix reports whether path is the docs prefix itself or
// falls under it. Routes in the docs subtree are infrastructure (the
// docs UI, the spec endpoints, embedded UI assets) and are excluded
// from the generated spec.
func (m *Mux) underDocsPrefix(path string) bool {
	prefix := m.cfg.DocsPrefix
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// JSON returns the OpenAPI spec as JSON bytes. The version depends on
// the mux's configured Version. First call is cached.
func (m *Mux) JSON() ([]byte, error) {
	m.specMu.Lock()
	defer m.specMu.Unlock()
	return m.cachedJSON()
}

// YAML returns the OpenAPI spec as YAML bytes. YAML emission is a
// hand-rolled minimal converter; only the fields we emit are supported.
func (m *Mux) YAML() ([]byte, error) {
	m.specMu.Lock()
	defer m.specMu.Unlock()
	if m.specYAML != nil {
		return m.specYAML, nil
	}
	jsonBytes, err := m.cachedJSON()
	if err != nil {
		return nil, err
	}
	y, err := yaml.FromJSON(jsonBytes)
	if err != nil {
		return nil, err
	}
	m.specYAML = y
	return y, nil
}

// cachedJSON returns the JSON bytes, building and caching them on the
// first call. The caller must hold m.specMu.
func (m *Mux) cachedJSON() ([]byte, error) {
	if m.specJSON != nil {
		return m.specJSON, nil
	}
	doc := m.buildDoc()
	// Run user hooks (WithOpenAPI escape hatch) before marshalling
	// and before validation — hook-added schemes count as
	// registered, so their use sites are valid.
	for _, h := range m.cfg.Hooks {
		h(doc)
	}
	if vErr := validateSecurity(doc); vErr != nil {
		return nil, vErr
	}
	b, err := marshalSpec(doc)
	if err != nil {
		return nil, err
	}
	m.specJSON = b
	return b, nil
}

// validateSecurity walks the spec document and returns an error if
// any operation-level security requirement references a scheme name
// that does not appear in components.securitySchemes (or in the
// top-level "security" array added by a WithOpenAPI hook). A
// misspelled scheme name produces a spec that is invalid per the
// OpenAPI 3.x standard and most consumers silently fail to render
// auth.
func validateSecurity(doc map[string]any) error {
	registered, _ := doc["components"].(map[string]any)
	schemes, _ := registered["securitySchemes"].(map[string]any)
	missing := func(name string) bool {
		if schemes == nil {
			return true
		}
		_, ok := schemes[name]
		return !ok
	}
	// Top-level (global) security set via WithGlobalSecurity or a
	// WithOpenAPI hook.
	if globalSec, ok := doc["security"].([]any); ok {
		for _, entry := range globalSec {
			em, _ := entry.(map[string]any)
			for name := range em {
				if missing(name) {
					return fmt.Errorf("stdocs: security scheme %q referenced in the global security requirement is not registered in components.securitySchemes", name)
				}
			}
		}
	}
	paths, _ := doc["paths"].(map[string]any)
	for path, pi := range paths {
		pim, _ := pi.(map[string]any)
		for method, op := range pim {
			if method == "parameters" {
				continue
			}
			om, _ := op.(map[string]any)
			sec, ok := om["security"]
			if !ok {
				continue
			}
			arr, _ := sec.([]any)
			for _, entry := range arr {
				em, _ := entry.(map[string]any)
				for name := range em {
					if missing(name) {
						return fmt.Errorf("stdocs: security scheme %q referenced in %s %s is not registered in components.securitySchemes", name, strings.ToUpper(method), path)
					}
				}
			}
		}
	}
	return nil
}

// buildDoc assembles the OpenAPI document as a map[string]any. The
// returned map is owned by the caller and may be mutated.
//
// All body and response schemas across all routes and webhooks are
// re-derived here through ONE shared schema.Reflector, so component
// names are unique document-wide: two same-named types from different
// packages get distinct components (User, User_2) with matching $ref
// strings at every use site.
func (m *Mux) buildDoc() map[string]any {
	// Visibility is decided before anything else: routes excluded by
	// Hidden/Internal never reach the reflector (their schemas cannot
	// leak into components), never get finalize defaults, and never
	// participate in operation-id disambiguation — the emitted
	// document is identical to one from a mux where they were never
	// registered.
	visible := &registry{routes: m.visibleRoutes()}
	visible.finalize(m.cfg)
	ref := schema.NewReflector()
	for _, rt := range visible.routes {
		if rb := rt.op.RequestBody; rb != nil && rb.BodyValue != nil {
			rb.Schema = ref.Reflect(rb.BodyValue)
		}
		// Iterate responses in sorted-key order so component-name
		// suffixes are assigned deterministically across rebuilds.
		for _, key := range slices.Sorted(maps.Keys(rt.op.Responses)) {
			if resp := rt.op.Responses[key]; resp != nil && resp.BodyValue != nil {
				resp.Schema = ref.Reflect(resp.BodyValue)
			}
		}
	}
	in := SpecInput{
		Info:            m.cfg.Info,
		Servers:         m.cfg.Servers,
		Tags:            m.cfg.Tags,
		Paths:           visible.toPathItems(),
		Version:         m.cfg.Version,
		SecuritySchemes: m.cfg.Security,
		GlobalSecurity:  m.cfg.GlobalSecurity,
		Webhooks:        m.reflectWebhooks(ref),
	}
	in.Components = ref.Components()
	switch m.cfg.Version {
	case OpenAPI31:
		return spec.BuildRoot31(in)
	case OpenAPI32:
		return spec.BuildRoot32(in, m.cfg.SelfURL)
	default:
		return spec.BuildRoot30(in)
	}
}

// visibleRoutes returns the routes that the current visibility policy
// documents: Hidden routes never appear; Internal routes appear only
// when WithInternal(true) was set, and then carry the conventional
// "x-internal": true extension so spec-filtering tools can recognise
// them.
func (m *Mux) visibleRoutes() []*route {
	out := make([]*route, 0, len(m.reg.routes))
	for _, rt := range m.reg.routes {
		if rt.op.Hidden {
			continue
		}
		if rt.op.Internal {
			if !m.cfg.ShowInternal {
				continue
			}
			if rt.op.Extensions == nil {
				rt.op.Extensions = map[string]any{}
			}
			rt.op.Extensions["x-internal"] = true
		}
		out = append(out, rt)
	}
	return out
}

// reflectWebhooks returns the configured webhooks with every BodyValue
// reflected into a schema through the shared document reflector. The
// user's Config is never mutated: request bodies and responses with a
// BodyValue are copied before their Schema field is filled in.
func (m *Mux) reflectWebhooks(ref *schema.Reflector) map[string]Webhook {
	if len(m.cfg.Webhooks) == 0 {
		return m.cfg.Webhooks
	}
	out := make(map[string]Webhook, len(m.cfg.Webhooks))
	for _, name := range slices.Sorted(maps.Keys(m.cfg.Webhooks)) {
		hook := m.cfg.Webhooks[name]
		if rb := hook.RequestBody; rb != nil && rb.BodyValue != nil {
			rbCopy := *rb
			rbCopy.Schema = ref.Reflect(rbCopy.BodyValue)
			hook.RequestBody = &rbCopy
		}
		if len(hook.Responses) > 0 {
			respCopy := make(map[string]*Response, len(hook.Responses))
			for _, key := range slices.Sorted(maps.Keys(hook.Responses)) {
				resp := hook.Responses[key]
				if resp != nil && resp.BodyValue != nil {
					rc := *resp
					rc.Schema = ref.Reflect(rc.BodyValue)
					resp = &rc
				}
				respCopy[key] = resp
			}
			hook.Responses = respCopy
		}
		out[name] = hook
	}
	return out
}

// Refresh invalidates the spec cache, forcing the next call to JSON or
// YAML to rebuild.
func (m *Mux) Refresh() {
	m.specMu.Lock()
	defer m.specMu.Unlock()
	m.specJSON = nil
	m.specYAML = nil
}

// Config returns the resolved configuration for the mux. It is useful
// for UI sub-packages that need to read or override Config fields.
func (m *Mux) Config() *Config {
	return m.cfg
}

// Mount registers the docs handler on the mux itself, at the
// configured DocsPrefix (default "/docs"). It registers exact patterns
// for the docs page and the two spec endpoints — plus a subtree
// fallback — so a user route like "GET /docs/{file}" can never shadow
// the spec endpoints (exact literals win over wildcards in ServeMux).
//
// The optional bool argument mirrors Docs: pass false to register
// nothing, pass true to register the docs even on a mux disabled via
// WithDisabled, or omit it to follow the WithDisabled config. An
// explicit per-call value wins over WithDisabled in both directions.
// Only one bool is accepted; passing more panics.
//
//	mux.Mount(os.Getenv("ENV") != "prod")
//
// Calling Mount more than once is a no-op after the first call that
// registered the docs. Call it after registering all routes.
func (m *Mux) Mount(enabled ...bool) {
	if len(enabled) > 1 {
		panic("stdocs: Mount accepts at most one bool argument")
	}
	on := !m.cfg.Disabled
	if len(enabled) == 1 {
		on = enabled[0]
	}
	if !on || m.mounted {
		return
	}
	prefix := m.cfg.DocsPrefix
	// Force-enable the handler: the decision was already taken above,
	// so a WithDisabled config must not turn the mounted docs into
	// 404s when the caller passed an explicit true.
	docs := m.Docs(true)
	m.ServeMux.Handle("GET "+prefix+"/{$}", docs)
	m.ServeMux.Handle("GET "+prefix+"/openapi.json", docs)
	m.ServeMux.Handle("GET "+prefix+"/openapi.yaml", docs)
	m.ServeMux.Handle("GET "+prefix+"/", docs)
	m.mounted = true
}

// Docs returns an http.Handler that serves the docs UI and the OpenAPI
// spec for this mux. The returned handler is a sub-mux that internally
// serves:
//
//	GET <prefix>/             -> the HTML docs UI
//	GET <prefix>/openapi.json -> the spec as JSON
//	GET <prefix>/openapi.yaml -> the spec as YAML
//
// In most setups, call Mount instead, which registers this handler at
// the configured docs prefix (default "/docs", changeable via
// WithDocsPrefix). When mounting manually, the registration pattern
// must match the configured prefix:
//
//	mux.ServeMux.Handle("GET /docs/", mux.Docs())
//
// (Plain mux.Handle works too — routes under the docs prefix are
// excluded from the generated spec.)
//
// The optional bool argument enables per-call toggling: pass false to
// get a handler that responds 404 to everything, pass true to force
// the docs on, or omit it to follow the WithDisabled config. An
// explicit per-call value wins over WithDisabled in both directions.
// Only the first value is consulted; passing more than one bool
// panics.
//
//	mux.ServeMux.Handle("GET /docs/", mux.Docs(os.Getenv("ENV") != "prod"))
//
// The decision is taken when Docs is called. For a per-request switch,
// wrap the returned handler in your own middleware.
func (m *Mux) Docs(enabled ...bool) http.Handler {
	if len(enabled) > 1 {
		panic("stdocs: Docs accepts at most one bool argument")
	}
	on := !m.cfg.Disabled
	if len(enabled) == 1 {
		on = enabled[0]
	}
	if !on {
		return http.NotFoundHandler()
	}
	core, err := newDocsCore(m.cfg, m.JSON, m.YAML)
	if err != nil {
		// A malformed UI constant is a programming error; surface
		// it as a 500 on every request.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return core
}

// funcNameOf returns the function name of f via reflection. Returns ""
// if it cannot be determined.
func funcNameOf(f any) string {
	if f == nil {
		return ""
	}
	fv := reflect.ValueOf(f)
	pc := fv.Pointer()
	if pc == 0 {
		return ""
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return ""
	}
	return fn.Name()
}

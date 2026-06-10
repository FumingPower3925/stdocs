package stdocs

import (
	"encoding/json"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/FumingPower3925/stdocs/internal/pattern"
	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/spec"
	"github.com/FumingPower3925/stdocs/internal/spec/yaml"
)

// jsonMarshalIndent marshals v as compact JSON (no whitespace). The
// emitters already sort keys, so indentation would only inflate the
// payload without aiding readability (UIs pretty-print on the client).
func jsonMarshalIndent(v any) ([]byte, error) {
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
	// Capture the function name for default summary inference.
	funcName := ""
	if h != nil {
		funcName = funcNameOf(h)
	}
	m.reg.add(p, funcName, parsed, m.cfg.Version, opts)
	m.ServeMux.HandleFunc(p, h)
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
	m.reg.add(p, "", parsed, m.cfg.Version, opts)
	m.ServeMux.Handle(p, h)
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
	doc, err := m.buildDoc()
	if err != nil {
		return nil, err
	}
	// Run user hooks (WithOpenAPI escape hatch) before marshalling.
	for _, h := range m.cfg.Hooks {
		h(doc)
	}
	b, err := jsonMarshalIndent(doc)
	if err != nil {
		return nil, err
	}
	m.specJSON = b
	return b, nil
}

// buildDoc assembles the OpenAPI document as a map[string]any. The
// returned map is owned by the caller and may be mutated.
func (m *Mux) buildDoc() (map[string]any, error) {
	m.reg.finalize(m.cfg)
	in := SpecInput{
		Info:            m.cfg.Info,
		Servers:         m.cfg.Servers,
		Tags:            m.cfg.Tags,
		Paths:           m.reg.toPathItems(),
		Version:         m.cfg.Version,
		SecuritySchemes: m.cfg.Security,
		GlobalSecurity:  m.cfg.GlobalSecurity,
		Webhooks:        m.cfg.Webhooks,
	}
	comps := make(map[string]*schema.Schema)
	for _, rt := range m.reg.routes {
		if rb := rt.op.RequestBody; rb != nil && rb.BodyValue != nil {
			_, c := schema.ReflectSchema(rb.BodyValue, m.cfg.Version)
			for n, s := range c {
				comps[n] = s
			}
		}
		for _, resp := range rt.op.Responses {
			if resp != nil && resp.BodyValue != nil {
				_, c := schema.ReflectSchema(resp.BodyValue, m.cfg.Version)
				for n, s := range c {
					comps[n] = s
				}
			}
		}
	}
	in.Components = comps
	if m.cfg.Version == OpenAPI31 {
		return spec.BuildRoot31(in), nil
	}
	return spec.BuildRoot30(in), nil
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

// Mount registers the docs handler on the mux itself, at the configured
// DocsPrefix. It is shorthand for:
//
//	m.ServeMux.Handle("GET "+m.cfg.DocsPrefix+"/", m.Docs())
//
// Call this after registering all routes.
func (m *Mux) Mount() {
	prefix := m.cfg.DocsPrefix
	m.ServeMux.Handle("GET "+prefix+"/", m.Docs())
}

// Docs returns an http.Handler that serves the docs UI and the OpenAPI
// spec for this mux. The returned handler is a sub-mux that internally
// serves:
//
//	GET <prefix>/             -> the HTML docs UI
//	GET <prefix>/openapi.json -> the spec as JSON
//	GET <prefix>/openapi.yaml -> the spec as YAML
//
// Mount it on a parent mux with mux.Handle("GET "+cfg.DocsPrefix+"/", m.Docs()).
// The docs prefix defaults to "/docs" but can be changed via
// WithDocsPrefix.
func (m *Mux) Docs() http.Handler {
	return &muxDocs{
		mux: m,
		cfg: m.cfg,
		ui:  m.cfg.UIDoc,
	}
}

// muxDocs is the handler returned by Mux.Docs.
type muxDocs struct {
	mux *Mux
	cfg *Config
	ui  string
}

func (d *muxDocs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip the prefix.
	path := strings.TrimPrefix(r.URL.Path, d.cfg.DocsPrefix)
	switch {
	case path == "" || path == "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := d.ui
		html = strings.ReplaceAll(html, "{{.Title}}", d.cfg.Info.Title)
		html = strings.ReplaceAll(html, "{{.SpecURL}}", d.cfg.DocsPrefix+"/openapi.json")
		w.Write([]byte(html))
	case path == "/openapi.json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		b, err := d.mux.JSON()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(b)
	case path == "/openapi.yaml":
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		b, err := d.mux.YAML()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(b)
	default:
		http.NotFound(w, r)
	}
}

// (buildSpec has been inlined into cachedJSON for clarity and to keep
// the lock scope small.)

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

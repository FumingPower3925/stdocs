package stdocs

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/FumingPower3925/stdocs/internal/spec"
)

// Config holds the resolved configuration for an stdocs Mux.
// It is built by applying a list of Options to a fresh Config and is
// shared with the registry and the spec emitter.
//
// Config is exported (rather than unexported like in many libraries)
// so that UI sub-packages (e.g. stdocs/ui/scalar) can mutate it via
// a stdocs.Option. UI sub-packages should not construct or copy a
// Config; they should only read or write the UIDoc field.
type Config struct {
	// Info carries the OpenAPI "info" object.
	Info Info
	// Servers is the list of OpenAPI "servers".
	Servers []Server
	// Tags are the declared top-level tags. Tags attached to operations
	// that aren't in this list are still emitted on the operation; the
	// declarations are just for richer descriptions.
	Tags []TagDecl
	// DocsPrefix is the URL prefix under which the docs UI and spec are
	// served. Defaults to "/docs".
	DocsPrefix string
	// SelfURL is the OpenAPI 3.2 "$self" field — the canonical URI
	// of the document. When non-empty, the 3.2 emitter includes it
	// in the spec root. Ignored for 3.0 and 3.1 (the field does
	// not exist in those versions).
	SelfURL string
	// Version is the OpenAPI version to emit. Defaults to OpenAPI30.
	Version SpecVersion
	// DefaultSummary is a fallback summary used for routes that have
	// neither a per-route Summary nor a function-name-based inference.
	DefaultSummary string
	// UIDoc is the HTML template for the docs UI page. The default is
	// the small dependency-free page in stdocs.defaultUIDoc. UI
	// sub-packages override this field via a stdocs.Option to swap in
	// their own page. The template may contain {{.Title}} and
	// {{.SpecURL}} placeholders; the page is rendered once, when the
	// docs handler is constructed.
	UIDoc string
	// UICSP is the Content-Security-Policy served with the docs HTML
	// page. It travels with the UI: each UI sub-package sets the policy
	// its bundle needs from its WithUI option, and the default is the
	// strict policy for the built-in page (see defaultDocsCSP). Emitted
	// only when DocsSecurityHeaders is true.
	UICSP string
	// UIConfig is UI-native configuration forwarded to the rendered docs
	// page. A UI sub-package's WithConfiguration option populates it; it
	// is nil by default and the default built-in page ignores it. Its
	// meaning is per-UI (see each sub-package's godoc): a JSON
	// configuration object for Scalar, Swagger UI, and Redoc, or a set of
	// element attributes for Stoplight. The map is rendered once, when the
	// docs handler is constructed.
	UIConfig map[string]any
	// DocsSecurityHeaders controls whether the docs handler sends the
	// CSP and the other hardening headers (nosniff, Referrer-Policy,
	// X-Frame-Options, Permissions-Policy). On by default; set via
	// WithDocsSecurityHeaders(false) for callers who set their own
	// headers or front the mux with their own middleware.
	DocsSecurityHeaders bool
	// Hooks is the list of post-build callbacks registered via
	// WithOpenAPI. Each is called once per spec build, with the
	// emitted spec as a map[string]any, and may mutate it in place.
	Hooks []func(doc map[string]any)
	// Security is the list of registered security schemes. The user
	// adds entries via WithSecurityScheme / WithBearerAuth / etc.;
	// each is rendered into components.securitySchemes at build time.
	Security []spec.NamedSecurityScheme
	// GlobalSecurity is the default security requirement applied to
	// every operation. Operations may override with WithSecurity or
	// opt out with WithNoSecurity.
	GlobalSecurity []SecurityRequirement
	// Webhooks are emitted for 3.1 and 3.2 specs and ignored for 3.0
	// (the field does not exist there). The map is keyed by webhook
	// name.
	Webhooks map[string]Webhook
	// Disabled turns off the docs handler. When true, Mux.Docs and
	// DocsHandler return a 404 handler instead of serving the UI and
	// the spec. Mux.Mount respects this and registers nothing.
	Disabled bool
	// StaticSpec is a hand-written OpenAPI document (JSON bytes) set
	// via WithSpec. It is served verbatim by DocsHandler (Tier 1) and
	// ignored by *Mux (Tier 2), which generates its own document.
	StaticSpec []byte
	// ShowInternal controls whether routes marked with the Internal
	// route opt appear in the generated document. Defaults to false
	// (internal routes hidden). Set via WithInternal.
	ShowInternal bool
	// DefaultResponses are mux-level response entries documented on
	// every operation that does not itself declare the same status.
	// Populated via WithDefaultResponse.
	DefaultResponses []DefaultResponse
	// DisableAutoUnauthorized turns off the automatic 401 response
	// documented on operations that carry a security requirement.
	// The zero value keeps the feature on; set via
	// WithAutoUnauthorized(false).
	DisableAutoUnauthorized bool
	// PathPrefix is prepended to every documented path. Documentation
	// only — routing is unaffected. Set via WithPathPrefix.
	PathPrefix string
	// CleanOutput strips stdocs vendor noise from the generated
	// document. On by default; set via WithCleanOutput(false) to keep
	// the annotations.
	CleanOutput bool
	// ExternalDocs is the document-level externalDocumentation
	// object. Set via WithExternalDocs.
	ExternalDocs *spec.ExternalDocs
	// OperationIDFunc overrides the automatic operationId derivation.
	// Set via WithOperationIDFunc.
	OperationIDFunc func(method, path string) string
	// TagFunc overrides the automatic tag inference. Set via
	// WithTagFunc.
	TagFunc func(method, path string) string
	// Assets, when non-nil, serves the docs UI's static assets. The
	// embedded UI sub-packages set it from their WithUI option so
	// Mount can register the <prefix>/_assets/ route automatically.
	Assets http.Handler
}

// DefaultResponse is a mux-level response declaration applied to
// every documented operation; see WithDefaultResponse.
type DefaultResponse struct {
	// Status is the HTTP status code; 0 means the OpenAPI "default"
	// response.
	Status int
	// Body is a zero value whose type is reflected into the response
	// schema, like the body argument of WithResponse; nil means no
	// body. Ignored when RawContentType is set.
	Body any
	// RawContentType, when set, declares the entry as a raw
	// string-typed body under this content type instead — the
	// fallback/default twin of WithRawResponse. Populated via
	// WithDefaultRawResponse and WithFallbackRawResponse.
	RawContentType string
}

// Option is a function that mutates a config. Options are applied by
// New and DocsHandler at construction time.
type Option func(*Config)

// WithCleanOutput controls whether stdocs strips its own annotation
// noise from the generated document:
//
//   - the "Generated from Go type main.T." fallback schema
//     descriptions (user-supplied doc: tags are always kept), and
//   - the x-stdocs-type and x-stdocs-warning annotation extensions.
//
// It is ON by default — the published document is a contract for
// client generators, linters, and API portals, and the annotations
// leak package layout into it. Pass WithCleanOutput(false) to keep
// the annotations, useful when debugging which Go types produced
// which schemas.
//
// The x-stdocs-additionalOperations extension is NEVER stripped: on
// 3.0/3.1 it is the only representation of custom-method operations,
// and removing it would silently drop documented routes. Hooks
// registered with WithOpenAPI run after cleaning and may add
// anything back.
func WithCleanOutput(enabled bool) Option {
	return func(c *Config) {
		c.CleanOutput = enabled
	}
}

// WithDocsSecurityHeaders controls the hardening headers the docs
// handler sends with the UI page and the spec endpoints: a
// Content-Security-Policy on the HTML, plus X-Content-Type-Options,
// Referrer-Policy, X-Frame-Options, and Permissions-Policy.
//
// It is ON by default. The CSP is scoped to whichever UI is active —
// the built-in page gets a strict default-src 'none' policy with its
// inline script and style pinned by hash; each rich UI sub-package
// ships the policy its bundle needs. Pass WithDocsSecurityHeaders(false)
// when the docs are served behind your own header middleware, or when
// you embed the page cross-origin, which the frame-ancestors 'self'
// policy blocks. There is intentionally no Strict-Transport-Security header:
// HSTS governs the whole origin over TLS, which is the server's or the
// edge's job, not a docs sub-handler's.
func WithDocsSecurityHeaders(enabled bool) Option {
	return func(c *Config) {
		c.DocsSecurityHeaders = enabled
	}
}

// WithCSP replaces the Content-Security-Policy served with the docs
// HTML page. The default is the policy for the active UI; pass this to
// tighten it, to widen it for an extra source (an analytics endpoint,
// say), or to match a policy you enforce elsewhere. Apply it after the
// UI option, since the UI sets its own policy:
//
//	mux := stdocs.New(scalar.WithUI(), stdocs.WithCSP(myPolicy))
//
// It has no effect when WithDocsSecurityHeaders(false) is set.
func WithCSP(policy string) Option {
	return func(c *Config) {
		c.UICSP = policy
	}
}

// WithPathPrefix prepends prefix to every path in the generated
// document. Use it when the mux is mounted under a prefix the
// application never sees — http.StripPrefix("/api", mux) or a
// reverse proxy that strips "/api" — so the documented paths match
// the URLs clients actually call:
//
//	mux := stdocs.New(
//	    stdocs.WithTitle("My API"),
//	    stdocs.WithPathPrefix("/api"),
//	)
//	// GET /users is documented as /api/users.
//
// Documentation only: routing, the docs prefix, and FromDocs are
// unaffected. The value is normalized like WithDocsPrefix (leading
// slash added, trailing slash removed); an empty value means no
// prefix and the root prefix "/" is rejected with a panic.
func WithPathPrefix(prefix string) Option {
	return func(c *Config) {
		if prefix == "" {
			c.PathPrefix = ""
			return
		}
		if strings.ContainsAny(prefix, "{}? \t") {
			panic("stdocs: WithPathPrefix(" + strconv.Quote(prefix) + ") must be a literal path prefix without wildcards, query strings, or whitespace")
		}
		normalized := "/" + strings.Trim(prefix, "/")
		if normalized == "/" {
			panic("stdocs: WithPathPrefix(" + strconv.Quote(prefix) + ") resolves to the root prefix, which is not supported; use no prefix instead")
		}
		c.PathPrefix = normalized
	}
}

// WithAutoUnauthorized controls the automatic 401 response: by
// default, every operation that carries a security requirement —
// per-route via WithSecurity, or inherited from WithGlobalSecurity
// and not opted out with WithNoSecurity — documents a 401
// ("Unauthorized") response, since an authenticated endpoint can
// always reject the credentials. A per-route WithResponse(401, ...)
// or a WithDefaultResponse(401, ...) body wins over the bare entry —
// note that WithDefaultResponse(401, ...) documents the 401 on every
// operation, including unsecured ones, per its own contract.
// Pass false to suppress the automatic 401 mux-wide.
func WithAutoUnauthorized(enabled bool) Option {
	return func(c *Config) {
		c.DisableAutoUnauthorized = !enabled
	}
}

// WithDefaultResponse documents a response on every operation that
// does not itself declare the same status — typically the API's
// shared error envelope, declared once instead of on every route:
//
//	mux := stdocs.New(
//	    stdocs.WithTitle("My API"),
//	    stdocs.WithDefaultResponse(500, APIError{}),
//	)
//
// A per-route WithResponse (even with a nil body) or WithRawResponse
// for the same status wins; an entry materialized only by decorators
// (WithResponseDescription and friends) still takes the default
// body. Pass status 0 for the OpenAPI "default" response and
// nil for a body-less entry. The entry applies to every operation —
// to document a 401 only on secured routes, rely on the automatic
// 401 instead (see WithAutoUnauthorized). Multiple calls accumulate;
// repeating a status panics, as does a status outside 100-599
// (other than 0).
func WithDefaultResponse(status int, body any) Option {
	if status != 0 && (status < 100 || status > 599) {
		panic("stdocs: WithDefaultResponse status must be 0 (default) or 100-599, got " + itoa(status))
	}
	return func(c *Config) {
		for _, dr := range c.DefaultResponses {
			if dr.Status == status {
				panic("stdocs: WithDefaultResponse: status " + statusKey(status) + " already has a mux-level default response")
			}
		}
		c.DefaultResponses = append(c.DefaultResponses, DefaultResponse{Status: status, Body: body})
	}
}

// WithDefaultRawResponse is [WithDefaultResponse] for raw bodies: it
// documents a string-typed response under the given content type on
// every operation that does not itself declare the status — an API
// whose shared error surface is plain text declares it once:
//
//	mux := stdocs.New(
//	    stdocs.WithTitle("My API"),
//	    stdocs.WithDefaultRawResponse(500, "text/plain; charset=utf-8"),
//	)
//
// Precedence and accumulation follow WithDefaultResponse exactly: a
// per-route declaration or fallback wins, repeating a status across
// either default form panics, and status 0 means the OpenAPI
// "default" response. The filled entry is what [WithRawResponse]
// declares, except a content type set by WithResponseContentType on
// the route survives. The content type is required.
func WithDefaultRawResponse(status int, contentType string) Option {
	if status != 0 && (status < 100 || status > 599) {
		panic("stdocs: WithDefaultRawResponse status must be 0 (default) or 100-599, got " + itoa(status))
	}
	if contentType == "" {
		panic("stdocs: WithDefaultRawResponse requires a content type")
	}
	return func(c *Config) {
		for _, dr := range c.DefaultResponses {
			if dr.Status == status {
				panic("stdocs: WithDefaultRawResponse: status " + statusKey(status) + " already has a mux-level default response")
			}
		}
		c.DefaultResponses = append(c.DefaultResponses, DefaultResponse{Status: status, RawContentType: contentType})
	}
}

// WithTitle sets the API title. The default is "API".
func WithTitle(title string) Option {
	return func(c *Config) { c.Info.Title = title }
}

// WithVersion sets the OpenAPI spec version. Accepts OpenAPI30
// (3.0.4), OpenAPI31 (3.1.2), or OpenAPI32 (3.2.0). A string literal
// like "3.0.4" is also accepted because SpecVersion is a defined
// string type with the same underlying values.
//
// WithVersion panics on an unknown version string. Options run at
// New()/DocsHandler() time, the same fail-fast window where bad patterns
// already panic; silently coercing to a default would mask user
// errors.
func WithVersion(v SpecVersion) Option {
	return func(c *Config) {
		switch v {
		case OpenAPI30, OpenAPI31, OpenAPI32:
			c.Version = v
		default:
			panic("stdocs: WithVersion: unknown OpenAPI version " + string(v) +
				" (expected " + string(OpenAPI30) + ", " + string(OpenAPI31) +
				", or " + string(OpenAPI32) + ")")
		}
	}
}

// WithDescription sets the API description.
func WithDescription(s string) Option {
	return func(c *Config) { c.Info.Description = s }
}

// WithAPIVersion sets the API version string in the OpenAPI "info"
// block (e.g. "1.0.0"). This is independent of WithVersion which sets
// the OpenAPI specification version (3.0.4 vs 3.1.2 vs 3.2.0).
func WithAPIVersion(v string) Option {
	return func(c *Config) { c.Info.Version = v }
}

// WithServer adds a server entry.
func WithServer(url, description string) Option {
	return func(c *Config) {
		c.Servers = append(c.Servers, Server{URL: url, Description: description})
	}
}

// WithContact sets the contact info.
func WithContact(name, email, url string) Option {
	return func(c *Config) {
		c.Info.Contact = &Contact{Name: name, Email: email, URL: url}
	}
}

// WithLicense sets the license info. For an SPDX license expression
// instead of a URL, use WithSPDXLicense; the spec treats url and
// identifier as mutually exclusive, and whichever of the two options
// is applied last wins.
func WithLicense(name, url string) Option {
	return func(c *Config) {
		c.Info.License = &License{Name: name, URL: url}
	}
}

// WithSPDXLicense sets the license as an SPDX expression:
//
//	stdocs.WithSPDXLicense("Apache 2.0", "Apache-2.0")
//
// The identifier field exists from OpenAPI 3.1 on; a 3.0 document
// degrades to the name alone.
func WithSPDXLicense(name, identifier string) Option {
	return func(c *Config) {
		c.Info.License = &License{Name: name, Identifier: identifier}
	}
}

// WithExternalDocs sets the document-level link to external
// documentation. The URL is required and must parse as a URI; tags
// take their own link via WithTagExternalDocs and operations via the
// ExternalDocs route opt.
func WithExternalDocs(url, description string) Option {
	mustValidDocsURL("WithExternalDocs", url)
	return func(c *Config) {
		c.ExternalDocs = &spec.ExternalDocs{URL: url, Description: description}
	}
}

// WithTagExternalDocs attaches an external-documentation link to a
// declared tag, creating the declaration if WithTag has not (yet)
// declared it — the order of the two options does not matter.
func WithTagExternalDocs(tag, url, description string) Option {
	mustValidDocsURL("WithTagExternalDocs", url)
	return func(c *Config) {
		for i := range c.Tags {
			if c.Tags[i].Name == tag {
				c.Tags[i].ExternalDocs = &spec.ExternalDocs{URL: url, Description: description}
				return
			}
		}
		c.Tags = append(c.Tags, TagDecl{Name: tag, ExternalDocs: &spec.ExternalDocs{URL: url, Description: description}})
	}
}

// WithOperationIDFunc overrides the automatic operationId derivation
// with f, called with the route's HTTP method (upper-case, "" for
// method-less patterns) and path:
//
//	stdocs.WithOperationIDFunc(func(method, path string) string {
//	    return strings.ToLower(method) + strings.ReplaceAll(path, "/", ".")
//	})
//
// An explicit OperationID route opt still wins, an empty result
// falls back to the default derivation, and document-wide
// uniqueness suffixing applies to the result as usual.
func WithOperationIDFunc(f func(method, path string) string) Option {
	return func(c *Config) {
		c.OperationIDFunc = f
	}
}

// WithTagFunc overrides the automatic tag inference (first non-version
// path segment) with f, called with the route's HTTP method
// (upper-case, "" for method-less patterns) and path. An explicit
// Tags route opt still wins; an empty result falls back to the
// default inference.
//
//	stdocs.WithTagFunc(func(method, path string) string {
//	    return strings.Split(path, "/")[2] // /api/{group}/...
//	})
func WithTagFunc(f func(method, path string) string) Option {
	return func(c *Config) {
		c.TagFunc = f
	}
}

// mustValidDocsURL panics unless u is a non-empty, parseable URI —
// the OpenAPI externalDocumentation object requires its url field.
func mustValidDocsURL(what, u string) {
	if u == "" {
		panic("stdocs: " + what + " requires a URL")
	}
	if _, err := url.Parse(u); err != nil {
		panic("stdocs: " + what + " URL " + strconv.Quote(u) + " does not parse: " + err.Error())
	}
}

// WithDocsPrefix overrides the URL prefix for the docs UI. The
// default is "/docs". The value is normalized: a leading slash is
// added if missing, and a trailing slash is removed so the prefix
// is comparable to strings.TrimPrefix results. An empty prefix is
// replaced with the default "/docs"; to turn the docs UI off, use
// WithDisabled or pass false to Mux.Docs.
//
// The root prefix "/" is rejected with a panic: it would claim the
// whole URL space and produce an invalid ServeMux pattern in Mount.
func WithDocsPrefix(prefix string) Option {
	return func(c *Config) {
		if prefix == "" {
			c.DocsPrefix = "/docs"
			return
		}
		normalized := "/" + strings.Trim(prefix, "/")
		if normalized == "/" {
			panic(`stdocs: WithDocsPrefix("/") is not supported; the docs prefix must be a non-root path like "/docs"`)
		}
		c.DocsPrefix = normalized
	}
}

// WithDisabled turns off the docs UI and the spec endpoints. Useful
// for environment-based toggling (e.g. don't expose docs in production,
// or behind a feature flag):
//
//	mux := stdocs.New(
//	    stdocs.WithDisabled(os.Getenv("ENV") == "prod"),
//	)
//
// When the mux is disabled, Mux.Docs returns a 404 handler and
// Mux.Mount registers nothing. JSON and YAML still produce the spec
// bytes — disabling the docs UI does not stop spec generation.
//
// For per-call toggling (e.g. a config that may change at runtime),
// pass the bool directly to Mux.Docs(enabled) instead.
func WithDisabled(disabled bool) Option {
	return func(c *Config) { c.Disabled = disabled }
}

// WithInternal sets whether routes marked with the Internal route
// opt appear in the generated OpenAPI document. The default is
// false: internal routes are hidden, so forgetting this option can
// never leak a sensitive endpoint into a published spec. When shown,
// internal operations carry an "x-internal": true extension.
//
// Typical environment wiring, together with WithDisabled:
//
//	env := os.Getenv("ENV")
//	mux := stdocs.New(
//	    stdocs.WithDisabled(env == "prod"),  // prod: no docs at all
//	    stdocs.WithInternal(env == "dev"),   // dev: everything; staging: internal hidden
//	)
//
// Visibility only shapes the published documentation; hidden and
// internal routes still serve traffic in every environment.
func WithInternal(show bool) Option {
	return func(c *Config) { c.ShowInternal = show }
}

// WithSelfURL sets the OpenAPI 3.2 "$self" field. This is the
// canonical URI of the document. It is emitted only in 3.2 specs;
// setting it on a 3.0 or 3.1 mux has no effect because those
// versions do not have the field.
//
// The value must be a valid RFC 3986 URI reference without a
// fragment (both constraints come from the OpenAPI 3.2
// specification and its published JSON Schema). WithSelfURL panics
// on invalid input, consistent with WithVersion's fail-fast
// behavior at New()/DocsHandler() time.
func WithSelfURL(selfURL string) Option {
	return func(c *Config) {
		if strings.Contains(selfURL, "#") {
			panic("stdocs: WithSelfURL: $self must not contain a fragment (OpenAPI 3.2 requires a fragment-free URI reference)")
		}
		if _, err := url.Parse(selfURL); err != nil || strings.ContainsAny(selfURL, " \t\n") {
			panic("stdocs: WithSelfURL: " + strconv.Quote(selfURL) + " is not a valid RFC 3986 URI reference")
		}
		c.SelfURL = selfURL
	}
}

// WithTag declares a top-level tag and its description. Tags attached to
// operations that match a declared tag are also valid; undeclared tags
// are still emitted.
func WithTag(name, description string) Option {
	return func(c *Config) {
		// Merge into an existing declaration (tag names must be
		// unique in OpenAPI, and WithTagExternalDocs may have created
		// the entry first — the order of the two options does not
		// matter).
		for i := range c.Tags {
			if c.Tags[i].Name == name {
				c.Tags[i].Description = description
				return
			}
		}
		c.Tags = append(c.Tags, TagDecl{Name: name, Description: description})
	}
}

// WithDefaultSummary sets a default summary template used for routes
// that do not provide one and whose function name does not yield a
// useful inference. Use {resource} as a placeholder for the first path
// segment (the inferred tag).
func WithDefaultSummary(template string) Option {
	return func(c *Config) { c.DefaultSummary = template }
}

// WithOpenAPI registers a callback that runs after the spec is built
// and before it is cached. The callback receives the spec as a
// map[string]any and may mutate it in place. This is the escape hatch
// for features stdocs does not expose directly: security schemes,
// webhooks, custom x-extensions, vendor extensions, etc.
//
// The callback is invoked once per build (i.e. once before the cache
// is populated; subsequent reads use the cache). Call Refresh to force
// the callback to run again.
//
// Example:
//
//	stdocs.WithOpenAPI(func(doc map[string]any) {
//	    doc["security"] = []map[string]any{{
//	        "bearerAuth": []string{},
//	    }}
//	    doc["components"].(map[string]any)["securitySchemes"] = map[string]any{
//	        "bearerAuth": map[string]any{
//	            "type":         "http",
//	            "scheme":       "bearer",
//	            "bearerFormat": "JWT",
//	        },
//	    }
//	})
func WithOpenAPI(fn func(doc map[string]any)) Option {
	return func(c *Config) {
		c.Hooks = append(c.Hooks, fn)
	}
}

// WithGlobalSecurity sets the default security requirement applied to
// every operation that does not specify its own. Operations can opt
// out with stdocs.WithNoSecurity or override with stdocs.WithSecurity.
//
//	stdocs.WithGlobalSecurity("bearerAuth")
//	stdocs.WithGlobalSecurity("oauth2Auth", "read:users")
func WithGlobalSecurity(name string, scopes ...string) Option {
	return func(c *Config) {
		if name == "" {
			return
		}
		c.GlobalSecurity = append(c.GlobalSecurity, SecurityRequirement{name: append([]string{}, scopes...)})
	}
}

// newConfig returns a config with sane defaults.
func newConfig() *Config {
	return &Config{
		Info: Info{
			Title:   "API",
			Version: "0.0.0",
		},
		Servers:             []Server{{URL: "/"}},
		DocsPrefix:          "/docs",
		Version:             OpenAPI30,
		UIDoc:               defaultUIDoc,
		UICSP:               defaultDocsCSP,
		DocsSecurityHeaders: true,
		CleanOutput:         true,
	}
}

// applyOptions returns a config with opts applied.
func applyOptions(opts []Option) *Config {
	c := newConfig()
	for _, o := range opts {
		o(c)
	}
	return c
}

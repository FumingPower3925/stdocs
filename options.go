package stdocs

import "github.com/FumingPower3925/stdocs/internal/spec"

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
	// the raw zero-JS UI in stdocs.defaultUIDoc. UI sub-packages
	// override this field via a stdocs.Option to swap in their own
	// page. The template may contain {{.Title}} and {{.SpecURL}}
	// placeholders, which are substituted at request time.
	UIDoc string
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
	// Webhooks are 3.1-only. The map is keyed by webhook name. The
	// emitter ignores this field for 3.0.3 specs.
	Webhooks map[string]Webhook
	// Disabled turns off the docs handler. When true, Mux.Docs and
	// DocsHandler return a 404 handler instead of serving the UI and
	// the spec. Mux.Mount respects this and registers nothing.
	Disabled bool
}

// Option is a function that mutates a config. Used at New() and Mount()
// time.
type Option func(*Config)

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
// New()/Mount() time, the same fail-fast window where bad patterns
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
// the OpenAPI specification version (3.0.3 vs 3.1.0).
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

// WithLicense sets the license info.
func WithLicense(name, url string) Option {
	return func(c *Config) {
		c.Info.License = &License{Name: name, URL: url}
	}
}

// WithDocsPrefix overrides the URL prefix for the docs UI. The
// default is "/docs". The value is normalized: a leading slash is
// added if missing, and a trailing slash is removed so the prefix
// is comparable to strings.TrimPrefix results. An empty prefix
// disables the docs UI (the user is expected to call Mux.Docs()
// themselves, but a sensible user keeps a non-empty prefix).
func WithDocsPrefix(prefix string) Option {
	return func(c *Config) {
		if prefix == "" {
			prefix = "/docs"
		}
		if prefix[0] != '/' {
			prefix = "/" + prefix
		}
		// Strip trailing slash; strings.TrimPrefix expects to
		// match exactly.
		if len(prefix) > 1 && prefix[len(prefix)-1] == '/' {
			prefix = prefix[:len(prefix)-1]
		}
		c.DocsPrefix = prefix
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

// WithSelfURL sets the OpenAPI 3.2 "$self" field. This is the
// canonical URI of the document. It is emitted only in 3.2 specs;
// setting it on a 3.0 or 3.1 mux has no effect because those
// versions do not have the field.
func WithSelfURL(selfURL string) Option {
	return func(c *Config) { c.SelfURL = selfURL }
}

// WithTag declares a top-level tag and its description. Tags attached to
// operations that match a declared tag are also valid; undeclared tags
// are still emitted.
func WithTag(name, description string) Option {
	return func(c *Config) {
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
		Servers:    []Server{{URL: "/"}},
		DocsPrefix: "/docs",
		Version:    OpenAPI30,
		UIDoc:      defaultUIDoc,
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

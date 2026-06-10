package stdocs

import (
	"net/http"

	"github.com/FumingPower3925/stdocs/internal/spec/yaml"
)

// WithSpec sets a static OpenAPI document (JSON bytes) for
// DocsHandler to serve at <prefix>/openapi.json (and, converted, at
// <prefix>/openapi.yaml). This is the Tier-1 path for exposing a
// hand-written spec through the docs UI:
//
//	specJSON, _ := os.ReadFile("openapi.json")
//	mux.Handle("GET /docs/", stdocs.DocsHandler(
//	    stdocs.WithTitle("My API"),
//	    stdocs.WithSpec(specJSON),
//	))
//
// WithSpec has no effect on *stdocs.Mux (Tier 2), which always
// generates its spec from the registered routes.
func WithSpec(specJSON []byte) Option {
	return func(c *Config) { c.StaticSpec = specJSON }
}

// DocsHandler returns an http.Handler that serves the docs UI and a
// static OpenAPI spec. This is the Tier-1 entry point: it does not
// introspect any mux. Provide the document with WithSpec; without it,
// a minimal valid placeholder built from WithTitle / WithAPIVersion /
// WithVersion is served, so every bundled UI still renders.
//
// The handler serves:
//
//	GET <prefix>/             -> the HTML docs UI
//	GET <prefix>/openapi.json -> the spec as JSON
//	GET <prefix>/openapi.yaml -> the spec as YAML
//
// To customise the docs UI, pass a UI option from one of the
// github.com/FumingPower3925/stdocs/ui/* sub-packages:
//
//	mux := http.NewServeMux()
//	mux.Handle("GET /docs/", stdocs.DocsHandler(
//	    stdocs.WithTitle("My API"),
//	    scalar.WithUI(),
//	))
//	mux.HandleFunc("GET /users", listUsers)
//
// If you want a spec generated from registered routes instead, use
// Tier 2: build the application on a *stdocs.Mux (see New).
//
// If WithDisabled(true) is passed, the returned handler responds with
// 404 on every request, equivalent to Mux.Docs(false).
func DocsHandler(opts ...Option) http.Handler {
	cfg := applyOptions(opts)
	if cfg.Disabled {
		return http.NotFoundHandler()
	}
	jsonBytes := cfg.StaticSpec
	if jsonBytes == nil {
		jsonBytes = placeholderSpec(cfg)
	}
	// Convert once at construction; the document is static.
	yamlBytes, yamlErr := yaml.FromJSON(jsonBytes)
	yamlFn := func() ([]byte, error) { return yamlBytes, yamlErr }
	jsonFn := func() ([]byte, error) { return jsonBytes, nil }
	core, err := newDocsCore(cfg, jsonFn, yamlFn)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return core
}

// placeholderSpec builds the minimal valid OpenAPI document served by
// DocsHandler when no WithSpec document was provided: the configured
// version, title, and API version, with an empty paths object. All
// bundled UIs render it (an empty "{}" is not a valid document and
// makes every rich UI show a load error).
func placeholderSpec(cfg *Config) []byte {
	doc := map[string]any{
		"openapi": string(cfg.Version),
		"info": map[string]any{
			"title":   cfg.Info.Title,
			"version": cfg.Info.Version,
		},
		"paths": map[string]any{},
	}
	b, err := marshalSpec(doc)
	if err != nil {
		// Unreachable: the map above always marshals.
		return []byte(`{"openapi":"3.0.4","info":{"title":"API","version":"0.0.0"},"paths":{}}`)
	}
	return b
}

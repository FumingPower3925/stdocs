// Package stdocs: docs handler for use with a plain *http.ServeMux.
//
// This file used to expose stdocs.Mount(mux, opts...) — a Tier-1
// entry point that implied the user's *http.ServeMux would be
// introspected to populate the spec. That was never implemented: the
// mux argument was stored but never read, the spec was permanently
// `{}`, and the referenced WithSpecFile option did not exist.
//
// In v0.1.1 we cut the feature rather than ship a signature that
// implies introspection that does not happen. Tier-1 was renamed
// to DocsHandler, the unused mux parameter removed, and the README
// updated accordingly. For route enumeration (the actual value of
// stdocs), use *stdocs.Mux.
package stdocs

import (
	"net/http"

	"github.com/FumingPower3925/stdocs/internal/spec/yaml"
)

// DocsHandler returns an http.Handler that serves the docs UI and a
// placeholder OpenAPI spec. The spec is empty: this handler does not
// introspect any mux. It exists so users who already have a hand-
// written OpenAPI spec (or who don't need a populated spec) can
// expose a docs UI at a configurable prefix.
//
// The handler serves:
//
//	GET <prefix>/             -> the HTML docs UI
//	GET <prefix>/openapi.json -> the spec as JSON (placeholder)
//	GET <prefix>/openapi.yaml -> the spec as YAML (placeholder)
//
// The spec is currently `{}`. If you have a spec, see Tier 2
// (*stdocs.Mux) which builds the spec from registered routes.
//
// To customise the docs UI, pass a UI option from one of the
// github.com/FumingPower3925/stdocs/ui/* sub-packages:
//
//	mux := http.NewServeMux()
//	mux.Handle("GET /docs/", stdocs.DocsHandler(
//	    stdocs.WithTitle("My API"),
//	    stdocs.WithDescription("Hand-written spec follows"),
//	))
//	mux.HandleFunc("GET /users", listUsers)
func DocsHandler(opts ...Option) http.Handler {
	cfg := applyOptions(opts)
	// Pre-compute the placeholder YAML once. The JSON is "{}"
	// trivially; we keep the bytes for symmetry with the
	// dynamic path.
	yml, err := yaml.FromJSON([]byte("{}"))
	if err != nil {
		// yaml.FromJSON can only fail on structurally invalid
		// input. "{}" is valid; we treat the error as a 500.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	jsonBytes := []byte("{}")
	core, err := newDocsCore(cfg, func() ([]byte, []byte, error) {
		return jsonBytes, yml, nil
	})
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return core
}

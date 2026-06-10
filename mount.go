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
	"strings"

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
	return &docsHandler{
		cfg: cfg,
		ui:  cfg.UIDoc,
	}
}

// docsHandler is the handler returned by DocsHandler. It serves the
// docs UI page and a placeholder spec at the configured prefix.
type docsHandler struct {
	cfg *Config
	ui  string
}

func (h *docsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, h.cfg.DocsPrefix)
	switch {
	case path == "" || path == "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := h.ui
		html = strings.ReplaceAll(html, "{{.Title}}", h.cfg.Info.Title)
		html = strings.ReplaceAll(html, "{{.SpecURL}}", h.cfg.DocsPrefix+"/openapi.json")
		w.Write([]byte(html))
	case path == "/openapi.json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		// Placeholder: DocsHandler does not produce a spec. Users
		// who want a populated spec should use *stdocs.Mux.
		w.Write([]byte("{}"))
	case path == "/openapi.yaml":
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		// We round-trip the JSON placeholder through the YAML
		// converter so the response is syntactically valid YAML
		// (rather than emitting "{}" as raw YAML, which is missing
		// a top-level document marker and confuses some tools).
		y, err := yaml.FromJSON([]byte("{}"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(y)
	default:
		http.NotFound(w, r)
	}
}

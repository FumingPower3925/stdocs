package stdocs

import (
	"net/http"
	"strings"

	"github.com/FumingPower3925/stdocs/internal/spec/yaml"
)

// Mount returns an http.Handler that serves the docs UI and the OpenAPI
// spec for a plain *http.ServeMux whose routes are not directly
// observed by stdocs. It is the "Tier 1" entry point.
//
// Mount's spec is empty by default: the wrapped *http.ServeMux's routes
// are not introspectable. The returned handler can be combined with
// stdocs.Scan to produce a spec from observed traffic (future work) or
// used as a UI-only mount while routes are documented by hand via
// stdocs.ScanRefs (future work).
//
// For now, Mount is most useful as a UI shell that points at a static
// openapi.json the user provides via WithSpecFile. Tier 1 is best for
// apps that have a hand-written OpenAPI spec they want to expose at
// /docs; full route enumeration requires the *stdocs.Mux (Tier 2).
//
// opts are stdocs.Options for the docs UI (title, version, etc.).
func Mount(mux *http.ServeMux, opts ...Option) http.Handler {
	cfg := applyOptions(opts)
	sm := &stdMux{
		parent: mux,
		cfg:    cfg,
		ui:     defaultUIDoc,
	}
	return sm
}

// stdMux is the docs handler returned by Mount. It is a small wrapper
// that serves the docs UI and the (empty) spec.
type stdMux struct {
	parent *http.ServeMux
	cfg    *Config
	ui     string
	// specJSON is a static spec; if non-nil, it is served at the
	// openapi.json endpoint. Mount does not generate a spec from the
	// parent mux (which is not introspectable); the user is expected
	// to either set this via a future stdocs.WithSpecFile option or
	// use Tier 2 for full route enumeration.
	specJSON []byte
}

func (s *stdMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, s.cfg.DocsPrefix)
	switch {
	case path == "" || path == "/":
		// Serve the docs UI.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := s.ui
		html = strings.ReplaceAll(html, "{{.Title}}", s.cfg.Info.Title)
		html = strings.ReplaceAll(html, "{{.SpecURL}}", s.cfg.DocsPrefix+"/openapi.json")
		w.Write([]byte(html))
	case path == "/openapi.json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if s.specJSON == nil {
			w.Write([]byte("{}"))
			return
		}
		w.Write(s.specJSON)
	case path == "/openapi.yaml":
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		if s.specJSON == nil {
			w.Write([]byte("{}\n"))
			return
		}
		y, err := yaml.FromJSON(s.specJSON)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(y)
	default:
		http.NotFound(w, r)
	}
}

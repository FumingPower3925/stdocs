package stdocs

import (
	"html/template"
	"net/http"
	"strings"
)

// docsCore is the shared HTTP handler logic for the docs UI
// returned by Mux.Docs and DocsHandler. It is parameterised on the
// spec source: Tier-1 (DocsHandler) returns a static placeholder,
// Tier-2 (Mux.Docs) returns the dynamically-built spec. Both share
// the same HTML wrapping, prefix stripping, and routing logic.
//
// The HTML is parsed once at construction time using html/template
// to escape the title and spec URL; raw string substitution is not
// safe against a config value that contains a literal `{{`.
type docsCore struct {
	cfg    *Config
	ui     *template.Template
	specFn func() (json []byte, yaml []byte, err error)
}

type docsHTML struct {
	Title   string
	SpecURL string
}

func newDocsCore(cfg *Config, specFn func() (json []byte, yaml []byte, err error)) (*docsCore, error) {
	// Parse the template at construction time so a malformed UI
	// constant is reported eagerly (panic-or-warn at first request).
	// A failure to parse is a programming error: the UI constants
	// are package-level.
	t, err := template.New("ui").Parse(cfg.UIDoc)
	if err != nil {
		return nil, err
	}
	return &docsCore{cfg: cfg, ui: t, specFn: specFn}, nil
}

func (d *docsCore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, d.cfg.DocsPrefix)
	switch path {
	case "", "/":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var b strings.Builder
		// The spec URL is relative: the browser is already at
		// <prefix>/, so "openapi.json" resolves to
		// <prefix>/openapi.json. This works under any reverse
		// proxy path prefix without further configuration.
		err := d.ui.Execute(&b, docsHTML{
			Title:   d.cfg.Info.Title,
			SpecURL: "openapi.json",
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(b.String()))
	case "/openapi.json":
		b, _, err := d.specFn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(b)
	case "/openapi.yaml":
		_, b, err := d.specFn()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(b)
	default:
		http.NotFound(w, r)
	}
}

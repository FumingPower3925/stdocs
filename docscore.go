package stdocs

import (
	"bytes"
	"html/template"
	"net/http"
	"path"
	"strings"
)

// docsCore is the shared HTTP handler logic for the docs UI
// returned by Mux.Docs and DocsHandler. It is parameterised on the
// spec source: Tier-1 (DocsHandler) serves a static document,
// Tier-2 (Mux.Docs) serves the dynamically-built spec. Both share
// the same HTML page, prefix stripping, and routing logic.
//
// The HTML is parsed with html/template and rendered ONCE at
// construction time: both template inputs (the title and the
// relative spec URL) are fixed when the handler is built, and
// html/template escapes them per context — raw string substitution
// would not be safe against a title containing markup.
type docsCore struct {
	cfg    *Config
	page   []byte
	jsonFn func() ([]byte, error)
	yamlFn func() ([]byte, error)
}

type docsHTML struct {
	Title   string
	SpecURL string
}

func newDocsCore(cfg *Config, jsonFn, yamlFn func() ([]byte, error)) (*docsCore, error) {
	// Parse and execute the template at construction time so a
	// malformed UI constant is reported eagerly (the callers turn the
	// error into a handler that responds 500 to every request).
	t, err := template.New("ui").Parse(cfg.UIDoc)
	if err != nil {
		return nil, err
	}
	var page bytes.Buffer
	// The spec URL is relative: the browser is already at
	// <prefix>/, so "openapi.json" resolves to
	// <prefix>/openapi.json. This works under any reverse
	// proxy path prefix without further configuration.
	if err := t.Execute(&page, docsHTML{
		Title:   cfg.Info.Title,
		SpecURL: "openapi.json",
	}); err != nil {
		return nil, err
	}
	return &docsCore{cfg: cfg, page: page.Bytes(), jsonFn: jsonFn, yamlFn: yamlFn}, nil
}

func (d *docsCore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if d.cfg.DocsSecurityHeaders {
		setDocsBaselineHeaders(w.Header())
	}
	rest := strings.TrimPrefix(r.URL.Path, d.cfg.DocsPrefix)
	switch rest {
	case "":
		// Request for the bare prefix ("/docs", no trailing slash).
		// Redirect to the canonical slash-terminated form so the
		// page's relative spec and asset URLs resolve inside the
		// prefix. (Mount-registered handlers never see this case —
		// ServeMux issues the redirect itself — but manual mounts at
		// exact patterns do.) The target is relative and derived from
		// the config, never from request data: from ".../docs" the
		// browser resolves "docs/" to ".../docs/", which also works
		// behind path-rewriting proxies.
		http.Redirect(w, r, path.Base(d.cfg.DocsPrefix)+"/", http.StatusMovedPermanently)
	case "/":
		if d.cfg.DocsSecurityHeaders && d.cfg.UICSP != "" {
			w.Header().Set("Content-Security-Policy", d.cfg.UICSP)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(d.page)
	case "/openapi.json":
		d.serveSpec(w, d.jsonFn, "application/json; charset=utf-8")
	case "/openapi.yaml":
		d.serveSpec(w, d.yamlFn, "application/yaml")
	default:
		http.NotFound(w, r)
	}
}

func (d *docsCore) serveSpec(w http.ResponseWriter, fn func() ([]byte, error), contentType string) {
	b, err := fn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(b)
}

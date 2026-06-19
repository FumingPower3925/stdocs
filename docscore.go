package stdocs

import (
	"bytes"
	"encoding/json"
	"html"
	"html/template"
	"net/http"
	"path"
	"sort"
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
	// ConfigJSON is the marshaled UIConfig for use as the body of a
	// <script type="application/json"> data block (Swagger UI, Redoc).
	// Empty when no config was supplied.
	ConfigJSON template.JS
	// ConfigAttr is the marshaled UIConfig (raw JSON) for use as the
	// value of an HTML attribute (Scalar data-configuration). It is a
	// plain string so html/template applies attribute escaping; the
	// browser decodes it back to JSON when reading the attribute. Empty
	// when no config was supplied.
	ConfigAttr string
	// ConfigAttrs is the UIConfig rendered as space-separated element
	// attributes (Stoplight). Empty when no config was supplied.
	ConfigAttrs template.HTMLAttr
}

func newDocsCore(cfg *Config, jsonFn, yamlFn func() ([]byte, error)) (*docsCore, error) {
	// Parse and execute the template at construction time so a
	// malformed UI constant is reported eagerly (the callers turn the
	// error into a handler that responds 500 to every request).
	t, err := template.New("ui").Parse(cfg.UIDoc)
	if err != nil {
		return nil, err
	}
	// The spec URL is relative: the browser is already at
	// <prefix>/, so "openapi.json" resolves to
	// <prefix>/openapi.json. This works under any reverse
	// proxy path prefix without further configuration.
	data := docsHTML{Title: cfg.Info.Title, SpecURL: "openapi.json"}
	// UI-native configuration (from a sub-package's WithConfiguration)
	// is marshaled once here and exposed to the template in the carrier
	// each UI understands. Marshaling failure is reported eagerly, like
	// a malformed template. When no config is supplied the fields stay
	// empty and the templates render byte-identically to the no-config
	// page.
	if len(cfg.UIConfig) > 0 {
		b, err := json.Marshal(cfg.UIConfig)
		if err != nil {
			return nil, err
		}
		//nolint:gosec // G203: b is encoding/json output (HTML-escaped, so no </script> breakout) emitted into a non-executable <script type="application/json"> block under a CSP with no script unsafe-inline; the parity test guards it.
		data.ConfigJSON = template.JS(b)
		data.ConfigAttr = string(b)
		data.ConfigAttrs = uiConfigElementAttrs(cfg.UIConfig)
	}
	var page bytes.Buffer
	if err := t.Execute(&page, data); err != nil {
		return nil, err
	}
	return &docsCore{cfg: cfg, page: page.Bytes(), jsonFn: jsonFn, yamlFn: yamlFn}, nil
}

// uiConfigElementAttrs renders a UIConfig map as space-separated HTML
// element attributes, for UIs configured through attributes rather than
// a JSON object (Stoplight). Keys are used verbatim as attribute names
// (skipped when not a valid attribute name); string values are emitted
// as-is and any other value as its JSON encoding, each escaped for a
// double-quoted attribute. Keys are sorted so the output is
// deterministic.
func uiConfigElementAttrs(m map[string]any) template.HTMLAttr {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if !validAttrName(k) {
			continue
		}
		val, ok := m[k].(string)
		if !ok {
			j, err := json.Marshal(m[k])
			if err != nil {
				continue
			}
			val = string(j)
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(html.EscapeString(val))
		b.WriteString(`"`)
	}
	//nolint:gosec // G203: attribute names are validated by validAttrName and values are html.EscapeString-escaped above, so the string cannot break out of the tag.
	return template.HTMLAttr(b.String())
}

// validAttrName reports whether s is safe to emit as an HTML attribute
// name (letters, digits, hyphen, underscore) — a guard against a config
// key breaking out of the tag.
func validAttrName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
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
		d.serveSpec(w, d.jsonFn, "application/json; charset=utf-8", "openapi.json")
	case "/openapi.yaml":
		d.serveSpec(w, d.yamlFn, "application/yaml", "openapi.yaml")
	default:
		http.NotFound(w, r)
	}
}

func (d *docsCore) serveSpec(w http.ResponseWriter, fn func() ([]byte, error), contentType, filename string) {
	b, err := fn()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	// Suggest a filename for "Save as" without forcing a download:
	// inline keeps the spec viewable in a browser tab, which is the
	// common case for openapi.json/.yaml.
	w.Header().Set("Content-Disposition", `inline; filename="`+filename+`"`)
	_, _ = w.Write(b)
}

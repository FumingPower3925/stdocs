package stdocs

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// inlineBlocks returns the text content of every executable inline
// <script> (no src, executable type) or every <style> element in html,
// exactly as a browser hashes it for a CSP sha256-... source.
func inlineBlocks(html []byte, tag string) [][]byte {
	var out [][]byte
	lower := bytes.ToLower(html)
	open := []byte("<" + tag)
	closeT := []byte("</" + tag + ">")
	for i := 0; ; {
		s := bytes.Index(lower[i:], open)
		if s < 0 {
			break
		}
		s += i
		gt := bytes.IndexByte(html[s:], '>')
		if gt < 0 {
			break
		}
		openTag := bytes.ToLower(html[s : s+gt+1])
		cs := s + gt + 1
		e := bytes.Index(lower[cs:], closeT)
		if e < 0 {
			break
		}
		content := html[cs : cs+e]
		i = cs + e + len(closeT)
		if tag == "script" {
			if bytes.Contains(openTag, []byte("src=")) {
				continue
			}
			if bytes.Contains(openTag, []byte("type=")) &&
				!bytes.Contains(openTag, []byte("text/javascript")) &&
				!bytes.Contains(openTag, []byte("module")) {
				continue // data block (e.g. application/json) — not executed
			}
		}
		if len(content) > 0 {
			out = append(out, content)
		}
	}
	return out
}

func sriHash(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256-" + base64.StdEncoding.EncodeToString(sum[:])
}

func fetchDocs(t *testing.T, path string, opts ...Option) *httptest.ResponseRecorder {
	t.Helper()
	mux := New(append([]Option{WithTitle("Sec")}, opts...)...)
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	mux.Mount()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestDocsSecurityHeadersDefault(t *testing.T) {
	resp := fetchDocs(t, "/docs/")
	want := map[string]string{
		"Content-Security-Policy": defaultDocsCSP,
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "SAMEORIGIN",
		"Permissions-Policy":      docsPermissionsPolicy,
	}
	for h, v := range want {
		if got := resp.Header().Get(h); got != v {
			t.Errorf("%s = %q, want %q", h, got, v)
		}
	}
}

func TestDocsSpecEndpointHeaders(t *testing.T) {
	resp := fetchDocs(t, "/docs/openapi.json")
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("spec X-Content-Type-Options = %q, want nosniff", got)
	}
	// CSP is a document policy; it does nothing for a JSON body and is
	// not sent there.
	if got := resp.Header().Get("Content-Security-Policy"); got != "" {
		t.Errorf("spec Content-Security-Policy = %q, want empty", got)
	}
}

func TestDocsSpecContentDisposition(t *testing.T) {
	for path, want := range map[string]string{
		"/docs/openapi.json": `inline; filename="openapi.json"`,
		"/docs/openapi.yaml": `inline; filename="openapi.yaml"`,
	} {
		resp := fetchDocs(t, path)
		if got := resp.Header().Get("Content-Disposition"); got != want {
			t.Errorf("%s Content-Disposition = %q, want %q", path, got, want)
		}
	}
}

// TestDefaultUIIgnoresUIConfig pins the contract that the built-in docs
// page ignores Config.UIConfig: the field is exported and a caller could
// set it directly (not via a UI sub-package), but the default template
// references none of the config carriers, so a value set there must not
// reach the page, and the strict default CSP must be unchanged.
func TestDefaultUIIgnoresUIConfig(t *testing.T) {
	evil := "</script><script>alert(1)</script>"
	resp := fetchDocs(t, "/docs/", func(c *Config) {
		c.UIConfig = map[string]any{"evilkey": evil}
	})
	body := resp.Body.String()
	if strings.Contains(body, "evilkey") || strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("built-in page leaked Config.UIConfig into the rendered page")
	}
	if got := resp.Header().Get("Content-Security-Policy"); got != defaultDocsCSP {
		t.Errorf("built-in page CSP changed with UIConfig set: %q", got)
	}
}

func TestDocsSecurityHeadersOff(t *testing.T) {
	resp := fetchDocs(t, "/docs/", WithDocsSecurityHeaders(false))
	for _, h := range []string{
		"Content-Security-Policy", "X-Content-Type-Options",
		"Referrer-Policy", "X-Frame-Options", "Permissions-Policy",
	} {
		if got := resp.Header().Get(h); got != "" {
			t.Errorf("with headers off, %s = %q, want empty", h, got)
		}
	}
}

func TestWithCSPOverride(t *testing.T) {
	const custom = "default-src 'self'"
	resp := fetchDocs(t, "/docs/", WithCSP(custom))
	if got := resp.Header().Get("Content-Security-Policy"); got != custom {
		t.Errorf("CSP = %q, want %q", got, custom)
	}
}

// TestDefaultUINotice checks the built-in page ships the dismissable
// notice that points users at the richer UIs.
func TestDefaultUINotice(t *testing.T) {
	body := fetchDocs(t, "/docs/").Body.String()
	for _, want := range []string{
		`id="stdocs-note" hidden`,      // starts hidden; JS reveals it when not dismissed
		`id="stdocs-note-x"`,           // the dismiss button
		"stdocs-docs-notice-dismissed", // the localStorage key
		"minimal built-in docs UI",
		"WithUI()",
		"Scalar",
		`href="https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Docs_UIs"`,
		`id="lock-tmpl"`,    // the auth padlock template (no emoji)
		`<svg class="lock"`, // a real inline SVG, cloned per secured op
		`id="chev-tmpl"`,    // the accordion chevron template
		"op clickable",      // operations are expandable
		"buildDetail",       // the expand panel builder
		`href="openapi.json"`,
		`href="openapi.yaml"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("default docs page missing %q", want)
		}
	}
}

// TestDefaultDocsCSP recomputes the inline script/style hashes from the
// actually-served built-in page and asserts each appears in the default
// CSP, so the policy cannot silently drift from the HTML it secures.
func TestDefaultDocsCSP(t *testing.T) {
	rec := httptest.NewRecorder()
	mux := New(WithTitle("Sec"))
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	mux.Mount()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/", nil))
	html := rec.Body.Bytes()

	scripts := inlineBlocks(html, "script")
	styles := inlineBlocks(html, "style")
	if len(scripts) != 1 || len(styles) != 1 {
		t.Fatalf("built-in page: got %d inline scripts and %d styles, want 1 and 1", len(scripts), len(styles))
	}
	for _, s := range scripts {
		if h := sriHash(s); !strings.Contains(defaultDocsCSP, h) {
			t.Errorf("inline script hash %s missing from defaultDocsCSP", h)
		}
	}
	for _, s := range styles {
		if h := sriHash(s); !strings.Contains(defaultDocsCSP, h) {
			t.Errorf("inline style hash %s missing from defaultDocsCSP", h)
		}
	}
	if strings.Contains(defaultDocsCSP, "unsafe-inline") {
		t.Error("defaultDocsCSP must not use 'unsafe-inline'")
	}
}

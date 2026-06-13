package stdocs_test

// Parity guard for the per-UI Content-Security-Policy. For every bundled
// UI it renders the docs page, pulls out the executable inline scripts,
// and checks each one's sha256 is pinned in the served script-src — so a
// change to a UI's inline init script that is not reflected in its CSP
// fails the build instead of silently breaking the page in a browser.
// It also asserts no UI allows 'unsafe-inline' for scripts.

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/redoc"
	"github.com/FumingPower3925/stdocs/ui/redocemb"
	"github.com/FumingPower3925/stdocs/ui/scalar"
	"github.com/FumingPower3925/stdocs/ui/scalaremb"
	"github.com/FumingPower3925/stdocs/ui/stoplight"
	"github.com/FumingPower3925/stdocs/ui/stoplightemb"
	"github.com/FumingPower3925/stdocs/ui/swaggerui"
	"github.com/FumingPower3925/stdocs/ui/swaggeruiemb"
)

func execInlineScripts(html []byte) [][]byte {
	var out [][]byte
	lower := bytes.ToLower(html)
	for i := 0; ; {
		s := bytes.Index(lower[i:], []byte("<script"))
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
		e := bytes.Index(lower[cs:], []byte("</script>"))
		if e < 0 {
			break
		}
		content := html[cs : cs+e]
		i = cs + e + len("</script>")
		if bytes.Contains(openTag, []byte("src=")) {
			continue
		}
		if bytes.Contains(openTag, []byte("type=")) &&
			!bytes.Contains(openTag, []byte("text/javascript")) &&
			!bytes.Contains(openTag, []byte("module")) {
			continue
		}
		if len(content) > 0 {
			out = append(out, content)
		}
	}
	return out
}

func directive(csp, name string) (string, bool) {
	for _, d := range strings.Split(csp, ";") {
		d = strings.TrimSpace(d)
		if d == name || strings.HasPrefix(d, name+" ") {
			return strings.TrimSpace(strings.TrimPrefix(d, name)), true
		}
	}
	return "", false
}

func TestPerUICSPParity(t *testing.T) {
	uis := []struct {
		name string
		opt  stdocs.Option
	}{
		{"scalar", scalar.WithUI()},
		{"swaggerui", swaggerui.WithUI()},
		{"redoc", redoc.WithUI()},
		{"stoplight", stoplight.WithUI()},
		{"scalaremb", scalaremb.WithUI()},
		{"swaggeruiemb", swaggeruiemb.WithUI()},
		{"redocemb", redocemb.WithUI()},
		{"stoplightemb", stoplightemb.WithUI()},
	}
	for _, ui := range uis {
		t.Run(ui.name, func(t *testing.T) {
			mux := stdocs.New(stdocs.WithTitle("X"), ui.opt)
			mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
			mux.Mount()
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/docs/", nil))

			csp := rec.Header().Get("Content-Security-Policy")
			if csp == "" {
				t.Fatal("no Content-Security-Policy header")
			}
			scriptSrc, ok := directive(csp, "script-src")
			if !ok {
				t.Fatal("CSP has no script-src directive")
			}
			if strings.Contains(scriptSrc, "'unsafe-inline'") {
				t.Errorf("script-src allows 'unsafe-inline': %q", scriptSrc)
			}
			if _, ok := directive(csp, "default-src"); !ok {
				t.Error("CSP has no default-src directive")
			}
			if fa, _ := directive(csp, "frame-ancestors"); fa != "'self'" {
				t.Errorf("frame-ancestors = %q, want 'self'", fa)
			}

			for _, script := range execInlineScripts(rec.Body.Bytes()) {
				sum := sha256.Sum256(script)
				h := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
				if !strings.Contains(scriptSrc, h) {
					t.Errorf("inline script hash %s not pinned in script-src %q", h, scriptSrc)
				}
			}
		})
	}
}

package stoplightemb_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/stoplightemb"
)

func TestWithUI_ReplacesUIDoc(t *testing.T) {
	cfg := &stdocs.Config{}
	stoplightemb.WithUI()(cfg)
	if !strings.Contains(cfg.UIDoc, "web-components.min.js") {
		t.Errorf("WithUI should set UIDoc to the stoplight HTML, got: %.50s", cfg.UIDoc)
	}
}

func TestWithUI_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Stoplight Demo"), stoplightemb.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	mux.Docs().ServeHTTP(rr, req)
	body := rr.Body.String()
	// The asset URLs must be relative so the page works under
	// WithDocsPrefix and reverse proxies.
	if !strings.Contains(body, `src="_assets/web-components.min.js"`) {
		t.Errorf("body should reference the embedded script relatively: %s", body)
	}
	if !strings.Contains(body, `href="_assets/styles.min.css"`) {
		t.Errorf("body should reference the embedded stylesheet relatively: %s", body)
	}
	if strings.Contains(body, "/docs/_assets/") {
		t.Errorf("body must not hardcode the /docs prefix: %s", body)
	}
	if !strings.Contains(body, "Stoplight Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

func TestAssetHandler_ServesBundle(t *testing.T) {
	handler := stoplightemb.AssetHandler()
	for _, name := range []string{"/web-components.min.js", "/styles.min.css"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, name, nil)
		handler.ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("GET %s = %d, want 200 (the bundle ships vendored in assets/)", name, rr.Code)
		}
		if got := rr.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Errorf("GET %s Cache-Control = %q, want immutable caching", name, got)
		}
	}
}

func TestAssetHandler_DirectoryIs404(t *testing.T) {
	handler := stoplightemb.AssetHandler()
	for _, p := range []string{"/", "/subdir/"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, p, nil)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (no directory listings)", p, rr.Code)
		}
	}
}

func TestAssetHandler_MissingFileIs404(t *testing.T) {
	handler := stoplightemb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /nope.js = %d, want 404", rr.Code)
	}
}

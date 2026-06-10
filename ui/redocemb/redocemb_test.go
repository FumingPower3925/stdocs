package redocemb_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/redocemb"
)

func TestWithUI_ReplacesUIDoc(t *testing.T) {
	cfg := &stdocs.Config{}
	redocemb.WithUI()(cfg)
	if !strings.Contains(cfg.UIDoc, "redoc.standalone.js") {
		t.Errorf("WithUI should set UIDoc to the redoc HTML, got: %.50s", cfg.UIDoc)
	}
}

func TestWithUI_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Redoc Demo"), redocemb.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	mux.Docs().ServeHTTP(rr, req)
	body := rr.Body.String()
	// The asset URL must be relative so the page works under
	// WithDocsPrefix and reverse proxies.
	if !strings.Contains(body, `src="_assets/redoc.standalone.js"`) {
		t.Errorf("body should reference the embedded asset relatively: %s", body)
	}
	if strings.Contains(body, "/docs/_assets/") {
		t.Errorf("body must not hardcode the /docs prefix: %s", body)
	}
	if !strings.Contains(body, "Redoc Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

func TestAssetHandler_ServesBundle(t *testing.T) {
	handler := redocemb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/redoc.standalone.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET /redoc.standalone.js = %d, want 200 (the bundle ships vendored in assets/)", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want immutable caching", got)
	}
}

func TestAssetHandler_DirectoryIs404(t *testing.T) {
	handler := redocemb.AssetHandler()
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
	handler := redocemb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /nope.js = %d, want 404", rr.Code)
	}
}

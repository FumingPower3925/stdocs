package scalaremb_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/scalaremb"
)

func TestWithUI_ReplacesUIDoc(t *testing.T) {
	cfg := &stdocs.Config{}
	scalaremb.WithUI()(cfg)
	if cfg.UIDoc == "" {
		t.Fatal("UIDoc not set")
	}
	// Should reference the embedded asset path.
	if !strings.Contains(cfg.UIDoc, "/docs/_assets/standalone.js") {
		t.Errorf("HTML should reference embedded asset: %s", cfg.UIDoc)
	}
	// And should NOT be the CDN URL.
	if strings.Contains(cfg.UIDoc, "cdn.jsdelivr.net") {
		t.Errorf("embedded HTML should not reference CDN: %s", cfg.UIDoc)
	}
}

func TestWithUI_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Scalar Embed Demo"), scalaremb.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	mux.Docs().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "/docs/_assets/standalone.js") {
		t.Errorf("body should reference embedded asset: %s", body)
	}
	if !strings.Contains(body, "Scalar Embed Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

func TestAssetHandler(t *testing.T) {
	// The handler should serve the embedded files. The placeholder
	// file is there; the real one is fetched on `go generate`.
	handler := scalaremb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/.placeholder.txt", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

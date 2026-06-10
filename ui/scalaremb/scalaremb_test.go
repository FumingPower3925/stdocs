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
	// Should reference the embedded asset with a RELATIVE URL so the
	// page works under WithDocsPrefix and reverse proxies.
	if !strings.Contains(cfg.UIDoc, `src="_assets/standalone.js"`) {
		t.Errorf("HTML should reference the embedded asset relatively: %s", cfg.UIDoc)
	}
	if strings.Contains(cfg.UIDoc, "/docs/_assets/") {
		t.Errorf("HTML must not hardcode the /docs prefix: %s", cfg.UIDoc)
	}
	// And should NOT be the CDN URL.
	if strings.Contains(cfg.UIDoc, "cdn.jsdelivr.net") {
		t.Errorf("embedded HTML should not reference CDN: %s", cfg.UIDoc)
	}
	// Scalar must receive the spec URL via the data-url attribute,
	// not as <script> element content. Otherwise the bundle treats
	// the URL as the document and fails with "Invalid YAML object".
	if !strings.Contains(cfg.UIDoc, `data-url="{{.SpecURL}}"`) {
		t.Errorf("HTML must use data-url attribute: %s", cfg.UIDoc)
	}
	if strings.Contains(cfg.UIDoc, `type="application/json"`) {
		t.Errorf("HTML must not embed the URL as <script> element content: %s", cfg.UIDoc)
	}
}

func TestWithUI_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Scalar Embed Demo"), scalaremb.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	mux.Docs().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, `src="_assets/standalone.js"`) {
		t.Errorf("body should reference embedded asset: %s", body)
	}
	if !strings.Contains(body, "Scalar Embed Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

func TestAssetHandler_ServesBundle(t *testing.T) {
	handler := scalaremb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/standalone.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET /standalone.js = %d, want 200 (the bundle ships vendored in assets/)", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, want immutable caching", got)
	}
}

func TestAssetHandler_DirectoryIs404(t *testing.T) {
	handler := scalaremb.AssetHandler()
	for _, p := range []string{"/", "/subdir/"} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", p, nil)
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 (no directory listings)", p, rr.Code)
		}
	}
}

func TestAssetHandler_MissingFileIs404(t *testing.T) {
	handler := scalaremb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/nope.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET /nope.js = %d, want 404", rr.Code)
	}
}

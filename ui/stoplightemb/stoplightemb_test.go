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
	if !strings.Contains(body, "/docs/_assets/web-components.min.js") {
		t.Errorf("body should reference the embedded asset: %s", body)
	}
	if !strings.Contains(body, "Stoplight Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

func TestAssetHandler(t *testing.T) {
	handler := stoplightemb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/web-components.min.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Skipf("web-components.min.js not embedded (status %d); re-vendor the bundle", rr.Code)
	}
	if rr.Body.Len() < 1000 {
		t.Errorf("web-components.min.js unexpectedly small: %d bytes", rr.Body.Len())
	}
}

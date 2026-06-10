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
	if !strings.Contains(body, "/docs/_assets/redoc.standalone.js") {
		t.Errorf("body should reference the embedded asset: %s", body)
	}
	if !strings.Contains(body, "Redoc Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

func TestAssetHandler(t *testing.T) {
	handler := redocemb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/redoc.standalone.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Skipf("redoc.standalone.js not embedded (status %d); re-vendor the bundle", rr.Code)
	}
	if rr.Body.Len() < 1000 {
		t.Errorf("redoc.standalone.js unexpectedly small: %d bytes", rr.Body.Len())
	}
}

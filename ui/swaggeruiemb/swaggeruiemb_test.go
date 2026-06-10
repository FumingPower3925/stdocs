package swaggeruiemb_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/swaggeruiemb"
)

func TestWithUI_ReplacesUIDoc(t *testing.T) {
	cfg := &stdocs.Config{}
	swaggeruiemb.WithUI()(cfg)
	if !strings.Contains(cfg.UIDoc, "swagger-ui-bundle.js") {
		t.Errorf("WithUI should set UIDoc to the swagger UI HTML, got: %.50s", cfg.UIDoc)
	}
}

func TestWithUI_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Swagger Demo"), swaggeruiemb.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/docs/", nil)
	mux.Docs().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "/docs/_assets/swagger-ui-bundle.js") {
		t.Errorf("body should reference the embedded asset: %s", body)
	}
	if !strings.Contains(body, "Swagger Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

func TestAssetHandler(t *testing.T) {
	handler := swaggeruiemb.AssetHandler()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/swagger-ui-bundle.js", nil)
	handler.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Skipf("swagger-ui-bundle.js not embedded (status %d); re-vendor the bundle", rr.Code)
	}
	if rr.Body.Len() < 1000 {
		t.Errorf("swagger-ui-bundle.js unexpectedly small: %d bytes", rr.Body.Len())
	}
}

package swaggerui_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/swaggerui"
)

func TestSwaggerUI_Option(t *testing.T) {
	cfg := &stdocs.Config{}
	swaggerui.WithUI()(cfg)
	if !strings.Contains(cfg.UIDoc, "swagger-ui") {
		t.Errorf("expected swagger-ui reference, got %s", cfg.UIDoc)
	}
}

func TestSwaggerUI_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Swagger Demo"), swaggerui.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	mux.Docs().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "cdn.jsdelivr.net") {
		t.Errorf("body should reference CDN: %s", body)
	}
	if !strings.Contains(body, "Swagger Demo") {
		t.Errorf("body should contain title: %s", body)
	}
	if !strings.Contains(body, "openapi.json") {
		t.Errorf("body should contain spec URL: %s", body)
	}
}

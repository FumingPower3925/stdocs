package stoplight_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/stoplight"
)

func TestStoplight_Option(t *testing.T) {
	cfg := &stdocs.Config{}
	stoplight.WithUI()(cfg)
	if !strings.Contains(cfg.UIDoc, "stoplight") {
		t.Errorf("expected stoplight reference, got %s", cfg.UIDoc)
	}
}

func TestStoplight_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Stoplight Demo"), stoplight.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	mux.Docs().ServeHTTP(rr, req)
	body := rr.Body.String()
	if !strings.Contains(body, "cdn.jsdelivr.net") {
		t.Errorf("body should reference CDN: %s", body)
	}
	if !strings.Contains(body, "Stoplight Demo") {
		t.Errorf("body should contain title: %s", body)
	}
}

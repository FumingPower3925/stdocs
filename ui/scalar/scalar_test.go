package scalar_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/ui/scalar"
)

func TestScalar_ProvidesOption(t *testing.T) {
	opt := scalar.WithUI()
	if opt == nil {
		t.Fatal("WithUI() returned nil")
	}
}

func TestScalar_ReplacesUIDoc(t *testing.T) {
	cfg := &stdocs.Config{}
	scalar.WithUI()(cfg)
	if cfg.UIDoc == "" {
		t.Fatal("UIDoc not set")
	}
	// The Scalar HTML should reference the CDN script.
	if !strings.Contains(cfg.UIDoc, "cdn.jsdelivr.net") {
		t.Errorf("Scalar HTML should reference CDN: %s", cfg.UIDoc)
	}
	// And it should NOT be the raw default UI.
	if strings.Contains(cfg.UIDoc, "<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>API docs</title>") {
		t.Errorf("Scalar HTML looks like the raw default UI")
	}
}

func TestScalar_EndToEnd(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("Scalar Demo"), scalar.WithUI())
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})
	// Mount the docs handler on a parent.
	parent := http.NewServeMux()
	parent.Handle("GET /docs/", mux.Docs())

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	parent.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, "cdn.jsdelivr.net") {
		t.Errorf("served body should contain Scalar CDN reference: %s", body)
	}
	if !strings.Contains(body, "Scalar Demo") {
		t.Errorf("served body should contain title: %s", body)
	}
}

func TestScalar_TitleSubstitution(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("My App"), scalar.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	docs := mux.Docs()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	docs.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "My App") {
		t.Errorf("body should contain title: %s", rr.Body.String())
	}
}

func TestScalar_SpecURLSubstitution(t *testing.T) {
	mux := stdocs.New(stdocs.WithTitle("T"), scalar.WithUI())
	mux.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	docs := mux.Docs()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	docs.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "/docs/openapi.json") {
		t.Errorf("body should contain spec URL: %s", rr.Body.String())
	}
}

func TestScalar_DefaultUIDocIsRaw(t *testing.T) {
	// Without the scalar import, the default UI is the raw one.
	mux := stdocs.New(stdocs.WithTitle("T"))
	if strings.Contains(mux.Config().UIDoc, "cdn.jsdelivr.net") {
		t.Errorf("default UI should not contain Scalar CDN reference")
	}
}

// TestScalar_UsesDataURL guards the regression where Scalar received
// the spec URL as <script> element content (which it interpreted as
// the spec document, not a URL) instead of in the data-url attribute
// (which it interprets as "fetch the spec from this URL"). The bug
// produced a blank page with "Invalid YAML object" in the console.
func TestScalar_UsesDataURL(t *testing.T) {
	cfg := &stdocs.Config{}
	scalar.WithUI()(cfg)
	// data-url must be present.
	if !strings.Contains(cfg.UIDoc, `data-url="{{.SpecURL}}"`) {
		t.Errorf("Scalar HTML must use data-url attribute; got: %s", cfg.UIDoc)
	}
	// The wrong form (URL as <script> content) must NOT be present.
	if strings.Contains(cfg.UIDoc, `type="application/json"`) {
		t.Errorf("Scalar HTML must not embed the URL as <script> element content; got: %s", cfg.UIDoc)
	}
}

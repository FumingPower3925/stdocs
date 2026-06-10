package stdocs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsHandler_EscapesTitle(t *testing.T) {
	h := DocsHandler(WithTitle(`<script>alert(1)</script>`))
	rr := serve(h, "/docs/")
	if !strings.Contains(rr.Body.String(), "&lt;script&gt;") {
		t.Errorf("title should be HTML-escaped; body: %s", rr.Body.String())
	}
}

func TestDocsHandler_DefaultJSON(t *testing.T) {
	// Without WithSpec, DocsHandler serves a minimal VALID OpenAPI
	// document (an empty "{}" would make every rich UI error out).
	h := DocsHandler(WithTitle("Placeholder"))
	rr := serve(h, "/docs/openapi.json")
	body := rr.Body.String()
	for _, want := range []string{`"openapi":"3.0.4"`, `"title":"Placeholder"`, `"paths":{}`} {
		if !strings.Contains(body, want) {
			t.Errorf("default JSON missing %s; body: %s", want, body)
		}
	}
}

func TestDocsHandler_DefaultYAML(t *testing.T) {
	h := DocsHandler()
	rr := serve(h, "/docs/openapi.yaml")
	body := rr.Body.String()
	for _, want := range []string{`openapi: "3.0.4"`, "paths: {}"} {
		if !strings.Contains(body, want) {
			t.Errorf("default YAML missing %q; body: %s", want, body)
		}
	}
}

func TestDocsHandler_WithSpec(t *testing.T) {
	spec := []byte(`{"openapi":"3.1.2","info":{"title":"Static","version":"9.9.9"},"paths":{}}`)
	h := DocsHandler(WithSpec(spec))
	rr := serve(h, "/docs/openapi.json")
	if rr.Body.String() != string(spec) {
		t.Errorf("WithSpec JSON = %q, want the supplied document verbatim", rr.Body.String())
	}
	rr = serve(h, "/docs/openapi.yaml")
	if !strings.Contains(rr.Body.String(), `title: "Static"`) {
		t.Errorf("WithSpec YAML = %q, want converted document", rr.Body.String())
	}
}

func TestDocsHandler_Tier1NotFound(t *testing.T) {
	h := DocsHandler()
	rr := serve(h, "/docs/missing")
	if rr.Code != 404 {
		t.Errorf("not-found code = %d, want 404", rr.Code)
	}
}

func serve(h http.Handler, target string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, target, nil))
	return rr
}

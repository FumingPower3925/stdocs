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
	h := DocsHandler()
	rr := serve(h, "/docs/openapi.json")
	if rr.Body.String() != "{}" {
		t.Errorf("default JSON = %q, want %q", rr.Body.String(), "{}")
	}
}

func TestDocsHandler_DefaultYAML(t *testing.T) {
	h := DocsHandler()
	rr := serve(h, "/docs/openapi.yaml")
	if !strings.Contains(rr.Body.String(), "{}") {
		t.Errorf("default YAML = %q, want contains %q", rr.Body.String(), "{}")
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

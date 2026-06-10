package stdocs

import (
	"net/http"
	"testing"
)

func TestResponseDescription_Override(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, struct{}{}),
		ResponseDescription(200, "Custom OK description"),
	)
	b, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	doc := jx(t, b)
	got := doc["paths"].(map[string]any)["/x"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["description"]
	if got != "Custom OK description" {
		t.Errorf("description = %v, want %q", got, "Custom OK description")
	}
}

func TestResponseDescription_NoResponse(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		ResponseDescription(200, "nope"),
	)
	if _, err := m.JSON(); err != nil {
		t.Errorf("JSON() error: %v", err)
	}
}

func TestResponseHeader_AddsHeader(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, struct{}{}),
		ResponseHeader(200, "X-Trace-Id", "string", "Trace id"),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	hdr := doc["paths"].(map[string]any)["/x"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["headers"].(map[string]any)["X-Trace-Id"].(map[string]any)["schema"].(map[string]any)
	if hdr["type"] != "string" {
		t.Errorf("X-Trace-Id type = %v, want string", hdr["type"])
	}
	if hdr["description"] != "Trace id" {
		t.Errorf("X-Trace-Id description = %v", hdr["description"])
	}
}

func TestResponseHeader_NoResponse(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		ResponseHeader(200, "X-Trace-Id", "string", "Trace"),
	)
	if _, err := m.JSON(); err != nil {
		t.Errorf("JSON() error: %v", err)
	}
}

func TestBodyContentType_Override(t *testing.T) {
	m := New(WithTitle("T"))
	type Req struct {
		X string `json:"x"`
	}
	m.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {},
		WithBody(Req{}),
		BodyContentType("application/xml"),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	content := doc["paths"].(map[string]any)["/x"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)["content"].(map[string]any)
	if _, ok := content["application/xml"]; !ok {
		keys := make([]string, 0, len(content))
		for k := range content {
			keys = append(keys, k)
		}
		t.Errorf("expected application/xml content type, got keys %v", keys)
	}
	if _, ok := content["application/json"]; ok {
		t.Errorf("application/json should not be present when overridden")
	}
}

func TestBodyContentType_NoBody(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		BodyContentType("application/xml"),
	)
	if _, err := m.JSON(); err != nil {
		t.Errorf("JSON() error: %v", err)
	}
}

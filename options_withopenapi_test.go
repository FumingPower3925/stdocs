package stdocs

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestWithOpenAPI_RunsHook(t *testing.T) {
	called := false
	m := New(
		WithTitle("T"),
		WithOpenAPI(func(doc map[string]any) {
			called = true
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	_, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Errorf("hook not called")
	}
}

func TestWithOpenAPI_MutatesDoc(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithOpenAPI(func(doc map[string]any) {
			doc["x-custom"] = "hello"
			if comps, ok := doc["components"].(map[string]any); ok {
				comps["securitySchemes"] = map[string]any{
					"bearerAuth": map[string]any{
						"type":   "http",
						"scheme": "bearer",
					},
				}
			}
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	if doc["x-custom"] != "hello" {
		t.Errorf("x-custom = %v", doc["x-custom"])
	}
	sec := jget(t, doc, "components", "securitySchemes", "bearerAuth").(map[string]any)
	if sec["scheme"] != "bearer" {
		t.Errorf("scheme = %v", sec["scheme"])
	}
}

func TestWithOpenAPI_AddsSecurity(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithOpenAPI(func(doc map[string]any) {
			doc["security"] = []map[string]any{{"bearerAuth": []string{}}}
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	sec, ok := doc["security"].([]any)
	if !ok {
		t.Fatalf("security = %T, want []any", doc["security"])
	}
	if len(sec) != 1 {
		t.Errorf("security = %v", sec)
	}
}

func TestWithOpenAPI_AddsWebhooks(t *testing.T) {
	// Webhooks are 3.1 only; verify we can add them to the 3.1 doc.
	m := New(
		WithTitle("T"),
		WithVersion(OpenAPI31),
		WithOpenAPI(func(doc map[string]any) {
			doc["webhooks"] = map[string]any{
				"newUser": map[string]any{
					"post": map[string]any{
						"summary":     "New user created",
						"description": "Fired when a new user is created",
					},
				},
			}
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	wh := jget(t, doc, "webhooks", "newUser", "post").(map[string]any)
	if wh["summary"] != "New user created" {
		t.Errorf("summary = %v", wh["summary"])
	}
}

func TestWithOpenAPI_MultipleHooks(t *testing.T) {
	count := 0
	m := New(
		WithTitle("T"),
		WithOpenAPI(func(doc map[string]any) { count++ }),
		WithOpenAPI(func(doc map[string]any) { count++ }),
		WithOpenAPI(func(doc map[string]any) { count++ }),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	_, _ = m.JSON()
	if count != 3 {
		t.Errorf("hook count = %d, want 3", count)
	}
}

func TestWithOpenAPI_OnlyRunsOnBuild(t *testing.T) {
	count := 0
	m := New(
		WithTitle("T"),
		WithOpenAPI(func(doc map[string]any) { count++ }),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	for i := 0; i < 5; i++ {
		_, _ = m.JSON()
	}
	if count != 1 {
		t.Errorf("hook count = %d, want 1 (cache should prevent re-runs)", count)
	}
}

func TestWithOpenAPI_RunsAgainAfterRefresh(t *testing.T) {
	count := 0
	m := New(
		WithTitle("T"),
		WithOpenAPI(func(doc map[string]any) { count++ }),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	_, _ = m.JSON()
	m.Refresh()
	_, _ = m.JSON()
	if count != 2 {
		t.Errorf("hook count = %d, want 2", count)
	}
}

func TestWithOpenAPI_RawJSON(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithOpenAPI(func(doc map[string]any) {
			doc["x-custom-object"] = map[string]any{
				"nested": "value",
			}
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	// Check that the JSON parses cleanly.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	xco := raw["x-custom-object"].(map[string]any)
	if xco["nested"] != "value" {
		t.Errorf("nested = %v", xco["nested"])
	}
}

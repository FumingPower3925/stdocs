package stdocs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMux_BasicConstruction(t *testing.T) {
	m := New(WithTitle("Test"))
	if m.ServeMux == nil {
		t.Fatal("embedded ServeMux is nil")
	}
	if m.cfg.Info.Title != "Test" {
		t.Errorf("Title = %q", m.cfg.Info.Title)
	}
}

func TestMux_HandleFunc_RegistersRoute(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	if len(m.reg.routes) != 1 {
		t.Errorf("routes = %d, want 1", len(m.reg.routes))
	}
}

func TestMux_HandleFunc_InvalidPatternPanics(t *testing.T) {
	m := New(WithTitle("T"))
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on invalid pattern")
		}
	}()
	m.HandleFunc("not a pattern", func(w http.ResponseWriter, r *http.Request) {})
}

func TestMux_Dispatch(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("world"))
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/hello", nil)
	m.ServeHTTP(rr, req)
	if rr.Body.String() != "world" {
		t.Errorf("body = %q, want world", rr.Body.String())
	}
}

func TestMux_JSON_Empty(t *testing.T) {
	m := New(WithTitle("Empty"))
	b, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"openapi"`) {
		t.Errorf("expected openapi field, got %s", b)
	}
}

func TestMux_JSON_Tier2(t *testing.T) {
	type User struct {
		ID   string `json:"id" doc:"unique id"`
		Name string `json:"name"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	},
		Summary("Get a user"),
		Tags("users"),
		WithResponse(200, User{}),
		WithResponse(404, nil),
	)
	b, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	doc := jx(t, b)
	// /users/{id} path
	op := jget(t, doc, "paths", "/users/{id}", "get").(map[string]any)
	if op["summary"] != "Get a user" {
		t.Errorf("summary = %v, want Get a user", op["summary"])
	}
	tags := op["tags"].([]any)
	if len(tags) != 1 || tags[0] != "users" {
		t.Errorf("tags = %v", tags)
	}
	// 200 should have a $ref to User.
	r200 := jget(t, doc, "paths", "/users/{id}", "get", "responses", "200").(map[string]any)
	ct := jget(t, r200, "content", "application/json").(map[string]any)
	sch := ct["schema"].(map[string]any)
	if sch["$ref"] != "#/components/schemas/User" {
		t.Errorf("$ref = %v, want #/components/schemas/User", sch["$ref"])
	}
	// User component should be in components.
	user := jget(t, doc, "components", "schemas", "User").(map[string]any)
	if user["type"] != "object" {
		t.Errorf("User.type = %v", user["type"])
	}
	// 404 should have no schema.
	r404 := jget(t, doc, "paths", "/users/{id}", "get", "responses", "404").(map[string]any)
	if _, ok := r404["content"]; ok {
		t.Errorf("404 should have no content, got %v", r404["content"])
	}
}

func TestMux_JSON_DefaultInferences(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /users", listUsers)
	b, _ := m.JSON()
	doc := jx(t, b)
	op := jget(t, doc, "paths", "/users", "get").(map[string]any)
	if op["summary"] != "List users" {
		t.Errorf("summary = %v, want List users", op["summary"])
	}
	tags := op["tags"].([]any)
	if len(tags) != 1 || tags[0] != "Users" {
		t.Errorf("tags = %v, want [Users]", tags)
	}
}

func listUsers(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("list"))
}

func TestMux_JSON_31(t *testing.T) {
	type T struct {
		Name *string `json:"name"`
	}
	m := New(WithTitle("T"), WithVersion(OpenAPI31))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, T{}),
	)
	b, _ := m.JSON()
	if !strings.Contains(string(b), `"3.1.2"`) {
		t.Errorf("expected 3.1.2 in output: %s", b)
	}
	// The nullable pointer field should be a type array, not nullable:true.
	if strings.Contains(string(b), `"nullable"`) {
		t.Errorf("3.1 should not contain nullable: %s", b)
	}
}

func TestMux_YAML(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	y, err := m.YAML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(y), "openapi:") {
		t.Errorf("YAML should contain `openapi:`, got %s", y)
	}
	if !strings.Contains(string(y), `title: "T"`) {
		t.Errorf("YAML should contain `title: \"T\"`, got %s", y)
	}
}

func TestMux_Refresh(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b1, _ := m.JSON()
	m.Refresh()
	b2, _ := m.JSON()
	// Two fresh builds should produce the same bytes (deterministic).
	if string(b1) != string(b2) {
		t.Errorf("Refresh should produce stable output")
	}
}

func TestDocsHandler_ServesUI(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /docs/", DocsHandler(WithTitle("M")))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<!doctype html>") {
		t.Errorf("body should contain doctype, got: %s", rr.Body.String())
	}
}

func TestDocsHandler_ServesOpenAPIJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /docs/", DocsHandler(WithTitle("M")))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/openapi.json", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
	// The placeholder is `{}` — DocsHandler does not produce a
	// populated spec. Users who want a real spec should use
	// *stdocs.Mux.
	body := strings.TrimSpace(rr.Body.String())
	if body != "{}" {
		t.Errorf("body = %q, want %q", body, "{}")
	}
}

func TestDocsHandler_ServesOpenAPIYAML(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /docs/", DocsHandler(WithTitle("M")))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/openapi.yaml", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/yaml") {
		t.Errorf("Content-Type = %q", ct)
	}
	// The placeholder YAML is `{}` — the JSON placeholder, round-
	// tripped through the YAML converter (rather than emitted raw,
	// which would lack a top-level document marker).
	body := strings.TrimSpace(rr.Body.String())
	if body != "{}" {
		t.Errorf("body = %q, want %q", body, "{}")
	}
}

func TestDocsHandler_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /docs/", DocsHandler(WithTitle("M")))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/unknown", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != 404 {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

// TestDocsHandler_HonorsUIDoc guards the regression where the docs
// handler permanently served the default zero-JS UI even when a UI
// option was passed. The fix is to read cfg.UIDoc.
func TestDocsHandler_HonorsUIDoc(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /docs/", DocsHandler(WithTitle("My Custom Title")))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/docs/", nil)
	mux.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "My Custom Title") {
		t.Errorf("body should contain the custom title, got: %s", rr.Body.String())
	}
}

// TestDocsHandler_CustomPrefix guards the prefix-handling path.
func TestDocsHandler_CustomPrefix(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("GET /api-docs/", DocsHandler(WithTitle("T"), WithDocsPrefix("/api-docs")))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api-docs/", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestMux_MultipleRoutesSamePath(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {})
	m.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	users := jget(t, doc, "paths", "/users").(map[string]any)
	if _, ok := users["get"]; !ok {
		t.Errorf("get missing")
	}
	if _, ok := users["post"]; !ok {
		t.Errorf("post missing")
	}
}

func TestMux_SharedComponent(t *testing.T) {
	type User struct {
		ID string `json:"id"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("GET /a", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, User{}),
	)
	m.HandleFunc("GET /b", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, User{}),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	// User should appear once in components.
	comps := jget(t, doc, "components", "schemas").(map[string]any)
	if _, ok := comps["User"]; !ok {
		t.Errorf("User missing from components: %v", comps)
	}
}

// TestMux_SharedComponent_NullableUseSite guards showstopper 8:
// the shared User component must NOT inherit nullability from a
// *User use site. Other sites that use plain User must see the same
// non-nullable component. The use site itself emits an allOf/anyOf
// wrapper.
func TestMux_SharedComponent_NullableUseSite(t *testing.T) {
	type User struct {
		ID string `json:"id"`
	}
	m := New(WithTitle("T"))
	// *User (nullable) and User (non-nullable) on different routes.
	m.HandleFunc("GET /nullable", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, (*User)(nil)),
	)
	m.HandleFunc("GET /plain", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, User{}),
	)
	b, _ := m.JSON()
	doc := jx(t, b)

	// The User component itself must NOT be nullable.
	user := jget(t, doc, "components", "schemas", "User").(map[string]any)
	if user["nullable"] == true {
		t.Errorf("User component must not be nullable; use-site handles it")
	}
	// Check the 3.0 wrapper form on the nullable route.
	nullable := jget(t, doc, "paths", "/nullable", "get", "responses", "200", "content", "application/json", "schema").(map[string]any)
	if _, has := nullable["allOf"]; !has {
		t.Errorf("nullable use site should use 3.0 allOf wrapper, got: %v", nullable)
	}
	if nullable["nullable"] != true {
		t.Errorf("nullable use site should set nullable:true, got: %v", nullable["nullable"])
	}
	// Check the plain route does NOT have a wrapper.
	plain := jget(t, doc, "paths", "/plain", "get", "responses", "200", "content", "application/json", "schema").(map[string]any)
	if _, has := plain["allOf"]; has {
		t.Errorf("plain use site should not have allOf wrapper, got: %v", plain)
	}
	if plain["$ref"] != "#/components/schemas/User" {
		t.Errorf("plain use site should be a bare $ref, got: %v", plain)
	}
}

// TestMux_SharedComponent_31AnyOf guards the same fix in 3.1 mode:
// the use-site wrapper is anyOf + {type: null}, not a 3.0 allOf.
func TestMux_SharedComponent_31AnyOf(t *testing.T) {
	type User struct {
		ID string `json:"id"`
	}
	m := New(WithTitle("T"), WithVersion(OpenAPI31))
	m.HandleFunc("GET /nullable", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, (*User)(nil)),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	user := jget(t, doc, "components", "schemas", "User").(map[string]any)
	if user["nullable"] == true {
		t.Errorf("User component must not be nullable in 3.1")
	}
	nullable := jget(t, doc, "paths", "/nullable", "get", "responses", "200", "content", "application/json", "schema").(map[string]any)
	if _, has := nullable["anyOf"]; !has {
		t.Errorf("nullable use site should use 3.1 anyOf wrapper, got: %v", nullable)
	}
}

func TestMux_RecursiveType(t *testing.T) {
	type Node struct {
		Value    string  `json:"value"`
		Children []*Node `json:"children"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("GET /tree", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, Node{}),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	node := jget(t, doc, "components", "schemas", "Node").(map[string]any)
	children := jget(t, node, "properties", "children").(map[string]any)
	if children["type"] != "array" {
		t.Errorf("children.type = %v, want array", children["type"])
	}
	items := children["items"].(map[string]any)
	if items["$ref"] != "#/components/schemas/Node" {
		t.Errorf("items.$ref = %v", items["$ref"])
	}
}

func TestMux_Handle_PathLevel(t *testing.T) {
	m := New(WithTitle("T"))
	m.Handle("GET /x", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	if len(m.reg.routes) != 1 {
		t.Errorf("routes = %d, want 1", len(m.reg.routes))
	}
}

func TestMux_ThreadSafe(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = m.JSON()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestMux_JSON_NoEmptyNameParameter guards the regression where a
// route at "/" or a trailing-slash pattern emitted a path parameter
// with an empty name (the implicit anonymous multi). OpenAPI
// validators reject this; a health endpoint at "/" is enough to
// trigger it.
func TestMux_JSON_NoEmptyNameParameter(t *testing.T) {
	cases := []string{
		"GET /",
		"GET /health",
		"GET /static/",
		"GET /api/v1/users/{id}/posts/",
	}
	for _, pattern := range cases {
		t.Run(pattern, func(t *testing.T) {
			m := New(WithTitle("T"))
			m.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {})
			b, err := m.JSON()
			if err != nil {
				t.Fatal(err)
			}
			doc := jx(t, b)
			// Walk every parameter at every operation and every path
			// item. None may have an empty name.
			checkParams(t, doc["paths"], pattern)
		})
	}
}

func checkParams(t *testing.T, v any, label string) {
	t.Helper()
	paths, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s: not a map", label)
	}
	for path, item := range paths {
		pathItem, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if ps, ok := pathItem["parameters"].([]any); ok {
			for _, p := range ps {
				pm := p.(map[string]any)
				if name, _ := pm["name"].(string); name == "" {
					t.Errorf("%s: empty parameter name in path-level: %v", path, pm)
				}
			}
		}
		for method, op := range pathItem {
			if method == "parameters" {
				continue
			}
			om, ok := op.(map[string]any)
			if !ok {
				continue
			}
			if ps, ok := om["parameters"].([]any); ok {
				for _, p := range ps {
					pm := p.(map[string]any)
					if name, _ := pm["name"].(string); name == "" {
						t.Errorf("%s %s: empty parameter name at operation: %v", path, method, pm)
					}
				}
			}
		}
	}
}

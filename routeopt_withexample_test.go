package stdocs

import (
	"net/http"
	"strings"
	"testing"
)

func TestWithExample_OnRequestBody(t *testing.T) {
	type Req struct {
		Title string `json:"title"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {},
		WithBody(Req{}),
		WithResponse(201, Req{}),
		WithExample(Req{Title: "Buy milk"}),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	ex := jget(t, doc, "paths", "/x", "post", "requestBody", "content", "application/json", "example").(map[string]any)
	if ex["title"] != "Buy milk" {
		t.Errorf("example.title = %v, want Buy milk", ex["title"])
	}
}

func TestWithExample_OnMostRecentResponse(t *testing.T) {
	type User struct {
		ID string `json:"id"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, User{}),
		WithResponse(404, map[string]string{"error": "not found"}),
		WithExample(User{ID: "u-1"}),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	// The most recent WithResponse is 404, so WithExample should
	// attach to 404, not 200.
	ex404 := jget(t, doc, "paths", "/x", "get", "responses", "404", "content", "application/json", "example").(map[string]any)
	if ex404["id"] != "u-1" {
		t.Errorf("404 example = %v, want {id: u-1}", ex404)
	}
	// 200 should have no example.
	if _, has := mustGetMap(t, doc, "paths", "/x", "get", "responses", "200", "content", "application/json")["example"]; has {
		t.Errorf("200 should not have example")
	}
}

func TestWithResponseExample_ByStatus(t *testing.T) {
	type User struct {
		ID string `json:"id"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, User{}),
		WithResponse(404, map[string]string{"error": "not found"}),
		WithResponseExample(200, User{ID: "u-1"}),
		WithResponseExample(404, map[string]string{"error": "task not found"}),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	ex200 := jget(t, doc, "paths", "/x", "get", "responses", "200", "content", "application/json", "example").(map[string]any)
	if ex200["id"] != "u-1" {
		t.Errorf("200 example = %v, want u-1", ex200)
	}
	ex404 := jget(t, doc, "paths", "/x", "get", "responses", "404", "content", "application/json", "example").(map[string]any)
	if ex404["error"] != "task not found" {
		t.Errorf("404 example = %v", ex404)
	}
}

func TestWithExample_NoOpWithoutBodyOrResponse(t *testing.T) {
	// Should not panic.
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithExample("anything"),
	)
	b, _ := m.JSON()
	// No example should be in the spec because there's no body and
	// no response.
	doc := jx(t, b)
	op := jget(t, doc, "paths", "/x", "get").(map[string]any)
	if _, has := op["requestBody"]; has {
		t.Errorf("should not have requestBody")
	}
	resp := op["responses"].(map[string]any)
	for _, v := range resp {
		rm := v.(map[string]any)
		if _, has := rm["content"]; has {
			t.Errorf("response should not have content: %v", rm["content"])
		}
	}
}

func TestWithExample_BothRequestAndResponse(t *testing.T) {
	// When both are present, WithExample targets the request body.
	type Req struct {
		Title string `json:"title"`
	}
	type Resp struct {
		ID string `json:"id"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {},
		WithBody(Req{}),
		WithResponse(201, Resp{}),
		WithExample(Req{Title: "Buy milk"}),       // -> request body
		WithResponseExample(201, Resp{ID: "u-1"}), // -> response 201
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	reqEx := jget(t, doc, "paths", "/x", "post", "requestBody", "content", "application/json", "example").(map[string]any)
	if reqEx["title"] != "Buy milk" {
		t.Errorf("request example = %v", reqEx)
	}
	respEx := jget(t, doc, "paths", "/x", "post", "responses", "201", "content", "application/json", "example").(map[string]any)
	if respEx["id"] != "u-1" {
		t.Errorf("response example = %v", respEx)
	}
}

func TestWithExample_StringValue(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithResponse(200, nil),
		WithExample("just a string"),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	ex := jget(t, doc, "paths", "/x", "get", "responses", "200", "content", "application/json", "example")
	if ex != "just a string" {
		t.Errorf("example = %v", ex)
	}
}

func TestWithExample_EncodesValuesCorrectly(t *testing.T) {
	type Req struct {
		Count int      `json:"count"`
		Tags  []string `json:"tags"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {},
		WithBody(Req{}),
		WithResponse(201, Req{}),
		WithExample(Req{Count: 3, Tags: []string{"a", "b"}}),
	)
	b, _ := m.JSON()
	// The example should be a JSON object, not a stringified version.
	if !strings.Contains(string(b), `"count":3`) {
		t.Errorf("expected count:3 in output, got %s", b)
	}
	if !strings.Contains(string(b), `"a"`) || !strings.Contains(string(b), `"b"`) {
		t.Errorf("expected tags in output, got %s", b)
	}
}

// mustGetMap returns a sub-map at path, failing the test if any key
// is missing.
func mustGetMap(t *testing.T, m map[string]any, path ...string) map[string]any {
	t.Helper()
	cur := any(m)
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("mustGetMap: %v is not a map at %v", cur, path)
		}
		v, ok := mm[k]
		if !ok {
			t.Fatalf("mustGetMap: key %q missing at %v", k, path)
		}
		cur = v
	}
	mm, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("mustGetMap: %v is not a map", cur)
	}
	return mm
}

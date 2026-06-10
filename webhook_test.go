package stdocs

import (
	"net/http"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

func TestWithWebhooks_3_1(t *testing.T) {
	type UserPayload struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	hook := Webhook{
		Method:      "POST",
		Summary:     "New user created",
		Description: "Fired when a new user is created",
		OperationID: "newUserWebhook",
		RequestBody: &RequestBody{
			Schema: mustSchema(t, UserPayload{}),
		},
		Responses: map[string]*Response{
			"200": {Description: "OK"},
		},
	}
	m := New(
		WithTitle("T"),
		WithVersion("3.1.0"),
		WithWebhooks(map[string]Webhook{"newUser": hook}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	wh := jget(t, doc, "webhooks", "newUser", "post").(map[string]any)
	if wh["summary"] != "New user created" {
		t.Errorf("summary = %v", wh["summary"])
	}
	if wh["operationId"] != "newUserWebhook" {
		t.Errorf("operationId = %v", wh["operationId"])
	}
	// Responses should be present.
	resp := jget(t, wh, "responses", "200").(map[string]any)
	if resp["description"] != "OK" {
		t.Errorf("description = %v", resp["description"])
	}
}

func TestWithWebhooks_3_0_3_Ignored(t *testing.T) {
	// Webhooks are 3.1-only; on 3.0.3 they should be silently
	// ignored without error.
	m := New(
		WithTitle("T"),
		WithVersion("3.0.3"),
		WithWebhooks(map[string]Webhook{
			"newUser": {Method: "POST", Summary: "x"},
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	if _, has := doc["webhooks"]; has {
		t.Errorf("3.0.3 should not include webhooks, got %v", doc["webhooks"])
	}
}

func TestWithWebhooks_Multiple(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithVersion("3.1.0"),
		WithWebhooks(map[string]Webhook{
			"newUser":     {Method: "POST", Summary: "new user"},
			"deletedUser": {Method: "POST", Summary: "user deleted"},
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	wh := jget(t, doc, "webhooks").(map[string]any)
	if _, ok := wh["newUser"]; !ok {
		t.Errorf("newUser missing")
	}
	if _, ok := wh["deletedUser"]; !ok {
		t.Errorf("deletedUser missing")
	}
}

func TestWithWebhooks_Default200WhenNoResponses(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithVersion("3.1.0"),
		WithWebhooks(map[string]Webhook{
			"x": {Method: "POST", Summary: "x"},
		}),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	wh := jget(t, doc, "webhooks", "x", "post").(map[string]any)
	// Should auto-add a 200 response.
	if _, has := wh["responses"]; !has {
		t.Errorf("responses missing: %v", wh)
	}
}

func TestWithWebhooks_MethodCase(t *testing.T) {
	// Methods should be lower-cased in the output.
	m := New(
		WithTitle("T"),
		WithVersion("3.1.0"),
		WithWebhooks(map[string]Webhook{
			"x": {Method: "POST", Summary: "x"},
		}),
	)
	m.HandleFunc("GET /y", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	// Just check that the JSON has "post" as the key, not "POST".
	if !strings.Contains(string(b), `"post":{`) {
		t.Errorf("webhook method should be lowercased: %s", b)
	}
}

// mustSchema is a small helper to build a Schema from a Go value for
// the Webhook tests, avoiding a full ReflectSchema call.
func mustSchema(t *testing.T, v any) *schema.Schema {
	t.Helper()
	s, _ := schema.ReflectSchema(v, version.OpenAPI30)
	return s
}

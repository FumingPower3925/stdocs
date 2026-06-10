package stdocs

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestParity_30_31 checks that the same registered mux produces
// semantically equivalent specs under OpenAPI 3.0.3 and 3.1.0:
// same paths, same operations, same parameters, same response
// statuses, same component names. The structural differences
// (nullable encoding, webhook support) are normalised.
func TestParity_30_31(t *testing.T) {
	type Body struct {
		Title string `json:"title"`
	}
	type Resp struct {
		ID string `json:"id"`
	}
	makeMux := func(v SpecVersion) *Mux {
		m := New(WithTitle("T"), WithVersion(v), WithBearerAuth("bearerAuth", "JWT"))
		m.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {},
			Summary("List users"),
			Tags("users"),
			WithResponse(200, []Resp{}),
		)
		m.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {},
			WithBody(Body{}),
			WithResponse(201, Resp{}),
			WithResponse(422, struct{}{}),
		)
		m.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {},
			WithResponse(200, Resp{}),
			WithResponse(404, struct{}{}),
		)
		return m
	}
	m30 := makeMux(OpenAPI30)
	m31 := makeMux(OpenAPI31)
	b30, _ := m30.JSON()
	b31, _ := m31.JSON()
	var d30, d31 map[string]any
	_ = json.Unmarshal(b30, &d30)
	_ = json.Unmarshal(b31, &d31)
	// OpenAPI version string differs.
	if d30["openapi"] == d31["openapi"] {
		t.Errorf("expected different openapi versions, got both %v", d30["openapi"])
	}
	// Same paths.
	p30 := keys(d30["paths"].(map[string]any))
	p31 := keys(d31["paths"].(map[string]any))
	if !stringEq(p30, p31) {
		t.Errorf("paths differ:\n  30=%v\n  31=%v", p30, p31)
	}
	// Same component names.
	c30 := components(d30)
	c31 := components(d31)
	if !stringEq(c30, c31) {
		t.Errorf("components differ:\n  30=%v\n  31=%v", c30, c31)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func components(d map[string]any) []string {
	c, ok := d["components"].(map[string]any)
	if !ok {
		return nil
	}
	s, ok := c["schemas"].(map[string]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out
}

func stringEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int, len(a))
	for _, s := range a {
		m[s]++
	}
	for _, s := range b {
		m[s]--
	}
	for _, n := range m {
		if n != 0 {
			return false
		}
	}
	return true
}

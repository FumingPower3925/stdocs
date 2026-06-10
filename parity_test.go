package stdocs

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

// TestParity checks that the same registered mux produces
// semantically equivalent specs under OpenAPI 3.0, 3.1, and 3.2:
// same paths, same method sets per path, same parameter (name,in)
// sets, same response statuses, same operationIds, and same
// component names. The structural differences (nullable encoding,
// webhook support, $self) are normalised or excluded. As a stronger
// check, the 3.1 and 3.2 documents must be deeply identical apart
// from the openapi version string (3.2 adds nothing for these
// inputs).
func TestParity(t *testing.T) {
	type Body struct {
		Title string `json:"title"`
	}
	type Resp struct {
		ID   string  `json:"id"`
		Next *Resp   `json:"next"` // nullable ref: encoding differs by version
		Tags []*Resp `json:"tags"`
	}
	makeMux := func(v SpecVersion) *Mux {
		m := New(WithTitle("T"), WithVersion(v), WithBearerAuth("bearerAuth", "JWT"))
		m.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {},
			Summary("List users"),
			Tags("users"),
			WithResponse(200, []Resp{}),
			QueryParam("page", "integer", "page number"),
		)
		m.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {},
			WithBody(Body{}),
			WithResponse(201, Resp{}),
			WithResponse(422, struct{}{}),
			WithSecurity("bearerAuth"),
		)
		m.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {},
			WithResponse(200, Resp{}),
			WithResponse(404, struct{}{}),
		)
		return m
	}

	docs := map[SpecVersion]map[string]any{}
	for _, v := range []SpecVersion{OpenAPI30, OpenAPI31, OpenAPI32} {
		b, err := makeMux(v).JSON()
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		var d map[string]any
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		docs[v] = d
	}

	base := docs[OpenAPI30]
	for _, v := range []SpecVersion{OpenAPI31, OpenAPI32} {
		d := docs[v]
		if base["openapi"] == d["openapi"] {
			t.Errorf("expected different openapi versions, got both %v", d["openapi"])
		}
		// Same paths.
		if !stringEq(keys(base["paths"].(map[string]any)), keys(d["paths"].(map[string]any))) {
			t.Errorf("%s: paths differ from 3.0", v)
		}
		// Same component names.
		if !stringEq(components(base), components(d)) {
			t.Errorf("%s: components differ from 3.0:\n  30=%v\n  %s=%v", v, components(base), v, components(d))
		}
		// Per path: same method sets, operationIds, response statuses,
		// and parameter (name, in) pairs.
		for path, pi := range base["paths"].(map[string]any) {
			basePI := pi.(map[string]any)
			otherPI := d["paths"].(map[string]any)[path].(map[string]any)
			if !stringEq(keys(basePI), keys(otherPI)) {
				t.Errorf("%s %s: path-item keys differ: %v vs %v", v, path, keys(basePI), keys(otherPI))
				continue
			}
			for method, op := range basePI {
				if method == "parameters" {
					if !reflect.DeepEqual(paramPairs(op), paramPairs(otherPI[method])) {
						t.Errorf("%s %s: path params differ", v, path)
					}
					continue
				}
				baseOp := op.(map[string]any)
				otherOp := otherPI[method].(map[string]any)
				if baseOp["operationId"] != otherOp["operationId"] {
					t.Errorf("%s %s %s: operationId %v vs %v", v, path, method, baseOp["operationId"], otherOp["operationId"])
				}
				baseResp, _ := baseOp["responses"].(map[string]any)
				otherResp, _ := otherOp["responses"].(map[string]any)
				if !stringEq(keys(baseResp), keys(otherResp)) {
					t.Errorf("%s %s %s: response statuses differ", v, path, method)
				}
				if !reflect.DeepEqual(paramPairs(baseOp["parameters"]), paramPairs(otherOp["parameters"])) {
					t.Errorf("%s %s %s: parameters differ", v, path, method)
				}
			}
		}
	}

	// 3.1 vs 3.2 must be byte-identical apart from the version string
	// (no $self was set and these inputs exercise no 3.2-only paths).
	d31, d32 := docs[OpenAPI31], docs[OpenAPI32]
	d31["openapi"], d32["openapi"] = "X", "X"
	b31, _ := json.Marshal(d31)
	b32, _ := json.Marshal(d32)
	if string(b31) != string(b32) {
		t.Errorf("3.1 and 3.2 documents differ beyond the version string:\n31: %s\n32: %s", b31, b32)
	}
}

// paramPairs extracts the (name, in) pairs from a parameters array.
func paramPairs(v any) [][2]string {
	arr, _ := v.([]any)
	out := make([][2]string, 0, len(arr))
	for _, p := range arr {
		pm, _ := p.(map[string]any)
		name, _ := pm["name"].(string)
		in, _ := pm["in"].(string)
		out = append(out, [2]string{name, in})
	}
	return out
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

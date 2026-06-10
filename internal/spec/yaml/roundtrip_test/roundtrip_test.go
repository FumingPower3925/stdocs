// This is a SEPARATE GO MODULE (note the go.mod below). It exists
// to verify the hand-rolled YAML emitter in
// internal/spec/yaml using a real YAML parser (gopkg.in/yaml.v3)
// without making yaml.v3 a transitive dependency of the main
// github.com/FumingPower3925/stdocs module.
//
// Downstream users of stdocs see zero third-party deps. This
// test module is opt-in: run it manually with `go test ./...`
// inside this directory, or via the `roundtrip` CI job.
//
// See ../../../../.github/workflows/ci.yml for the job.
package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	yaml "gopkg.in/yaml.v3"

	"github.com/FumingPower3925/stdocs"
)

// TestYAMLRoundTrip verifies that the YAML emitted by
// *Mux.YAML round-trips through yaml.v3 to the same logical
// document as the JSON. Catches lost fields, type coercions,
// and ordering changes.
func TestYAMLRoundTrip(t *testing.T) {
	m := stdocs.New(
		stdocs.WithTitle("Round-Trip API"),
		stdocs.WithDescription("Verifies YAML emission is parseable"),
		stdocs.WithVersion(stdocs.OpenAPI30),
		stdocs.WithBearerAuth("bearerAuth", "JWT"),
	)
	type CreateReq struct {
		Title string `json:"title" description:"The title"`
		Body  string `json:"body"`
	}
	type CreateResp struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	m.HandleFunc("POST /articles", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.WithBody(CreateReq{}),
		stdocs.WithResponse(201, CreateResp{}),
		stdocs.WithResponse(404, struct{}{}),
		stdocs.Summary("Create an article"),
		stdocs.Tags("articles"),
	)
	jsonBytes, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	yamlBytes, err := m.YAML()
	if err != nil {
		t.Fatal(err)
	}
	var jsonDoc, yamlDoc map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonDoc); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if err := yaml.Unmarshal(yamlBytes, &yamlDoc); err != nil {
		t.Fatalf("unmarshal YAML: %v\n--- yaml ---\n%s", err, yamlBytes)
	}
	if !jsonEqual(t, jsonDoc, yamlDoc) {
		t.Errorf("JSON and YAML docs differ:\nJSON: %s\nYAML: %s", jsonBytes, yamlBytes)
	}
}

// TestYAMLParseable covers a wider variety of value types
// (nested objects, slices, maps) to make sure the emitter
// produces well-formed YAML for each.
func TestYAMLParseable(t *testing.T) {
	type s struct {
		A string         `json:"a"`
		B int            `json:"b"`
		C []int          `json:"c"`
		D map[string]any `json:"d"`
	}
	m := stdocs.New(stdocs.WithTitle("T"), stdocs.WithVersion(stdocs.OpenAPI30))
	m.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.WithBody(s{}),
		stdocs.WithResponse(200, s{}),
	)
	b, err := m.YAML()
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		t.Fatalf("YAML did not parse: %v\nbody:\n%s", err, b)
	}
}

// TestYAMLKeysStayStrings walks the emitted YAML as a yaml.Node tree
// and asserts every mapping key has the !!str tag. JSON object keys
// are always strings, and the OpenAPI spec requires YAML mapping keys
// to stay strings — an unquoted "200" response key would resolve to
// !!int. (A coercing comparison like yamlToJSONString would mask
// exactly this bug, so this test inspects node tags directly.)
func TestYAMLKeysStayStrings(t *testing.T) {
	m := stdocs.New(stdocs.WithTitle("T"), stdocs.WithVersion(stdocs.OpenAPI30))
	m.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.WithResponse(200, struct {
			Name string `json:"name"`
		}{}),
		stdocs.WithResponse(404, nil),
	)
	b, err := m.YAML()
	if err != nil {
		t.Fatal(err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(b, &root); err != nil {
		t.Fatalf("YAML did not parse: %v\nbody:\n%s", err, b)
	}
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i < len(n.Content); i += 2 {
				key := n.Content[i]
				if key.Tag != "!!str" {
					t.Errorf("mapping key %q has tag %s, want !!str", key.Value, key.Tag)
				}
				walk(n.Content[i+1])
			}
			return
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(&root)
}

// TestYAMLControlChars verifies that control characters in
// user-supplied strings are escaped: the YAML must parse and the
// value must round-trip exactly (a raw CR would either break the
// parse or silently fold into a space).
func TestYAMLControlChars(t *testing.T) {
	const desc = "bad\rchar and \x01 ctrl and \u0085 nel"
	m := stdocs.New(
		stdocs.WithTitle("T"),
		stdocs.WithDescription(desc),
		stdocs.WithVersion(stdocs.OpenAPI30),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, err := m.YAML()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("YAML did not parse: %v\nbody:\n%s", err, b)
	}
	info, _ := doc["info"].(map[string]any)
	if got, _ := info["description"].(string); got != desc {
		t.Errorf("description round-tripped to %q, want %q", got, desc)
	}
}

// jsonEqual re-marshals both sides to JSON and compares bytes.
// yaml.v3 unmarshals nested maps as map[interface{}]interface{};
// we recursively convert those to map[string]any so stdlib
// json can re-marshal.
func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	ab, err := json.Marshal(yamlToJSONable(a))
	if err != nil {
		t.Fatal(err)
	}
	bb, err := json.Marshal(yamlToJSONable(b))
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(ab, bb)
}

func yamlToJSONable(v any) any {
	switch t := v.(type) {
	case map[interface{}]interface{}:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			out[yamlToJSONString(k)] = yamlToJSONable(vv)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, vv := range t {
			out[k] = yamlToJSONable(vv)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, vv := range t {
			out[i] = yamlToJSONable(vv)
		}
		return out
	}
	return v
}

func yamlToJSONString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return ""
}

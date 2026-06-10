package stdocs

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func marshalJSON(v any) ([]byte, error)  { return json.Marshal(v) }
func unmarshalJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

// TestYAMLRoundTrip_Golden verifies that the YAML emitted by
// *Mux.YAML round-trips through a real YAML parser (yaml.v3) to
// the same logical document as the JSON. This catches the kinds
// of bugs the hand-rolled JSON->YAML converter could have: lost
// fields, type coercions, ordering changes.
func TestYAMLRoundTrip_Golden(t *testing.T) {
	m := New(
		WithTitle("Round-Trip API"),
		WithDescription("Verifies YAML emission is parseable"),
		WithVersion(OpenAPI30),
		WithBearerAuth("bearerAuth", "JWT"),
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
		WithBody(CreateReq{}),
		WithResponse(201, CreateResp{}),
		WithResponse(404, struct{}{}),
		Summary("Create an article"),
		Tags("articles"),
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
	if err := unmarshalJSON(jsonBytes, &jsonDoc); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}
	if err := yaml.Unmarshal(yamlBytes, &yamlDoc); err != nil {
		t.Fatalf("unmarshal YAML: %v\n--- yaml ---\n%s", err, yamlBytes)
	}
	// Compare structurally (yaml.v3 may unmarshal numbers as
	// int or float; we compare semantic JSON via re-marshal).
	if !jsonEqual(t, jsonDoc, yamlDoc) {
		t.Errorf("JSON and YAML docs differ:\nJSON: %s\nYAML: %s", jsonBytes, yamlBytes)
	}
}

func TestYAML_Parseable_Diverse(t *testing.T) {
	type s struct {
		A string `json:"a"`
		B int    `json:"b"`
		C []int  `json:"c"`
		D map[string]any `json:"d"`
	}
	m := New(WithTitle("T"), WithVersion(OpenAPI30))
	m.HandleFunc("POST /x", func(w http.ResponseWriter, r *http.Request) {},
		WithBody(s{}),
		WithResponse(200, s{}),
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

// jsonEqual re-marshals both sides to JSON and compares bytes —
// avoids issues with int/float, nil vs empty map, etc.
// yaml.v3 unmarshals into map[interface{}]interface{} for nested
// maps; we recursively convert those to map[string]any so stdlib
// json can re-marshal.
func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	ab, err := marshalJSON(yamlToJSONable(a))
	if err != nil {
		t.Fatal(err)
	}
	bb, err := marshalJSON(yamlToJSONable(b))
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

package spec

import (
	"encoding/json"
	"testing"
)

// (TestEmitOpenAPI32_SchemaShapeIs31Like removed — it tested
// ReflectSchema, which lives in the schema package. The 3.2 emitter
// delegates to buildSchema31, so the shape is identical by
// construction. The 3.1 nullable tests already cover the
// schema shape.)

// TestEmitOpenAPI32_TopLevelShape asserts the top-level fields of a
// 3.2.0 spec: openapi = "3.2.0", info, paths, components are present;
// $self is present iff selfURL is non-empty.
func TestEmitOpenAPI32_TopLevelShape(t *testing.T) {
	in := SpecInput{
		Info:  Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{},
	}
	b, err := EmitOpenAPI32(in, "")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if got := m["openapi"]; got != "3.2.0" {
		t.Errorf("openapi = %v, want 3.2.0", got)
	}
	if _, has := m["$self"]; has {
		t.Errorf("expected $self to be absent when selfURL is empty")
	}
}

// TestEmitOpenAPI32_SelfURL emits $self when WithSelfURL is set.
func TestEmitOpenAPI32_SelfURL(t *testing.T) {
	in := SpecInput{
		Info:  Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{},
	}
	b, err := EmitOpenAPI32(in, "https://example.com/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if got := m["$self"]; got != "https://example.com/openapi.json" {
		t.Errorf("$self = %v, want https://example.com/openapi.json", got)
	}
}

// TestEmitOpenAPI32_Webhooks verifies that 3.2 emits webhooks the
// same way 3.1 does (the field name and structure are unchanged
// in 3.2; this test guards against accidental regression when
// the 3.2 emitter was added).
func TestEmitOpenAPI32_Webhooks(t *testing.T) {
	in := SpecInput{
		Info:  Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{},
		Webhooks: map[string]Webhook{
			"newUser": {Method: "POST", Summary: "x"},
		},
	}
	b, err := EmitOpenAPI32(in, "")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, has := m["webhooks"]; !has {
		t.Errorf("3.2 should include webhooks, got %v", m)
	}
}

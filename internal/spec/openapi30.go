package spec

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// EmitOpenAPI30 produces the OpenAPI 3.0 (3.0.4) JSON bytes for the
// given input. The result is deterministic across runs: every nested
// object is emitted from sorted keys (useful for tests and golden
// files).
func EmitOpenAPI30(in SpecInput) ([]byte, error) {
	root := BuildRoot30(in)
	return json.Marshal(root)
}

// BuildRoot30 builds the top-level OpenAPI 3.0 (3.0.4) object.
func BuildRoot30(in SpecInput) map[string]any {
	e := &emitter{openapi: string(version.OpenAPI30), buildSchema: buildSchema30}
	return e.buildRoot(in)
}

// buildSchema30 converts a *schema.Schema into the map[string]any form
// for OpenAPI 3.0.
//
// $ref schemas with extra facets (nullability, a doc-tag description,
// or an example) use the OpenAPI 3.0 allOf wrapper, because 3.0
// ignores siblings placed next to $ref: the shared component stays
// clean and the use site wraps it in
// {"allOf": [{"$ref": "..."}], "nullable": true, "description": ...}.
// This is required so that two routes — one with *T and one with T —
// both reference the same component without one contaminating the
// other.
// refSchema30 renders a $ref use site, wrapping in allOf when the use
// site carries extra facets (3.0 ignores siblings next to $ref).
func refSchema30(s *schema.Schema) map[string]any {
	if !s.Nullable && s.Description == "" && s.Example == nil {
		return map[string]any{"$ref": s.Ref}
	}
	m := map[string]any{
		"allOf": []any{map[string]any{"$ref": s.Ref}},
	}
	if s.Nullable {
		m["nullable"] = true
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Example != nil {
		m["example"] = s.Example
	}
	return m
}

func buildSchema30(s *schema.Schema) map[string]any {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		return refSchema30(s)
	}
	m := make(map[string]any)
	if s.Type != "" {
		m["type"] = s.Type
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Nullable && s.Type != "" {
		// nullable is only meaningful next to a type in 3.0; untyped
		// schemas accept null anyway (3.1/3.2 are silent here too).
		m["nullable"] = true
	}
	if s.Type == "object" {
		if len(s.Properties) > 0 {
			props := make(map[string]any, len(s.Properties))
			for _, k := range sortedKeys(s.Properties) {
				props[k] = buildSchema30(s.Properties[k])
			}
			m["properties"] = props
		}
		if len(s.Required) > 0 {
			req := slices.Clone(s.Required)
			slices.Sort(req)
			m["required"] = req
		}
		if s.AdditionalProperties != nil {
			m["additionalProperties"] = buildSchema30(s.AdditionalProperties)
		}
	}
	if s.Type == "array" && s.Items != nil {
		m["items"] = buildSchema30(s.Items)
	}
	if len(s.Enum) > 0 {
		enum := s.Enum
		if s.Nullable {
			// enum is independent of type/nullable in JSON Schema:
			// null must be listed for a nullable field's null value to
			// validate against its own enum.
			enum = append(append(make([]any, 0, len(enum)+1), enum...), nil)
		}
		m["enum"] = enum
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if s.Example != nil {
		m["example"] = s.Example
	}
	applyConstraintFacets(m, s)
	// OpenAPI 3.0 uses the draft-4 boolean form: the bound goes in
	// minimum/maximum and exclusiveMinimum/exclusiveMaximum is a
	// boolean flag. The model guarantees a bound is either inclusive
	// or exclusive, never both.
	if s.ExclusiveMinimum != "" {
		m["minimum"] = s.ExclusiveMinimum
		m["exclusiveMinimum"] = true
	}
	if s.ExclusiveMaximum != "" {
		m["maximum"] = s.ExclusiveMaximum
		m["exclusiveMaximum"] = true
	}
	maps.Copy(m, s.Extensions)
	if len(m) == 0 {
		// A truly empty schema — emit {} to make JSON happy.
		return map[string]any{}
	}
	return m
}

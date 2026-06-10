package spec

import (
	"encoding/json"
	"sort"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// EmitOpenAPI31 produces the OpenAPI 3.1.0 JSON bytes for the given input.
// The output is the 3.0.3 emitter with two adjustments:
//
//   - Nullable is rendered as a type array ("type": ["T", "null"]) instead
//     of a "nullable": true sibling.
//   - The top-level "openapi" field is "3.1.0" and webhooks are emitted.
//
// All other structure is identical to 3.0.3.
func EmitOpenAPI31(in SpecInput) ([]byte, error) {
	root := BuildRoot31(in)
	return json.Marshal(root)
}

// BuildRoot31 builds the top-level OpenAPI 3.1.0 object.
func BuildRoot31(in SpecInput) map[string]any {
	e := &emitter{openapi: string(version.OpenAPI31), buildSchema: buildSchema31}
	return e.buildRoot(in)
}

// buildSchema31 converts a *schema.Schema into the map[string]any form for 3.1.0.
//
// Nullability for $ref uses the OpenAPI 3.1 anyOf + null pattern:
// the shared component is non-nullable, and the use site wraps it
// in {"anyOf": [{"$ref": "..."}, {"type": "null"}]}. This keeps
// the shared component clean across multiple use sites with
// different nullability.
func buildSchema31(s *schema.Schema) map[string]any {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		if s.Nullable {
			return map[string]any{
				"anyOf": []any{
					map[string]any{"$ref": s.Ref},
					map[string]any{"type": "null"},
				},
			}
		}
		return map[string]any{"$ref": s.Ref}
	}
	m := make(map[string]any)
	switch {
	case s.Type != "" && s.Nullable:
		m["type"] = []string{s.Type, "null"}
	case s.Type != "":
		m["type"] = s.Type
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Type == "object" {
		if len(s.Properties) > 0 {
			props := make(map[string]any, len(s.Properties))
			keys := make([]string, 0, len(s.Properties))
			for k := range s.Properties {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				props[k] = buildSchema31(s.Properties[k])
			}
			m["properties"] = props
		}
		if len(s.Required) > 0 {
			req := make([]string, len(s.Required))
			copy(req, s.Required)
			sort.Strings(req)
			m["required"] = req
		}
		if s.AdditionalProperties != nil {
			m["additionalProperties"] = buildSchema31(s.AdditionalProperties)
		}
	}
	if s.Type == "array" && s.Items != nil {
		m["items"] = buildSchema31(s.Items)
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if s.Example != nil {
		m["example"] = s.Example
	}
	for k, v := range s.Extensions {
		m[k] = v
	}
	if len(m) == 0 {
		return map[string]any{}
	}
	return m
}

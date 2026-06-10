package spec

import (
	"encoding/json"
	"sort"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// EmitOpenAPI30 produces the OpenAPI 3.0.3 JSON bytes for the given
// input. The result is sorted by key in every nested object so the
// output is deterministic across runs (useful for tests and golden
// files).
func EmitOpenAPI30(in SpecInput) ([]byte, error) {
	root := BuildRoot30(in)
	return json.Marshal(root)
}

// BuildRoot30 builds the top-level OpenAPI 3.0.3 object.
func BuildRoot30(in SpecInput) map[string]any {
	e := &emitter{openapi: string(version.OpenAPI30), buildSchema: buildSchema30}
	return e.buildRoot(in)
}

// buildSchema30 converts a *schema.Schema into the map[string]any form for 3.0.3.
//
// Nullability for $ref uses the OpenAPI 3.0 allOf + nullable
// pattern: the shared component is non-nullable, and the use site
// wraps it in {"allOf": [{"$ref": "..."}], "nullable": true}. This
// is required so that two routes — one with *T and one with T —
// both reference the same component without one contaminating the
// other.
func buildSchema30(s *schema.Schema) map[string]any {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		if s.Nullable {
			return map[string]any{
				"allOf":    []any{map[string]any{"$ref": s.Ref}},
				"nullable": true,
			}
		}
		return map[string]any{"$ref": s.Ref}
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
	if s.Nullable {
		m["nullable"] = true
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
				props[k] = buildSchema30(s.Properties[k])
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
			m["additionalProperties"] = buildSchema30(s.AdditionalProperties)
		}
	}
	if s.Type == "array" && s.Items != nil {
		m["items"] = buildSchema30(s.Items)
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
		// A truly empty schema — emit {} to make JSON happy.
		return map[string]any{}
	}
	return m
}

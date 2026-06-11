package spec

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// EmitOpenAPI31 produces the OpenAPI 3.1 (3.1.2) JSON bytes for the
// given input. The output is the 3.0 emitter with two adjustments:
//
//   - Nullable is rendered as a type array ("type": ["T", "null"])
//     instead of a "nullable": true sibling.
//   - The top-level "openapi" field is "3.1.2" and webhooks are
//     emitted.
//
// All other structure is identical to 3.0.
func EmitOpenAPI31(in SpecInput) ([]byte, error) {
	root := BuildRoot31(in)
	return json.Marshal(root)
}

// BuildRoot31 builds the top-level OpenAPI 3.1 (3.1.2) object.
func BuildRoot31(in SpecInput) map[string]any {
	e := &emitter{openapi: string(version.OpenAPI31), buildSchema: buildSchema31}
	return e.buildRoot(in)
}

// buildSchema31 converts a *schema.Schema into the map[string]any form
// for OpenAPI 3.1 (also reused by 3.2, whose schema dialect is the
// same for everything stdocs emits).
//
// Nullability for $ref uses the JSON Schema 2020-12 anyOf + null
// pattern: the shared component is non-nullable, and the use site
// wraps it in {"anyOf": [{"$ref": "..."}, {"type": "null"}]}. This
// keeps the shared component clean across multiple use sites with
// different nullability. Unlike 3.0, siblings next to $ref are legal
// in 2020-12, so doc-tag descriptions and examples are emitted
// directly alongside the reference.
// refSchema31 renders a $ref use site; nullable references use the
// anyOf form, and 2020-12 allows description/example as siblings.
func refSchema31(s *schema.Schema) map[string]any {
	var m map[string]any
	if s.Nullable {
		m = map[string]any{
			"anyOf": []any{
				map[string]any{"$ref": s.Ref},
				map[string]any{"type": "null"},
			},
		}
	} else {
		m = map[string]any{"$ref": s.Ref}
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if s.Example != nil {
		m["example"] = s.Example
	}
	return m
}

func buildSchema31(s *schema.Schema) map[string]any {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		return refSchema31(s)
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
	if s.Type == "object" {
		if len(s.Properties) > 0 {
			props := make(map[string]any, len(s.Properties))
			for _, k := range sortedKeys(s.Properties) {
				props[k] = buildSchema31(s.Properties[k])
			}
			m["properties"] = props
		}
		if len(s.Required) > 0 {
			req := slices.Clone(s.Required)
			slices.Sort(req)
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
		// The enum lives in the typed branch and needs no null member:
		// in the nullable anyOf form below, null validates against the
		// {"type": "null"} branch instead.
		m["enum"] = s.Enum
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if s.Example != nil {
		m["example"] = s.Example
	}
	applyConstraintFacets(m, s)
	// JSON Schema 2020-12: exclusive bounds are numeric keywords.
	if s.ExclusiveMinimum != "" {
		m["exclusiveMinimum"] = s.ExclusiveMinimum
	}
	if s.ExclusiveMaximum != "" {
		m["exclusiveMaximum"] = s.ExclusiveMaximum
	}
	maps.Copy(m, s.Extensions)
	if s.Type != "" && s.Nullable {
		// Nullable typed schemas emit the anyOf form rather than a
		// type array ("type": ["string", "null"]): both are valid
		// 2020-12, but real-world consumers digest anyOf more
		// reliably (ogen's parser rejects the array form outright),
		// and the $ref use sites above already use anyOf, so nullable
		// emission is uniform. Value-level decoration moves to the
		// wrapper; type-level facets stay in the typed branch.
		wrapper := map[string]any{
			"anyOf": []any{m, map[string]any{"type": "null"}},
		}
		for _, k := range []string{"description", "default", "example"} {
			if v, ok := m[k]; ok {
				delete(m, k)
				wrapper[k] = v
			}
		}
		for k := range s.Extensions {
			delete(m, k)
			wrapper[k] = s.Extensions[k]
		}
		return wrapper
	}
	if len(m) == 0 {
		return map[string]any{}
	}
	return m
}

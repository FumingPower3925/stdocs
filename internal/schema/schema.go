package schema

import (
	"reflect"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/FumingPower3925/stdocs/internal/version"
)

// Schema is a version-agnostic JSON Schema (Draft 2020-12 / OpenAPI 3.1 flavour)
// value. The OpenAPI 3.0.3 emitter in openapi30.go and the 3.1.0 emitter in
// openapi31.go serialize the same Schema value into their respective JSON
// representations.
//
// Fields that only make sense in 3.1 (TypeArray) or only in 3.0 (Nullable) are
// populated based on the target version when the schema is built; see
// ReflectSchema's version parameter.
type Schema struct {
	// Type is the JSON Schema type: "object", "array", "string", "number",
	// "integer", "boolean", or "null". Empty means "no type constraint"
	// (matches anything). For interfaces, Type is left empty.
	Type string
	// TypeArray is the JSON Schema 2020-12 / OpenAPI 3.1 form of a union
	// type, e.g. ["string", "null"]. The 3.0.3 emitter does not emit this
	// field; the 3.1.0 emitter does.
	TypeArray []string
	// Format is the JSON Schema format hint: "int32", "int64", "float",
	// "double", "date-time", "date", "time", "byte", "binary", "email", etc.
	Format string
	// Description is a human-readable description of this value.
	Description string
	// Properties maps JSON field name to its Schema. Only for Type=="object".
	Properties map[string]*Schema
	// Required is the list of property names that must be present.
	Required []string
	// Items is the element schema. Only for Type=="array".
	Items *Schema
	// AdditionalProperties constrains map values. Only for Type=="object"
	// when used as a map type.
	AdditionalProperties *Schema
	// Ref is a $ref pointer into the components.schemas section. When set,
	// the emitter emits a $ref object and ignores the other fields.
	Ref string
	// Enum, when non-empty, restricts the value to a fixed set.
	Enum []any
	// Default is the default value, if any.
	Default any
	// Example is an example value, if any.
	Example any
	// Nullable is true when the value may be null (Go: a pointer or interface).
	// In OpenAPI 3.0.3 this is emitted as `"nullable": true`. In 3.1.0 it is
	// expressed by adding "null" to TypeArray.
	Nullable bool
	// Extensions is a map of x-stdocs-* and similar extension fields. The
	// keys are emitted as-is (e.g. "x-stdocs-type"); values must be
	// JSON-serializable.
	Extensions map[string]any
}

// reflector builds a set of named component schemas and a primary schema
// for the value being reflected. The mutex protects the seen map when
// reflectSchema is called concurrently (it is not expected to be, but
// the API surface is).
type reflector struct {
	version version.SpecVersion
	mu      sync.Mutex
	seen    map[reflect.Type]string // type -> component schema name
	counter int
	out     map[string]*Schema // accumulated components
}

// ReflectSchema produces a JSON Schema for the given Go type. The version
// controls how Nullable is serialized (3.0.3 emits nullable:true; 3.1.0
// uses type arrays). Named structs are emitted as $ref pointers into a
// components.schemas map that is returned alongside the primary schema.
//
// The zero value of value is fine — only the type is used, not the value
// itself.
func ReflectSchema(value any, v version.SpecVersion) (*Schema, map[string]*Schema) {
	t := reflect.TypeOf(value)
	if t == nil {
		// value is an untyped nil. The most honest schema is {} (anything).
		return &Schema{}, nil
	}
	r := &reflector{
		version: v,
		seen:    make(map[reflect.Type]string),
		out:     make(map[string]*Schema),
	}
	s := r.reflect(t, nil)
	return s, r.out
}

// reflect is the recursive worker. parent is the parent segment for
// path-tracking (currently unused but reserved for future context-aware
// emission of names like "ParentChild").
func (r *reflector) reflect(t reflect.Type, _ []int) *Schema {
	// Unwrap pointers, but mark nullable.
	nullable := false
	for t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}

	// Channels and functions: not representable in JSON. Return nil so the
	// caller (struct field loop) skips the field.
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return nil
	}

	// If we have a named type and we have seen it before, return a $ref.
	if t.Name() != "" {
		if name, ok := r.seen[t]; ok {
			return &Schema{Ref: "#/components/schemas/" + name}
		}
	}

	// time.Time gets a fixed string/date-time schema. It is a struct but
	// its representation is always the RFC3339 string.
	if t == reflect.TypeFor[time.Time]() {
		return r.primitiveString("date-time", nullable)
	}

	switch t.Kind() {
	case reflect.String:
		return r.primitiveString("", nullable)
	case reflect.Bool:
		s := &Schema{Type: "boolean", Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		s := &Schema{Type: "integer", Format: "int32", Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Int64:
		s := &Schema{Type: "integer", Format: "int64", Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		s := &Schema{Type: "integer", Format: "int32", Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Uint64:
		s := &Schema{Type: "integer", Format: "int64", Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Float32:
		s := &Schema{Type: "number", Format: "float", Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Float64:
		s := &Schema{Type: "number", Format: "double", Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Complex64, reflect.Complex128:
		// JSON has no representation for complex numbers. We emit a string
		// and tag it so users know.
		s := &Schema{Type: "string", Format: "", Nullable: nullable, Extensions: map[string]any{
			"x-stdocs-type": "complex",
		}}
		applyVersion(s, r.version)
		return s
	case reflect.Slice, reflect.Array:
		// []byte is base64-encoded string in Go's encoding/json.
		if t.Elem().Kind() == reflect.Uint8 {
			s := &Schema{Type: "string", Format: "byte", Nullable: nullable}
			applyVersion(s, r.version)
			return s
		}
		items := r.reflect(t.Elem(), nil)
		s := &Schema{Type: "array", Items: items, Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Map:
		// Map keys in Go's encoding/json must be strings or implement
		// TextMarshaler. We assume string keys.
		addl := r.reflect(t.Elem(), nil)
		s := &Schema{Type: "object", AdditionalProperties: addl, Nullable: nullable}
		applyVersion(s, r.version)
		return s
	case reflect.Struct:
		return r.reflectStruct(t, nullable)
	case reflect.Interface:
		// any, error, named interfaces: emit {} with an extension noting
		// the kind.
		s := &Schema{Nullable: nullable, Extensions: map[string]any{
			"x-stdocs-type": "interface",
		}}
		applyVersion(s, r.version)
		return s
	}

	// Fallback: empty schema, marked as unknown.
	s := &Schema{Extensions: map[string]any{"x-stdocs-type": "unknown:" + t.Kind().String()}}
	applyVersion(s, r.version)
	return s
}

// primitiveString builds a string-typed schema, optionally with a format.
func (r *reflector) primitiveString(format string, nullable bool) *Schema {
	s := &Schema{Type: "string", Format: format, Nullable: nullable}
	applyVersion(s, r.version)
	return s
}

// reflectStruct emits an object schema for a struct type, recursing into
// fields. Named structs are registered as components and the returned
// schema is a $ref. Anonymous structs are inlined.
func (r *reflector) reflectStruct(t reflect.Type, nullable bool) *Schema {
	if t.Name() == "" {
		// Anonymous struct. Inline.
		return r.buildStructSchema(t, "", nullable)
	}
	// Named struct: register and return a ref. The build happens here
	// (not lazily) so that the component is always populated.
	name := t.Name()
	r.seen[t] = name
	s := r.buildStructSchema(t, name, nullable)
	r.out[name] = s
	ref := &Schema{Ref: "#/components/schemas/" + name, Nullable: nullable}
	applyVersion(ref, r.version)
	return ref
}

// buildStructSchema walks the fields of a struct and produces the
// object schema. For a named struct, name is set and the result is
// also stored in r.out by the caller. For an anonymous struct, name
// is empty and the schema is inlined.
func (r *reflector) buildStructSchema(t reflect.Type, name string, nullable bool) *Schema {
	s := &Schema{Type: "object", Nullable: nullable, Properties: make(map[string]*Schema)}
	if name != "" {
		s.Description = "Generated from Go type " + t.String() + "."
	}

	var required []string

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		// Skip unexported fields: encoding/json does, we do the same.
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name, opts := parseJSONTag(tag)
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		// Channels and functions: skip.
		ft := f.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Chan || ft.Kind() == reflect.Func || ft.Kind() == reflect.UnsafePointer {
			continue
		}
		// Build the field schema.
		fieldSchema := r.reflect(f.Type, []int{i})
		if fieldSchema == nil {
			continue
		}
		// Apply the doc tag if present.
		if doc := f.Tag.Get("doc"); doc != "" {
			fieldSchema.Description = doc
		} else if desc := f.Tag.Get("description"); desc != "" {
			fieldSchema.Description = desc
		}
		// Apply the example tag if present.
		if ex := f.Tag.Get("example"); ex != "" {
			if fieldSchema.Example == nil {
				fieldSchema.Example = ex
			}
		}
		// For embedded structs, the field is a struct; we want to inline
		// its properties into the parent.
		if f.Anonymous {
			// Resolve a possible $ref to the actual object.
			inner := fieldSchema
			if inner.Ref != "" {
				// Look up the named component we already built.
				resolved := r.resolveRef(inner.Ref)
				if resolved != nil {
					inner = resolved
				}
			}
			if inner.Type == "object" && len(inner.Properties) > 0 {
				for k, v := range inner.Properties {
					if _, exists := s.Properties[k]; !exists {
						s.Properties[k] = v
					}
				}
				required = append(required, inner.Required...)
				continue
			}
		}
		s.Properties[name] = fieldSchema
		// Required unless the json tag has "omitempty" and the field is
		// not a pointer (Go's json:omitempty semantics for "required"
		// do not exist, so we approximate: a pointer field is optional,
		// a value field with omitempty is optional, everything else is
		// required).
		if !slices.Contains(opts, "omitempty") && f.Type.Kind() != reflect.Ptr {
			required = append(required, name)
		}
	}
	s.Required = required
	applyVersion(s, r.version)
	return s
}

// resolveRef returns the named component schema for a $ref pointer that
// points into our own components map. Returns nil if not found.
func (r *reflector) resolveRef(ref string) *Schema {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return nil
	}
	return r.out[ref[len(prefix):]]
}

// applyVersion adjusts schema fields that differ between 3.0.3 and 3.1.0.
// Specifically, Nullable=true is rendered as type-array in 3.1.0.
func applyVersion(s *Schema, v version.SpecVersion) {
	if s == nil {
		return
	}
	if s.Nullable && v == version.OpenAPI31 {
		base := s.Type
		if base == "" {
			// For schemas with no type (e.g. interfaces), there is no
			// base to combine with "null". Leave the type empty and
			// rely on the extension to convey the meaning.
			return
		}
		s.TypeArray = []string{base, "null"}
		s.Type = "" // 3.1 prefers TypeArray; Type is mutually exclusive
	}
}

// parseJSONTag splits a struct tag's json value into the field name and
// the comma-separated options (e.g. "omitempty").
func parseJSONTag(tag string) (name string, opts []string) {
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	for _, p := range parts[1:] {
		opts = append(opts, p)
	}
	return name, opts
}

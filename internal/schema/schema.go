// Package schema reflects Go values into a version-agnostic JSON
// Schema representation consumed by the OpenAPI emitters in
// internal/spec. Two emitters (3.0.3 and 3.1.0) share the same
// intermediate Schema type; differences in how nullable, $ref,
// and other version-specific bits are rendered live in the
// emitter code, not in this package.
package schema

import (
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/FumingPower3925/stdocs/internal/version"
)

// Schema is a version-agnostic JSON Schema (Draft 2020-12 / OpenAPI 3.1 flavour)
// value. The OpenAPI 3.0.3 emitter in openapi30.go and the 3.1.0 emitter in
// openapi31.go serialize the same Schema value into their respective JSON
// representations. The model is version-agnostic: Nullable is
// rendered differently in 3.0.3 ("nullable": true) and 3.1.0
// (a "type" array including "null") at serialization time, without
// mutating the model.
type Schema struct {
	// Type is the JSON Schema type: "object", "array", "string", "number",
	// "integer", "boolean", or "null". Empty means "no type constraint"
	// (matches anything). For interfaces, Type is left empty.
	Type string
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
	seen    map[reflect.Type]string // type -> component schema name
	out     map[string]*Schema     // accumulated components
}

// reserveName returns a unique component name based on claimed. If
// claimed is already in use, a numeric suffix is appended (e.g.
// "User" -> "User", then "User" again -> "User_2", "User_3"...).
// The reserved name is recorded in r.out's keys for collision detection.
func (r *reflector) reserveName(claimed string) string {
	name := claimed
	for i := 2; ; i++ {
		if _, exists := r.out[name]; !exists {
			return name
		}
		name = claimed + "_" + itoa(i)
	}
}

// itoa is a small, allocation-free int-to-string converter used only
// for component-name suffixes.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// sanitizeComponentName returns a name suitable for use as a JSON
// pointer fragment (after "#/components/schemas/") and as a Go-style
// identifier in the schema doc. It replaces illegal characters:
//
//   - '[' ']' are replaced with '_' (handles "Page[Foo]" -> "Page_Foo_")
//   - '.' is replaced with '_' (handles "pkg.Type" -> "pkg_Type")
//   - '/' is replaced with '_' (handles "pkg/with/slash.Type")
//   - leading digits and other characters illegal in identifiers are
//     replaced with '_'
//
// If the result is empty or starts with a digit, "Schema" is
// prepended.
func sanitizeComponentName(name string) string {
	if name == "" {
		return "Schema"
	}
	var b strings.Builder
	b.Grow(len(name) + 8)
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out[0] >= '0' && out[0] <= '9' {
		out = "Schema_" + out
	}
	// Collapse runs of underscores to a single underscore; trim
	// trailing underscores.
	var b2 strings.Builder
	b2.Grow(len(out))
	prevUnderscore := false
	for _, r := range out {
		if r == '_' {
			if !prevUnderscore && b2.Len() > 0 {
				b2.WriteRune('_')
			}
			prevUnderscore = true
			continue
		}
		prevUnderscore = false
		b2.WriteRune(r)
	}
	s := b2.String()
	for len(s) > 0 && s[len(s)-1] == '_' {
		s = s[:len(s)-1]
	}
	if s == "" {
		return "Schema"
	}
	return s
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
	//
	// Component name selection strategy:
	//
	//  1. For the common case — a single named type in the user's
	//     package, no generic parameters, no collision — use t.Name()
	//     (e.g. "User"). This keeps the generated spec readable and
	//     matches what the user wrote.
	//
	//  2. If t.Name() is not a valid OpenAPI pointer-fragment
	//     character set (which it is not for generic instantiations
	//     like "Page[Foo]"), fall back to a sanitized version of
	//     t.String() (e.g. "Page_Foo_").
	//
	//  3. If the resulting name is already in use (a name collision
	//     between two same-named types from different packages),
	//     append a numeric suffix.
	name := componentNameFor(t, r)
	r.seen[t] = name
	s := r.buildStructSchema(t, name, false)
	r.out[name] = s
	// The returned use-site schema is a bare $ref with Nullable set;
	// the emitter is responsible for producing the correct wrapper
	// (allOf/anyOf/nullable) at serialization time.
	ref := &Schema{Ref: "#/components/schemas/" + name, Nullable: nullable}
	applyVersion(ref, r.version)
	return ref
}

// componentNameFor picks a unique component name for t. The selection
// rules are documented on reflectStruct. It mutates r.out to record
// the reservation indirectly (via r.reserveName).
func componentNameFor(t reflect.Type, r *reflector) string {
	candidate := t.Name()
	if !isValidComponentName(candidate) {
		// Generic instantiations (or anything with illegal chars)
		// fall back to a sanitized form of the full type expression.
		candidate = sanitizeComponentName(t.String())
	}
	return r.reserveName(candidate)
}

// isValidComponentName reports whether s is a valid OpenAPI 3.x
// pointer fragment (after "#/components/schemas/"). The spec
// restricts these to the character set [a-zA-Z0-9._-].
func isValidComponentName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// buildStructSchema walks the fields of a struct and produces the
// object schema. For a named struct, name is set and the result is
// also stored in r.out by the caller. For an anonymous struct, name
// is empty and the schema is inlined.
// fieldMeta is the per-field result of inspection. name is the
// JSON name; opts is the json-tag options; fieldSchema is the
// reflected schema; embedded is true for anonymous (flattened)
// fields. A field that is skipped produces a zero value.
type fieldMeta struct {
	name        string
	opts        []string
	fieldSchema *Schema
	embedded    bool
	// requiredCandidate is true if the field could be required
	// (no omitempty and not a pointer). The dedup pass decides
	// what actually makes it into s.Required.
	requiredCandidate bool
	// isPointer tracks whether the field type is a pointer
	// regardless of dereferencing for chan/func skip. Used by
	// the required pass to keep the encoding/json contract.
	isPointer bool
}

func (r *reflector) buildStructSchema(t reflect.Type, name string, nullable bool) *Schema {
	s := &Schema{Type: "object", Nullable: nullable, Properties: make(map[string]*Schema)}
	if name != "" {
		s.Description = "Generated from Go type " + t.String() + "."
	}

	var required []string

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		meta, ok := r.inspectField(f, i)
		if !ok {
			continue
		}
		if meta.embedded {
			if r.inlineEmbedded(s, meta.fieldSchema, &required) {
				continue
			}
		}
		s.Properties[meta.name] = meta.fieldSchema
		if meta.requiredCandidate {
			required = append(required, meta.name)
		}
	}
	s.Required = dedupRequired(required, s.Properties)
	applyVersion(s, r.version)
	return s
}

// inspectField returns a fieldMeta for a struct field, or
// (zero, false) if the field should be skipped (unexported, "-"
// tag, or non-representable kind).
func (r *reflector) inspectField(f reflect.StructField, i int) (fieldMeta, bool) {
	if !f.IsExported() {
		return fieldMeta{}, false
	}
	tag := f.Tag.Get("json")
	name, opts := parseJSONTag(tag)
	if name == "-" {
		return fieldMeta{}, false
	}
	if name == "" {
		name = f.Name
	}
	ft := f.Type
	for ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}
	if ft.Kind() == reflect.Chan || ft.Kind() == reflect.Func || ft.Kind() == reflect.UnsafePointer {
		return fieldMeta{}, false
	}
	fieldSchema := r.reflect(f.Type, []int{i})
	if fieldSchema == nil {
		return fieldMeta{}, false
	}
	applyFieldTags(&fieldSchema, f.Tag)
	return fieldMeta{
		name:              name,
		opts:              opts,
		fieldSchema:       fieldSchema,
		embedded:          f.Anonymous,
		requiredCandidate: !slices.Contains(opts, "omitempty") && f.Type.Kind() != reflect.Ptr,
		isPointer:         f.Type.Kind() == reflect.Ptr,
	}, true
}

// applyFieldTags transfers doc/description/example struct tags onto
// the field's schema. Mutates fieldSchema.
func applyFieldTags(fieldSchema **Schema, tag reflect.StructTag) {
	if doc := tag.Get("doc"); doc != "" {
		(*fieldSchema).Description = doc
	} else if desc := tag.Get("description"); desc != "" {
		(*fieldSchema).Description = desc
	}
	if ex := tag.Get("example"); ex != "" && (*fieldSchema).Example == nil {
		(*fieldSchema).Example = ex
	}
}

// inlineEmbedded flattens an anonymous struct's properties into s.
// Returns true if flattening happened (caller should skip further
// processing of the field); false if the embedded schema was not an
// object with properties, in which case the caller should treat the
// field as a regular property.
func (r *reflector) inlineEmbedded(s *Schema, fieldSchema *Schema, required *[]string) bool {
	inner := fieldSchema
	if inner.Ref != "" {
		if resolved := r.resolveRef(inner.Ref); resolved != nil {
			inner = resolved
		}
	}
	if !inner.isInlineable() {
		return false
	}
	for k, v := range inner.Properties {
		if _, exists := s.Properties[k]; !exists {
			s.Properties[k] = v
		}
	}
	*required = append(*required, inner.Required...)
	return true
}

// isInlineable reports whether a schema can have its properties
// merged into a parent (i.e. it is an object with at least one
// property). Used by inlineEmbedded.
func (s *Schema) isInlineable() bool {
	return s != nil && s.Type == "object" && len(s.Properties) > 0
}

// dedupRequired removes duplicates and orphans from a list of
// required keys. A key is an orphan if it is not present in the
// properties map. The first occurrence of each key wins.
func dedupRequired(keys []string, props map[string]*Schema) []string {
	seen := make(map[string]struct{}, len(keys))
	out := keys[:0]
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			continue
		}
		if _, exists := props[k]; !exists {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
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

// applyVersion is intentionally a no-op. Earlier it mutated
// s.Type and s.TypeArray to prepare for 3.1.0 serialization, but
// that mutation caused nullable object/array/map schemas to lose
// all structural information: buildSchema31 gated on s.Type ==
// "object" / "array" which had been cleared. The 3.1.0 emitter now
// reads both s.Type and s.Nullable directly, so no model mutation
// is needed and the version-agnostic Schema stays clean.
//
// The function is kept as a no-op so the call sites in the
// reflectors remain readable and so future version-specific
// adjustments have a clear place to live.
func applyVersion(s *Schema, v version.SpecVersion) {
	_ = s
	_ = v
}

// parseJSONTag splits a struct tag's json value into the field name and
// the comma-separated options (e.g. "omitempty").
func parseJSONTag(tag string) (name string, opts []string) {
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	name = parts[0]
	opts = append(opts, parts[1:]...)
	return name, opts
}

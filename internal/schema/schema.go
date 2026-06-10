// Package schema reflects Go values into a version-agnostic JSON
// Schema representation consumed by the OpenAPI emitters in
// internal/spec. The three emitters (3.0, 3.1, and 3.2) share the
// same intermediate Schema type; differences in how nullable, $ref,
// and other version-specific bits are rendered live in the emitter
// code, not in this package.
package schema

import (
	"encoding"
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Schema is a version-agnostic JSON Schema (Draft 2020-12 / OpenAPI 3.1 flavour)
// value. The OpenAPI emitters serialize the same Schema value into their
// respective JSON representations. The model is version-agnostic: Nullable
// is rendered differently in 3.0 ("nullable": true) and 3.1/3.2 (a "type"
// array including "null") at serialization time, without mutating the model.
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
	// the emitter emits a $ref object (wrapped when Nullable, Description,
	// or Example are also set) and ignores the structural fields.
	Ref string
	// Enum, when non-empty, restricts the value to a fixed set.
	Enum []any
	// Default is the default value, if any.
	Default any
	// Example is an example value, if any.
	Example any
	// Nullable is true when the value may be null (Go: a pointer or interface).
	// In OpenAPI 3.0 this is emitted as `"nullable": true`. In 3.1/3.2 it is
	// expressed by adding "null" to the type array.
	Nullable bool
	// Extensions is a map of x-stdocs-* and similar extension fields. The
	// keys are emitted as-is (e.g. "x-stdocs-type"); values must be
	// JSON-serializable.
	Extensions map[string]any
}

// Reflector accumulates named component schemas across multiple Reflect
// calls. A single Reflector must be used for one OpenAPI document build so
// that component-name collisions between same-named types from different
// packages are detected and suffixed (User, User_2, ...) consistently with
// the $ref strings handed out at use sites.
//
// A Reflector is not safe for concurrent use.
//
// The Schema model is version-agnostic (the emitters render Nullable and
// $ref wrappers per OpenAPI version), so a Reflector needs no version.
type Reflector struct {
	seen map[reflect.Type]string // type -> component schema name
	out  map[string]*Schema      // accumulated components
}

// NewReflector returns an empty Reflector.
func NewReflector() *Reflector {
	return &Reflector{
		seen: make(map[reflect.Type]string),
		out:  make(map[string]*Schema),
	}
}

// Reflect produces a JSON Schema for the dynamic type of value. Named
// structs are registered as components on the Reflector and returned as
// $ref schemas. The zero value of value is fine — only the type is used.
func (r *Reflector) Reflect(value any) *Schema {
	t := reflect.TypeOf(value)
	if t == nil {
		// value is an untyped nil. The most honest schema is {} (anything).
		return &Schema{}
	}
	return r.reflect(t)
}

// Components returns the accumulated named component schemas. The map is
// the Reflector's own; callers must not mutate it while still reflecting.
func (r *Reflector) Components() map[string]*Schema {
	return r.out
}

// ReflectSchema produces a JSON Schema for the given Go value plus the
// named components it references. It is a convenience wrapper around a
// single-use Reflector; for whole-document builds use NewReflector so all
// values share one component namespace.
func ReflectSchema(value any) (*Schema, map[string]*Schema) {
	r := NewReflector()
	s := r.Reflect(value)
	return s, r.out
}

// reserveName returns a unique component name based on claimed. If
// claimed is already in use, a numeric suffix is appended (e.g.
// "User" -> "User", then "User" again -> "User_2", "User_3"...).
func (r *Reflector) reserveName(claimed string) string {
	name := claimed
	for i := 2; ; i++ {
		if _, exists := r.out[name]; !exists {
			return name
		}
		name = claimed + "_" + strconv.Itoa(i)
	}
}

// sanitizeComponentName returns a name suitable for use as a JSON
// pointer fragment (after "#/components/schemas/"). Characters outside
// [a-zA-Z0-9_] are replaced with '_' (handling generic instantiations
// like "Page[pkg.Foo]"), runs of underscores are collapsed, and a name
// that would start with a digit gains a "Schema_" prefix.
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

var (
	timeType          = reflect.TypeFor[time.Time]()
	rawMessageType    = reflect.TypeFor[json.RawMessage]()
	jsonMarshalerType = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// implementsAsValueOrPointer reports whether t or *t implements iface.
// encoding/json consults pointer-receiver method sets for addressable
// values, so for documentation purposes we treat both the same.
func implementsAsValueOrPointer(t reflect.Type, iface reflect.Type) bool {
	return t.Implements(iface) || reflect.PointerTo(t).Implements(iface)
}

// reflect is the recursive worker.
func (r *Reflector) reflect(t reflect.Type) *Schema {
	// Unwrap pointers, but mark nullable.
	nullable := false
	for t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}

	// Channels and functions: not representable in JSON. Return nil so the
	// caller (struct field loop) skips the field, matching encoding/json.
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return nil
	}

	// If we have a named type and we have seen it before, return a $ref
	// carrying the use site's nullability.
	if t.Name() != "" {
		if name, ok := r.seen[t]; ok {
			return &Schema{Ref: "#/components/schemas/" + name, Nullable: nullable}
		}
	}

	// Types with custom JSON representations. time.Time first (it
	// implements json.Marshaler but has a well-known shape), then
	// json.RawMessage (inline raw JSON: any shape), then arbitrary
	// json.Marshaler implementors (shape unknowable from the type), then
	// encoding.TextMarshaler implementors (always a JSON string).
	switch {
	case t == timeType:
		return &Schema{Type: "string", Format: "date-time", Nullable: nullable}
	case t == rawMessageType:
		return &Schema{Nullable: nullable, Extensions: map[string]any{
			"x-stdocs-type": "json.RawMessage",
		}}
	case implementsAsValueOrPointer(t, jsonMarshalerType):
		return &Schema{Nullable: nullable, Extensions: map[string]any{
			"x-stdocs-type": "custom-marshaler:" + t.String(),
		}}
	case implementsAsValueOrPointer(t, textMarshalerType):
		return &Schema{Type: "string", Nullable: nullable}
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string", Nullable: nullable}
	case reflect.Bool:
		return &Schema{Type: "boolean", Nullable: nullable}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return &Schema{Type: "integer", Format: "int32", Nullable: nullable}
	case reflect.Int64:
		return &Schema{Type: "integer", Format: "int64", Nullable: nullable}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return &Schema{Type: "integer", Format: "int32", Nullable: nullable}
	case reflect.Uint64:
		return &Schema{Type: "integer", Format: "int64", Nullable: nullable}
	case reflect.Float32:
		return &Schema{Type: "number", Format: "float", Nullable: nullable}
	case reflect.Float64:
		return &Schema{Type: "number", Format: "double", Nullable: nullable}
	case reflect.Complex64, reflect.Complex128:
		// JSON has no representation for complex numbers. We emit a string
		// and tag it so users know.
		return &Schema{Type: "string", Nullable: nullable, Extensions: map[string]any{
			"x-stdocs-type": "complex",
		}}
	case reflect.Slice:
		// []byte (and other uint8 slices) are base64-encoded strings in
		// Go's encoding/json. Byte ARRAYS are not — see the Array case.
		if t.Elem().Kind() == reflect.Uint8 {
			return &Schema{Type: "string", Format: "byte", Nullable: nullable}
		}
		return &Schema{Type: "array", Items: r.reflect(t.Elem()), Nullable: nullable}
	case reflect.Array:
		// encoding/json marshals [N]byte as an array of numbers, not
		// base64 (only slices get the base64 treatment).
		return &Schema{Type: "array", Items: r.reflect(t.Elem()), Nullable: nullable}
	case reflect.Map:
		// Map keys in Go's encoding/json must be strings, integers, or
		// implement TextMarshaler; they are always emitted as JSON object
		// keys (strings).
		return &Schema{Type: "object", AdditionalProperties: r.reflect(t.Elem()), Nullable: nullable}
	case reflect.Struct:
		return r.reflectStruct(t, nullable)
	case reflect.Interface:
		// any, error, named interfaces: emit {} with an extension noting
		// the kind.
		return &Schema{Nullable: nullable, Extensions: map[string]any{
			"x-stdocs-type": "interface",
		}}
	}

	// Fallback: empty schema, marked as unknown.
	return &Schema{Extensions: map[string]any{"x-stdocs-type": "unknown:" + t.Kind().String()}}
}

// reflectStruct emits an object schema for a struct type, recursing into
// fields. Named structs are registered as components and the returned
// schema is a $ref. Anonymous structs are inlined.
func (r *Reflector) reflectStruct(t reflect.Type, nullable bool) *Schema {
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
	//     t.String() (e.g. "Page_Foo").
	//
	//  3. If the resulting name is already in use (a name collision
	//     between two same-named types from different packages),
	//     append a numeric suffix.
	name := r.componentNameFor(t)
	r.seen[t] = name
	s := r.buildStructSchema(t, name, false)
	r.out[name] = s
	// The returned use-site schema is a bare $ref with Nullable set;
	// the emitter is responsible for producing the correct wrapper
	// (allOf/anyOf/nullable) at serialization time.
	return &Schema{Ref: "#/components/schemas/" + name, Nullable: nullable}
}

// componentNameFor picks a unique component name for t. The selection
// rules are documented on reflectStruct.
func (r *Reflector) componentNameFor(t reflect.Type) string {
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

// fieldMeta is the per-field result of inspection: the JSON name, the
// reflected schema, whether the field is flattened into the parent
// (untagged embedded), and whether it is a required candidate.
type fieldMeta struct {
	name        string
	fieldSchema *Schema
	embedded    bool
	// requiredCandidate is true if the field could be required
	// (no omitempty/omitzero and not a pointer). The dedup pass
	// decides what actually makes it into s.Required.
	requiredCandidate bool
}

func (r *Reflector) buildStructSchema(t reflect.Type, name string, nullable bool) *Schema {
	s := &Schema{Type: "object", Nullable: nullable, Properties: make(map[string]*Schema)}
	if name != "" {
		s.Description = "Generated from Go type " + t.String() + "."
	}

	var required []string

	for i := range t.NumField() {
		f := t.Field(i)
		meta, ok := r.inspectField(f)
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
	return s
}

// inspectField returns a fieldMeta for a struct field, or
// (zero, false) if the field should be skipped (unexported, "-"
// tag, or non-representable kind).
func (r *Reflector) inspectField(f reflect.StructField) (fieldMeta, bool) {
	tag := f.Tag.Get("json")
	tagName, opts := parseJSONTag(tag)
	if !f.IsExported() {
		// encoding/json promotes the exported fields of an embedded
		// unexported struct type (but not of an embedded unexported
		// pointer type); everything else unexported is skipped.
		if f.Anonymous && f.Type.Kind() == reflect.Struct && tagName == "" {
			inner := r.buildStructSchema(f.Type, "", false)
			return fieldMeta{fieldSchema: inner, embedded: true}, true
		}
		return fieldMeta{}, false
	}
	if tagName == "-" {
		return fieldMeta{}, false
	}
	name := tagName
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
	fieldSchema := r.reflect(f.Type)
	if fieldSchema == nil {
		return fieldMeta{}, false
	}
	// The json ",string" option wraps integer, number, boolean, and
	// string values in a JSON string on the wire.
	if slices.Contains(opts, "string") {
		switch fieldSchema.Type {
		case "integer", "number", "boolean":
			fieldSchema = &Schema{
				Type:     "string",
				Nullable: fieldSchema.Nullable,
				Extensions: map[string]any{
					"x-stdocs-type": "json-string-encoded " + fieldSchema.Type,
				},
			}
		}
	}
	applyFieldTags(fieldSchema, f.Tag)
	return fieldMeta{
		name:        name,
		fieldSchema: fieldSchema,
		// encoding/json only flattens anonymous fields whose json tag
		// has no name; `Inner `json:"inner"`` marshals as a nested
		// object under "inner".
		embedded: f.Anonymous && tagName == "",
		requiredCandidate: !slices.Contains(opts, "omitempty") &&
			!slices.Contains(opts, "omitzero") &&
			f.Type.Kind() != reflect.Ptr,
	}, true
}

// applyFieldTags transfers doc/description/example struct tags onto
// the field's schema. Mutates fieldSchema.
func applyFieldTags(fieldSchema *Schema, tag reflect.StructTag) {
	if doc := tag.Get("doc"); doc != "" {
		fieldSchema.Description = doc
	} else if desc := tag.Get("description"); desc != "" {
		fieldSchema.Description = desc
	}
	if ex := tag.Get("example"); ex != "" && fieldSchema.Example == nil {
		fieldSchema.Example = ex
	}
}

// inlineEmbedded flattens an anonymous struct's properties into s.
// Returns true if flattening happened (caller should skip further
// processing of the field); false if the embedded schema was not an
// object with properties, in which case the caller should treat the
// field as a regular property.
func (r *Reflector) inlineEmbedded(s *Schema, fieldSchema *Schema, required *[]string) bool {
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
func (r *Reflector) resolveRef(ref string) *Schema {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return nil
	}
	return r.out[ref[len(prefix):]]
}

// parseJSONTag splits a struct tag's json value into the field name and
// the comma-separated options (e.g. "omitempty").
func parseJSONTag(tag string) (name string, opts []string) {
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	return parts[0], parts[1:]
}

package schema

import (
	"reflect"
	"strconv"
)

// ParamField is one parameter reflected from a params struct; see
// ParamFields.
type ParamField struct {
	// Name is the wire name, taken from the location tag's value.
	Name string
	// In is the parameter location: "query", "header", or "cookie".
	In string
	// Required is the value of the field's required tag (default
	// false — query, header, and cookie parameters are optional by
	// convention).
	Required bool
	// Description is the field's doc/description tag.
	Description string
	// Schema is the parameter's schema, with constraint tags applied.
	Schema *Schema
}

// paramLocations are the struct tags that place a field in a request,
// in the order they are searched.
var paramLocations = [...]string{"query", "header", "cookie"}

// ParamFields reflects a flat struct into parameter declarations.
// Every exported field must carry exactly one location tag (query,
// header, or cookie) whose value is the wire name; a value of "-"
// skips the field. The field's Go type provides the schema (scalars
// and slices of scalars only), the constraint tag vocabulary applies
// as on body fields, and a required:"true" tag marks the parameter
// required. Violations panic, consistent with the module's fail-fast
// policy: a parameter that silently failed to apply would publish a
// wrong contract.
func ParamFields(v any) []ParamField {
	t := reflect.TypeOf(v)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		panic("stdocs: WithParams requires a struct value")
	}
	r := NewReflector()
	out := make([]ParamField, 0, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, in := paramLocation(f)
		if name == "-" {
			continue
		}
		var fs *Schema
		switch override := f.Tag.Get("openapi"); override {
		case "":
			fs = r.reflect(f.Type)
		case "-":
			continue
		default:
			fs = overrideSchema(override, f.Name)
		}
		if fs == nil {
			panic("stdocs: params field " + f.Name + " has a type that cannot be represented in JSON")
		}
		if !paramSchemaOK(fs) {
			panic("stdocs: params field " + f.Name + " must be a scalar or a slice of scalars")
		}
		// A pointer field means "optional" for a parameter, which is
		// already the default; null is not a parameter concept.
		fs.Nullable = false
		applyFieldTags(fs, f.Tag, f.Name)
		pf := ParamField{Name: name, In: in, Description: fs.Description, Schema: fs}
		// The description belongs to the parameter object, not its
		// schema.
		fs.Description = ""
		if req, ok := f.Tag.Lookup("required"); ok {
			b, err := strconv.ParseBool(req)
			if err != nil {
				panic("stdocs: required tag " + strconv.Quote(req) + " on field " + f.Name + " is not a valid boolean")
			}
			pf.Required = b
		}
		out = append(out, pf)
	}
	return out
}

// paramLocation finds the field's single location tag and returns its
// value and the location. Missing, empty, or multiple location tags
// panic.
func paramLocation(f reflect.StructField) (name, in string) {
	for _, loc := range paramLocations {
		v, ok := f.Tag.Lookup(loc)
		if !ok {
			continue
		}
		if in != "" {
			panic("stdocs: params field " + f.Name + " has both " + in + " and " + loc + " tags; use one")
		}
		if v == "" {
			panic("stdocs: params field " + f.Name + " has an empty " + loc + " tag; the value is the parameter name")
		}
		name, in = v, loc
	}
	if in == "" {
		panic("stdocs: params field " + f.Name + ` has no location tag; tag it query:"name", header:"name", or cookie:"name" (or "-" to skip)`)
	}
	return name, in
}

// paramSchemaOK reports whether s can be a parameter schema: a scalar
// or an array of scalars, never an object or a $ref.
func paramSchemaOK(s *Schema) bool {
	if s.Ref != "" || s.Type == "object" {
		return false
	}
	if s.Type == "array" && s.Items != nil {
		return s.Items.Ref == "" && s.Items.Type != "object" && s.Items.Type != "array"
	}
	return true
}

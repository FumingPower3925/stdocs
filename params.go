package stdocs

import (
	"encoding/json"
	"math"
	"reflect"
	"strconv"

	"github.com/FumingPower3925/stdocs/internal/schema"
)

// ParamOpt refines one parameter declared with WithParam, QueryParam,
// HeaderParam, or CookieParam:
//
//	stdocs.QueryParam("limit", "integer", "Page size",
//	    stdocs.ParamDefault(20),
//	    stdocs.ParamMinimum(1),
//	    stdocs.ParamMaximum(100),
//	)
//
// Values are validated against the parameter's declared type at
// registration time; a mismatch (ParamDefault("x") on an integer
// parameter) panics.
//
// The modifier set mirrors the constraint-tag vocabulary; the same
// declarations are also available as a [WithParams] struct.
type ParamOpt func(p *Param)

// ItemOpt refines the elements of an "array" parameter. Item options
// are passed to [ParamItems], which owns the element schema:
//
//	stdocs.QueryParam("severity", "array", "Repeated severity filter",
//	    stdocs.ParamItems("string",
//	        stdocs.ItemEnum("info", "low", "medium", "high", "critical"),
//	    ),
//	)
//
// emits schema.items.enum, documenting ?severity=high&severity=low. The
// set mirrors the scalar [ParamOpt] modifiers that describe a value's
// shape; values are validated against the declared element type at
// registration time, and a mismatch (ItemMinLength on integer elements)
// panics.
//
// There is deliberately no ItemDefault or ItemExample: a default or an
// example is a value for the parameter, not for one of its elements.
type ItemOpt func(it *paramItems)

// paramItems is the element view of an array parameter handed to
// [ItemOpt] modifiers: the element schema plus the owning parameter's
// name for panic messages. It is unexported — like route behind
// [RouteOpt] — so the item-option set stays closed: every element
// constraint stdocs supports has a constructor in this file.
type paramItems struct {
	name   string
	schema *schema.Schema
}

// ParamRequired marks the parameter required. Path parameters are
// always required; query, header, and cookie parameters default to
// optional.
func ParamRequired() ParamOpt {
	return func(p *Param) { p.Required = true }
}

// ParamDefault documents the parameter's default value.
func ParamDefault(v any) ParamOpt {
	return func(p *Param) { p.Schema.Default = paramValue(v, p, "ParamDefault") }
}

// ParamExample documents an example value for the parameter.
func ParamExample(v any) ParamOpt {
	return func(p *Param) { p.Schema.Example = paramValue(v, p, "ParamExample") }
}

// ParamEnum restricts the parameter to a fixed set of values.
func ParamEnum(values ...any) ParamOpt {
	return func(p *Param) {
		enum := make([]any, len(values))
		for i, v := range values {
			enum[i] = paramValue(v, p, "ParamEnum")
		}
		p.Schema.Enum = enum
	}
}

// ParamMinimum documents the parameter's inclusive lower bound.
// Numeric parameters only.
func ParamMinimum(n float64) ParamOpt {
	return func(p *Param) {
		requireNumericParam(p, "ParamMinimum")
		p.Schema.Minimum = finiteNumber(n, p.Name, "ParamMinimum")
	}
}

// ParamMaximum documents the parameter's inclusive upper bound.
// Numeric parameters only.
func ParamMaximum(n float64) ParamOpt {
	return func(p *Param) {
		requireNumericParam(p, "ParamMaximum")
		p.Schema.Maximum = finiteNumber(n, p.Name, "ParamMaximum")
	}
}

// finiteNumber renders a bound as a JSON number; NaN and infinities
// have no JSON representation and panic. It takes the parameter's name
// rather than the Param so the element options, which carry the owning
// parameter's name, can reuse it.
func finiteNumber(n float64, name, what string) json.Number {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		panic("stdocs: " + what + " value for parameter " + strconv.Quote(name) + " must be finite")
	}
	return json.Number(strconv.FormatFloat(n, 'f', -1, 64))
}

// ParamMinLength documents the parameter's minimum string length.
// String parameters only.
func ParamMinLength(n uint64) ParamOpt {
	return func(p *Param) {
		requireStringParam(p, "ParamMinLength")
		p.Schema.MinLength = &n
	}
}

// ParamMaxLength documents the parameter's maximum string length.
// String parameters only.
func ParamMaxLength(n uint64) ParamOpt {
	return func(p *Param) {
		requireStringParam(p, "ParamMaxLength")
		p.Schema.MaxLength = &n
	}
}

// ParamPattern documents an ECMA-262 regular expression the parameter
// must match. String parameters only.
func ParamPattern(pattern string) ParamOpt {
	return func(p *Param) {
		requireStringParam(p, "ParamPattern")
		p.Schema.Pattern = pattern
	}
}

// ParamFormat overrides the parameter's format hint (e.g. "uuid",
// "date-time", "email"). An array parameter has no format of its own —
// its elements do — so use [ItemFormat] inside [ParamItems] for those.
func ParamFormat(format string) ParamOpt {
	return func(p *Param) {
		if p.Schema.Type == "array" {
			panic("stdocs: ParamFormat is not supported on array parameter " + strconv.Quote(p.Name) +
				`; format the elements instead: ParamItems("string", ItemFormat(...))`)
		}
		p.Schema.Format = format
	}
}

// ParamExclusiveMinimum documents the parameter's exclusive lower
// bound. Numeric parameters only; mutually exclusive with
// ParamMinimum.
func ParamExclusiveMinimum(n float64) ParamOpt {
	return func(p *Param) {
		requireNumericParam(p, "ParamExclusiveMinimum")
		if p.Schema.Minimum != "" {
			panic("stdocs: parameter " + strconv.Quote(p.Name) + " sets both ParamMinimum and ParamExclusiveMinimum; use one")
		}
		p.Schema.ExclusiveMinimum = finiteNumber(n, p.Name, "ParamExclusiveMinimum")
	}
}

// ParamExclusiveMaximum documents the parameter's exclusive upper
// bound. Numeric parameters only; mutually exclusive with
// ParamMaximum.
func ParamExclusiveMaximum(n float64) ParamOpt {
	return func(p *Param) {
		requireNumericParam(p, "ParamExclusiveMaximum")
		if p.Schema.Maximum != "" {
			panic("stdocs: parameter " + strconv.Quote(p.Name) + " sets both ParamMaximum and ParamExclusiveMaximum; use one")
		}
		p.Schema.ExclusiveMaximum = finiteNumber(n, p.Name, "ParamExclusiveMaximum")
	}
}

// ParamMinItems documents the array parameter's minimum length.
func ParamMinItems(n uint64) ParamOpt {
	return func(p *Param) {
		requireArrayParam(p, "ParamMinItems")
		p.Schema.MinItems = &n
	}
}

// ParamMaxItems documents the array parameter's maximum length.
func ParamMaxItems(n uint64) ParamOpt {
	return func(p *Param) {
		requireArrayParam(p, "ParamMaxItems")
		p.Schema.MaxItems = &n
	}
}

// ParamUniqueItems requires the array parameter's elements to be
// unique.
func ParamUniqueItems() ParamOpt {
	return func(p *Param) {
		requireArrayParam(p, "ParamUniqueItems")
		p.Schema.UniqueItems = true
	}
}

// ParamItems sets the element type of an "array" parameter, which
// otherwise defaults to string items, and refines the elements with
// [ItemOpt] modifiers. typ is one of "string", "integer", "number", or
// "boolean".
//
//	stdocs.QueryParam("severity", "array", "Repeated severity filter",
//	    stdocs.ParamItems("string", stdocs.ItemEnum("info", "low")))
//
// The element options nest here because ParamItems owns the element
// schema: a sibling option could not survive a later ParamItems
// replacing it. For the same reason, declaring ParamItems twice on one
// parameter panics when the earlier call carried element options.
func ParamItems(typ string, opts ...ItemOpt) ParamOpt {
	return func(p *Param) {
		requireArrayParam(p, "ParamItems")
		switch typ {
		case "string", "integer", "number", "boolean":
		default:
			panic("stdocs: ParamItems type must be \"string\", \"integer\", \"number\", or \"boolean\"; got " + strconv.Quote(typ))
		}
		if itemsConstrained(p.Schema.Items) {
			panic("stdocs: parameter " + strconv.Quote(p.Name) + " declares ParamItems more than once;" +
				" the later call replaces the element schema and would discard the earlier element options")
		}
		// Refine before installing, so the guards run against the
		// element type just declared and nothing observes a half-built
		// element schema.
		it := &paramItems{name: p.Name, schema: schemaForType(typ)}
		for _, o := range opts {
			if o != nil {
				o(it)
			}
		}
		p.Schema.Items = it.schema
	}
}

// itemsConstrained reports whether an element schema carries anything
// beyond its type — i.e. whether replacing it would silently drop an
// element option. A bare schemaForType result (the default installed by
// WithParam, or a previous option-free ParamItems) is not constrained,
// so re-declaring the element type alone stays the harmless no-op it
// has always been.
func itemsConstrained(s *schema.Schema) bool {
	if s == nil {
		return false
	}
	return len(s.Enum) > 0 || s.Format != "" || s.Pattern != "" ||
		s.MinLength != nil || s.MaxLength != nil ||
		s.Minimum != "" || s.Maximum != "" ||
		s.ExclusiveMinimum != "" || s.ExclusiveMaximum != ""
}

// requireStringItems panics unless the array parameter's elements are
// string typed.
func requireStringItems(it *paramItems, what string) {
	if it.schema.Type != "string" {
		panic("stdocs: " + what + " requires string items; parameter " +
			strconv.Quote(it.name) + " has " + describeSchemaType(it.schema) + " items")
	}
}

// requireNumericItems panics unless the array parameter's elements are
// integer or number typed.
func requireNumericItems(it *paramItems, what string) {
	if it.schema.Type != "integer" && it.schema.Type != "number" {
		panic("stdocs: " + what + " requires numeric items; parameter " +
			strconv.Quote(it.name) + " has " + describeSchemaType(it.schema) + " items")
	}
}

// ItemEnum restricts the array parameter's elements to a fixed set of
// values, emitting schema.items.enum.
func ItemEnum(values ...any) ItemOpt {
	return func(it *paramItems) {
		enum := make([]any, len(values))
		for i, v := range values {
			enum[i] = schemaValue(v, it.schema, it.name, "ItemEnum")
		}
		it.schema.Enum = enum
	}
}

// ItemFormat sets the elements' format hint (e.g. "uuid", "date-time",
// "email").
func ItemFormat(format string) ItemOpt {
	return func(it *paramItems) { it.schema.Format = format }
}

// ItemPattern documents an ECMA-262 regular expression every element
// must match. String elements only.
func ItemPattern(pattern string) ItemOpt {
	return func(it *paramItems) {
		requireStringItems(it, "ItemPattern")
		it.schema.Pattern = pattern
	}
}

// ItemMinLength documents the elements' minimum string length. String
// elements only.
func ItemMinLength(n uint64) ItemOpt {
	return func(it *paramItems) {
		requireStringItems(it, "ItemMinLength")
		it.schema.MinLength = &n
	}
}

// ItemMaxLength documents the elements' maximum string length. String
// elements only.
func ItemMaxLength(n uint64) ItemOpt {
	return func(it *paramItems) {
		requireStringItems(it, "ItemMaxLength")
		it.schema.MaxLength = &n
	}
}

// ItemMinimum documents the elements' inclusive lower bound. Numeric
// elements only.
func ItemMinimum(n float64) ItemOpt {
	return func(it *paramItems) {
		requireNumericItems(it, "ItemMinimum")
		it.schema.Minimum = finiteNumber(n, it.name, "ItemMinimum")
	}
}

// ItemMaximum documents the elements' inclusive upper bound. Numeric
// elements only.
func ItemMaximum(n float64) ItemOpt {
	return func(it *paramItems) {
		requireNumericItems(it, "ItemMaximum")
		it.schema.Maximum = finiteNumber(n, it.name, "ItemMaximum")
	}
}

// ItemExclusiveMinimum documents the elements' exclusive lower bound.
// Numeric elements only; mutually exclusive with ItemMinimum.
func ItemExclusiveMinimum(n float64) ItemOpt {
	return func(it *paramItems) {
		requireNumericItems(it, "ItemExclusiveMinimum")
		if it.schema.Minimum != "" {
			panic("stdocs: parameter " + strconv.Quote(it.name) +
				" sets both ItemMinimum and ItemExclusiveMinimum; use one")
		}
		it.schema.ExclusiveMinimum = finiteNumber(n, it.name, "ItemExclusiveMinimum")
	}
}

// ItemExclusiveMaximum documents the elements' exclusive upper bound.
// Numeric elements only; mutually exclusive with ItemMaximum.
func ItemExclusiveMaximum(n float64) ItemOpt {
	return func(it *paramItems) {
		requireNumericItems(it, "ItemExclusiveMaximum")
		if it.schema.Maximum != "" {
			panic("stdocs: parameter " + strconv.Quote(it.name) +
				" sets both ItemMaximum and ItemExclusiveMaximum; use one")
		}
		it.schema.ExclusiveMaximum = finiteNumber(n, it.name, "ItemExclusiveMaximum")
	}
}

// WithParams declares query, header, and cookie parameters by
// reflecting a struct, sharing the body fields' tag vocabulary:
//
//	type ListParams struct {
//	    Cursor string `query:"cursor" doc:"Opaque pagination cursor"`
//	    Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
//	    Trace  string `header:"X-Trace-Id" doc:"Propagated trace id"`
//	}
//
//	mux.HandleFunc("GET /tasks", listTasks, stdocs.WithParams(ListParams{}))
//
// Every exported field carries exactly one location tag — query,
// header, or cookie — whose value is the parameter name ("-" skips
// the field). The Go type provides the schema (scalars and slices of
// scalars), the constraint tags apply as on body fields, and
// required:"true" marks a parameter required. Embedded structs are
// not flattened — tag them query:"-" or declare their fields
// directly. Violations panic at registration time. Multiple
// WithParams and WithParam declarations accumulate, but a duplicate
// (name, location) pair across them panics at document build.
func WithParams(v any) RouteOpt {
	fields := schema.ParamFields(v)
	return func(r *route) {
		for _, f := range fields {
			r.op.Parameters = append(r.op.Parameters, Param{
				Name:        f.Name,
				In:          f.In,
				Required:    f.Required,
				Description: f.Description,
				Schema:      f.Schema,
			})
		}
	}
}

// paramValue validates v against the parameter's schema type and
// returns its canonical form (int64, float64, bool, or string).
func paramValue(v any, p *Param, what string) any {
	return schemaValue(v, p.Schema, p.Name, what)
}

// schemaValue validates v against s's type and returns its canonical
// form (int64, float64, bool, or string). name is the parameter's name:
// the element options carry the owning parameter's name, so every
// message still names something the caller wrote.
func schemaValue(v any, s *schema.Schema, name, what string) any {
	rv := reflect.ValueOf(v)
	switch s.Type {
	case "integer":
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rv.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u := rv.Uint()
			if u > math.MaxInt64 {
				panic("stdocs: " + what + " value for parameter " + strconv.Quote(name) + " exceeds the int64 range")
			}
			return int64(u)
		}
	case "number":
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			f := rv.Float()
			if math.IsNaN(f) || math.IsInf(f, 0) {
				panic("stdocs: " + what + " value for parameter " + strconv.Quote(name) + " must be finite")
			}
			return f
		}
	case "boolean":
		if rv.Kind() == reflect.Bool {
			return rv.Bool()
		}
	case "string":
		if rv.Kind() == reflect.String {
			return rv.String()
		}
	default:
		panic("stdocs: " + what + " is not supported on " + describeSchemaType(s) +
			" parameter " + strconv.Quote(name) + arrayValueHint(s, what))
	}
	panic("stdocs: " + what + " value for parameter " + strconv.Quote(name) + " does not match its " + s.Type + " type")
}

// arrayValueHint points a whole-parameter value option at the element
// form. ParamEnum has one (ItemEnum); ParamDefault and ParamExample
// deliberately do not — a lone value cannot say whether it is the array
// or one element, which is why an array parameter has no default.
func arrayValueHint(s *schema.Schema, what string) string {
	if s == nil || s.Type != "array" {
		return ""
	}
	if what == "ParamEnum" {
		return `; restrict the elements instead: ParamItems("string", ItemEnum(...))`
	}
	return "; a value for an array parameter is ambiguous — the array itself or one element"
}

// requireNumericParam panics unless the parameter is integer or
// number typed.
func requireNumericParam(p *Param, what string) {
	if p.Schema.Type != "integer" && p.Schema.Type != "number" {
		panic("stdocs: " + what + " requires a numeric parameter; " + strconv.Quote(p.Name) + " is " + describeParamType(p))
	}
}

// requireArrayParam panics unless the parameter is array typed.
func requireArrayParam(p *Param, what string) {
	if p.Schema.Type != "array" {
		panic("stdocs: " + what + " requires an array parameter; " + strconv.Quote(p.Name) + " is " + describeParamType(p))
	}
}

// requireStringParam panics unless the parameter is string typed.
func requireStringParam(p *Param, what string) {
	if p.Schema.Type != "string" {
		panic("stdocs: " + what + " requires a string parameter; " + strconv.Quote(p.Name) + " is " + describeParamType(p))
	}
}

// describeParamType renders the parameter's schema type for panic
// messages.
func describeParamType(p *Param) string { return describeSchemaType(p.Schema) }

// describeSchemaType renders a schema's type for panic messages. It
// takes the schema rather than the Param so the element guards, which
// describe an element schema, can reuse it.
func describeSchemaType(s *schema.Schema) string {
	if s == nil || s.Type == "" {
		return "an untyped"
	}
	return s.Type
}

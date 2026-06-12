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
		p.Schema.Minimum = finiteNumber(n, p, "ParamMinimum")
	}
}

// ParamMaximum documents the parameter's inclusive upper bound.
// Numeric parameters only.
func ParamMaximum(n float64) ParamOpt {
	return func(p *Param) {
		requireNumericParam(p, "ParamMaximum")
		p.Schema.Maximum = finiteNumber(n, p, "ParamMaximum")
	}
}

// finiteNumber renders a bound as a JSON number; NaN and infinities
// have no JSON representation and panic.
func finiteNumber(n float64, p *Param, what string) json.Number {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		panic("stdocs: " + what + " value for parameter " + strconv.Quote(p.Name) + " must be finite")
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
// "date-time", "email").
func ParamFormat(format string) ParamOpt {
	return func(p *Param) { p.Schema.Format = format }
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
		p.Schema.ExclusiveMinimum = finiteNumber(n, p, "ParamExclusiveMinimum")
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
		p.Schema.ExclusiveMaximum = finiteNumber(n, p, "ParamExclusiveMaximum")
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
// otherwise defaults to string items. typ is one of "string",
// "integer", "number", or "boolean".
func ParamItems(typ string) ParamOpt {
	return func(p *Param) {
		requireArrayParam(p, "ParamItems")
		switch typ {
		case "string", "integer", "number", "boolean":
		default:
			panic("stdocs: ParamItems type must be \"string\", \"integer\", \"number\", or \"boolean\"; got " + strconv.Quote(typ))
		}
		p.Schema.Items = schemaForType(typ)
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
	rv := reflect.ValueOf(v)
	switch p.Schema.Type {
	case "integer":
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return rv.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			u := rv.Uint()
			if u > math.MaxInt64 {
				panic("stdocs: " + what + " value for parameter " + strconv.Quote(p.Name) + " exceeds the int64 range")
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
				panic("stdocs: " + what + " value for parameter " + strconv.Quote(p.Name) + " must be finite")
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
		panic("stdocs: " + what + " is not supported on " + describeParamType(p) + " parameter " + strconv.Quote(p.Name))
	}
	panic("stdocs: " + what + " value for parameter " + strconv.Quote(p.Name) + " does not match its " + p.Schema.Type + " type")
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
func describeParamType(p *Param) string {
	if p.Schema == nil || p.Schema.Type == "" {
		return "an untyped"
	}
	return p.Schema.Type
}

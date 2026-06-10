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
		p.Schema.Minimum = json.Number(strconv.FormatFloat(n, 'f', -1, 64))
	}
}

// ParamMaximum documents the parameter's inclusive upper bound.
// Numeric parameters only.
func ParamMaximum(n float64) ParamOpt {
	return func(p *Param) {
		requireNumericParam(p, "ParamMaximum")
		p.Schema.Maximum = json.Number(strconv.FormatFloat(n, 'f', -1, 64))
	}
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
// required:"true" marks a parameter required. Violations panic at
// registration time. Multiple WithParams and WithParam declarations
// accumulate.
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
		case reflect.Float32, reflect.Float64:
			return rv.Float()
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

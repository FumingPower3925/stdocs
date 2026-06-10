package stdocs

import (
	"strconv"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/spec"
)

// RouteOpt is a function that mutates a route's metadata. RouteOpt is
// passed as variadic to *stdocs.Mux.HandleFunc and *stdocs.Mux.Handle.
type RouteOpt func(*route)

// Opts combines several route opts into one, enabling reusable
// bundles shared across registrations:
//
//	paginated := stdocs.Opts(
//	    stdocs.QueryParam("cursor", "string", "Opaque cursor"),
//	    stdocs.QueryParam("limit", "integer", "Page size"),
//	)
//	mux.HandleFunc("GET /tasks", listTasks, paginated, stdocs.WithResponse(200, TaskPage{}))
//	mux.HandleFunc("GET /users", listUsers, paginated, stdocs.WithResponse(200, UserPage{}))
//
// Opts applies its opts in order; nil opts are skipped. A bundle is
// stateless and safe to reuse across any number of routes.
func Opts(opts ...RouteOpt) RouteOpt {
	return func(r *route) {
		for _, opt := range opts {
			if opt != nil {
				opt(r)
			}
		}
	}
}

// Summary sets the route's summary (the short single-line description
// shown next to the operation in UIs).
func Summary(s string) RouteOpt {
	return func(r *route) { r.op.Summary = s }
}

// Description sets the route's longer Markdown description.
func Description(s string) RouteOpt {
	return func(r *route) { r.op.Description = s }
}

// Tags sets the route's tags. Multiple Tags calls accumulate.
func Tags(names ...string) RouteOpt {
	return func(r *route) { r.op.Tags = append(r.op.Tags, names...) }
}

// Deprecated marks the route as deprecated.
func Deprecated() RouteOpt {
	return func(r *route) { r.op.Deprecated = true }
}

// Hidden excludes the route from the generated OpenAPI document
// unconditionally, in every environment. Use it for endpoints that
// are not part of any contract (debug hooks, health probes).
//
// Hiding a route only shapes the published documentation — the
// handler still serves traffic. It is NOT access control.
func Hidden() RouteOpt {
	return func(r *route) { r.op.Hidden = true }
}

// Internal marks the route as internal: it is excluded from the
// generated OpenAPI document unless the mux was configured with
// WithInternal(true). When shown, the operation carries an
// "x-internal": true extension, which spec-filtering tools
// (e.g. Redocly) understand.
//
// Like Hidden, this only shapes the published documentation — the
// handler still serves traffic in every environment. It is NOT
// access control.
func Internal() RouteOpt {
	return func(r *route) { r.op.Internal = true }
}

// OperationID sets the operationId. If not set, stdocs auto-derives one
// from the method and path.
func OperationID(id string) RouteOpt {
	return func(r *route) { r.op.OperationID = id }
}

// WithBody sets the route's request body. body is a zero value of the
// type to reflect; its type is used to build the JSON Schema when the
// spec document is assembled. Struct fields may carry doc: (or
// description:) and example: tags, plus the constraint tag
// vocabulary — minimum, maximum, exclusiveMinimum, exclusiveMaximum,
// minLength, maxLength, pattern, minItems, maxItems, uniqueItems,
// enum, default, and format. Values are parsed according to the
// field type (enum:"1,2,3" on an int emits numbers) and validated
// against it; a misapplied or unparseable constraint panics at
// document-build time. The default content type is application/json
// (override with WithBodyContentType).
//
// Mark the body as not required with Optional().
func WithBody(body any) RouteOpt {
	return func(r *route) {
		rb := ensureRequestBody(r.op)
		rb.BodyValue = body
	}
}

// ensureRequestBody returns op's request body, creating a default one
// (required, application/json) if none exists yet. Having a single
// creation point makes WithBody, Optional, and WithBodyContentType
// order-independent.
func ensureRequestBody(op *Operation) *RequestBody {
	if op.RequestBody == nil {
		op.RequestBody = &RequestBody{Required: true}
	}
	return op.RequestBody
}

// Optional marks the route's request body as not required. Only
// meaningful when called after WithBody.
func Optional() RouteOpt {
	return func(r *route) {
		if r.op.RequestBody != nil {
			r.op.RequestBody.Required = false
		}
	}
}

// WithResponse adds a response entry. body is a zero value whose type
// is reflected into a JSON Schema when the spec document is assembled
// (struct fields may carry doc:/description:, example:, and the
// constraint tags listed on [WithBody]); pass
// nil if there is no body (e.g. 204 No Content). Pass status 0 to
// declare the OpenAPI "default" response — the catch-all entry
// consumers use for undeclared status codes, conventionally the
// shared error shape. Multiple WithResponse calls accumulate. Calling
// WithResponse twice for the same status replaces the body but keeps
// any description, headers, or example attached via the other
// response opts.
func WithResponse(status int, body any) RouteOpt {
	return func(r *route) {
		resp := ensureResponse(r.op, statusKey(status))
		resp.BodyValue = body
	}
}

// ensureResponse returns op's response entry for key, creating one with
// the default description if none exists yet. Having a single creation
// point makes WithResponse, WithResponseDescription, WithResponseHeader,
// and WithResponseExample order-independent.
func ensureResponse(op *Operation, key string) *Response {
	if op.Responses == nil {
		op.Responses = make(map[string]*Response)
	}
	if resp, ok := op.Responses[key]; ok {
		return resp
	}
	resp := &Response{
		Status:      key,
		Description: defaultResponseDescriptionForKey(key),
	}
	op.Responses[key] = resp
	op.ResponseOrder = append(op.ResponseOrder, key)
	return resp
}

// WithSecurity requires the named security scheme on this operation.
// scopes are only meaningful for OAuth2 schemes; pass no scopes for
// non-OAuth schemes.
//
// Use WithNoSecurity to opt out of a globally-applied scheme for one
// route.
//
// Example:
//
//	mux.HandleFunc("GET /me", getUser,
//	    stdocs.WithSecurity("bearerAuth"),
//	)
//
//	mux.HandleFunc("POST /posts", createPost,
//	    stdocs.WithSecurity("oauth2Auth", "write:posts", "read:posts"),
//	)
func WithSecurity(name string, scopes ...string) RouteOpt {
	return func(r *route) {
		if name == "" {
			return
		}
		r.op.Security = append(r.op.Security, SecurityRequirement{name: append([]string{}, scopes...)})
	}
}

// WithNoSecurity opts the operation out of any security requirement.
// This emits an empty `security: []` array at the operation level,
// which the OpenAPI spec defines as overriding a globally-applied
// scheme. Without this (i.e. leaving Security empty), the operation
// inherits the global security requirement.
func WithNoSecurity() RouteOpt {
	return func(r *route) {
		r.op.NoSecurity = true
	}
}

// WithExample adds an example value to the request body if one has
// been declared (via WithBody), otherwise to the most recently
// declared response (via WithResponse). If neither has been declared,
// WithExample is a no-op. To target a specific response regardless of
// declaration order, use WithResponseExample.
//
// The value is encoded as JSON and stored under the OpenAPI "example"
// field of the media type object (content.<type>.example).
//
// Example:
//
//	mux.HandleFunc("POST /users", createUser,
//	    stdocs.WithBody(CreateUserRequest{}),
//	    stdocs.WithResponse(201, User{}),
//	    stdocs.WithExample(CreateUserRequest{Title: "Buy milk"}),
//	)
//
// Subsequent WithExample calls overwrite the previous example.
func WithExample(value any) RouteOpt {
	return func(r *route) {
		encoded, err := encodeExample(value)
		if err != nil {
			return
		}
		if r.op.RequestBody != nil {
			r.op.RequestBody.Example = encoded
			return
		}
		if len(r.op.Responses) == 0 {
			return
		}
		if resp, ok := lastResponse(r.op); ok {
			resp.Example = encoded
		}
	}
}

// WithResponseExample attaches an example to a specific response
// status, creating the response entry if it does not exist yet. The
// order relative to WithResponse does not matter.
//
//	stdocs.WithResponse(200, User{}),
//	stdocs.WithResponseExample(200, User{ID: "u-1", Name: "Alice"}),
func WithResponseExample(status int, value any) RouteOpt {
	return func(r *route) {
		encoded, err := encodeExample(value)
		if err != nil {
			return
		}
		ensureResponse(r.op, statusKey(status)).Example = encoded
	}
}

// WithResponseDescription sets a custom description for a response
// status, creating the response entry if it does not exist yet. The
// order relative to WithResponse does not matter.
//
//	stdocs.WithResponse(200, User{}),
//	stdocs.WithResponseDescription(200, "The user, or 404 if not found"),
func WithResponseDescription(status int, description string) RouteOpt {
	return func(r *route) {
		ensureResponse(r.op, statusKey(status)).Description = description
	}
}

// WithResponseHeader adds a header entry to a response status,
// creating the response entry if it does not exist yet. Useful for
// documenting rate-limit, pagination, or other custom headers. The
// order relative to WithResponse does not matter.
//
//	stdocs.WithResponse(200, User{}),
//	stdocs.WithResponseHeader(200, "X-RateLimit-Remaining", "integer", "Remaining quota"),
func WithResponseHeader(status int, name, typ, description string) RouteOpt {
	return func(r *route) {
		resp := ensureResponse(r.op, statusKey(status))
		if resp.Headers == nil {
			resp.Headers = make(map[string]*schema.Schema)
		}
		resp.Headers[name] = &schema.Schema{
			Type:        typ,
			Description: description,
		}
	}
}

// WithBodyContentType sets the content type for the request body,
// creating the request-body entry if it does not exist yet. The
// default is "application/json". The order relative to WithBody does
// not matter.
//
//	stdocs.WithBody(MyRequest{}),
//	stdocs.WithBodyContentType("application/xml"),
func WithBodyContentType(ct string) RouteOpt {
	return func(r *route) {
		ensureRequestBody(r.op).ContentType = ct
	}
}

// lastResponse returns the most recently added response in r.op. We
// track insertion order via a hidden slice on the Operation.
func lastResponse(op *Operation) (*Response, bool) {
	if op == nil || len(op.Responses) == 0 {
		return nil, false
	}
	if len(op.ResponseOrder) == 0 {
		for _, resp := range op.Responses {
			return resp, true
		}
		return nil, false
	}
	last := op.ResponseOrder[len(op.ResponseOrder)-1]
	return op.Responses[last], true
}

// encodeExample serializes value to JSON for the OpenAPI "example" field.
func encodeExample(value any) (any, error) {
	b, err := spec.MarshalJSON(value)
	if err != nil {
		return nil, err
	}
	var v any
	if err := spec.UnmarshalJSON(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

// WithParam adds a parameter to the route. in is one of "path", "query",
// "header", "cookie". typ is the JSON Schema type name ("string",
// "integer", "number", "boolean", "array"). For arrays, the items are
// also string-typed.
//
// Multiple WithParam calls accumulate.
func WithParam(name, in, typ, description string) RouteOpt {
	return func(r *route) {
		s := schemaForType(typ)
		r.op.Parameters = append(r.op.Parameters, Param{
			Name:        name,
			In:          in,
			Description: description,
			Required:    in == "path",
			Schema:      s,
		})
	}
}

// QueryParam is shorthand for WithParam(name, "query", typ, desc).
func QueryParam(name, typ, desc string) RouteOpt { return WithParam(name, "query", typ, desc) }

// HeaderParam is shorthand for WithParam(name, "header", typ, desc).
func HeaderParam(name, typ, desc string) RouteOpt { return WithParam(name, "header", typ, desc) }

// CookieParam is shorthand for WithParam(name, "cookie", typ, desc).
func CookieParam(name, typ, desc string) RouteOpt { return WithParam(name, "cookie", typ, desc) }

// schemaForType builds a Schema for one of the supported primitive types.
// For arrays, pass "array" and a follow-up element type via []string.
func schemaForType(typ string) *schema.Schema {
	switch typ {
	case "string":
		return &schema.Schema{Type: "string"}
	case "integer":
		return &schema.Schema{Type: "integer"}
	case "number":
		return &schema.Schema{Type: "number"}
	case "boolean":
		return &schema.Schema{Type: "boolean"}
	case "array":
		return &schema.Schema{Type: "array", Items: &schema.Schema{Type: "string"}}
	}
	return &schema.Schema{}
}

// statusKey converts a numeric status code to its string form for the
// OpenAPI "responses" map. "default" is used for 0.
func statusKey(status int) string {
	if status == 0 {
		return "default"
	}
	return itoa(status)
}

// itoa is a small wrapper around strconv.Itoa kept for use by other
// files in this package (e.g. registry.go).
func itoa(n int) string { return strconv.Itoa(n) }

// defaultResponseDescriptionForKey is defaultResponseDescription for an
// already-stringified responses-map key ("200", "default", ...).
func defaultResponseDescriptionForKey(key string) string {
	if key == "default" {
		return "Default response"
	}
	if n, err := strconv.Atoi(key); err == nil {
		return defaultResponseDescription(n)
	}
	return "Response"
}

func defaultResponseDescription(status int) string {
	switch status {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 202:
		return "Accepted"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 409:
		return "Conflict"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	}
	return "Response"
}

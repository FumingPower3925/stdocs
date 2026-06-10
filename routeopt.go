package stdocs

import (
	"strconv"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/spec"
)

// RouteOpt is a function that mutates a route's metadata. RouteOpt is
// passed as variadic to *stdocs.Mux.HandleFunc and *stdocs.Mux.Handle.
type RouteOpt func(*route)

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

// OperationID sets the operationId. If not set, stdocs auto-derives one
// from the method and path.
func OperationID(id string) RouteOpt {
	return func(r *route) { r.op.OperationID = id }
}

// WithBody sets the route's request body. body is a zero value of the
// type to reflect; its type is used to build the JSON Schema. The
// default content type is application/json.
//
// Mark the body as not required with Optional().
func WithBody(body any) RouteOpt {
	return func(r *route) {
		s, _ := schema.ReflectSchema(body, r.version)
		r.op.RequestBody = &RequestBody{
			Required:  true,
			Schema:    s,
			BodyValue: body,
		}
	}
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

// WithResponse adds a response entry. body is a zero value; pass nil
// if there is no body (e.g. 204 No Content). Multiple WithResponse
// calls accumulate.
func WithResponse(status int, body any) RouteOpt {
	return func(r *route) {
		key := statusKey(status)
		desc := defaultResponseDescription(status)
		var s *schema.Schema
		if body != nil {
			s, _ = schema.ReflectSchema(body, r.version)
		}
		if r.op.Responses == nil {
			r.op.Responses = make(map[string]*Response)
		}
		if _, exists := r.op.Responses[key]; !exists {
			r.op.ResponseOrder = append(r.op.ResponseOrder, key)
		}
		r.op.Responses[key] = &Response{
			Status:      key,
			Description: desc,
			Schema:      s,
			BodyValue:   body,
		}
	}
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

// WithExample adds an example value to the most recently declared
// response (via WithResponse) or request body (via WithBody). If neither
// has been declared, WithExample is a no-op.
//
// The value is encoded as JSON and stored under the standard OpenAPI
// "example" field on the response's content schema (or
// "requestBody.content.schema.example" for request bodies).
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
// status. Useful when a route has multiple responses and you want to
// give each a different example.
//
//	stdocs.WithResponse(200, User{}),
//	stdocs.WithResponseExample(200, User{ID: "u-1", Name: "Alice"}),
func WithResponseExample(status int, value any) RouteOpt {
	return func(r *route) {
		key := statusKey(status)
		encoded, err := encodeExample(value)
		if err != nil {
			return
		}
		if r.op.Responses == nil {
			return
		}
		if resp, ok := r.op.Responses[key]; ok {
			resp.Example = encoded
		}
	}
}

// ResponseDescription sets a custom description for a response
// status. Use after WithResponse() to override the default.
//
//	stdocs.WithResponse(200, User{}),
//	stdocs.ResponseDescription(200, "The user, or 404 if not found"),
func ResponseDescription(status int, description string) RouteOpt {
	return func(r *route) {
		key := statusKey(status)
		if r.op.Responses == nil {
			return
		}
		if resp, ok := r.op.Responses[key]; ok {
			resp.Description = description
		}
	}
}

// ResponseHeader adds a header entry to a response status. Useful
// for documenting rate-limit, pagination, or other custom headers.
//
//	stdocs.WithResponse(200, User{}),
//	stdocs.ResponseHeader(200, "X-RateLimit-Remaining", "integer", "Remaining quota"),
func ResponseHeader(status int, name, typ, description string) RouteOpt {
	return func(r *route) {
		key := statusKey(status)
		if r.op.Responses == nil {
			return
		}
		resp, ok := r.op.Responses[key]
		if !ok {
			return
		}
		if resp.Headers == nil {
			resp.Headers = make(map[string]*schema.Schema)
		}
		resp.Headers[name] = &schema.Schema{
			Type:        typ,
			Description: description,
		}
	}
}

// BodyContentType sets the content type for the request body. The
// default is "application/json". Use after WithBody().
//
//	stdocs.WithBody(MyRequest{}),
//	stdocs.BodyContentType("application/xml"),
func BodyContentType(ct string) RouteOpt {
	return func(r *route) {
		if r.op.RequestBody != nil {
			r.op.RequestBody.ContentType = ct
		}
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

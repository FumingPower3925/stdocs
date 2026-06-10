package spec

import (
	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// Info holds the values for the OpenAPI top-level "info" object.
type Info struct {
	Title          string
	Version        string
	Description    string
	TermsOfService string
	Contact        *Contact
	License        *License
}

// Contact is the OpenAPI "contact" object.
type Contact struct {
	Name  string
	URL   string
	Email string
}

// License is the OpenAPI "license" object.
type License struct {
	Name string
	URL  string
}

// Server is the OpenAPI "server" object.
type Server struct {
	URL         string
	Description string
}

// TagDecl declares a top-level tag and its description. Tags attached to
// operations (via stdocs.Tags) are auto-collected; this type is for the
// optional extended description.
type TagDecl struct {
	Name        string
	Description string
}

// Param describes a single OpenAPI parameter (path, query, header, or cookie).
type Param struct {
	Name        string
	In          string // "path", "query", "header", "cookie"
	Required    bool
	Description string
	Schema      *schema.Schema
}

// Response describes one response in the OpenAPI "responses" map.
type Response struct {
	Status      string // e.g. "200", "404", "default"
	Description string
	Schema      *schema.Schema // optional body schema
	Headers     map[string]*schema.Schema
	// Example is the JSON-serialised example value for this response
	// (set via WithExample or WithResponseExample). nil means no
	// example is set. It is not serialized into the spec; the
	// emitter reads it from a parallel field.
	Example any
	// BodyValue is the original Go value passed to WithResponse. It is
	// used internally to re-derive component schemas at spec-build
	// time. It is not serialized into the spec.
	BodyValue any
}

// RequestBody describes the request body of an operation.
type RequestBody struct {
	Description string
	Required    bool
	Schema      *schema.Schema
	ContentType string // defaults to "application/json"
	// Example is the JSON-serialised example value (set via
	// WithExample). nil means no example is set.
	Example any
	// BodyValue is the original Go value passed to WithBody. Used
	// internally to re-derive component schemas at spec-build time.
	BodyValue any
}

// Operation is a single method's worth of metadata for a route.
type Operation struct {
	Method      string
	Summary     string
	Description string
	Tags        []string
	OperationID string
	Deprecated  bool
	Parameters  []Param
	RequestBody *RequestBody
	Responses   map[string]*Response // status -> response
	// Security is the list of security requirements for this
	// operation. Each requirement is a map of scheme name to scopes.
	// An empty Security slice means "use the global security"; to
	// opt out of auth on a specific route, set NoSecurity to true
	// (which emits an empty `security: []` array — required by the
	// OpenAPI spec to override a globally-applied scheme).
	Security []SecurityRequirement
	// NoSecurity, when true, emits an empty `security` array on
	// this operation, overriding any global security requirement.
	// See WithNoSecurity in the root package.
	NoSecurity bool
	// ResponseOrder tracks the order in which responses were added
	// to the Responses map. Used by WithExample to pick the
	// "most recent" response when no explicit status is given.
	ResponseOrder []string
	// Extensions is a map of x-* extension fields. These are
	// emitted directly onto the operation object. The reflector
	// uses this to surface warnings about non-standard method
	// names, custom status codes, etc. The map is never modified
	// by the emitter.
	Extensions map[string]any
}

// PathItem groups operations for a single path across HTTP methods.
// It also captures path-level parameters (e.g. shared path params).
type PathItem struct {
	Path       string
	Operations map[string]*Operation // method -> operation
	Parameters []Param               // path-level, shared across methods
}

// SpecInput is the input to an emitter. It is produced by the registry in
// spec.go; emitters below consume it and produce the OpenAPI JSON bytes.
type SpecInput struct {
	Info            Info
	Servers         []Server
	Tags            []TagDecl
	Paths           []PathItem
	Components      map[string]*schema.Schema
	Version         version.SpecVersion
	SecuritySchemes []NamedSecurityScheme
	GlobalSecurity  []SecurityRequirement
	Webhooks        map[string]Webhook
}

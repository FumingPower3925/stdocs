package stdocs

import (
	"github.com/FumingPower3925/stdocs/internal/spec"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// Public re-exports of spec types. These are type aliases (not new
// types), so values are interchangeable between stdocs.X and the
// internal spec.X. Each alias carries its own doc comment because
// the aliased fields live in an internal package that pkg.go.dev
// does not render.
type (
	// Info is the OpenAPI "info" object. Fields: Title, Version,
	// Description, TermsOfService, Contact (*Contact), License
	// (*License). Title and Version are set via WithTitle and
	// WithAPIVersion.
	Info = spec.Info
	// Contact is the OpenAPI "info.contact" object. Fields: Name,
	// URL, Email. Set via WithContact; empty fields are omitted from
	// the document.
	Contact = spec.Contact
	// License is the OpenAPI "info.license" object. Fields: Name,
	// URL. Set via WithLicense; empty fields are omitted.
	License = spec.License
	// Server is one OpenAPI "servers" entry. Fields: URL,
	// Description. Added via WithServer (the default relative "/"
	// entry stays first).
	Server = spec.Server
	// TagDecl declares a top-level tag. Fields: Name, Description.
	// Added via WithTag.
	TagDecl = spec.TagDecl
	// Param describes one OpenAPI parameter. Fields: Name, In
	// ("path", "query", "header", "cookie"), Required, Description,
	// Schema. Usually created via WithParam / QueryParam /
	// HeaderParam / CookieParam.
	Param = spec.Param
	// Response describes one entry of an operation's "responses"
	// map. Fields: Status, Description, Headers, Example, and
	// BodyValue — a zero value of the Go response type, reflected
	// into a JSON Schema at document-build time. Usually created via
	// WithResponse and decorated with WithResponseDescription /
	// WithResponseHeader / WithResponseExample; constructed directly
	// only for webhooks.
	Response = spec.Response
	// RequestBody describes an operation's request body. Fields:
	// Description, Required, ContentType (defaults to
	// "application/json"), Example, and BodyValue — a zero value of
	// the Go body type, reflected at document-build time. Usually
	// created via WithBody; constructed directly only for webhooks.
	RequestBody = spec.RequestBody
	// Operation is the per-route OpenAPI operation under
	// construction. It is the value RouteOpts mutate; most users
	// never touch it directly.
	Operation = spec.Operation
	// PathItem groups the operations of one path. Produced
	// internally by the registry; exposed for advanced use only.
	PathItem = spec.PathItem
	// SpecInput is the input consumed by the internal emitters.
	// Exposed for advanced use only.
	SpecInput = spec.SpecInput
)

// SpecVersion is the OpenAPI spec version ("3.0.4", "3.1.2", or
// "3.2.0"). Use the OpenAPI30 / OpenAPI31 / OpenAPI32 constants with
// WithVersion.
type SpecVersion = version.SpecVersion

// The version constants accepted by WithVersion: the latest patch of
// each supported OpenAPI minor.
const (
	// OpenAPI30 is the latest 3.0.x patch (3.0.4), the default.
	OpenAPI30 = version.OpenAPI30
	// OpenAPI31 is the latest 3.1.x patch (3.1.2).
	OpenAPI31 = version.OpenAPI31
	// OpenAPI32 is the latest 3.2.x patch (3.2.0).
	OpenAPI32 = version.OpenAPI32
)

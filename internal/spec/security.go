package spec

// SecuritySchemeType identifies the kind of security scheme.
type SecuritySchemeType string

const (
	// SecurityHTTP is an HTTP authentication scheme (bearer, basic, etc.).
	// Set the scheme name (lowercase) via Scheme in SecurityScheme.
	SecurityHTTP SecuritySchemeType = "http"
	// SecurityAPIKey is an API key passed via header, query, or cookie.
	// Set the location via In and the name via Name.
	SecurityAPIKey SecuritySchemeType = "apiKey"
	// SecurityOAuth2 is OAuth 2.0 with one or more flows.
	// Set Flows.
	SecurityOAuth2 SecuritySchemeType = "oauth2"
	// SecurityOpenIDConnect is OpenID Connect discovery.
	// Set OpenIDConnectURL.
	SecurityOpenIDConnect SecuritySchemeType = "openIdConnect"
	// SecurityAPIKeyInHeader means the API key is passed in a header.
	SecurityAPIKeyInHeader = "header"
	// SecurityAPIKeyInQuery means the API key is passed in a query parameter.
	SecurityAPIKeyInQuery = "query"
	// SecurityAPIKeyInCookie means the API key is passed in a cookie.
	SecurityAPIKeyInCookie = "cookie"
)

// SecurityScheme describes one scheme in components.securitySchemes.
type SecurityScheme struct {
	Type             SecuritySchemeType
	Description      string
	Name             string // for apiKey: the parameter name
	In               string // for apiKey: "header" | "query" | "cookie"
	Scheme           string // for http: "bearer" | "basic" | ...
	BearerFormat     string // for http bearer: e.g. "JWT"
	Flows            *OAuthFlows
	OpenIDConnectURL string
}

// OAuthFlows describes the supported OAuth 2.0 flows.
type OAuthFlows struct {
	Implicit          *OAuthFlow
	Password          *OAuthFlow
	ClientCredentials *OAuthFlow
	AuthorizationCode *OAuthFlow
	// DeviceAuthorization is the RFC 8628 device flow added in
	// OpenAPI 3.2. It is emitted for every version (3.0/3.1
	// validators will flag it), so only set it on a 3.2 mux. Its
	// flow object requires DeviceAuthorizationURL and TokenURL.
	DeviceAuthorization *OAuthFlow
}

// OAuthFlow describes one OAuth flow.
type OAuthFlow struct {
	AuthorizationURL string
	TokenURL         string
	RefreshURL       string
	// DeviceAuthorizationURL is required for (and only meaningful
	// on) the OpenAPI 3.2 deviceAuthorization flow.
	DeviceAuthorizationURL string
	Scopes                 map[string]string
}

// SecurityRequirement is a single entry in the operation's "security"
// array: a set of scheme names mapped to the scopes required (empty
// for non-OAuth schemes).
type SecurityRequirement map[string][]string

// NamedSecurityScheme is an internal pair (name, scheme) used when
// constructing the spec from the registry.
type NamedSecurityScheme struct {
	Name   string
	Scheme SecurityScheme
}

// Webhook describes an OpenAPI 3.1 webhook. A webhook is a path-and-
// method pair (like a regular operation) but is documented under
// "webhooks" rather than "paths". Webhooks are emitted for 3.1 and
// 3.2, and ignored when the mux's Version is 3.0.
type Webhook struct {
	// Method is the HTTP method (POST, GET, etc.) used to deliver
	// the webhook payload. Required.
	Method string
	// Summary is a short description.
	Summary string
	// Description is a long description (Markdown).
	Description string
	// OperationID identifies the webhook uniquely within the API.
	OperationID string
	// RequestBody is the payload structure (optional).
	RequestBody *RequestBody
	// Responses lists expected responses (e.g. 200, 202).
	Responses map[string]*Response
	// Security is the webhook operation's own security requirement.
	// Webhook operations never inherit the document-level security:
	// a webhook is the provider calling out, so the receiving
	// endpoint's auth is a different contract — without an explicit
	// requirement here, the emitter writes "security": [] to override
	// any global requirement (which would otherwise make generated
	// clients reference schemes their webhook plumbing does not have).
	Security []SecurityRequirement
}

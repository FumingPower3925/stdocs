package stdocs

import (
	"strings"

	"github.com/FumingPower3925/stdocs/internal/spec"
)

// Public re-exports of security and webhook types so users can write
// stdocs.SecurityScheme, stdocs.Webhook, etc. without importing the
// internal sub-package. These are type aliases (not new types), so
// values are interchangeable between stdocs.X and internal/spec.X.
type (
	// SecuritySchemeType is the "type" of a security scheme:
	// SecurityHTTP, SecurityAPIKey, SecurityOAuth2, or
	// SecurityOpenIDConnect.
	SecuritySchemeType = spec.SecuritySchemeType
	// SecurityScheme describes one components.securitySchemes entry.
	// Set Type plus the type-specific fields: Scheme/BearerFormat
	// for http, In/Name for apiKey, Flows for oauth2, and
	// OpenIDConnectURL for openIdConnect. Description is optional.
	SecurityScheme = spec.SecurityScheme
	// OAuthFlows groups the OAuth 2.0 flows of a scheme: Implicit,
	// Password, ClientCredentials, AuthorizationCode, and (OpenAPI
	// 3.2 only) DeviceAuthorization — each an optional *OAuthFlow.
	OAuthFlows = spec.OAuthFlows
	// OAuthFlow describes one OAuth 2.0 flow. Fields:
	// AuthorizationURL, TokenURL, RefreshURL,
	// DeviceAuthorizationURL (3.2 device flow only), and Scopes
	// (scope name -> description; always emitted, required by the
	// spec).
	OAuthFlow = spec.OAuthFlow
	// SecurityRequirement is one entry of a "security" array: scheme
	// name -> required scopes (empty for non-OAuth schemes).
	SecurityRequirement = spec.SecurityRequirement
	// Webhook describes one OpenAPI 3.1/3.2 webhook. Fields: Method,
	// Summary, Description, OperationID, RequestBody, Responses.
	// Describe payloads by setting BodyValue on the request body or
	// responses; schemas are reflected at document-build time.
	Webhook = spec.Webhook
)

const (
	SecurityHTTP           = spec.SecurityHTTP
	SecurityAPIKey         = spec.SecurityAPIKey
	SecurityOAuth2         = spec.SecurityOAuth2
	SecurityOpenIDConnect  = spec.SecurityOpenIDConnect
	SecurityAPIKeyInHeader = spec.SecurityAPIKeyInHeader
	SecurityAPIKeyInQuery  = spec.SecurityAPIKeyInQuery
	SecurityAPIKeyInCookie = spec.SecurityAPIKeyInCookie
)

// registerSecurityScheme adds a scheme to the config and returns the
// name the caller should use in SecurityRequirement. The name defaults
// to "bearerAuth" / "basicAuth" / "apiKeyAuth" / "oauth2Auth" /
// "openIdConnectAuth" if the user did not supply one.
func (c *Config) registerSecurityScheme(scheme SecurityScheme, name string) string {
	if name == "" {
		switch scheme.Type {
		case SecurityHTTP:
			if scheme.Scheme == "bearer" {
				name = "bearerAuth"
			} else {
				name = strings.ToLower(scheme.Scheme) + "Auth"
			}
		case SecurityAPIKey:
			name = "apiKeyAuth"
		case SecurityOAuth2:
			name = "oauth2Auth"
		case SecurityOpenIDConnect:
			name = "openIdConnectAuth"
		default:
			name = "securityScheme"
		}
	}
	c.Security = append(c.Security, spec.NamedSecurityScheme{Name: name, Scheme: scheme})
	return name
}

// WithSecurityScheme registers a security scheme under the given
// name. The name is the key in the components.securitySchemes
// map and is the value you pass to WithSecurity on a route. If
// name is empty, a sensible default is chosen (e.g. "bearerAuth"
// for HTTP bearer).
//
// Use the chosen name with WithSecurity to require this scheme
// on a route.
func WithSecurityScheme(name string, scheme SecurityScheme) Option {
	return func(c *Config) {
		c.registerSecurityScheme(scheme, name)
	}
}

// WithBearerAuth is a convenience for HTTP bearer authentication.
func WithBearerAuth(name, bearerFormat string) Option {
	return WithSecurityScheme(name, SecurityScheme{
		Type:         SecurityHTTP,
		Scheme:       "bearer",
		BearerFormat: bearerFormat,
	})
}

// WithBasicAuth is a convenience for HTTP basic authentication.
func WithBasicAuth(name string) Option {
	return WithSecurityScheme(name, SecurityScheme{
		Type:   SecurityHTTP,
		Scheme: "basic",
	})
}

// WithAPIKeyAuth is a convenience for API key authentication.
// in is one of stdocs.SecurityAPIKeyInHeader, stdocs.SecurityAPIKeyInQuery,
// stdocs.SecurityAPIKeyInCookie.
func WithAPIKeyAuth(name, in, paramName string) Option {
	return WithSecurityScheme(name, SecurityScheme{
		Type: SecurityAPIKey,
		In:   in,
		Name: paramName,
	})
}

// WithOAuth2Auth is a convenience for OAuth 2.0 with one or more flows.
func WithOAuth2Auth(name string, flows OAuthFlows) Option {
	return WithSecurityScheme(name, SecurityScheme{
		Type:  SecurityOAuth2,
		Flows: &flows,
	})
}

// WithWebhooks registers one or more webhooks under their given names.
// Webhooks require OpenAPI 3.1 or 3.2; on 3.0 they are silently
// ignored (the field does not exist in that version).
//
// Webhook payloads are described the same way route bodies are: set
// RequestBody.BodyValue (or Response.BodyValue) to a zero value of
// the Go type and its JSON Schema is reflected when the document is
// built.
//
// Example:
//
//	type UserPayload struct {
//	    ID string `json:"id"`
//	}
//
//	stdocs.WithWebhooks(map[string]stdocs.Webhook{
//	    "newUser": {
//	        Method:      "POST",
//	        Summary:     "New user created",
//	        RequestBody: &stdocs.RequestBody{Required: true, BodyValue: UserPayload{}},
//	        Responses: map[string]*stdocs.Response{
//	            "200": {Description: "OK"},
//	        },
//	    },
//	})
func WithWebhooks(hooks map[string]Webhook) Option {
	return func(c *Config) {
		if c.Webhooks == nil {
			c.Webhooks = make(map[string]Webhook)
		}
		for name, hook := range hooks {
			c.Webhooks[name] = hook
		}
	}
}

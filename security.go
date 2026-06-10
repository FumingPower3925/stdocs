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
	SecuritySchemeType  = spec.SecuritySchemeType
	SecurityScheme      = spec.SecurityScheme
	OAuthFlows          = spec.OAuthFlows
	OAuthFlow           = spec.OAuthFlow
	SecurityRequirement = spec.SecurityRequirement
	Webhook             = spec.Webhook
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

// WithSecurityScheme registers a security scheme under the given name
// and returns the name. If name is empty, a sensible default is chosen
// (e.g. "bearerAuth" for HTTP bearer).
//
// Use the returned name with stdocs.WithSecurity to require this
// scheme on a route.
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
// Webhooks are 3.1-only; on 3.0.3 they are silently ignored.
//
// Example:
//
//	stdocs.WithWebhooks(map[string]stdocs.Webhook{
//	    "newUser": {
//	        Method:  "POST",
//	        Summary: "New user created",
//	        RequestBody: &stdocs.RequestBody{Schema: ...},
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

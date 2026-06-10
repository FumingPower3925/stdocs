package stdocs

import (
	"net/http"
	"strings"
	"testing"
)

func TestWithBearerAuth_RegistersScheme(t *testing.T) {
	m := New(WithTitle("T"), WithBearerAuth("bearerAuth", "JWT"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	scheme := jget(t, doc, "components", "securitySchemes", "bearerAuth").(map[string]any)
	if scheme["type"] != "http" {
		t.Errorf("type = %v", scheme["type"])
	}
	if scheme["scheme"] != "bearer" {
		t.Errorf("scheme = %v", scheme["scheme"])
	}
	if scheme["bearerFormat"] != "JWT" {
		t.Errorf("bearerFormat = %v", scheme["bearerFormat"])
	}
}

func TestWithBearerAuth_NoFormat(t *testing.T) {
	m := New(WithTitle("T"), WithBearerAuth("bearerAuth", ""))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	scheme := jget(t, doc, "components", "securitySchemes", "bearerAuth").(map[string]any)
	if _, has := scheme["bearerFormat"]; has {
		t.Errorf("bearerFormat should be absent when empty, got %v", scheme["bearerFormat"])
	}
}

func TestWithBasicAuth(t *testing.T) {
	m := New(WithTitle("T"), WithBasicAuth("basicAuth"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	scheme := jget(t, doc, "components", "securitySchemes", "basicAuth").(map[string]any)
	if scheme["scheme"] != "basic" {
		t.Errorf("scheme = %v", scheme["scheme"])
	}
}

func TestWithAPIKeyAuth_Header(t *testing.T) {
	m := New(WithTitle("T"), WithAPIKeyAuth("apiKeyAuth", "header", "X-API-Key"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	scheme := jget(t, doc, "components", "securitySchemes", "apiKeyAuth").(map[string]any)
	if scheme["type"] != "apiKey" {
		t.Errorf("type = %v", scheme["type"])
	}
	if scheme["in"] != "header" {
		t.Errorf("in = %v", scheme["in"])
	}
	if scheme["name"] != "X-API-Key" {
		t.Errorf("name = %v", scheme["name"])
	}
}

func TestWithAPIKeyAuth_Query(t *testing.T) {
	m := New(WithTitle("T"), WithAPIKeyAuth("k", "query", "api_key"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	scheme := jget(t, doc, "components", "securitySchemes", "k").(map[string]any)
	if scheme["in"] != "query" {
		t.Errorf("in = %v", scheme["in"])
	}
}

func TestWithOAuth2Auth(t *testing.T) {
	m := New(WithTitle("T"), WithOAuth2Auth("oauth2", OAuthFlows{
		AuthorizationCode: &OAuthFlow{
			AuthorizationURL: "https://example.com/auth",
			TokenURL:         "https://example.com/token",
			Scopes: map[string]string{
				"read":  "Read access",
				"write": "Write access",
			},
		},
	}))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	scheme := jget(t, doc, "components", "securitySchemes", "oauth2").(map[string]any)
	if scheme["type"] != "oauth2" {
		t.Errorf("type = %v", scheme["type"])
	}
	flows := jget(t, scheme, "flows", "authorizationCode").(map[string]any)
	if flows["authorizationUrl"] != "https://example.com/auth" {
		t.Errorf("authorizationUrl = %v", flows["authorizationUrl"])
	}
	if flows["tokenUrl"] != "https://example.com/token" {
		t.Errorf("tokenUrl = %v", flows["tokenUrl"])
	}
	scopes := jget(t, flows, "scopes").(map[string]any)
	if scopes["read"] != "Read access" {
		t.Errorf("scopes[read] = %v", scopes["read"])
	}
}

func TestWithSecurity_PerRoute(t *testing.T) {
	m := New(WithTitle("T"), WithBearerAuth("bearerAuth", "JWT"))
	m.HandleFunc("GET /public", func(w http.ResponseWriter, r *http.Request) {},
		WithNoSecurity(),
	)
	m.HandleFunc("GET /private", func(w http.ResponseWriter, r *http.Request) {},
		WithSecurity("bearerAuth"),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	// /public has an empty `security: []` array (opt-out).
	pubOp := jget(t, doc, "paths", "/public", "get").(map[string]any)
	secPub, has := pubOp["security"]
	if !has {
		t.Errorf("/public should have a security key (empty array), got nothing")
	}
	arrPub, ok := secPub.([]any)
	if !ok || len(arrPub) != 0 {
		t.Errorf("/public.security = %v, want empty array (opt-out)", secPub)
	}
	// /private has the bearer requirement.
	privOp := jget(t, doc, "paths", "/private", "get").(map[string]any)
	sec := jget(t, privOp, "security").([]any)
	if len(sec) != 1 {
		t.Fatalf("len(security) = %d, want 1", len(sec))
	}
	entry := sec[0].(map[string]any)
	if _, has := entry["bearerAuth"]; !has {
		t.Errorf("entry = %v, want bearerAuth", entry)
	}
}

func TestWithSecurity_OAuthScopes(t *testing.T) {
	m := New(WithTitle("T"), WithOAuth2Auth("oauth2", OAuthFlows{
		ClientCredentials: &OAuthFlow{
			TokenURL: "https://example.com/token",
			Scopes:   map[string]string{"read": "Read"},
		},
	}))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithSecurity("oauth2", "read", "write"),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	sec := jget(t, doc, "paths", "/x", "get", "security").([]any)
	entry := sec[0].(map[string]any)
	scopes := entry["oauth2"].([]any)
	if len(scopes) != 2 {
		t.Errorf("scopes = %v, want [read write]", scopes)
	}
}

func TestWithGlobalSecurity(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithBearerAuth("bearerAuth", "JWT"),
		WithGlobalSecurity("bearerAuth"),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	// Top-level security should be present.
	sec := jget(t, doc, "security").([]any)
	if len(sec) != 1 {
		t.Fatalf("top-level security = %d, want 1", len(sec))
	}
	entry := sec[0].(map[string]any)
	if _, has := entry["bearerAuth"]; !has {
		t.Errorf("entry = %v", entry)
	}
}

func TestWithGlobalSecurity_OverriddenByRoute(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithBearerAuth("bearerAuth", "JWT"),
		WithGlobalSecurity("bearerAuth"),
	)
	m.HandleFunc("GET /public", func(w http.ResponseWriter, r *http.Request) {},
		WithNoSecurity(),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	pubOp := jget(t, doc, "paths", "/public", "get").(map[string]any)
	// WithNoSecurity emits an empty `security: []` array on the
	// operation, overriding the globally-applied bearerAuth. The
	// presence of the key (with an empty array) is what does the
	// override; a missing key would inherit.
	sec, has := pubOp["security"]
	if !has {
		t.Fatalf("/public should have a security key, got nothing")
	}
	arr, ok := sec.([]any)
	if !ok {
		t.Fatalf("/public.security = %T, want []any", sec)
	}
	if len(arr) != 0 {
		t.Errorf("/public.security = %v, want empty array (opt-out)", arr)
	}
}

func TestWithSecurityScheme_Custom(t *testing.T) {
	m := New(WithTitle("T"), WithSecurityScheme("customAuth", SecurityScheme{
		Type:        "apiKey",
		In:          "cookie",
		Name:        "session",
		Description: "Session cookie",
	}))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {})
	b, _ := m.JSON()
	doc := jx(t, b)
	scheme := jget(t, doc, "components", "securitySchemes", "customAuth").(map[string]any)
	if scheme["description"] != "Session cookie" {
		t.Errorf("description = %v", scheme["description"])
	}
	if scheme["in"] != "cookie" {
		t.Errorf("in = %v", scheme["in"])
	}
}

// TestWithSecurity_UnregisteredScheme guards the new validation:
// a security requirement that references a scheme name not
// registered in components.securitySchemes is an invalid spec
// and JSON() should return an error.
func TestWithSecurity_UnregisteredScheme(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithSecurity("missingScheme"),
	)
	_, err := m.JSON()
	if err == nil {
		t.Fatal("expected error for unregistered security scheme, got nil")
	}
	if !strings.Contains(err.Error(), "missingScheme") {
		t.Errorf("error = %q, expected it to mention the missing scheme name", err)
	}
}

// TestWithSecurity_RegisteredScheme passes validation: the scheme
// is in components.securitySchemes, so JSON() succeeds.
func TestWithSecurity_RegisteredScheme(t *testing.T) {
	m := New(WithTitle("T"), WithBearerAuth("bearerAuth", "JWT"))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithSecurity("bearerAuth"),
	)
	_, err := m.JSON()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWithSecurity_MultipleSchemes(t *testing.T) {
	m := New(
		WithTitle("T"),
		WithBearerAuth("bearerAuth", ""),
		WithAPIKeyAuth("apiKeyAuth", "header", "X-Key"),
	)
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		// Operation accepts EITHER auth.
		WithSecurity("bearerAuth"),
		WithSecurity("apiKeyAuth"),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	sec := jget(t, doc, "paths", "/x", "get", "security").([]any)
	if len(sec) != 2 {
		t.Errorf("security entries = %d, want 2", len(sec))
	}
	// Both schemes should be in components.
	comps := jget(t, doc, "components", "securitySchemes").(map[string]any)
	if _, ok := comps["bearerAuth"]; !ok {
		t.Errorf("bearerAuth missing from components")
	}
	if _, ok := comps["apiKeyAuth"]; !ok {
		t.Errorf("apiKeyAuth missing from components")
	}
}

func TestWithSecurity_EmptyNameIgnored(t *testing.T) {
	// Should not panic or add an empty-named requirement.
	m := New(WithTitle("T"), WithBearerAuth("bearerAuth", ""))
	m.HandleFunc("GET /x", func(w http.ResponseWriter, r *http.Request) {},
		WithSecurity(""),
	)
	_, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
}

package stdocs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sub "github.com/FumingPower3925/stdocs/internal/spec"
)

func noop(w http.ResponseWriter, r *http.Request) {}

// Two same-named types from different packages must get distinct
// components with matching $refs at every use site (one shared
// reflector per document build).
func TestCrossRouteComponentCollision(t *testing.T) {
	type User struct { // local "User"
		Name string `json:"name"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("GET /a", noop, WithResponse(200, User{}))
	m.HandleFunc("GET /b", noop, WithResponse(200, sub.Info{})) // spec.Info as a stand-in second package type
	// Force an actual name collision: a second, structurally
	// different "User" from another package.
	m.HandleFunc("GET /c", noop, WithResponse(200, otherUser()))
	b, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	doc := jx(t, b)
	schemas := jget(t, doc, "components", "schemas").(map[string]any)
	if _, ok := schemas["User"]; !ok {
		t.Fatalf("User component missing; have %v", mapKeysAny(schemas))
	}
	if _, ok := schemas["User_2"]; !ok {
		t.Fatalf("User_2 component missing (collision not suffixed); have %v", mapKeysAny(schemas))
	}
	// The /a route's ref and the /c route's ref must point at the
	// schemas that contain their respective fields.
	var refA, refC string
	for path, ref := range map[string]*string{"/a": &refA, "/c": &refC} {
		sch := jget(t, doc, "paths", path, "get", "responses", "200", "content", "application/json", "schema").(map[string]any)
		*ref = strings.TrimPrefix(sch["$ref"].(string), "#/components/schemas/")
	}
	if refA == refC {
		t.Fatalf("routes /a and /c share component %q for different types", refA)
	}
	aProps := jget(t, doc, "components", "schemas", refA, "properties").(map[string]any)
	if _, ok := aProps["name"]; !ok {
		t.Errorf("component %q for /a lacks the local User's `name` property", refA)
	}
}

func mapKeysAny(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Nullability must survive on second and later uses of the same
// named type within one document.
func TestNullableSecondUse(t *testing.T) {
	type Pet struct {
		Name string `json:"name"`
	}
	type Holder struct {
		First  *Pet `json:"first"`
		Second *Pet `json:"second"`
	}
	m := New(WithTitle("T"))
	m.HandleFunc("GET /h", noop, WithResponse(200, Holder{}))
	b, _ := m.JSON()
	doc := jx(t, b)
	props := jget(t, doc, "components", "schemas", "Holder", "properties").(map[string]any)
	for _, field := range []string{"first", "second"} {
		fs := props[field].(map[string]any)
		if fs["nullable"] != true {
			t.Errorf("%s: nullable = %v, want true (allOf wrapper)", field, fs["nullable"])
		}
		if _, ok := fs["allOf"]; !ok {
			t.Errorf("%s: missing allOf ref wrapper: %v", field, fs)
		}
	}
	// The shared component itself must stay non-nullable.
	pet := jget(t, doc, "components", "schemas", "Pet").(map[string]any)
	if _, ok := pet["nullable"]; ok {
		t.Errorf("shared Pet component must not be nullable")
	}
}

// Auto-suffixing must never collide with explicit ids and must be
// stable across Refresh-driven rebuilds.
func TestOperationIDStability(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /one", noop, OperationID("x_2"))
	m.HandleFunc("GET /two", noop, OperationID("x"))
	m.HandleFunc("GET /three", noop, OperationID("x"))
	b1, _ := m.JSON()
	m.Refresh()
	b2, _ := m.JSON()
	if string(b1) != string(b2) {
		t.Fatalf("operation ids drifted across rebuilds:\n%s\n%s", b1, b2)
	}
	doc := jx(t, b1)
	ids := map[string]int{}
	for _, p := range []string{"/one", "/two", "/three"} {
		id := jget(t, doc, "paths", p, "get", "operationId").(string)
		ids[id]++
	}
	for id, n := range ids {
		if n > 1 {
			t.Errorf("operationId %q used %d times; must be unique", id, n)
		}
	}
}

// QUERY is a first-class Path Item key in 3.2 (no warning); custom
// methods go to additionalOperations in 3.2 and to the
// x-stdocs-additionalOperations extension in 3.0/3.1, never to an
// illegal top-level key.
func TestCustomMethods(t *testing.T) {
	make32 := func() *Mux {
		m := New(WithTitle("T"), WithVersion(OpenAPI32))
		m.HandleFunc("QUERY /search", noop)
		m.HandleFunc("PURGE /cache", noop)
		return m
	}
	b, _ := make32().JSON()
	doc := jx(t, b)
	q := jget(t, doc, "paths", "/search", "query").(map[string]any)
	if _, warned := q["x-stdocs-warning"]; warned {
		t.Errorf("QUERY falsely warned on a 3.2 mux")
	}
	if _, ok := jget(t, doc, "paths", "/cache", "additionalOperations").(map[string]any)["PURGE"]; !ok {
		t.Errorf("PURGE not under additionalOperations in 3.2")
	}
	if pi := jget(t, doc, "paths", "/cache").(map[string]any); pi["purge"] != nil {
		t.Errorf("3.2 must not emit a lowercase purge path-item key")
	}

	m30 := New(WithTitle("T"))
	m30.HandleFunc("PURGE /cache", noop)
	b30, _ := m30.JSON()
	doc30 := jx(t, b30)
	pi := jget(t, doc30, "paths", "/cache").(map[string]any)
	if pi["purge"] != nil {
		t.Errorf("3.0 must not emit an illegal purge path-item key")
	}
	ops, ok := pi["x-stdocs-additionalOperations"].(map[string]any)
	if !ok {
		t.Fatalf("3.0 custom method not under x-stdocs-additionalOperations: %v", pi)
	}
	if _, ok := ops["PURGE"]; !ok {
		t.Errorf("PURGE missing from extension: %v", ops)
	}
}

// Webhook request bodies declared via BodyValue must be reflected
// into real schemas — never serialized as "schema": null.
func TestWebhookBodyReflected(t *testing.T) {
	type Payload struct {
		Event string `json:"event"`
	}
	m := New(WithTitle("T"), WithVersion(OpenAPI31), WithWebhooks(map[string]Webhook{
		"newUser": {
			Method:      "POST",
			Summary:     "New user created",
			RequestBody: &RequestBody{Required: true, BodyValue: Payload{}},
			Responses:   map[string]*Response{"200": {Description: "OK"}},
		},
	}))
	m.HandleFunc("GET /x", noop)
	b, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"schema":null`) {
		t.Fatalf("webhook body emitted schema:null: %s", b)
	}
	doc := jx(t, b)
	sch := jget(t, doc, "webhooks", "newUser", "post", "requestBody", "content", "application/json", "schema").(map[string]any)
	if sch["$ref"] != "#/components/schemas/Payload" {
		t.Errorf("webhook body schema = %v, want Payload ref", sch)
	}
	if _, ok := jget(t, doc, "components", "schemas", "Payload").(map[string]any); !ok {
		t.Errorf("Payload component missing")
	}
}

// $self plumbing: emitted for 3.2 via WithSelfURL, absent for 3.0/3.1.
func TestSelfURLPlumbing(t *testing.T) {
	const self = "https://api.example.com/openapi.json"
	for _, v := range []SpecVersion{OpenAPI30, OpenAPI31, OpenAPI32} {
		m := New(WithTitle("T"), WithVersion(v), WithSelfURL(self))
		m.HandleFunc("GET /x", noop)
		b, _ := m.JSON()
		has := strings.Contains(string(b), `"$self"`)
		if v == OpenAPI32 && !has {
			t.Errorf("%s: $self missing", v)
		}
		if v != OpenAPI32 && has {
			t.Errorf("%s: $self must not be emitted", v)
		}
	}
	for _, bad := range []string{"https://x/doc.json#frag", "not a uri"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("WithSelfURL(%q) should panic", bad)
				}
			}()
			New(WithSelfURL(bad))
		}()
	}
}

// A typo in WithGlobalSecurity must error like per-route typos do.
func TestGlobalSecurityValidated(t *testing.T) {
	m := New(WithTitle("T"), WithGlobalSecurity("nosuchscheme"))
	m.HandleFunc("GET /x", noop)
	if _, err := m.JSON(); err == nil {
		t.Fatal("expected error for unregistered global security scheme")
	}
}

// An explicit per-call bool wins over WithDisabled in both directions.
func TestDocsOverridesDisabled(t *testing.T) {
	m := New(WithTitle("T"), WithDisabled(true))
	m.HandleFunc("GET /x", noop)
	rr := httptest.NewRecorder()
	m.Docs(true).ServeHTTP(rr, httptest.NewRequest("GET", "/docs/", nil))
	if rr.Code != 200 {
		t.Errorf("Docs(true) on a disabled mux = %d, want 200", rr.Code)
	}
	m2 := New(WithTitle("T"))
	rr2 := httptest.NewRecorder()
	m2.Docs(false).ServeHTTP(rr2, httptest.NewRequest("GET", "/docs/", nil))
	if rr2.Code != 404 {
		t.Errorf("Docs(false) on an enabled mux = %d, want 404", rr2.Code)
	}
}

// Mount is idempotent and its exact routes cannot be shadowed by
// user wildcard routes under the prefix.
func TestMountIdempotentAndUnshadowable(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /docs/{file}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("SHADOWED"))
	})
	m.Mount()
	m.Mount() // second call must not panic
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, httptest.NewRequest("GET", "/docs/openapi.json", nil))
	if strings.Contains(rr.Body.String(), "SHADOWED") {
		t.Fatalf("user wildcard shadowed the spec endpoint")
	}
	if !strings.Contains(rr.Body.String(), `"openapi"`) {
		t.Errorf("spec endpoint did not serve the spec: %s", rr.Body.String())
	}
}

// Routes registered under the docs prefix (the docs page itself,
// embedded UI asset handlers) must not appear in the generated spec.
func TestDocsPrefixRoutesExcludedFromSpec(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /users", noop)
	m.Handle("GET /docs/", m.Docs(false))
	m.Handle("GET /docs/_assets/", http.NotFoundHandler())
	b, _ := m.JSON()
	doc := jx(t, b)
	paths := jget(t, doc, "paths").(map[string]any)
	for p := range paths {
		if strings.HasPrefix(p, "/docs") {
			t.Errorf("docs route %q leaked into the spec", p)
		}
	}
	if _, ok := paths["/users"]; !ok {
		t.Errorf("real route missing from spec")
	}
}

// WithDocsPrefix("/") must fail fast instead of producing a broken
// Mount pattern; trailing slashes and missing leading slashes
// normalize.
func TestDocsPrefixValidation(t *testing.T) {
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf(`WithDocsPrefix("/") should panic`)
			}
		}()
		New(WithDocsPrefix("/"))
	}()
	m := New(WithDocsPrefix("apidocs/"))
	if m.cfg.DocsPrefix != "/apidocs" {
		t.Errorf("normalized prefix = %q, want /apidocs", m.cfg.DocsPrefix)
	}
	if New(WithDocsPrefix("")).cfg.DocsPrefix != "/docs" {
		t.Errorf("empty prefix should fall back to /docs")
	}
}

// Closures and method values must not produce garbage summaries.
func TestSummaryInferenceGarbage(t *testing.T) {
	for name, want := range map[string]string{
		"main.main.func1":                  "",
		"main.main.func1.2":                "",
		"github.com/x/y.(*svc).GetUser-fm": "Get user",
		"github.com/x/y.HandleUsers":       "Users",
	} {
		if got := summaryFromFuncName(name); got != want {
			t.Errorf("summaryFromFuncName(%q) = %q, want %q", name, got, want)
		}
	}
}

// Order-independence of the response/body decoration opts.
func TestRouteOptOrderIndependence(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /x", noop,
		WithResponseDescription(404, "Custom not found"),
		WithResponseHeader(200, "X-Rate", "integer", "remaining"),
		WithBodyContentType("application/xml"),
		WithBody(struct {
			A string `json:"a"`
		}{}),
		WithResponse(404, nil),
		WithResponse(200, struct {
			B string `json:"b"`
		}{}),
	)
	b, _ := m.JSON()
	doc := jx(t, b)
	desc := jget(t, doc, "paths", "/x", "get", "responses", "404", "description").(string)
	if desc != "Custom not found" {
		t.Errorf("404 description = %q (order-dependence regression)", desc)
	}
	if _, ok := jget(t, doc, "paths", "/x", "get", "responses", "200", "headers", "X-Rate").(map[string]any); !ok {
		t.Errorf("200 X-Rate header missing (order-dependence regression)")
	}
	if _, ok := jget(t, doc, "paths", "/x", "get", "requestBody", "content", "application/xml").(map[string]any); !ok {
		t.Errorf("request body content type lost (order-dependence regression)")
	}
}

// otherUser returns a value of a distinct type that is also named
// "User" (function-scoped types carry just their local name), so it
// collides with the test's package-level User in the component
// namespace.
func otherUser() any {
	type User struct {
		Email string `json:"email"`
	}
	return User{}
}

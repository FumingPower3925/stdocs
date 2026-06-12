package stdocs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	sub "github.com/FumingPower3925/stdocs/internal/spec"
)

func noop(w http.ResponseWriter, r *http.Request) {}

// buildDocMap builds the mux's document and decodes it for assertions.
func buildDocMap(t *testing.T, m *Mux) map[string]any {
	t.Helper()
	raw, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

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

// Hidden routes never appear in the document; Internal routes appear
// only under WithInternal(true), carrying x-internal: true. Excluded
// routes leave no trace: no components, no paths, no operation-id
// effects — and they still serve traffic.
func TestHiddenAndInternalRoutes(t *testing.T) {
	type Secret struct {
		Key string `json:"key"`
	}
	type Probe struct {
		OK bool `json:"ok"`
	}
	build := func(showInternal bool) (*Mux, map[string]any) {
		m := New(WithTitle("T"), WithInternal(showInternal))
		m.HandleFunc("GET /users", noop)
		m.HandleFunc("GET /admin/secrets", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("served"))
		}, Internal(), WithResponse(200, Secret{}))
		m.HandleFunc("GET /debug/probe", noop, Hidden(), WithResponse(200, Probe{}))
		b, err := m.JSON()
		if err != nil {
			t.Fatal(err)
		}
		return m, jx(t, b)
	}

	// Policy: internal hidden (staging).
	m, doc := build(false)
	paths := jget(t, doc, "paths").(map[string]any)
	if _, ok := paths["/users"]; !ok {
		t.Errorf("public route missing")
	}
	if _, ok := paths["/admin/secrets"]; ok {
		t.Errorf("internal route documented despite WithInternal(false)")
	}
	if _, ok := paths["/debug/probe"]; ok {
		t.Errorf("hidden route documented")
	}
	schemas := jget(t, doc, "components", "schemas").(map[string]any)
	for _, leaked := range []string{"Secret", "Probe"} {
		if _, ok := schemas[leaked]; ok {
			t.Errorf("component %s leaked from an excluded route", leaked)
		}
	}
	// Excluded routes still serve traffic.
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, httptest.NewRequest("GET", "/admin/secrets", nil))
	if rr.Code != 200 || rr.Body.String() != "served" {
		t.Errorf("internal route must still serve traffic: %d %q", rr.Code, rr.Body.String())
	}

	// Policy: internal shown (dev).
	_, doc = build(true)
	op := jget(t, doc, "paths", "/admin/secrets", "get").(map[string]any)
	if op["x-internal"] != true {
		t.Errorf("shown internal route lacks x-internal: true: %v", op)
	}
	if _, ok := jget(t, doc, "components", "schemas", "Secret").(map[string]any); !ok {
		t.Errorf("Secret component missing when internal routes are shown")
	}
	if _, ok := jget(t, doc, "paths").(map[string]any)["/debug/probe"]; ok {
		t.Errorf("hidden route documented even under WithInternal(true)")
	}
}

// An excluded route cannot influence operation-id disambiguation of
// the visible set: a public route keeps its unsuffixed id even when a
// hidden/internal route carries the same explicit id.
func TestExcludedRoutesDontAffectOperationIDs(t *testing.T) {
	m := New(WithTitle("T"))
	m.HandleFunc("GET /a", noop, OperationID("x"), Internal())
	m.HandleFunc("GET /b", noop, OperationID("x"))
	b, _ := m.JSON()
	doc := jx(t, b)
	if id := jget(t, doc, "paths", "/b", "get", "operationId").(string); id != "x" {
		t.Errorf("visible route id = %q, want unsuffixed x (hidden collision leaked)", id)
	}
}

// Mount accepts the same optional bool as Docs, with identical
// explicit-wins semantics.
func TestMountOverridesDisabled(t *testing.T) {
	// Mount(true) on a WithDisabled mux registers working docs.
	m := New(WithTitle("T"), WithDisabled(true))
	m.HandleFunc("GET /x", noop)
	m.Mount(true)
	rr := httptest.NewRecorder()
	m.ServeHTTP(rr, httptest.NewRequest("GET", "/docs/", nil))
	if rr.Code != 200 {
		t.Errorf("Mount(true) on a disabled mux: /docs/ = %d, want 200", rr.Code)
	}

	// Mount(false) on an enabled mux registers nothing.
	m2 := New(WithTitle("T"))
	m2.HandleFunc("GET /x", noop)
	m2.Mount(false)
	rr2 := httptest.NewRecorder()
	m2.ServeHTTP(rr2, httptest.NewRequest("GET", "/docs/", nil))
	if rr2.Code != 404 {
		t.Errorf("Mount(false): /docs/ = %d, want 404", rr2.Code)
	}

	// More than one bool is rejected, like Docs.
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("Mount(true, false) should panic")
			}
		}()
		New(WithTitle("T")).Mount(true, false)
	}()
}

// Manually registering a docs handler at the prefix and then calling
// Mount with a conflicting value cannot silently fight: the second
// registration of "GET <prefix>/" panics at startup with a ServeMux
// pattern conflict.
func TestManualDocsThenMountConflicts(t *testing.T) {
	m := New(WithTitle("T"))
	m.Handle("GET /docs/", m.Docs(false))
	defer func() {
		if recover() == nil {
			t.Errorf("Mount(true) after a manual GET /docs/ registration should panic with a pattern conflict")
		}
	}()
	m.Mount(true)
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

// Constraint struct tags flow into the served document with the
// version-correct exclusive-bound form and typed values.
func TestConstraintTagsEndToEnd(t *testing.T) {
	type CreateTask struct {
		Title    string   `json:"title" minLength:"1" maxLength:"200"`
		Priority int      `json:"priority" minimum:"1" maximum:"5" default:"3"`
		Ratio    float64  `json:"ratio" exclusiveMinimum:"0"`
		Status   string   `json:"status" enum:"pending,active,done"`
		Tags     []string `json:"tags" maxItems:"10" uniqueItems:"true"`
	}
	build := func(v SpecVersion) map[string]any {
		mux := New(WithTitle("T"), WithVersion(v))
		mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {}, WithBody(CreateTask{}))
		raw, err := mux.JSON()
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			t.Fatal(err)
		}
		schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
		return schemas["CreateTask"].(map[string]any)["properties"].(map[string]any)
	}

	props30 := build(OpenAPI30)
	prio := props30["priority"].(map[string]any)
	if prio["minimum"] != json.Number("1") || prio["maximum"] != json.Number("5") || prio["default"] != json.Number("3") {
		t.Errorf("3.0 priority = %#v", prio)
	}
	ratio := props30["ratio"].(map[string]any)
	if ratio["minimum"] != json.Number("0") || ratio["exclusiveMinimum"] != true {
		t.Errorf("3.0 ratio should use the boolean exclusive form, got %#v", ratio)
	}
	title := props30["title"].(map[string]any)
	if title["minLength"] != json.Number("1") || title["maxLength"] != json.Number("200") {
		t.Errorf("3.0 title = %#v", title)
	}
	tags := props30["tags"].(map[string]any)
	if tags["maxItems"] != json.Number("10") || tags["uniqueItems"] != true {
		t.Errorf("3.0 tags = %#v", tags)
	}
	status := props30["status"].(map[string]any)
	enum, _ := status["enum"].([]any)
	if len(enum) != 3 || enum[0] != "pending" {
		t.Errorf("3.0 status enum = %#v", status["enum"])
	}

	props32 := build(OpenAPI32)
	ratio32 := props32["ratio"].(map[string]any)
	if ratio32["exclusiveMinimum"] != json.Number("0") {
		t.Errorf("3.2 ratio should use the numeric exclusive form, got %#v", ratio32)
	}
	if _, ok := ratio32["minimum"]; ok {
		t.Errorf("3.2 ratio must not emit minimum for an exclusive bound")
	}

	// YAML keeps numbers unquoted (json.Number passthrough).
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /tasks", func(w http.ResponseWriter, r *http.Request) {}, WithBody(CreateTask{}))
	y, err := mux.YAML()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(y, []byte("minimum: 1")) || bytes.Contains(y, []byte(`minimum: "1"`)) {
		t.Errorf("YAML should contain unquoted minimum: 1")
	}
}

// Opts bundles apply in order, skip nils, and are reusable across
// routes without leaking state between them.
func TestOptsCombinator(t *testing.T) {
	paginated := Opts(
		QueryParam("cursor", "string", "Opaque cursor"),
		nil,
		QueryParam("limit", "integer", "Page size"),
	)
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /tasks", noop, paginated, Summary("List tasks"))
	mux.HandleFunc("GET /users", noop, paginated)
	doc := buildDocMap(t, mux)
	for _, p := range []string{"/tasks", "/users"} {
		op := doc["paths"].(map[string]any)[p].(map[string]any)["get"].(map[string]any)
		params := op["parameters"].([]any)
		if len(params) != 2 {
			t.Errorf("%s: %d params, want 2 (cursor, limit)", p, len(params))
			continue
		}
		if params[0].(map[string]any)["name"] != "cursor" || params[1].(map[string]any)["name"] != "limit" {
			t.Errorf("%s: param order = %v, %v", p, params[0].(map[string]any)["name"], params[1].(map[string]any)["name"])
		}
	}
	if got := doc["paths"].(map[string]any)["/tasks"].(map[string]any)["get"].(map[string]any)["summary"]; got != "List tasks" {
		t.Errorf("summary = %v; opts after a bundle must still apply", got)
	}
	// Nested bundles compose.
	nested := Opts(Opts(Summary("S")), Deprecated())
	mux2 := New(WithTitle("T"))
	mux2.HandleFunc("GET /x", noop, nested)
	op := buildDocMap(t, mux2)["paths"].(map[string]any)["/x"].(map[string]any)["get"].(map[string]any)
	if op["summary"] != "S" || op["deprecated"] != true {
		t.Errorf("nested bundle: summary=%v deprecated=%v", op["summary"], op["deprecated"])
	}
}

// Mux-level default responses appear on every operation, lose to
// per-route declarations, and survive rebuilds unchanged.
func TestDefaultResponses(t *testing.T) {
	type APIError struct {
		Message string `json:"message"`
	}
	type Custom struct {
		Code int `json:"code"`
	}
	mux := New(
		WithTitle("T"),
		WithDefaultResponse(500, APIError{}),
		WithDefaultResponse(0, APIError{}),
	)
	mux.HandleFunc("GET /tasks", noop)
	mux.HandleFunc("POST /tasks", noop, WithResponse(201, Custom{}), WithResponse(500, Custom{}))

	check := func(doc map[string]any) {
		get := doc["paths"].(map[string]any)["/tasks"].(map[string]any)["get"].(map[string]any)
		resps := get["responses"].(map[string]any)
		for _, key := range []string{"200", "500", "default"} {
			if _, ok := resps[key]; !ok {
				t.Errorf("GET /tasks missing %s response; got keys %v", key, mapKeysOf(resps))
			}
		}
		ref := resps["500"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"]
		if ref != "#/components/schemas/APIError" {
			t.Errorf("GET /tasks 500 schema = %v, want APIError ref", ref)
		}
		post := doc["paths"].(map[string]any)["/tasks"].(map[string]any)["post"].(map[string]any)
		postRef := post["responses"].(map[string]any)["500"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"]
		if postRef != "#/components/schemas/Custom" {
			t.Errorf("POST /tasks 500 schema = %v; the per-route declaration must win", postRef)
		}
		if _, ok := post["responses"].(map[string]any)["200"]; ok {
			t.Errorf("POST /tasks must not gain an auto-200 when it declares responses")
		}
	}
	check(buildDocMap(t, mux))
	mux.Refresh()
	check(buildDocMap(t, mux)) // identical after a rebuild

	defer func() {
		if recover() == nil {
			t.Errorf("invalid status should panic")
		}
	}()
	WithDefaultResponse(42, nil)
}

func mapKeysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Operations with a security requirement automatically document 401;
// overridable per route, body-able via WithDefaultResponse, and
// suppressible mux-wide.
func TestAutoUnauthorized(t *testing.T) {
	type APIError struct {
		Message string `json:"message"`
	}
	resps := func(doc map[string]any, path, method string) map[string]any {
		t.Helper()
		return doc["paths"].(map[string]any)[path].(map[string]any)[method].(map[string]any)["responses"].(map[string]any)
	}

	mux := New(WithTitle("T"), WithBearerAuth("bearerAuth", "JWT"))
	mux.HandleFunc("GET /public", noop)
	mux.HandleFunc("GET /me", noop, WithSecurity("bearerAuth"))
	mux.HandleFunc("GET /custom", noop, WithSecurity("bearerAuth"), WithResponse(401, APIError{}))
	doc := buildDocMap(t, mux)
	if _, ok := resps(doc, "/public", "get")["401"]; ok {
		t.Errorf("unsecured route must not gain a 401")
	}
	r401, ok := resps(doc, "/me", "get")["401"].(map[string]any)
	if !ok {
		t.Fatalf("secured route missing auto-401")
	}
	if r401["description"] != "Unauthorized" {
		t.Errorf("auto-401 description = %v", r401["description"])
	}
	if _, hasBody := r401["content"]; hasBody {
		t.Errorf("bare auto-401 must have no body")
	}
	custom := resps(doc, "/custom", "get")["401"].(map[string]any)
	if custom["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"] != "#/components/schemas/APIError" {
		t.Errorf("per-route 401 must win: %v", custom)
	}

	// Global security: every route is secured except explicit opt-outs.
	gmux := New(WithTitle("T"), WithBearerAuth("bearerAuth", "JWT"), WithGlobalSecurity("bearerAuth"))
	gmux.HandleFunc("GET /a", noop)
	gmux.HandleFunc("GET /open", noop, WithNoSecurity())
	gdoc := buildDocMap(t, gmux)
	if _, ok := resps(gdoc, "/a", "get")["401"]; !ok {
		t.Errorf("globally-secured route missing auto-401")
	}
	if _, ok := resps(gdoc, "/open", "get")["401"]; ok {
		t.Errorf("WithNoSecurity route must not gain a 401")
	}

	// WithDefaultResponse(401, body) supplies the body.
	bmux := New(WithTitle("T"), WithBearerAuth("bearerAuth", "JWT"), WithDefaultResponse(401, APIError{}))
	bmux.HandleFunc("GET /me", noop, WithSecurity("bearerAuth"))
	bdoc := buildDocMap(t, bmux)
	b401 := resps(bdoc, "/me", "get")["401"].(map[string]any)
	if b401["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"] != "#/components/schemas/APIError" {
		t.Errorf("WithDefaultResponse(401) body should apply: %v", b401)
	}

	// Suppressed mux-wide.
	smux := New(WithTitle("T"), WithBearerAuth("bearerAuth", "JWT"), WithAutoUnauthorized(false))
	smux.HandleFunc("GET /me", noop, WithSecurity("bearerAuth"))
	if _, ok := resps(buildDocMap(t, smux), "/me", "get")["401"]; ok {
		t.Errorf("WithAutoUnauthorized(false) must suppress the 401")
	}
}

// WithPathPrefix prefixes documented paths without touching routing,
// operation ids, or the docs-prefix exclusion.
func TestPathPrefix(t *testing.T) {
	mux := New(WithTitle("T"), WithPathPrefix("api/"))
	mux.HandleFunc("GET /users/{id}", noop)
	mux.HandleFunc("GET /", noop)
	mux.Mount()
	doc := buildDocMap(t, mux)
	paths := doc["paths"].(map[string]any)
	if _, ok := paths["/api/users/{id}"]; !ok {
		t.Errorf("documented paths = %v, want /api/users/{id}", mapKeysOf(paths))
	}
	if _, ok := paths["/users/{id}"]; ok {
		t.Errorf("unprefixed path must not appear")
	}
	if _, ok := paths["/api/"]; !ok {
		t.Errorf("root route should document as /api/, got %v", mapKeysOf(paths))
	}
	op := paths["/api/users/{id}"].(map[string]any)["get"].(map[string]any)
	if op["operationId"] != "get_users_by_id" {
		t.Errorf("operationId = %v; ids must not absorb the prefix", op["operationId"])
	}
	for p := range paths {
		if strings.Contains(p, "/docs") {
			t.Errorf("docs routes must stay excluded, found %s", p)
		}
	}
	// Routing is untouched: the route answers at its registered path.
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest("GET", "/users/7", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("GET /users/7 = %d; routing must be unaffected", rr.Code)
	}

	defer func() {
		if recover() == nil {
			t.Errorf("WithPathPrefix(\"/\") should panic")
		}
	}()
	New(WithTitle("T"), WithPathPrefix("/"))
}

// ParamOpt modifiers flow into the served document with typed values.
func TestParamOpts(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /tasks", noop,
		QueryParam("limit", "integer", "Page size",
			ParamDefault(20), ParamMinimum(1), ParamMaximum(100), ParamExample(25)),
		QueryParam("status", "string", "Filter",
			ParamRequired(), ParamEnum("pending", "active", "done"), ParamMinLength(1), ParamMaxLength(10)),
		QueryParam("q", "string", "Search", ParamPattern("^[a-z]+$")),
	)
	doc := buildDocMap(t, mux)
	params := doc["paths"].(map[string]any)["/tasks"].(map[string]any)["get"].(map[string]any)["parameters"].([]any)
	byName := map[string]map[string]any{}
	for _, p := range params {
		pm := p.(map[string]any)
		byName[pm["name"].(string)] = pm
	}
	limit := byName["limit"]["schema"].(map[string]any)
	if limit["default"] != json.Number("20") || limit["minimum"] != json.Number("1") ||
		limit["maximum"] != json.Number("100") || limit["example"] != json.Number("25") {
		t.Errorf("limit schema = %#v", limit)
	}
	if _, ok := byName["limit"]["required"]; ok {
		t.Errorf("limit must stay optional")
	}
	status := byName["status"]
	if status["required"] != true {
		t.Errorf("status should be required")
	}
	ss := status["schema"].(map[string]any)
	if enum := ss["enum"].([]any); len(enum) != 3 || enum[0] != "pending" {
		t.Errorf("status enum = %#v", ss["enum"])
	}
	if ss["minLength"] != json.Number("1") || ss["maxLength"] != json.Number("10") {
		t.Errorf("status lengths = %#v", ss)
	}
	if byName["q"]["schema"].(map[string]any)["pattern"] != "^[a-z]+$" {
		t.Errorf("q pattern missing")
	}
}

// Misuse of param declarations panics at registration time.
func TestParamValidationPanics(t *testing.T) {
	cases := []struct {
		name string
		f    func()
	}{
		{"unknown type", func() { QueryParam("x", "intger", "typo") }},
		{"unknown in", func() { WithParam("x", "body", "string", "") }},
		{"empty name", func() { WithParam("", "query", "string", "") }},
		{"default type mismatch", func() { QueryParam("n", "integer", "", ParamDefault("x")) }},
		{"minimum on string", func() { QueryParam("s", "string", "", ParamMinimum(1)) }},
		{"pattern on integer", func() { QueryParam("n", "integer", "", ParamPattern("a")) }},
		{"enum member mismatch", func() { QueryParam("n", "integer", "", ParamEnum(1, "two")) }},
		{"minLength on integer", func() { QueryParam("n", "integer", "", ParamMinLength(1)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic")
				}
			}()
			tc.f()
		})
	}
}

// WithParams reflects a struct into typed, constrained parameters.
func TestWithParams(t *testing.T) {
	type ListParams struct {
		Cursor  string    `query:"cursor" doc:"Opaque pagination cursor"`
		Limit   int       `query:"limit" default:"20" minimum:"1" maximum:"100"`
		Status  string    `query:"status" enum:"pending,active,done" required:"true"`
		Sort    []string  `query:"sort" maxItems:"3"`
		Since   time.Time `query:"since"`
		Trace   string    `header:"X-Trace-Id" doc:"Propagated trace id"`
		Session string    `cookie:"session"`
		hidden  string    //nolint:unused // unexported fields are skipped
		Skipped string    `query:"-"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /tasks", noop, WithParams(ListParams{}))
	doc := buildDocMap(t, mux)
	params := doc["paths"].(map[string]any)["/tasks"].(map[string]any)["get"].(map[string]any)["parameters"].([]any)
	if len(params) != 7 {
		t.Fatalf("%d params, want 7", len(params))
	}
	byName := map[string]map[string]any{}
	for _, p := range params {
		pm := p.(map[string]any)
		byName[pm["name"].(string)] = pm
	}
	if byName["cursor"]["description"] != "Opaque pagination cursor" || byName["cursor"]["in"] != "query" {
		t.Errorf("cursor = %#v", byName["cursor"])
	}
	if s := byName["cursor"]["schema"].(map[string]any); s["description"] != nil {
		t.Errorf("description must live on the parameter, not its schema")
	}
	limit := byName["limit"]["schema"].(map[string]any)
	if limit["type"] != "integer" || limit["format"] != "int64" ||
		limit["default"] != json.Number("20") || limit["minimum"] != json.Number("1") {
		t.Errorf("limit schema = %#v", limit)
	}
	if byName["status"]["required"] != true {
		t.Errorf("status should be required via the required tag")
	}
	sort := byName["sort"]["schema"].(map[string]any)
	if sort["type"] != "array" || sort["maxItems"] != json.Number("3") ||
		sort["items"].(map[string]any)["type"] != "string" {
		t.Errorf("sort schema = %#v", sort)
	}
	since := byName["since"]["schema"].(map[string]any)
	if since["type"] != "string" || since["format"] != "date-time" {
		t.Errorf("since schema = %#v", since)
	}
	if byName["X-Trace-Id"]["in"] != "header" || byName["session"]["in"] != "cookie" {
		t.Errorf("locations: trace=%v session=%v", byName["X-Trace-Id"]["in"], byName["session"]["in"])
	}
	if _, ok := byName["Skipped"]; ok {
		t.Errorf(`query:"-" field must be skipped`)
	}
}

// WithParams misuse panics at the WithParams call site.
func TestWithParamsPanics(t *testing.T) {
	type NoTag struct {
		X string
	}
	type TwoTags struct {
		X string `query:"x" header:"x"`
	}
	type EmptyTag struct {
		X string `query:""`
	}
	type StructField struct {
		X struct{ Y string } `query:"x"`
	}
	type BadRequired struct {
		X string `query:"x" required:"yes please"`
	}
	cases := []struct {
		name string
		f    func()
	}{
		{"not a struct", func() { WithParams(42) }},
		{"no location tag", func() { WithParams(NoTag{}) }},
		{"two location tags", func() { WithParams(TwoTags{}) }},
		{"empty tag value", func() { WithParams(EmptyTag{}) }},
		{"struct field", func() { WithParams(StructField{}) }},
		{"bad required", func() { WithParams(BadRequired{}) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic")
				}
			}()
			tc.f()
		})
	}
}

// Findings from the v0.3.0 verification pass, pinned.
func TestVerificationFindings(t *testing.T) {
	// Non-JSON numeric literals panic at build instead of 500ing the
	// docs endpoint.
	for _, bad := range []string{".5", "+5", "010", "1.", "NaN", "Inf", "1_0"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("minimum:%q should panic", bad)
				}
			}()
			schemaTagPanic(bad)
		}()
	}

	// Valid exponent forms still pass and YAML renders them 1.1-safe.
	type Exp struct {
		R float64 `json:"r" minimum:"1e3"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /e", noop, WithBody(Exp{}))
	y, err := mux.YAML()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(y, []byte("minimum: 1.0e+3")) {
		t.Errorf("YAML should reformat 1e3 to a YAML-1.1-safe number, got:\n%s", y)
	}

	// enum members are trimmed; empty members panic.
	type Spaced struct {
		S string `json:"s" enum:"a, b, c"`
	}
	mux2 := New(WithTitle("T"))
	mux2.HandleFunc("POST /s", noop, WithBody(Spaced{}))
	doc := buildDocMap(t, mux2)
	enum := doc["components"].(map[string]any)["schemas"].(map[string]any)["Spaced"].(map[string]any)["properties"].(map[string]any)["s"].(map[string]any)["enum"].([]any)
	if enum[1] != "b" || enum[2] != "c" {
		t.Errorf("enum members must be trimmed: %#v", enum)
	}

	// Nullable + enum lists null so the contract matches encoding/json.
	type Nullable struct {
		S *string `json:"s" enum:"a,b"`
	}
	mux3 := New(WithTitle("T"))
	mux3.HandleFunc("POST /n", noop, WithBody(Nullable{}))
	ndoc := buildDocMap(t, mux3)
	nenum := ndoc["components"].(map[string]any)["schemas"].(map[string]any)["Nullable"].(map[string]any)["properties"].(map[string]any)["s"].(map[string]any)["enum"].([]any)
	if len(nenum) != 3 || nenum[2] != nil {
		t.Errorf("nullable enum must list null: %#v", nenum)
	}

	// Duplicate (name, in) parameters panic at build.
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("duplicate parameter should panic")
			}
		}()
		type P struct {
			L int `query:"limit"`
		}
		dup := New(WithTitle("T"))
		dup.HandleFunc("GET /d", noop, WithParams(P{}), QueryParam("limit", "integer", "dup"))
		dup.JSON()
	}()

	// WithDefaultResponse(200, body) supplies the success body on
	// routes that declare nothing.
	type OK struct {
		Done bool `json:"done"`
	}
	d200 := New(WithTitle("T"), WithDefaultResponse(200, OK{}))
	d200.HandleFunc("GET /x", noop)
	xdoc := buildDocMap(t, d200)
	resp200 := xdoc["paths"].(map[string]any)["/x"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)
	if _, ok := resp200["content"]; !ok {
		t.Errorf("default 200 body should apply to undeclared routes: %#v", resp200)
	}

	// nil RouteOpts are tolerated like in Opts.
	nilmux := New(WithTitle("T"))
	nilmux.HandleFunc("GET /nil", noop, nil, Summary("S"))

	// default/example/format on non-scalar fields panic.
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("default on RawMessage should panic")
			}
		}()
		type Raw struct {
			R json.RawMessage `json:"r" default:"x"`
		}
		rm := New(WithTitle("T"))
		rm.HandleFunc("POST /r", noop, WithBody(Raw{}))
		rm.JSON()
	}()

	// example on a $ref field still decorates the use site (v0.2.x
	// behavior).
	type Inner struct {
		X string `json:"x"`
	}
	type Outer struct {
		I Inner `json:"i" example:"sample"`
	}
	ref := New(WithTitle("T"))
	ref.HandleFunc("POST /o", noop, WithBody(Outer{}))
	rdoc := buildDocMap(t, ref)
	iProp := rdoc["components"].(map[string]any)["schemas"].(map[string]any)["Outer"].(map[string]any)["properties"].(map[string]any)["i"].(map[string]any)
	if iProp["example"] != "sample" {
		t.Errorf("example on $ref field lost: %#v", iProp)
	}

	// WithPathPrefix rejects wildcards and whitespace.
	for _, bad := range []string{"/{tenant}", "/a b", "/x?y"} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("WithPathPrefix(%q) should panic", bad)
				}
			}()
			New(WithTitle("T"), WithPathPrefix(bad))
		}()
	}
}

// schemaTagPanic reflects a one-field struct whose minimum tag is the
// given literal, via the params path (eager validation).
func schemaTagPanic(literal string) {
	// The tag must be a compile-time constant per field, so exercise
	// the same numericConstraint path through reflection of a
	// runtime-built struct tag.
	typ := reflect.StructOf([]reflect.StructField{{
		Name: "N",
		Type: reflect.TypeOf(float64(0)),
		Tag:  reflect.StructTag(`json:"n" minimum:"` + literal + `"`),
	}})
	v := reflect.New(typ).Elem().Interface()
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /t", noop, WithBody(v))
	if _, err := mux.JSON(); err != nil {
		panic(err) // either way the test's recover() fires
	}
}

// WithCleanOutput strips vendor noise; default output is unchanged.
func TestCleanOutput(t *testing.T) {
	type Holder struct {
		Value any `json:"value"` // produces x-stdocs-type
	}
	build := func(clean bool) string {
		mux := New(WithTitle("T"), WithCleanOutput(clean))
		mux.HandleFunc("/anymethod", noop) // produces x-stdocs-warning
		mux.HandleFunc("POST /h", noop, WithBody(Holder{}))
		raw, err := mux.JSON()
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	dirty := build(false)
	for _, marker := range []string{"x-stdocs-type", "x-stdocs-warning", "Generated from Go type"} {
		if !strings.Contains(dirty, marker) {
			t.Errorf("default output should contain %s", marker)
		}
	}
	clean := build(true)
	for _, marker := range []string{"x-stdocs-type", "x-stdocs-warning", "Generated from Go type"} {
		if strings.Contains(clean, marker) {
			t.Errorf("clean output must not contain %s", marker)
		}
	}

	// User doc: tags survive cleaning; custom methods keep their
	// extension carrier.
	type Doc struct {
		X string `json:"x" doc:"Kept description"`
	}
	mux := New(WithTitle("T"), WithCleanOutput(true))
	mux.HandleFunc("PURGE /cache", noop)
	mux.HandleFunc("POST /d", noop, WithBody(Doc{}))
	raw, err := mux.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Kept description") {
		t.Errorf("user descriptions must survive cleaning")
	}
	if !strings.Contains(string(raw), "x-stdocs-additionalOperations") {
		t.Errorf("custom-method operations must not be dropped by cleaning")
	}
}

// WithResponseContentType overrides the response media type,
// order-independently, and DriftWarn respects the declaration.
func TestResponseContentType(t *testing.T) {
	type Report struct {
		Rows int `json:"rows"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /csv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte("rows\n1\n"))
	},
		WithResponseContentType(200, "text/csv"), // before WithResponse: order-independent
		WithResponse(200, Report{}),
	)
	doc := buildDocMap(t, mux)
	content := doc["paths"].(map[string]any)["/csv"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)
	if _, ok := content["text/csv"]; !ok {
		t.Errorf("content keys = %v, want text/csv", mapKeysOf(content))
	}
	if _, ok := content["application/json"]; ok {
		t.Errorf("application/json must be replaced")
	}

	logf, warnings := collectWarnings()
	h := DriftWarn(mux, logf)
	driftGet(h, "/csv")
	if got := warnings(); len(got) != 0 {
		t.Errorf("a declared text/csv response served as text/csv is not drift: %q", got)
	}
}

// The richness batch: externalDocs at all three levels, the SPDX
// identifier with 3.0 degradation, and operationId templating.
func TestSpecRichness(t *testing.T) {
	build := func(v SpecVersion) map[string]any {
		mux := New(
			WithTitle("T"),
			WithVersion(v),
			WithSPDXLicense("Apache 2.0", "Apache-2.0"),
			WithExternalDocs("https://docs.example.com", "Full guide"),
			WithTag("Tasks", "Task management"),
			WithTagExternalDocs("Tasks", "https://docs.example.com/tasks", ""),
			WithTagExternalDocs("Orphan", "https://docs.example.com/orphan", ""),
			WithOperationIDFunc(func(method, path string) string {
				return strings.ToLower(method) + strings.ReplaceAll(path, "/", ".")
			}),
		)
		mux.HandleFunc("GET /tasks", noop,
			ExternalDocs("https://docs.example.com/list", "List docs"))
		mux.HandleFunc("GET /named", noop, OperationID("explicit"))
		return buildDocMap(t, mux)
	}

	doc := build(OpenAPI31)
	if doc["externalDocs"].(map[string]any)["url"] != "https://docs.example.com" {
		t.Errorf("document externalDocs missing")
	}
	lic := doc["info"].(map[string]any)["license"].(map[string]any)
	if lic["identifier"] != "Apache-2.0" {
		t.Errorf("3.1 license = %v, want SPDX identifier", lic)
	}
	tags := doc["tags"].([]any)
	found := map[string]bool{}
	for _, tg := range tags {
		tm := tg.(map[string]any)
		if ed, ok := tm["externalDocs"].(map[string]any); ok {
			found[tm["name"].(string)] = strings.HasPrefix(ed["url"].(string), "https://docs.example.com/")
		}
	}
	if !found["Tasks"] || !found["Orphan"] {
		t.Errorf("tag externalDocs = %v; both declared-first and docs-first tags must carry links", found)
	}
	op := doc["paths"].(map[string]any)["/tasks"].(map[string]any)["get"].(map[string]any)
	if op["externalDocs"].(map[string]any)["url"] != "https://docs.example.com/list" {
		t.Errorf("operation externalDocs missing")
	}
	if op["operationId"] != "get.tasks" {
		t.Errorf("operationId = %v, want templated get.tasks", op["operationId"])
	}
	named := doc["paths"].(map[string]any)["/named"].(map[string]any)["get"].(map[string]any)
	if named["operationId"] != "explicit" {
		t.Errorf("explicit OperationID must win over the func")
	}

	// 3.0 degrades the SPDX license to name-only.
	lic30 := build(OpenAPI30)["info"].(map[string]any)["license"].(map[string]any)
	if _, ok := lic30["identifier"]; ok {
		t.Errorf("3.0 must not emit license.identifier")
	}
	if lic30["name"] != "Apache 2.0" {
		t.Errorf("3.0 license name lost: %v", lic30)
	}

	// Required-URL validation.
	defer func() {
		if recover() == nil {
			t.Errorf("empty externalDocs URL should panic")
		}
	}()
	ExternalDocs("", "broken")
}

// WithMultipartBody documents the standard file-upload shape.
func TestMultipartBody(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /upload", noop,
		WithMultipartBody(
			FilePart("attachment", "The file to upload"),
			FieldPart("caption", "string", "Optional caption"),
		),
	)
	doc := buildDocMap(t, mux)
	rb := doc["paths"].(map[string]any)["/upload"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)
	content := rb["content"].(map[string]any)
	mp, ok := content["multipart/form-data"].(map[string]any)
	if !ok {
		t.Fatalf("content keys = %v, want multipart/form-data", mapKeysOf(content))
	}
	props := mp["schema"].(map[string]any)["properties"].(map[string]any)
	att := props["attachment"].(map[string]any)
	if att["type"] != "string" || att["format"] != "binary" {
		t.Errorf("attachment = %v, want string/binary", att)
	}
	if props["caption"].(map[string]any)["description"] != "Optional caption" {
		t.Errorf("caption description lost")
	}

	for _, f := range []func(){
		func() { WithMultipartBody() },
		func() { WithMultipartBody(FilePart("a", ""), FilePart("a", "")) },
		func() { FilePart("", "x") },
		func() { FieldPart("x", "intger", "typo") },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic")
				}
			}()
			f()
		}()
	}
}

// Lint reports the advisory consumability findings.
func TestLint(t *testing.T) {
	type Holder struct {
		Value any `json:"value"`
	}
	type APIError struct {
		Message string `json:"message"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /bare", func(w http.ResponseWriter, r *http.Request) {}) // no error response, no summary
	mux.HandleFunc("POST /h", noop, WithBody(Holder{}),                          // untyped field
		Summary("Hold"), WithResponse(0, APIError{})) // has default: no error warning
	warnings := mux.Lint()
	byMessage := func(substr string) int {
		n := 0
		for _, w := range warnings {
			if strings.Contains(w.String(), substr) {
				n++
			}
		}
		return n
	}
	if byMessage("documents no error response") != 1 {
		t.Errorf("want exactly one no-error-response warning (GET /bare), got %d in %v", byMessage("documents no error response"), warnings)
	}
	if byMessage("has no summary") < 1 {
		t.Errorf("want a no-summary warning for the closure route")
	}
	if byMessage("has no schema type") != 1 {
		t.Errorf("want the untyped-field warning for Holder.value")
	}
	if byMessage("x-stdocs-") < 1 {
		t.Errorf("want the vendor-extensions advisory")
	}

	// A fully documented clean mux lints quiet.
	quiet := New(WithTitle("T"), WithCleanOutput(true), WithDefaultResponse(500, APIError{}))
	quiet.HandleFunc("GET /ok", noop, Summary("All good"))
	if w := quiet.Lint(); len(w) != 0 {
		t.Errorf("clean mux should lint quiet, got %v", w)
	}
}

// Concurrent Lint/JSON/Refresh must be safe (was a fatal concurrent
// map write through visibleRoutes stamping x-internal).
func TestLintConcurrency(t *testing.T) {
	type APIError struct {
		Message string `json:"message"`
	}
	mux := New(WithTitle("T"), WithInternal(true), WithDefaultResponse(500, APIError{}))
	mux.HandleFunc("GET /a", noop, Internal(), Summary("A"))
	mux.HandleFunc("GET /b", noop, Summary("B"))
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 10 {
				mux.Lint()
				mux.JSON()
			}
		}()
	}
	wg.Wait()
}

// Lint findings from the verification pass: exclusive-bound generator
// warning on 3.1+, webhook components included, warning-only vendor
// extensions flagged, and no false collision positives on legit
// underscore names.
func TestLintVerificationFindings(t *testing.T) {
	type Ratio struct {
		R float64 `json:"r" exclusiveMinimum:"0"`
	}
	mux := New(WithTitle("T"), WithVersion(OpenAPI31), WithDefaultResponse(500, Ratio{}))
	mux.HandleFunc("GET /r", noop, Summary("R"))
	found := false
	for _, w := range mux.Lint() {
		if strings.Contains(w.Message, "exclusive bound") {
			found = true
		}
	}
	if !found {
		t.Errorf("3.1 exclusive bounds should produce a generator warning")
	}

	// 3.0 documents emit the boolean form generators accept: no warning.
	mux30 := New(WithTitle("T"), WithDefaultResponse(500, Ratio{}))
	mux30.HandleFunc("GET /r", noop, Summary("R"))
	for _, w := range mux30.Lint() {
		if strings.Contains(w.Message, "exclusive bound") {
			t.Errorf("3.0 must not warn on exclusive bounds: %v", w)
		}
	}

	// x-stdocs-warning alone triggers the clean-output advisory.
	type E struct {
		Message string `json:"message"`
	}
	wmux := New(WithTitle("T"), WithDefaultResponse(500, E{}))
	wmux.HandleFunc("/anymethod", noop, Summary("Any"))
	advisory := false
	for _, w := range wmux.Lint() {
		if strings.Contains(w.Message, "WithCleanOutput") {
			advisory = true
		}
	}
	if !advisory {
		t.Errorf("x-stdocs-warning alone should trigger the clean-output advisory")
	}

	// A legit underscore-digit type name is not a collision.
	type User_2 struct { //nolint:revive // deliberate underscore-digit name to probe collision false positives
		X string `json:"x"`
	}
	umux := New(WithTitle("T"), WithDefaultResponse(500, User_2{}))
	umux.HandleFunc("GET /u", noop, Summary("U"))
	for _, w := range umux.Lint() {
		if strings.Contains(w.Message, "collision") {
			t.Errorf("User_2 without a collision must not warn: %v", w)
		}
	}
}

// Order-independence contracts: WithTag/WithTagExternalDocs in both
// orders yield one merged tag, and WithBody/WithMultipartBody replace
// each other in either order without clobbering WithBodyContentType.
func TestDeclarationOrderContracts(t *testing.T) {
	for _, order := range [][]Option{
		{WithTag("tasks", "Task ops"), WithTagExternalDocs("tasks", "https://x.test/t", "")},
		{WithTagExternalDocs("tasks", "https://x.test/t", ""), WithTag("tasks", "Task ops")},
	} {
		mux := New(append([]Option{WithTitle("T")}, order...)...)
		mux.HandleFunc("GET /tasks", noop)
		tags := buildDocMap(t, mux)["tags"].([]any)
		if len(tags) != 1 {
			t.Fatalf("tags = %d entries, want 1 merged declaration", len(tags))
		}
		tag := tags[0].(map[string]any)
		if tag["description"] != "Task ops" || tag["externalDocs"] == nil {
			t.Errorf("merged tag = %v", tag)
		}
	}

	type Body struct {
		X string `json:"x"`
	}
	// multipart -> WithBody: JSON body wins entirely.
	m1 := New(WithTitle("T"))
	m1.HandleFunc("POST /a", noop,
		WithMultipartBody(FilePart("f", "")),
		WithBody(Body{}),
	)
	rb := buildDocMap(t, m1)["paths"].(map[string]any)["/a"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)
	content := rb["content"].(map[string]any)
	if _, ok := content["application/json"]; !ok {
		t.Errorf("WithBody after multipart should produce JSON content, got %v", mapKeysOf(content))
	}
	if _, ok := content["multipart/form-data"]; ok {
		t.Errorf("multipart residue after WithBody")
	}
	// WithBodyContentType survives a later WithBody.
	m2 := New(WithTitle("T"))
	m2.HandleFunc("POST /b", noop,
		WithBodyContentType("application/xml"),
		WithBody(Body{}),
	)
	rb2 := buildDocMap(t, m2)["paths"].(map[string]any)["/b"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)
	if _, ok := rb2["content"].(map[string]any)["application/xml"]; !ok {
		t.Errorf("explicit WithBodyContentType must survive WithBody")
	}
}

// v0.4.1: Optional is order-independent, hyphenated paths produce
// clean operationIds.
func TestOptionalOrderAndHyphenIDs(t *testing.T) {
	type B struct {
		X string `json:"x"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /a", noop, Optional(), WithBody(B{})) // Optional first
	mux.HandleFunc("GET /internal/reconcile-status", noop)
	doc := buildDocMap(t, mux)
	rb := doc["paths"].(map[string]any)["/a"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)
	if req, ok := rb["required"]; ok && req == true {
		t.Errorf("Optional before WithBody must mark the body optional, got required=%v", req)
	}
	op := doc["paths"].(map[string]any)["/internal/reconcile-status"].(map[string]any)["get"].(map[string]any)
	if op["operationId"] != "get_internal_reconcile_status" {
		t.Errorf("operationId = %v, want get_internal_reconcile_status", op["operationId"])
	}
}

// v0.4.1: routes registered after a build appear on the next read —
// no manual Refresh needed — and repeated rebuilds stay stable.
func TestLateRegistrationRebuild(t *testing.T) {
	type APIError struct {
		Message string `json:"message"`
	}
	mux := New(WithTitle("T"), WithDefaultResponse(500, APIError{}))
	mux.HandleFunc("GET /a", noop, Summary("A"))
	doc1 := buildDocMap(t, mux)
	if _, ok := doc1["paths"].(map[string]any)["/a"]; !ok {
		t.Fatal("first build missing /a")
	}
	mux.HandleFunc("GET /late", noop, Summary("Late"))
	doc2 := buildDocMap(t, mux)
	late, ok := doc2["paths"].(map[string]any)["/late"].(map[string]any)
	if !ok {
		t.Fatal("late registration missing after rebuild")
	}
	op := late["get"].(map[string]any)
	if op["summary"] != "Late" {
		t.Errorf("late route not finalized: %v", op)
	}
	if _, has500 := op["responses"].(map[string]any)["500"]; !has500 {
		t.Errorf("late route missing mux-level default response")
	}
	// Lint sees the fresh document too.
	for _, w := range mux.Lint() {
		if strings.Contains(w.Where, "/late") && strings.Contains(w.Message, "no error response") {
			t.Errorf("Lint linted a stale build: %v", w)
		}
	}
	// Stability: a third read without changes is byte-identical.
	b1, _ := mux.JSON()
	b2, _ := mux.JSON()
	if !bytes.Equal(b1, b2) {
		t.Errorf("rebuilds must be stable")
	}
}

// v0.4.1: Mount auto-registers embedded assets and builds eagerly so
// tag panics fire at startup.
func TestMountEagerAndAssets(t *testing.T) {
	// Eager build: a bad constraint tag panics at Mount, not at the
	// first docs request.
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("Mount should surface tag panics eagerly")
			}
		}()
		type Bad struct {
			N int `json:"n" minLength:"1"`
		}
		mux := New(WithTitle("T"))
		mux.HandleFunc("POST /b", noop, WithBody(Bad{}))
		mux.Mount()
	}()

	// Assets handler from the config registers under the prefix.
	served := false
	mux := New(WithTitle("T"))
	mux.Config().Assets = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		w.Write([]byte("js"))
	})
	mux.Mount()
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest("GET", "/docs/_assets/bundle.js", nil))
	if rr.Code != http.StatusOK || !served {
		t.Errorf("assets route not auto-registered: %d served=%v", rr.Code, served)
	}
}

// v0.4.1: webhook operations never inherit document security; an
// explicit Webhook.Security still emits.
func TestWebhookSecurityIsolation(t *testing.T) {
	type Event struct {
		ID string `json:"id"`
	}
	mux := New(
		WithTitle("T"),
		WithVersion(OpenAPI31),
		WithBearerAuth("bearerAuth", "JWT"),
		WithGlobalSecurity("bearerAuth"),
		WithWebhooks(map[string]Webhook{
			"thing.created": {Method: "POST", RequestBody: &RequestBody{BodyValue: Event{}}},
			"thing.signed": {Method: "POST",
				Security: []SecurityRequirement{{"bearerAuth": []string{}}}},
		}),
	)
	mux.HandleFunc("GET /things", noop, Summary("List"))
	doc := buildDocMap(t, mux)
	hooks := doc["webhooks"].(map[string]any)
	created := hooks["thing.created"].(map[string]any)["post"].(map[string]any)
	sec, present := created["security"]
	if !present {
		t.Fatalf("webhook must carry an explicit security override, got %v", created)
	}
	if arr := sec.([]any); len(arr) != 0 {
		t.Errorf("webhook without explicit security must emit security: [], got %v", arr)
	}
	signed := hooks["thing.signed"].(map[string]any)["post"].(map[string]any)
	sarr := signed["security"].([]any)
	if len(sarr) != 1 {
		t.Errorf("explicit webhook security must emit: %v", signed["security"])
	}
}

// v0.4.1: host-scoped patterns are handled honestly — hostless wins,
// the survivor carries a warning, no dangling id suffixes, Lint
// reports the shadowed routes exactly once each.
func TestHostScopedPatterns(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET a.example.com/h", noop, Summary("Host A"))
	mux.HandleFunc("GET b.example.com/h", noop, Summary("Host B"))
	doc := buildDocMap(t, mux)
	paths := doc["paths"].(map[string]any)
	op := paths["/h"].(map[string]any)["get"].(map[string]any)
	if op["operationId"] != "get_h" {
		t.Errorf("operationId = %v, want get_h (no dangling suffix)", op["operationId"])
	}
	if w, _ := op["x-stdocs-warning"].(string); !strings.Contains(w, "host") {
		t.Errorf("survivor must carry the host warning, got %v", op["x-stdocs-warning"])
	}
	if tags, _ := op["tags"].([]any); len(tags) != 1 || tags[0] != "H" {
		t.Errorf("tag must come from the path, not the host: %v", op["tags"])
	}
	shadowFindings := 0
	for _, w := range mux.Lint() {
		if strings.Contains(w.Message, "shadowed in the document") {
			shadowFindings++
			if !strings.Contains(w.Message, "a.example.com") {
				t.Errorf("shadow finding should name the lost host: %v", w)
			}
		}
	}
	if shadowFindings != 1 {
		t.Errorf("want exactly one shadow finding, got %d", shadowFindings)
	}

	// Hostless registration wins over hosted ones regardless of order.
	mux2 := New(WithTitle("T"))
	mux2.HandleFunc("GET c.example.com/g", noop, Summary("Hosted"))
	mux2.HandleFunc("GET /g", noop, Summary("Generic"))
	mux2.HandleFunc("GET d.example.com/g", noop, Summary("Hosted too"))
	doc2 := buildDocMap(t, mux2)
	gop := doc2["paths"].(map[string]any)["/g"].(map[string]any)["get"].(map[string]any)
	if gop["summary"] != "Generic" {
		t.Errorf("hostless registration must win the document slot, got %v", gop["summary"])
	}
	if _, warned := gop["x-stdocs-warning"]; warned {
		t.Errorf("the hostless survivor needs no host warning")
	}

	// Both hosts still serve traffic.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://a.example.com/h", nil)
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("host routing must be unaffected: %d", rr.Code)
	}
}

// v0.4.1: every Lint finding carries a stable code; the new
// advisories fire.
func TestLintCodesAndNewAdvisories(t *testing.T) {
	type Contradiction struct {
		Country string `json:"country" default:"ES"` // required (no omitempty) + defaulted
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("POST /c", noop, Summary("C"), WithBody(Contradiction{}),
		WithResponse(400, nil), OperationID("explicit_7"))
	byCode := map[string]int{}
	for _, w := range mux.Lint() {
		if w.Code == "" {
			t.Errorf("finding without a code: %v", w)
		}
		byCode[w.Code]++
	}
	if byCode["required-with-default"] != 1 {
		t.Errorf("required-with-default advisory missing: %v", byCode)
	}
	if byCode["auto-descriptions"] != 1 {
		t.Errorf("auto-descriptions advisory missing: %v", byCode)
	}
	if byCode["dangling-id-suffix"] != 1 {
		t.Errorf("dangling-id-suffix advisory missing (explicit_7 has no explicit base): %v", byCode)
	}

	// Clean output silences the description advisory; omitempty
	// silences the contradiction.
	type Fine struct {
		Country string `json:"country,omitempty" default:"ES"`
	}
	quiet := New(WithTitle("T"), WithCleanOutput(true))
	quiet.HandleFunc("POST /c", noop, Summary("C"), WithBody(Fine{}), WithResponse(400, nil))
	for _, w := range quiet.Lint() {
		if w.Code == "required-with-default" || w.Code == "auto-descriptions" {
			t.Errorf("unexpected advisory on the clean mux: %v", w)
		}
	}
}

// v0.4.1: version segments skip tag inference, WithTagFunc overrides,
// and the completed ParamOpt vocabulary emits.
func TestTagInferenceAndParamVocabulary(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /v1/tasks", noop, Summary("L"))
	mux.HandleFunc("GET /v12/users/{id}", noop, Summary("G"))
	mux.HandleFunc("GET /vault/keys", noop, Summary("K")) // not a version segment
	doc := buildDocMap(t, mux)
	tag := func(p string) any {
		return doc["paths"].(map[string]any)[p].(map[string]any)["get"].(map[string]any)["tags"].([]any)[0]
	}
	if tag("/v1/tasks") != "Tasks" || tag("/v12/users/{id}") != "Users" {
		t.Errorf("version segments must not become tags: %v %v", tag("/v1/tasks"), tag("/v12/users/{id}"))
	}
	if tag("/vault/keys") != "Vault" {
		t.Errorf("non-version v-segments keep the old inference: %v", tag("/vault/keys"))
	}

	fmux := New(WithTitle("T"), WithTagFunc(func(method, path string) string {
		if strings.HasPrefix(path, "/admin/") {
			return "Admin"
		}
		return ""
	}))
	fmux.HandleFunc("GET /admin/keys", noop, Summary("K"))
	fmux.HandleFunc("GET /tasks", noop, Summary("L"))
	fdoc := buildDocMap(t, fmux)
	ftag := func(p string) any {
		return fdoc["paths"].(map[string]any)[p].(map[string]any)["get"].(map[string]any)["tags"].([]any)[0]
	}
	if ftag("/admin/keys") != "Admin" || ftag("/tasks") != "Tasks" {
		t.Errorf("WithTagFunc + fallback: %v %v", ftag("/admin/keys"), ftag("/tasks"))
	}

	pmux := New(WithTitle("T"))
	pmux.HandleFunc("GET /q", noop, Summary("Q"),
		QueryParam("ratio", "number", "", ParamExclusiveMinimum(0), ParamFormat("double")),
		QueryParam("ids", "array", "", ParamItems("integer"), ParamMinItems(1), ParamMaxItems(5), ParamUniqueItems()),
	)
	pdoc := buildDocMap(t, pmux)
	params := pdoc["paths"].(map[string]any)["/q"].(map[string]any)["get"].(map[string]any)["parameters"].([]any)
	byName := map[string]map[string]any{}
	for _, p := range params {
		pm := p.(map[string]any)
		byName[pm["name"].(string)] = pm["schema"].(map[string]any)
	}
	ratio := byName["ratio"]
	if ratio["exclusiveMinimum"] != true || ratio["minimum"] != json.Number("0") || ratio["format"] != "double" {
		t.Errorf("ratio schema (3.0 boolean form) = %#v", ratio)
	}
	ids := byName["ids"]
	if ids["items"].(map[string]any)["type"] != "integer" || ids["minItems"] != json.Number("1") ||
		ids["maxItems"] != json.Number("5") || ids["uniqueItems"] != true {
		t.Errorf("ids schema = %#v", ids)
	}

	for _, f := range []func(){
		func() { QueryParam("x", "integer", "", ParamMinimum(1), ParamExclusiveMinimum(0)) },
		func() { QueryParam("x", "string", "", ParamMinItems(1)) },
		func() { QueryParam("x", "array", "", ParamItems("object")) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic")
				}
			}()
			f()
		}()
	}
}

// v0.4.1 verification: registration is synchronized with serving —
// the late-registration feature must not be a data race — and a
// route registered mid-build is never permanently lost.
func TestConcurrentRegistrationAndServing(t *testing.T) {
	type APIError struct {
		Message string `json:"message"`
	}
	mux := New(WithTitle("T"), WithDefaultResponse(500, APIError{}))
	mux.HandleFunc("GET /seed", noop, Summary("Seed"))
	mux.Mount()
	h := DriftWarn(mux, func(string, ...any) {})

	var wg sync.WaitGroup
	for g := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 12 {
				mux.HandleFunc(fmt.Sprintf("GET /late/%d/%d", g, i), noop, Summary("Late"))
				mux.JSON()
				mux.YAML()
				mux.Lint()
				rr := httptest.NewRecorder()
				h.ServeHTTP(rr, httptest.NewRequest("GET", "/seed", nil))
			}
		}()
	}
	wg.Wait()
	doc := buildDocMap(t, mux)
	paths := doc["paths"].(map[string]any)
	for g := range 4 {
		for i := range 12 {
			if _, ok := paths[fmt.Sprintf("/late/%d/%d", g, i)]; !ok {
				t.Fatalf("route /late/%d/%d lost from the final document", g, i)
			}
		}
	}
}

// Operation ids depend only on the final route set, not on when
// intermediate builds happened.
func TestOperationIDBuildHistoryIndependence(t *testing.T) {
	build := func(interleave bool) string {
		mux := New(WithTitle("T"))
		mux.HandleFunc("GET /a", noop, Summary("A"), OperationID("x"))
		if interleave {
			mux.JSON()
		}
		mux.HandleFunc("GET /b", noop, Summary("B"), OperationID("x"))
		if interleave {
			mux.JSON()
		}
		mux.HandleFunc("GET /c", noop, Summary("C"), OperationID("x_2"))
		raw, err := mux.JSON()
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	plain, interleaved := build(false), build(true)
	if plain != interleaved {
		t.Errorf("ids depend on build history:\nplain:       %.300s\ninterleaved: %.300s", plain, interleaved)
	}
}

// A bad late registration panics at HandleFunc once a build happened.
func TestLateRegistrationFailsFast(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /ok", noop, Summary("OK"))
	mux.Mount() // builds eagerly
	defer func() {
		if recover() == nil {
			t.Errorf("late bad registration should panic at HandleFunc")
		}
	}()
	type Bad struct {
		N int `json:"n" minLength:"3"`
	}
	mux.HandleFunc("POST /bad", noop, WithBody(Bad{}))
}

// Optional without a body declaration materializes nothing.
func TestOptionalAloneIsNoOp(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /x", noop, Summary("X"), Optional())
	op := buildDocMap(t, mux)["paths"].(map[string]any)["/x"].(map[string]any)["get"].(map[string]any)
	if _, ok := op["requestBody"]; ok {
		t.Errorf("Optional alone must not materialize a request body")
	}
}

// v0.4.1: the pre-v0.4.1 manual asset registration plus Mount's new
// auto-registration coexist in both orders (no duplicate-pattern
// panic on upgrade).
func TestEmbAssetUpgradeCompatibility(t *testing.T) {
	assets := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("js")) })
	// Manual first (the v0.4.0 documented order), then Mount.
	m1 := New(WithTitle("T"))
	m1.Config().Assets = assets
	m1.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", assets))
	m1.Mount()
	rr := httptest.NewRecorder()
	m1.ServeHTTP(rr, httptest.NewRequest("GET", "/docs/_assets/x.js", nil))
	if rr.Code != http.StatusOK {
		t.Errorf("manual-then-Mount: %d", rr.Code)
	}
	// Mount first, then a (now-redundant) manual registration panics
	// in the USER's call — that is ServeMux's own conflict panic and
	// correctly attributed; only Mount's side must tolerate.
	m2 := New(WithTitle("T"))
	m2.Config().Assets = assets
	m2.Mount()
	rr2 := httptest.NewRecorder()
	m2.ServeHTTP(rr2, httptest.NewRequest("GET", "/docs/_assets/x.js", nil))
	if rr2.Code != http.StatusOK {
		t.Errorf("Mount-only: %d", rr2.Code)
	}
}

// v0.4.1 verification batch: uppercase version prefixes, wildcards
// after version segments, webhook security validation, and the
// dangling-id provenance fix.
func TestVerificationBatchC(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /V1/upper", noop, Summary("U"))
	mux.HandleFunc("GET /v1/{id}", noop, Summary("W"))
	mux.HandleFunc("GET /reports/2024", noop, Summary("R"), WithResponse(400, nil))
	doc := buildDocMap(t, mux)
	upper := doc["paths"].(map[string]any)["/V1/upper"].(map[string]any)["get"].(map[string]any)
	if tags, _ := upper["tags"].([]any); len(tags) != 1 || tags[0] != "Upper" {
		t.Errorf("/V1/upper tags = %v, want [Upper]", upper["tags"])
	}
	wild := doc["paths"].(map[string]any)["/v1/{id}"].(map[string]any)["get"].(map[string]any)
	if _, tagged := wild["tags"]; tagged {
		t.Errorf("a wildcard after the version prefix must not become a tag: %v", wild["tags"])
	}
	for _, w := range mux.Lint() {
		if w.Code == "dangling-id-suffix" {
			t.Errorf("auto-derived get_reports_2024 must not flag: %v", w)
		}
	}

	// Webhook security references must be validated like any other.
	bad := New(
		WithTitle("T"),
		WithVersion(OpenAPI31),
		WithBearerAuth("bearerAuth", "JWT"),
		WithWebhooks(map[string]Webhook{
			"x": {Method: "POST", Security: []SecurityRequirement{{"missing": nil}}},
		}),
	)
	bad.HandleFunc("GET /a", noop, Summary("A"))
	if _, err := bad.JSON(); err == nil || !strings.Contains(err.Error(), "webhook") {
		t.Errorf("unregistered webhook scheme must fail the build, got %v", err)
	}
}

// v0.4.2: route-scoped fallbacks and first-class raw responses.
func TestFallbackAndRawResponses(t *testing.T) {
	type LegacyError struct {
		Error string `json:"error"`
	}
	type Envelope struct {
		Message string `json:"message"`
	}
	legacy := Opts(WithFallbackResponse(500, LegacyError{}))
	mux := New(WithTitle("T"), WithDefaultResponse(500, Envelope{}))
	mux.HandleFunc("GET /old", noop, Summary("Old"), legacy)
	mux.HandleFunc("GET /new", noop, Summary("New"))
	mux.HandleFunc("GET /explicit", noop, Summary("E"), legacy, WithResponse(500, Envelope{}))
	mux.HandleFunc("GET /export", noop, Summary("X"),
		WithRawResponse(200, "text/csv"),
		WithResponseDescription(200, "CSV export"),
	)
	doc := buildDocMap(t, mux)
	ref := func(p string) string {
		r := doc["paths"].(map[string]any)[p].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["500"].(map[string]any)
		return r["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"].(string)
	}
	if ref("/old") != "#/components/schemas/LegacyError" {
		t.Errorf("/old 500 = %s; route fallback must beat the mux default", ref("/old"))
	}
	if ref("/new") != "#/components/schemas/Envelope" {
		t.Errorf("/new 500 = %s; mux default applies without a fallback", ref("/new"))
	}
	if ref("/explicit") != "#/components/schemas/Envelope" {
		t.Errorf("/explicit 500 = %s; explicit WithResponse beats the fallback", ref("/explicit"))
	}
	exp := doc["paths"].(map[string]any)["/export"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)
	content := exp["content"].(map[string]any)
	if _, ok := content["text/csv"]; !ok {
		t.Fatalf("raw response content = %v", mapKeysOf(content))
	}
	if content["text/csv"].(map[string]any)["schema"].(map[string]any)["type"] != "string" {
		t.Errorf("raw response schema must be string-typed")
	}
	if exp["description"] != "CSV export" {
		t.Errorf("raw responses compose with decorators: %v", exp["description"])
	}
	// Rebuild stability.
	b1, _ := mux.JSON()
	mux.Refresh()
	b2, _ := mux.JSON()
	if !bytes.Equal(b1, b2) {
		t.Errorf("fallbacks must be rebuild-stable")
	}

	defer func() {
		if recover() == nil {
			t.Errorf("bad fallback status should panic")
		}
	}()
	WithFallbackResponse(42, nil)
}

// v0.4.2 verification: fallback bodies fill decorator-created
// entries, explicit nil bodies win, raw/WithResponse replace each
// other in either order, and statuses validate everywhere.
func TestResponseDeclarationSemantics(t *testing.T) {
	type Envelope struct {
		Message string `json:"message"`
	}
	type OK struct {
		ID string `json:"id"`
	}
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /decorated", noop, Summary("D"),
		WithFallbackResponse(500, Envelope{}),
		WithResponseDescription(500, "Server error"), // creates the entry; fallback must still fill the body
	)
	mux.HandleFunc("GET /nilwins", noop, Summary("N"),
		WithResponse(500, nil), // explicit body-less declaration wins
		WithFallbackResponse(500, Envelope{}),
	)
	mux.HandleFunc("GET /rawthenresp", noop, Summary("R1"),
		WithRawResponse(200, "text/csv"),
		WithResponse(200, OK{}),
	)
	mux.HandleFunc("GET /respthenraw", noop, Summary("R2"),
		WithResponse(200, OK{}),
		WithRawResponse(200, "text/csv"),
	)
	doc := buildDocMap(t, mux)
	resp := func(p, status string) map[string]any {
		return doc["paths"].(map[string]any)[p].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)[status].(map[string]any)
	}
	dec := resp("/decorated", "500")
	if dec["description"] != "Server error" {
		t.Errorf("decorator description lost: %v", dec["description"])
	}
	if _, ok := dec["content"]; !ok {
		t.Errorf("fallback body must fill a decorator-created entry: %v", dec)
	}
	if _, ok := resp("/nilwins", "500")["content"]; ok {
		t.Errorf("an explicit nil body must beat the fallback")
	}
	r1 := resp("/rawthenresp", "200")["content"].(map[string]any)
	if _, ok := r1["application/json"]; !ok {
		t.Errorf("raw-then-WithResponse: JSON must fully win, got %v", mapKeysOf(r1))
	}
	r2 := resp("/respthenraw", "200")["content"].(map[string]any)
	if _, ok := r2["text/csv"]; !ok {
		t.Errorf("WithResponse-then-raw: raw must fully win, got %v", mapKeysOf(r2))
	}

	for _, f := range []func(){
		func() { WithRawResponse(99, "text/csv") },
		func() { WithResponse(42, nil) },
		func() { WithResponseDescription(1000, "x") },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("expected status panic")
				}
			}()
			f()
		}()
	}

	// Late-registered fallback bodies fail fast at registration.
	late := New(WithTitle("T"))
	late.HandleFunc("GET /seed", noop, Summary("S"))
	late.Mount()
	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("late fallback with a bad tag should panic at HandleFunc")
			}
		}()
		type Bad struct {
			N string `json:"n" minimum:"1"`
		}
		late.HandleFunc("GET /late", noop, Summary("L"), WithFallbackResponse(500, Bad{}))
	}()
}

// v0.4.2 verification: nullable Lint advisory and tag strictness.
func TestNullableSeams(t *testing.T) {
	type Risky struct {
		Tags []string `json:"tags" openapi:"nullable" uniqueItems:"true"`
		Note string   `json:"note,omitempty" openapi:"nullable" default:"hi"`
	}
	mux := New(WithTitle("T"), WithVersion(OpenAPI31))
	mux.HandleFunc("POST /r", noop, Summary("R"), WithBody(Risky{}), WithResponse(400, nil))
	hits := 0
	for _, w := range mux.Lint() {
		if w.Code == "nullable-facet-generators" {
			hits++
		}
	}
	if hits != 2 {
		t.Errorf("want 2 nullable-facet advisories, got %d", hits)
	}
	// 3.0 documents do not warn.
	mux30 := New(WithTitle("T"))
	mux30.HandleFunc("POST /r", noop, Summary("R"), WithBody(Risky{}), WithResponse(400, nil))
	for _, w := range mux30.Lint() {
		if w.Code == "nullable-facet-generators" {
			t.Errorf("3.0 must not warn: %v", w)
		}
	}

	for _, f := range []func(){
		func() {
			type T struct {
				X string `json:"x" openapi:"nullable,"`
			}
			mustReflect(T{})
		},
		func() {
			type T struct {
				X string `json:"x" openapi:"nullable=true"`
			}
			mustReflect(T{})
		},
		func() {
			type T struct {
				Ch chan int `json:"ch" openapi:"nullable"`
			}
			mustReflect(T{})
		},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic")
				}
			}()
			f()
		}()
	}
}

func mustReflect(v any) {
	m := New(WithTitle("T"))
	m.HandleFunc("POST /x", noop, WithBody(v))
	m.JSON()
}

// v0.4.2 user-sim findings: header descriptions on the Header
// Object, dots normalized in operationIds.
func TestHeaderDescriptionAndDotIDs(t *testing.T) {
	mux := New(WithTitle("T"))
	mux.HandleFunc("GET /v1/export/transactions.csv", noop, Summary("Export"),
		WithRawResponse(200, "text/csv"),
		WithResponseHeader(200, "Content-Disposition", "string", "attachment"),
	)
	doc := buildDocMap(t, mux)
	op := doc["paths"].(map[string]any)["/v1/export/transactions.csv"].(map[string]any)["get"].(map[string]any)
	if op["operationId"] != "get_v1_export_transactions_csv" {
		t.Errorf("operationId = %v, want dots normalized", op["operationId"])
	}
	hdr := op["responses"].(map[string]any)["200"].(map[string]any)["headers"].(map[string]any)["Content-Disposition"].(map[string]any)
	if hdr["description"] != "attachment" {
		t.Errorf("description belongs on the Header Object: %v", hdr)
	}
	if sch, ok := hdr["schema"].(map[string]any); ok {
		if _, leaked := sch["description"]; leaked {
			t.Errorf("description must not duplicate into the header schema: %v", sch)
		}
	}
}

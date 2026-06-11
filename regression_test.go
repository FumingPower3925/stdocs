package stdocs

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

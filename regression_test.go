package stdocs

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

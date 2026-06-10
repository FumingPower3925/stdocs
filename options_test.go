package stdocs

import (
	"reflect"
	"testing"

	"github.com/FumingPower3925/stdocs/internal/pattern"
)

// MustParsePattern is a test-only convenience wrapper.
func MustParsePattern(s string) *pattern.Pattern {
	p, err := pattern.ParsePattern(s)
	if err != nil {
		panic(err)
	}
	return p
}

func TestApplyOptions_Defaults(t *testing.T) {
	c := applyOptions(nil)
	if c.Info.Title != "API" {
		t.Errorf("Title = %q, want API", c.Info.Title)
	}
	if c.Info.Version != "0.0.0" {
		t.Errorf("Version = %q, want 0.0.0", c.Info.Version)
	}
	if c.DocsPrefix != "/docs" {
		t.Errorf("DocsPrefix = %q, want /docs", c.DocsPrefix)
	}
	if c.Version != OpenAPI30 {
		t.Errorf("Version = %q, want %q", c.Version, OpenAPI30)
	}
	if len(c.Servers) != 1 || c.Servers[0].URL != "/" {
		t.Errorf("Servers = %+v, want [{/}]", c.Servers)
	}
}

func TestWithTitle(t *testing.T) {
	c := applyOptions([]Option{WithTitle("My API")})
	if c.Info.Title != "My API" {
		t.Errorf("Title = %q", c.Info.Title)
	}
}

func TestWithVersion_Valid(t *testing.T) {
	for _, v := range []SpecVersion{"3.0.3", "3.1.0"} {
		c := applyOptions([]Option{WithVersion(v)})
		if c.Version != v {
			t.Errorf("WithVersion(%q): Version = %q, want %q", v, c.Version, v)
		}
	}
}

// TestWithVersion_Invalid asserts that an unknown version string
// panics. The previous behavior — silently coercing to 3.0.3 — was
// a footgun: a user typo (e.g. "3.1") would produce a different spec
// than the one they expected, with no error.
func TestWithVersion_Invalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on unknown version, got none")
		}
	}()
	_ = applyOptions([]Option{WithVersion(SpecVersion("2.0"))})
}

func TestWithDescription(t *testing.T) {
	c := applyOptions([]Option{WithDescription("Hello.")})
	if c.Info.Description != "Hello." {
		t.Errorf("Description = %q", c.Info.Description)
	}
}

func TestWithServer(t *testing.T) {
	c := applyOptions([]Option{WithServer("https://api.example.com", "prod")})
	if len(c.Servers) != 2 { // default + new
		t.Fatalf("Servers = %d, want 2", len(c.Servers))
	}
	if c.Servers[1].URL != "https://api.example.com" {
		t.Errorf("Servers[1].URL = %q", c.Servers[1].URL)
	}
}

func TestWithContact(t *testing.T) {
	c := applyOptions([]Option{WithContact("Me", "me@example.com", "https://me.example.com")})
	if c.Info.Contact == nil {
		t.Fatal("Contact nil")
	}
	if c.Info.Contact.Email != "me@example.com" {
		t.Errorf("Email = %q", c.Info.Contact.Email)
	}
}

func TestWithLicense(t *testing.T) {
	c := applyOptions([]Option{WithLicense("MIT", "https://opensource.org/licenses/MIT")})
	if c.Info.License == nil || c.Info.License.Name != "MIT" {
		t.Errorf("License = %+v", c.Info.License)
	}
}

func TestWithDocsPrefix(t *testing.T) {
	c := applyOptions([]Option{WithDocsPrefix("/api-docs")})
	if c.DocsPrefix != "/api-docs" {
		t.Errorf("DocsPrefix = %q", c.DocsPrefix)
	}
}

func TestWithDocsPrefix_LeadingSlashAdded(t *testing.T) {
	c := applyOptions([]Option{WithDocsPrefix("api-docs")})
	if c.DocsPrefix != "/api-docs" {
		t.Errorf("DocsPrefix = %q", c.DocsPrefix)
	}
}

func TestWithTag(t *testing.T) {
	c := applyOptions([]Option{WithTag("users", "User management")})
	if len(c.Tags) != 1 || c.Tags[0].Name != "users" {
		t.Errorf("Tags = %+v", c.Tags)
	}
}

func TestWithDefaultSummary(t *testing.T) {
	c := applyOptions([]Option{WithDefaultSummary("Manage {resource}")})
	if c.DefaultSummary != "Manage {resource}" {
		t.Errorf("DefaultSummary = %q", c.DefaultSummary)
	}
}

func TestSummaryFromFuncName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"getUser", "Get user"},
		{"listUsers", "List users"},
		{"deleteUser", "Delete user"},
		{"HandleUsers", "Users"},
		{"handlerUsers", "Users"},
		{"", ""},
		{"x", "X"},
		{"a", "A"},
		{"HTTPHandler", "HTTP handler"},
		{"parseXML", "Parse XML"},
		{"getAPI", "Get API"},
		{"createURL", "Create URL"},
	}
	for _, c := range cases {
		got := summaryFromFuncName(c.in)
		if got != c.want {
			t.Errorf("summaryFromFuncName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTagFromPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/users", "Users"},
		{"/users/{id}", "Users"},
		{"GET /users/{id}", "Users"},
		{"/v1/users", "V1"},
		{"/", ""},
		{"", ""},
		{"/{x}", "X"}, // first segment is a wildcard named "x"; we use the name as the tag
	}
	for _, c := range cases {
		got := tagFromPath(c.in)
		if got != c.want {
			t.Errorf("tagFromPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultOperationID(t *testing.T) {
	cases := []struct {
		pattern string
		want    string
	}{
		{"GET /users", "get_users"},
		{"GET /users/{id}", "get_users_by_id"},
		{"POST /v1/orders/{id}/items", "post_v1_orders_by_id_items"},
		{"/files/{path...}", "any_files_by_path_rest"},
		{"DELETE /users/{id}", "delete_users_by_id"},
	}
	for _, c := range cases {
		p := MustParsePattern(c.pattern)
		got := defaultOperationID(p)
		if got != c.want {
			t.Errorf("defaultOperationID(%q) = %q, want %q", c.pattern, got, c.want)
		}
	}
}

func TestRegistry_Add(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users/{id}")
	r.add("GET /users/{id}", "getUser", p, OpenAPI30, nil)
	if len(r.routes) != 1 {
		t.Errorf("len(routes) = %d, want 1", len(r.routes))
	}
}

func TestRegistry_Finalize_Auto200(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users")
	r.add("GET /users", "listUsers", p, OpenAPI30, nil)
	cfg := newConfig()
	r.finalize(cfg)
	rt := r.routes[0]
	if rt.op.Responses == nil {
		t.Fatal("Responses nil")
	}
	if _, ok := rt.op.Responses["200"]; !ok {
		t.Errorf("auto-200 missing")
	}
	if rt.op.Method != "GET" {
		t.Errorf("Method = %q", rt.op.Method)
	}
}

func TestRegistry_Finalize_PathParams(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users/{id}")
	r.add("GET /users/{id}", "getUser", p, OpenAPI30, nil)
	r.finalize(newConfig())
	rt := r.routes[0]
	if len(rt.op.Parameters) != 1 {
		t.Fatalf("Parameters = %d, want 1", len(rt.op.Parameters))
	}
	if rt.op.Parameters[0].Name != "id" {
		t.Errorf("Name = %q", rt.op.Parameters[0].Name)
	}
	if !rt.op.Parameters[0].Required {
		t.Errorf("Required = false, want true (path param)")
	}
}

func TestRegistry_Finalize_DefaultSummary(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users")
	r.add("GET /users", "listUsers", p, OpenAPI30, nil)
	r.finalize(newConfig())
	if r.routes[0].op.Summary != "List users" {
		t.Errorf("Summary = %q, want %q", r.routes[0].op.Summary, "List users")
	}
}

func TestRegistry_Finalize_DefaultTag(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users/{id}")
	r.add("GET /users/{id}", "getUser", p, OpenAPI30, nil)
	r.finalize(newConfig())
	tags := r.routes[0].op.Tags
	if len(tags) != 1 || tags[0] != "Users" {
		t.Errorf("Tags = %v, want [Users]", tags)
	}
}

func TestRegistry_Finalize_DefaultOperationID(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users/{id}")
	r.add("GET /users/{id}", "getUser", p, OpenAPI30, nil)
	r.finalize(newConfig())
	if r.routes[0].op.OperationID != "get_users_by_id" {
		t.Errorf("OperationID = %q", r.routes[0].op.OperationID)
	}
}

func TestRegistry_Finalize_OperationIDCollision(t *testing.T) {
	r := &registry{}
	r.add("GET /users/{id}", "getUser", MustParsePattern("GET /users/{id}"), OpenAPI30, nil)
	r.add("GET /users/{name}", "getUser", MustParsePattern("GET /users/{name}"), OpenAPI30, nil)
	r.add("GET /posts/{id}", "getPost", MustParsePattern("GET /posts/{id}"), OpenAPI30, nil)
	r.finalize(newConfig())
	got := []string{
		r.routes[0].op.OperationID,
		r.routes[1].op.OperationID,
		r.routes[2].op.OperationID,
	}
	want := []string{"get_users_by_id", "get_users_by_name", "get_posts_by_id"}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("routes[%d].operationId = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRegistry_Finalize_OperationIDUserOverride(t *testing.T) {
	r := &registry{}
	r.add("GET /a", "getA", MustParsePattern("GET /a"), OpenAPI30, nil)
	r.add("GET /b", "getB", MustParsePattern("GET /b"), OpenAPI30, nil)
	r.routes[0].op.OperationID = "do"
	r.routes[1].op.OperationID = "do"
	r.finalize(newConfig())
	if r.routes[0].op.OperationID != "do" {
		t.Errorf("first user-override changed: %q", r.routes[0].op.OperationID)
	}
	if r.routes[1].op.OperationID != "do_2" {
		t.Errorf("second collision not suffixed: %q", r.routes[1].op.OperationID)
	}
}

func TestRegistry_Finalize_NoMethodNoSummary(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("/")
	r.add("/", "x", p, OpenAPI30, nil)
	r.finalize(newConfig())
	// No method -> "any"; no useful summary from "x" -> "X"; path is "/"
	rt := r.routes[0]
	if rt.op.Method != "" {
		t.Errorf("Method = %q, want empty", rt.op.Method)
	}
}

func TestRegistry_Finalize_OverrideDefaults(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users")
	r.add("GET /users", "listUsers", p, OpenAPI30, []RouteOpt{
		Summary("List all users"),
		Tags("admin"),
	})
	r.finalize(newConfig())
	rt := r.routes[0]
	if rt.op.Summary != "List all users" {
		t.Errorf("Summary = %q", rt.op.Summary)
	}
	if len(rt.op.Tags) != 1 || rt.op.Tags[0] != "admin" {
		t.Errorf("Tags = %v", rt.op.Tags)
	}
}

func TestRegistry_ToPathItems_GroupsByPath(t *testing.T) {
	r := &registry{}
	p1 := MustParsePattern("GET /users")
	p2 := MustParsePattern("POST /users")
	r.add("GET /users", "listUsers", p1, OpenAPI30, nil)
	r.add("POST /users", "createUser", p2, OpenAPI30, nil)
	r.finalize(newConfig())
	items := r.toPathItems()
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].Path != "/users" {
		t.Errorf("Path = %q", items[0].Path)
	}
	if len(items[0].Operations) != 2 {
		t.Errorf("Operations = %d, want 2", len(items[0].Operations))
	}
}

func TestRegistry_ToPathItems_PathLevelParams(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /users/{id}/posts/{pid}")
	r.add("GET /users/{id}/posts/{pid}", "x", p, OpenAPI30, nil)
	r.finalize(newConfig())
	items := r.toPathItems()
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if len(items[0].Parameters) != 2 {
		t.Errorf("path-level params = %d, want 2", len(items[0].Parameters))
	}
}

func TestRouteOpt_Summary(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{Summary("hello")})
	if rt.op.Summary != "hello" {
		t.Errorf("Summary = %q", rt.op.Summary)
	}
}

func TestRouteOpt_Tags_Accumulates(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{Tags("a"), Tags("b", "c")})
	if len(rt.op.Tags) != 3 {
		t.Errorf("Tags = %v", rt.op.Tags)
	}
}

func TestRouteOpt_Response(t *testing.T) {
	type User struct {
		Name string `json:"name"`
	}
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{
		WithResponse(200, User{}),
		WithResponse(404, nil),
	})
	if len(rt.op.Responses) != 2 {
		t.Errorf("Responses = %d, want 2", len(rt.op.Responses))
	}
	r200 := rt.op.Responses["200"]
	if r200 == nil {
		t.Fatal("200 missing")
	}
	// User is a named struct, so the schema is a $ref.
	if r200.Schema == nil || r200.Schema.Ref != "#/components/schemas/User" {
		t.Errorf("200.Schema = %+v", r200.Schema)
	}
	r404 := rt.op.Responses["404"]
	if r404 == nil {
		t.Fatal("404 missing")
	}
	if r404.Schema != nil {
		t.Errorf("404.Schema should be nil")
	}
}

func TestRouteOpt_RequestBody(t *testing.T) {
	type Req struct {
		Title string `json:"title"`
	}
	r := &registry{}
	p := MustParsePattern("POST /x")
	rt := r.add("POST /x", "x", p, OpenAPI30, []RouteOpt{WithBody(Req{})})
	if rt.op.RequestBody == nil {
		t.Fatal("RequestBody nil")
	}
	if !rt.op.RequestBody.Required {
		t.Errorf("Required = false, want true by default")
	}
	if rt.op.RequestBody.Schema == nil {
		t.Fatal("Schema nil")
	}
}

func TestRouteOpt_Optional(t *testing.T) {
	type Req struct{}
	r := &registry{}
	p := MustParsePattern("POST /x")
	rt := r.add("POST /x", "x", p, OpenAPI30, []RouteOpt{WithBody(Req{}), Optional()})
	if rt.op.RequestBody.Required {
		t.Errorf("Required = true, want false after Optional()")
	}
}

func TestRouteOpt_QueryParam(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{QueryParam("q", "string", "search query")})
	if len(rt.op.Parameters) != 1 {
		t.Fatalf("Parameters = %d, want 1", len(rt.op.Parameters))
	}
	param := rt.op.Parameters[0]
	if param.In != "query" {
		t.Errorf("In = %q", param.In)
	}
	if param.Required {
		t.Errorf("Required = true, want false for query")
	}
}

func TestRouteOpt_HeaderParam(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{HeaderParam("X-Auth", "string", "auth token")})
	if len(rt.op.Parameters) != 1 {
		t.Fatalf("Parameters = %d, want 1", len(rt.op.Parameters))
	}
	if rt.op.Parameters[0].In != "header" {
		t.Errorf("In = %q", rt.op.Parameters[0].In)
	}
}

func TestRouteOpt_Param_ExplicitIn(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{WithParam("X-Trace", "header", "string", "trace")})
	if rt.op.Parameters[0].In != "header" {
		t.Errorf("In = %q", rt.op.Parameters[0].In)
	}
}

func TestRouteOpt_OperationID(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{OperationID("custom-id")})
	if rt.op.OperationID != "custom-id" {
		t.Errorf("OperationID = %q", rt.op.OperationID)
	}
}

func TestRouteOpt_Description(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{Description("long form")})
	if rt.op.Description != "long form" {
		t.Errorf("Description = %q", rt.op.Description)
	}
}

func TestRouteOpt_Deprecated(t *testing.T) {
	r := &registry{}
	p := MustParsePattern("GET /x")
	rt := r.add("GET /x", "x", p, OpenAPI30, []RouteOpt{Deprecated()})
	if !rt.op.Deprecated {
		t.Errorf("Deprecated = false, want true")
	}
}

func TestStatusKey(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{200, "200"},
		{404, "404"},
		{500, "500"},
		{0, "default"},
	}
	for _, c := range cases {
		if got := statusKey(c.in); got != c.want {
			t.Errorf("statusKey(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDefaultResponseDescription(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{404, "Not Found"},
		{500, "Internal Server Error"},
		{418, "Response"},
	}
	for _, c := range cases {
		if got := defaultResponseDescription(c.in); got != c.want {
			t.Errorf("defaultResponseDescription(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSchemaForType(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"string", "string"},
		{"integer", "integer"},
		{"number", "number"},
		{"boolean", "boolean"},
		{"unknown", ""},
	}
	for _, c := range cases {
		s := schemaForType(c.in, OpenAPI30)
		if s.Type != c.want {
			t.Errorf("schemaForType(%q) = Type %q, want %q", c.in, s.Type, c.want)
		}
	}
}

// Suppress unused warnings on reflect (kept for future-proofing).
var _ = reflect.TypeOf

package spec

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// jx is a small JSON-extract helper for tests. It unmarshals into a
// generic map and returns a sub-tree at "dotted.path" of keys.
func jx(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// jget returns m["a"]["b"]... as any. Fails the test if any key is missing.
func jget(t *testing.T, m map[string]any, path ...string) any {
	t.Helper()
	cur := any(m)
	for _, k := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("jget: %v is not a map at %v", cur, path)
		}
		v, ok := mm[k]
		if !ok {
			t.Fatalf("jget: key %q missing at %v", k, path)
		}
		cur = v
	}
	return cur
}

func TestEmitOpenAPI30_TopLevelShape(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "My API", Version: "1.0.0"},
		Version: version.OpenAPI30,
	}
	b, err := EmitOpenAPI30(in)
	if err != nil {
		t.Fatal(err)
	}
	m := jx(t, b)
	if got := m["openapi"]; got != "3.0.4" {
		t.Errorf("openapi = %v, want 3.0.4", got)
	}
	if got := jget(t, m, "info", "title"); got != "My API" {
		t.Errorf("info.title = %v, want My API", got)
	}
	if got := jget(t, m, "info", "version"); got != "1.0.0" {
		t.Errorf("info.version = %v, want 1.0.0", got)
	}
	if _, ok := m["paths"]; !ok {
		t.Errorf("paths missing")
	}
}

func TestEmitOpenAPI30_PathAndMethod(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/users",
				Operations: map[string]*Operation{
					"get": {Summary: "List users"},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	op := jget(t, m, "paths", "/users", "get").(map[string]any)
	if op["summary"] != "List users" {
		t.Errorf("summary = %v, want List users", op["summary"])
	}
}

func TestEmitOpenAPI30_PathParamsBecomeRequired(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/users/{id}",
				Operations: map[string]*Operation{
					"get": {
						Parameters: []Param{
							{Name: "id", In: "path", Schema: &schema.Schema{Type: "string"}},
						},
					},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	ps := jget(t, m, "paths", "/users/{id}", "get", "parameters").([]any)
	if len(ps) != 1 {
		t.Fatalf("params = %d, want 1", len(ps))
	}
	p := ps[0].(map[string]any)
	if p["required"] != true {
		t.Errorf("path param required = %v, want true", p["required"])
	}
}

func TestEmitOpenAPI30_QueryParamNotRequiredByDefault(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/search",
				Operations: map[string]*Operation{
					"get": {
						Parameters: []Param{
							{Name: "q", In: "query", Schema: &schema.Schema{Type: "string"}},
						},
					},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	ps := jget(t, m, "paths", "/search", "get", "parameters").([]any)
	p := ps[0].(map[string]any)
	if _, has := p["required"]; has {
		t.Errorf("query param should not have required key when false, got %v", p["required"])
	}
}

func TestEmitOpenAPI30_QueryParamRequired(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/search",
				Operations: map[string]*Operation{
					"get": {
						Parameters: []Param{
							{Name: "q", In: "query", Required: true, Schema: &schema.Schema{Type: "string"}},
						},
					},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	ps := jget(t, m, "paths", "/search", "get", "parameters").([]any)
	p := ps[0].(map[string]any)
	if p["required"] != true {
		t.Errorf("required = %v, want true", p["required"])
	}
}

func TestEmitOpenAPI30_RequestBody(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/users",
				Operations: map[string]*Operation{
					"post": {
						RequestBody: &RequestBody{
							Required: true,
							Schema:   &schema.Schema{Type: "object", Properties: map[string]*schema.Schema{"name": {Type: "string"}}},
						},
					},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	rb := jget(t, m, "paths", "/users", "post", "requestBody").(map[string]any)
	if rb["required"] != true {
		t.Errorf("required = %v, want true", rb["required"])
	}
	ct := jget(t, rb, "content", "application/json").(map[string]any)
	sch := ct["schema"].(map[string]any)
	if sch["type"] != "object" {
		t.Errorf("schema.type = %v, want object", sch["type"])
	}
}

func TestEmitOpenAPI30_Responses(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/users",
				Operations: map[string]*Operation{
					"get": {
						Responses: map[string]*Response{
							"200": {Description: "OK", Schema: &schema.Schema{Type: "array", Items: &schema.Schema{Type: "object"}}},
							"404": {Description: "Not Found"},
						},
					},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	resp := jget(t, m, "paths", "/users", "get", "responses").(map[string]any)
	if resp["200"] == nil {
		t.Fatal("200 missing")
	}
	if resp["404"] == nil {
		t.Fatal("404 missing")
	}
	r200 := resp["200"].(map[string]any)
	if r200["description"] != "OK" {
		t.Errorf("200.description = %v, want OK", r200["description"])
	}
}

func TestEmitOpenAPI30_Default200WhenNoResponses(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/users",
				Operations: map[string]*Operation{
					"get": {Summary: "x"},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	resp := jget(t, m, "paths", "/users", "get", "responses").(map[string]any)
	if resp["200"] == nil {
		t.Errorf("default 200 missing")
	}
}

func TestEmitOpenAPI30_Nullable(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Components: map[string]*schema.Schema{
			"User": {
				Type:     "object",
				Nullable: true,
				Properties: map[string]*schema.Schema{
					"name": {Type: "string", Nullable: true},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	// Nullable on the top-level component should produce "nullable": true.
	user := jget(t, m, "components", "schemas", "User").(map[string]any)
	if user["nullable"] != true {
		t.Errorf("User nullable = %v, want true", user["nullable"])
	}
	name := jget(t, m, "components", "schemas", "User", "properties", "name").(map[string]any)
	if name["nullable"] != true {
		t.Errorf("name nullable = %v, want true", name["nullable"])
	}
}

func TestEmitOpenAPI30_ComponentsRefs(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Components: map[string]*schema.Schema{
			"User": {Type: "object", Properties: map[string]*schema.Schema{"id": {Type: "string"}}},
		},
		Paths: []PathItem{
			{
				Path: "/users/{id}",
				Operations: map[string]*Operation{
					"get": {
						Responses: map[string]*Response{
							"200": {Description: "OK", Schema: &schema.Schema{Ref: "#/components/schemas/User"}},
						},
					},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	ref := jget(t, m, "paths", "/users/{id}", "get", "responses", "200", "content", "application/json", "schema").(map[string]any)
	if ref["$ref"] != "#/components/schemas/User" {
		t.Errorf("$ref = %v", ref["$ref"])
	}
	// The component itself should be expanded.
	user := jget(t, m, "components", "schemas", "User").(map[string]any)
	if user["type"] != "object" {
		t.Errorf("User.type = %v, want object", user["type"])
	}
}

func TestEmitOpenAPI30_Extensions(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Components: map[string]*schema.Schema{
			"X": {
				Type: "object",
				Extensions: map[string]any{
					"x-stdocs-type": "interface",
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	x := jget(t, m, "components", "schemas", "X").(map[string]any)
	if x["x-stdocs-type"] != "interface" {
		t.Errorf("x-stdocs-type = %v, want interface", x["x-stdocs-type"])
	}
}

func TestEmitOpenAPI30_DeterministicOrder(t *testing.T) {
	// Two calls with the same input must produce the same bytes.
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{Path: "/b", Operations: map[string]*Operation{"get": {}}},
			{Path: "/a", Operations: map[string]*Operation{"get": {}}},
		},
	}
	b1, _ := EmitOpenAPI30(in)
	b2, _ := EmitOpenAPI30(in)
	if string(b1) != string(b2) {
		t.Errorf("non-deterministic:\n%s\n%s", b1, b2)
	}
	// /a should come before /b.
	if !strings.Contains(string(b1), `"/a"`) || !strings.Contains(string(b1), `"/b"`) {
		t.Errorf("expected both /a and /b in output, got %s", b1)
	}
	ia := strings.Index(string(b1), `"/a"`)
	ib := strings.Index(string(b1), `"/b"`)
	if ia > ib {
		t.Errorf("expected /a before /b, got positions %d and %d", ia, ib)
	}
}

func TestEmitOpenAPI30_TagsOnOperation(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Tags: []TagDecl{
			{Name: "users", Description: "User ops"},
		},
		Paths: []PathItem{
			{
				Path: "/users",
				Operations: map[string]*Operation{
					"get": {Tags: []string{"users"}},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	tags := jget(t, m, "paths", "/users", "get", "tags").([]any)
	if len(tags) != 1 || tags[0] != "users" {
		t.Errorf("tags = %v, want [users]", tags)
	}
	declTags := jget(t, m, "tags").([]any)
	if len(declTags) != 1 {
		t.Errorf("declared tags = %d, want 1", len(declTags))
	}
}

func TestEmitOpenAPI30_OperationID(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/users",
				Operations: map[string]*Operation{
					"get": {OperationID: "list-users"},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	if got := jget(t, m, "paths", "/users", "get", "operationId"); got != "list-users" {
		t.Errorf("operationId = %v, want list-users", got)
	}
}

func TestEmitOpenAPI30_InfoContactAndLicense(t *testing.T) {
	in := SpecInput{
		Info: Info{
			Title:   "T",
			Version: "0.0.0",
			Contact: &Contact{Name: "Me", Email: "me@example.com"},
			License: &License{Name: "MIT"},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	c := jget(t, m, "info", "contact").(map[string]any)
	if c["email"] != "me@example.com" {
		t.Errorf("contact.email = %v", c["email"])
	}
	l := jget(t, m, "info", "license").(map[string]any)
	if l["name"] != "MIT" {
		t.Errorf("license.name = %v", l["name"])
	}
}

func TestEmitOpenAPI30_Servers(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Servers: []Server{{URL: "https://api.example.com", Description: "prod"}},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	servers := jget(t, m, "servers").([]any)
	if len(servers) != 1 {
		t.Fatalf("servers = %d", len(servers))
	}
	s := servers[0].(map[string]any)
	if s["url"] != "https://api.example.com" {
		t.Errorf("url = %v", s["url"])
	}
}

func TestEmitOpenAPI30_PathLevelParameters(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/users/{id}",
				Parameters: []Param{
					{Name: "id", In: "path", Schema: &schema.Schema{Type: "string"}},
				},
				Operations: map[string]*Operation{
					"get": {Summary: "x"},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	ps := jget(t, m, "paths", "/users/{id}", "parameters").([]any)
	if len(ps) != 1 {
		t.Errorf("path-level params = %d, want 1", len(ps))
	}
}

func TestEmitOpenAPI30_Enum(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Version: version.OpenAPI30,
		Components: map[string]*schema.Schema{
			"Color": {Type: "string", Enum: []any{"red", "green", "blue"}},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	enum := jget(t, m, "components", "schemas", "Color", "enum").([]any)
	if len(enum) != 3 {
		t.Errorf("enum = %v", enum)
	}
}

func TestEmitOpenAPI30_ArrayWithItems(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Version: version.OpenAPI30,
		Components: map[string]*schema.Schema{
			"List": {Type: "array", Items: &schema.Schema{Type: "string"}},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	list := jget(t, m, "components", "schemas", "List").(map[string]any)
	if list["type"] != "array" {
		t.Errorf("type = %v", list["type"])
	}
	items := list["items"].(map[string]any)
	if items["type"] != "string" {
		t.Errorf("items.type = %v", items["type"])
	}
}

func TestEmitOpenAPI30_MapWithAdditionalProperties(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Version: version.OpenAPI30,
		Components: map[string]*schema.Schema{
			"Bag": {Type: "object", AdditionalProperties: &schema.Schema{Type: "integer"}},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	bag := jget(t, m, "components", "schemas", "Bag").(map[string]any)
	ap := bag["additionalProperties"].(map[string]any)
	if ap["type"] != "integer" {
		t.Errorf("ap.type = %v", ap["type"])
	}
}

func TestEmitOpenAPI30_EmptySchema(t *testing.T) {
	// A Schema with no fields and no Ref should emit {}.
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Version: version.OpenAPI30,
		Components: map[string]*schema.Schema{
			"X": {},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	x := jget(t, m, "components", "schemas", "X").(map[string]any)
	if len(x) != 0 {
		t.Errorf("X = %v, want empty", x)
	}
}

func TestEmitOpenAPI30_RequiredFields(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Version: version.OpenAPI30,
		Components: map[string]*schema.Schema{
			"User": {
				Type: "object",
				Properties: map[string]*schema.Schema{
					"id":   {Type: "string"},
					"name": {Type: "string"},
				},
				Required: []string{"id", "name"},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	u := jget(t, m, "components", "schemas", "User").(map[string]any)
	req := u["required"].([]any)
	if len(req) != 2 || req[0] != "id" || req[1] != "name" {
		t.Errorf("required = %v", req)
	}
}

func TestEmitOpenAPI30_DeprecatedFlag(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/old",
				Operations: map[string]*Operation{
					"get": {Deprecated: true},
				},
			},
		},
	}
	b, _ := EmitOpenAPI30(in)
	m := jx(t, b)
	if got := jget(t, m, "paths", "/old", "get", "deprecated"); got != true {
		t.Errorf("deprecated = %v, want true", got)
	}
}

// End-to-end: build a realistic spec by reflecting a Go type.
func TestEmitOpenAPI30_EndToEndWithReflect(t *testing.T) {
	type User struct {
		ID    string `json:"id" doc:"unique id"`
		Name  string `json:"name"`
		Email string `json:"email,omitempty"`
	}
	type T struct {
		User *User `json:"user"`
	}
	_, comps := schema.ReflectSchema(T{})
	in := SpecInput{
		Info:       Info{Title: "T", Version: "0.0.0"},
		Components: comps,
		Paths: []PathItem{
			{
				Path: "/me",
				Operations: map[string]*Operation{
					"get": {
						Responses: map[string]*Response{
							"200": {Description: "OK", Schema: &schema.Schema{Ref: "#/components/schemas/T"}},
						},
					},
				},
			},
		},
	}
	b, err := EmitOpenAPI30(in)
	if err != nil {
		t.Fatal(err)
	}
	m := jx(t, b)
	// T should be in components.
	tComp := jget(t, m, "components", "schemas", "T").(map[string]any)
	props := tComp["properties"].(map[string]any)
	if _, ok := props["user"]; !ok {
		t.Errorf("T.user missing")
	}
	// User should also be there.
	userComp := jget(t, m, "components", "schemas", "User").(map[string]any)
	userProps := userComp["properties"].(map[string]any)
	// "id" and "name" should be required; "email" should not (omitempty).
	req := userComp["required"].([]any)
	if len(req) != 2 {
		t.Errorf("User required = %v, want 2 entries", req)
	}
	if _, ok := userProps["id"]; !ok {
		t.Errorf("User.id missing")
	}
}

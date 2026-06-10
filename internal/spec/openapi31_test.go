package spec

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

func TestEmitOpenAPI31_TopLevelShape(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Version: version.OpenAPI31,
	}
	b, err := EmitOpenAPI31(in)
	if err != nil {
		t.Fatal(err)
	}
	m := jx(t, b)
	if got := m["openapi"]; got != "3.1.2" {
		t.Errorf("openapi = %v, want 3.1.2", got)
	}
}

func TestEmitOpenAPI31_NullableBecomesTypeArray(t *testing.T) {
	// Reflect a pointer type so Nullable=true is set.
	type T struct {
		Name *string `json:"name"`
	}
	_, comps := schema.ReflectSchema(T{}, version.OpenAPI31)
	in := SpecInput{
		Info:       Info{Title: "T", Version: "0.0.0"},
		Components: comps,
		Version:    version.OpenAPI31,
	}
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	tComp := jget(t, m, "components", "schemas", "T").(map[string]any)
	name := jget(t, tComp, "properties", "name").(map[string]any)

	// In 3.1, the type field is a JSON array.
	arr, ok := name["type"].([]any)
	if !ok {
		t.Fatalf("name.type = %T, want []any", name["type"])
	}
	if len(arr) != 2 || arr[0] != "string" || arr[1] != "null" {
		t.Errorf("name.type = %v, want [string null]", arr)
	}
	// "nullable" must NOT be present in 3.1.
	if _, has := name["nullable"]; has {
		t.Errorf("name.nullable should not be present in 3.1")
	}
}

func TestEmitOpenAPI31_PlainTypeStillString(t *testing.T) {
	// A non-nullable type should still emit a plain string "type".
	type T struct {
		Name string `json:"name"`
	}
	_, comps := schema.ReflectSchema(T{}, version.OpenAPI31)
	in := SpecInput{
		Info:       Info{Title: "T", Version: "0.0.0"},
		Components: comps,
		Version:    version.OpenAPI31,
	}
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	tComp := jget(t, m, "components", "schemas", "T").(map[string]any)
	name := jget(t, tComp, "properties", "name").(map[string]any)
	if name["type"] != "string" {
		t.Errorf("name.type = %v, want string", name["type"])
	}
}

func TestEmitOpenAPI31_ComponentsRef(t *testing.T) {
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
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	ref := jget(t, m, "paths", "/users/{id}", "get", "responses", "200", "content", "application/json", "schema").(map[string]any)
	if ref["$ref"] != "#/components/schemas/User" {
		t.Errorf("$ref = %v", ref["$ref"])
	}
}

func TestEmitOpenAPI31_DeterministicOrder(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{Path: "/z", Operations: map[string]*Operation{"get": {}}},
			{Path: "/a", Operations: map[string]*Operation{"get": {}}},
		},
	}
	b1, _ := EmitOpenAPI31(in)
	b2, _ := EmitOpenAPI31(in)
	if string(b1) != string(b2) {
		t.Errorf("non-deterministic")
	}
	ia := strings.Index(string(b1), `"/a"`)
	iz := strings.Index(string(b1), `"/z"`)
	if ia > iz {
		t.Errorf("expected /a before /z, got %d and %d", ia, iz)
	}
}

func TestEmitOpenAPI31_PathParamsRequired(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/u/{id}",
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
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	ps := jget(t, m, "paths", "/u/{id}", "get", "parameters").([]any)
	p := ps[0].(map[string]any)
	if p["required"] != true {
		t.Errorf("required = %v", p["required"])
	}
}

func TestEmitOpenAPI31_RequestBodyJSON(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Paths: []PathItem{
			{
				Path: "/x",
				Operations: map[string]*Operation{
					"post": {
						RequestBody: &RequestBody{
							Required: true,
							Schema:   &schema.Schema{Type: "object"},
						},
					},
				},
			},
		},
	}
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	rb := jget(t, m, "paths", "/x", "post", "requestBody").(map[string]any)
	ct := jget(t, rb, "content", "application/json").(map[string]any)
	if ct["schema"].(map[string]any)["type"] != "object" {
		t.Errorf("schema.type = %v", ct["schema"])
	}
}

func TestEmitOpenAPI31_ArrayItems(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Components: map[string]*schema.Schema{
			"L": {Type: "array", Items: &schema.Schema{Type: "string"}},
		},
	}
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	l := jget(t, m, "components", "schemas", "L").(map[string]any)
	items := l["items"].(map[string]any)
	if items["type"] != "string" {
		t.Errorf("items.type = %v", items["type"])
	}
}

func TestEmitOpenAPI31_MapAdditionalProperties(t *testing.T) {
	in := SpecInput{
		Info: Info{Title: "T", Version: "0.0.0"},
		Components: map[string]*schema.Schema{
			"M": {Type: "object", AdditionalProperties: &schema.Schema{Type: "integer"}},
		},
	}
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	mc := jget(t, m, "components", "schemas", "M").(map[string]any)
	ap := mc["additionalProperties"].(map[string]any)
	if ap["type"] != "integer" {
		t.Errorf("ap.type = %v", ap["type"])
	}
}

func TestEmitOpenAPI31_EmptySchema(t *testing.T) {
	in := SpecInput{
		Info:    Info{Title: "T", Version: "0.0.0"},
		Version: version.OpenAPI31,
		Components: map[string]*schema.Schema{
			"E": {},
		},
	}
	b, _ := EmitOpenAPI31(in)
	m := jx(t, b)
	e := jget(t, m, "components", "schemas", "E").(map[string]any)
	if len(e) != 0 {
		t.Errorf("E = %v, want empty", e)
	}
}

func TestEmitOpenAPI31_31HasNoNullableField(t *testing.T) {
	// Belt-and-suspenders: search the raw JSON output for "nullable":
	// when emitting 3.1, it must not appear.
	type T struct {
		X *string `json:"x"`
	}
	_, comps := schema.ReflectSchema(T{}, version.OpenAPI31)
	in := SpecInput{
		Info:       Info{Title: "T", Version: "0.0.0"},
		Components: comps,
		Version:    version.OpenAPI31,
	}
	b, _ := EmitOpenAPI31(in)
	if strings.Contains(string(b), `"nullable"`) {
		t.Errorf("3.1 output should not contain `nullable`; got: %s", b)
	}
}

// Cross-version: the same input should produce structurally equivalent
// specs with only the version-specific differences.
func TestEmitOpenAPI31_CrossVersionDifferences(t *testing.T) {
	type T struct {
		Field *int `json:"field"`
	}
	_, comps := schema.ReflectSchema(T{}, version.OpenAPI30)
	in30 := SpecInput{
		Info:       Info{Title: "T", Version: "0.0.0"},
		Components: comps,
		Version:    version.OpenAPI30,
	}
	_, comps31 := schema.ReflectSchema(T{}, version.OpenAPI31)
	in31 := SpecInput{
		Info:       Info{Title: "T", Version: "0.0.0"},
		Components: comps31,
		Version:    version.OpenAPI31,
	}
	b30, _ := EmitOpenAPI30(in30)
	b31, _ := EmitOpenAPI31(in31)

	var m30, m31 map[string]any
	json.Unmarshal(b30, &m30)
	json.Unmarshal(b31, &m31)

	// Same info, paths, components structure.
	{
		j30, _ := json.Marshal(m30["info"])
		j31, _ := json.Marshal(m31["info"])
		if string(j30) != string(j31) {
			t.Errorf("info differs:\n%s\n%s", j30, j31)
		}
	}
	// Different openapi field.
	if m30["openapi"] == m31["openapi"] {
		t.Errorf("expected different openapi versions")
	}
	// 3.0 has nullable:true for the field; 3.1 has a type array.
	field30 := jget(t, m30, "components", "schemas", "T", "properties", "field").(map[string]any)
	field31 := jget(t, m31, "components", "schemas", "T", "properties", "field").(map[string]any)
	if _, ok := field30["nullable"]; !ok {
		t.Errorf("3.0.4 should have nullable: true on field")
	}
	if _, ok := field31["nullable"]; ok {
		t.Errorf("3.1.2 should NOT have nullable on field")
	}
	if _, ok := field31["type"].([]any); !ok {
		t.Errorf("3.1.2 field type should be a JSON array")
	}
}

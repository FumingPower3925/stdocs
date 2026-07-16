package schema

import (
	"encoding/json"
	"maps"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// Helper: build a schema and assert it has the expected JSON when serialized
// to OpenAPI 3.0.3.
func schema30(t *testing.T, value any) (*Schema, map[string]*Schema) {
	t.Helper()
	return ReflectSchema(value)
}

func schema31(t *testing.T, value any) (*Schema, map[string]*Schema) {
	t.Helper()
	return ReflectSchema(value)
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestReflectSchema_String(t *testing.T) {
	s, _ := schema30(t, "")
	if s.Type != "string" {
		t.Errorf("Type = %q, want string", s.Type)
	}
}

func TestReflectSchema_Bool(t *testing.T) {
	s, _ := schema30(t, true)
	if s.Type != "boolean" {
		t.Errorf("Type = %q, want boolean", s.Type)
	}
}

func TestReflectSchema_IntegerSizes(t *testing.T) {
	cases := []struct {
		value  any
		format string
	}{
		{int(0), "int64"},
		{int8(0), "int32"},
		{int16(0), "int32"},
		{int32(0), "int32"},
		{int64(0), "int64"},
		{uint(0), "int64"},
		{uint8(0), "int32"},
		{uint16(0), "int32"},
		{uint32(0), "int64"},
		{uint64(0), "int64"},
	}
	for _, c := range cases {
		s, _ := schema30(t, c.value)
		if s.Type != "integer" {
			t.Errorf("%T: Type = %q, want integer", c.value, s.Type)
		}
		if s.Format != c.format {
			t.Errorf("%T: Format = %q, want %q", c.value, s.Format, c.format)
		}
	}
}

func TestReflectSchema_FloatSizes(t *testing.T) {
	if s, _ := schema30(t, float32(0)); s.Format != "float" {
		t.Errorf("float32 Format = %q, want float", s.Format)
	}
	if s, _ := schema30(t, float64(0)); s.Format != "double" {
		t.Errorf("float64 Format = %q, want double", s.Format)
	}
}

func TestReflectSchema_Time(t *testing.T) {
	s, _ := schema30(t, time.Time{})
	if s.Type != "string" || s.Format != "date-time" {
		t.Errorf("Type = %q, Format = %q, want string/date-time", s.Type, s.Format)
	}
}

func TestReflectSchema_Pointer(t *testing.T) {
	// *string is a nullable string.
	v := (*string)(nil)
	s, _ := schema30(t, v)
	if s.Type != "string" {
		t.Errorf("Type = %q, want string", s.Type)
	}
	if !s.Nullable {
		t.Errorf("Nullable = false, want true")
	}
	// In 3.0.3, Nullable stays as a separate field; in 3.1.0, the
	// emitter renders it as a "type" array at serialization time.
	// The model preserves Type="string" + Nullable=true and lets
	// buildSchema31 produce the type array.
	s31, _ := schema31(t, v)
	if s31.Type != "string" {
		t.Errorf("Type = %q, want string (preserved in 3.1 for type-array rendering)", s31.Type)
	}
	if !s31.Nullable {
		t.Errorf("Nullable = false, want true (preserved in 3.1)")
	}
}

func TestReflectSchema_PointerToPointer(t *testing.T) {
	// **string is still just nullable string.
	v := (**string)(nil)
	s, _ := schema30(t, v)
	if s.Type != "string" || !s.Nullable {
		t.Errorf("got Type=%q Nullable=%v, want string/true", s.Type, s.Nullable)
	}
}

func TestReflectSchema_Slice(t *testing.T) {
	s, _ := schema30(t, []string{})
	if s.Type != "array" {
		t.Errorf("Type = %q, want array", s.Type)
	}
	if s.Items == nil || s.Items.Type != "string" {
		t.Errorf("Items = %+v, want string", s.Items)
	}
}

func TestReflectSchema_SliceOfBytes(t *testing.T) {
	s, _ := schema30(t, []byte{})
	if s.Type != "string" || s.Format != "byte" {
		t.Errorf("Type = %q, Format = %q, want string/byte", s.Type, s.Format)
	}
}

func TestReflectSchema_Array(t *testing.T) {
	s, _ := schema30(t, [3]int{})
	if s.Type != "array" {
		t.Errorf("Type = %q, want array", s.Type)
	}
	if s.Items == nil || s.Items.Type != "integer" {
		t.Errorf("Items = %+v, want integer", s.Items)
	}
}

func TestReflectSchema_Map(t *testing.T) {
	s, _ := schema30(t, map[string]int{})
	if s.Type != "object" {
		t.Errorf("Type = %q, want object", s.Type)
	}
	if s.AdditionalProperties == nil || s.AdditionalProperties.Type != "integer" {
		t.Errorf("AdditionalProperties = %+v, want integer", s.AdditionalProperties)
	}
}

func TestReflectSchema_Interface(t *testing.T) {
	// The real test: a field of type any in a struct should produce a
	// schema with no type and the x-stdocs-type extension.
	type Holder struct {
		Value any `json:"value"`
	}
	_, out := schema30(t, Holder{})
	comp := out["Holder"]
	val := comp.Properties["value"]
	if val == nil {
		t.Fatal("value property missing")
	}
	if val.Type != "" {
		t.Errorf("Type = %q, want empty for interface", val.Type)
	}
	if val.Extensions["x-stdocs-type"] != "interface" {
		t.Errorf("Extensions[x-stdocs-type] = %v, want interface", val.Extensions["x-stdocs-type"])
	}
}

func TestReflectSchema_ChannelSkipped(t *testing.T) {
	type HasChan struct {
		Name string
		Ch   chan int `json:"-"` // explicit
	}
	_ = HasChan{}
	// Implicit: a field of type chan int should be skipped in the emitted
	// schema. We test via json tag handling below.
}

func TestReflectSchema_StructBasic(t *testing.T) {
	type Person struct {
		Name string `json:"name" doc:"Full name"`
		Age  int    `json:"age"`
	}
	s, _ := schema30(t, Person{})
	if s.Ref == "" {
		t.Fatalf("expected $ref for named struct, got Type=%q", s.Type)
	}
	if s.Ref != "#/components/schemas/Person" {
		t.Errorf("Ref = %q, want #/components/schemas/Person", s.Ref)
	}
}

func TestReflectSchema_StructComponentPopulated(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	_, out := schema30(t, Person{})
	if _, ok := out["Person"]; !ok {
		t.Fatalf("components.schemas[Person] missing; got %v", mapKeys(out))
	}
}

func TestReflectSchema_StructFields(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
	}
	_, out := schema30(t, Address{})
	comp := out["Address"]
	if comp == nil {
		t.Fatal("Address component missing")
	}
	if comp.Type != "object" {
		t.Errorf("Type = %q, want object", comp.Type)
	}
	if _, ok := comp.Properties["street"]; !ok {
		t.Errorf("Properties[street] missing; got %v", mapKeys(comp.Properties))
	}
	if _, ok := comp.Properties["city"]; !ok {
		t.Errorf("Properties[city] missing")
	}
	// All fields are required (no omitempty, no pointer).
	if len(comp.Required) != 2 {
		t.Errorf("Required = %v, want [street city]", comp.Required)
	}
}

func TestReflectSchema_OptionalField(t *testing.T) {
	type T struct {
		Name  string  `json:"name"`
		Other *string `json:"other"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}
	// "name" is required; "other" is not (pointer field).
	if len(comp.Required) != 1 || comp.Required[0] != "name" {
		t.Errorf("Required = %v, want [name]", comp.Required)
	}
	if !comp.Properties["other"].Nullable {
		t.Errorf("Properties[other].Nullable = false, want true")
	}
}

func TestReflectSchema_Omitempty(t *testing.T) {
	type T struct {
		Maybe string `json:"maybe,omitempty"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}
	if len(comp.Required) != 0 {
		t.Errorf("Required = %v, want [] (omitempty should drop the field from required)", comp.Required)
	}
}

func TestReflectSchema_JsonNameOverride(t *testing.T) {
	type T struct {
		UserID string `json:"id"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if _, ok := comp.Properties["id"]; !ok {
		t.Errorf("Properties[id] missing; got %v", mapKeys(comp.Properties))
	}
	if _, ok := comp.Properties["UserID"]; ok {
		t.Errorf("Properties[UserID] should not exist (renamed to id)")
	}
}

func TestReflectSchema_JsonSkip(t *testing.T) {
	type T struct {
		Shown  string `json:"shown"`
		Hidden string `json:"-"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if _, ok := comp.Properties["shown"]; !ok {
		t.Errorf("shown missing")
	}
	if _, ok := comp.Properties["Hidden"]; ok {
		t.Errorf("Hidden should be skipped")
	}
}

func TestReflectSchema_UnexportedField(t *testing.T) {
	type T struct {
		Exported string `json:"exp"`
		_        string // unexported placeholder
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if _, ok := comp.Properties["exp"]; !ok {
		t.Errorf("exp missing")
	}
	if _, ok := comp.Properties["unexported"]; ok {
		t.Errorf("unexported should be skipped (lowercase = unexported)")
	}
}

func TestReflectSchema_EmbeddedStructFlattened(t *testing.T) {
	type Base struct {
		ID string `json:"id"`
	}
	type User struct {
		Base
		Name string `json:"name"`
	}
	_, out := schema30(t, User{})
	comp := out["User"]
	if comp == nil {
		t.Fatal("User component missing")
	}
	if _, ok := comp.Properties["id"]; !ok {
		t.Errorf("embedded id missing; got %v", mapKeys(comp.Properties))
	}
	if _, ok := comp.Properties["name"]; !ok {
		t.Errorf("name missing; got %v", mapKeys(comp.Properties))
	}
}

func TestReflectSchema_EmbeddedStructFlattened_RequiredDedup(t *testing.T) {
	type Base struct {
		ID    string `json:"id"`
		Owner string `json:"owner"`
	}
	type User struct {
		Base
		// Same name as in Base — non-embedded should win for the
		// property, but the "required" list must contain "id" only once.
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_, out := schema30(t, User{})
	comp := out["User"]
	if comp == nil {
		t.Fatal("User component missing")
	}
	count := 0
	for _, r := range comp.Required {
		if r == "id" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("required contains id %d times, want 1; got %v", count, comp.Required)
	}
}

func TestReflectSchema_RecursiveType(t *testing.T) {
	type Node struct {
		Value    string  `json:"value"`
		Children []*Node `json:"children"`
	}
	_, out := schema30(t, Node{})
	comp := out["Node"]
	if comp == nil {
		t.Fatal("Node component missing")
	}
	children := comp.Properties["children"]
	if children == nil {
		t.Fatal("children property missing")
	}
	if children.Type != "array" {
		t.Errorf("children Type = %q, want array", children.Type)
	}
	if children.Items == nil {
		t.Fatal("children.Items missing")
	}
	if children.Items.Ref != "#/components/schemas/Node" {
		t.Errorf("children.Items.Ref = %q, want #/components/schemas/Node", children.Items.Ref)
	}
}

func TestReflectSchema_ChannelFieldSkipped(t *testing.T) {
	type T struct {
		Name string `json:"name"`
		Ch   chan int
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}
	if _, ok := comp.Properties["name"]; !ok {
		t.Errorf("name missing")
	}
	if _, ok := comp.Properties["Ch"]; ok {
		t.Errorf("Ch should be skipped (chan type)")
	}
}

func TestReflectSchema_FunctionFieldSkipped(t *testing.T) {
	type T struct {
		Name string `json:"name"`
		Fn   func() error
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if _, ok := comp.Properties["Fn"]; ok {
		t.Errorf("Fn should be skipped (func type)")
	}
}

func TestReflectSchema_DocTag(t *testing.T) {
	type T struct {
		Name string `json:"name" doc:"the user's display name"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if got := comp.Properties["name"].Description; got != "the user's display name" {
		t.Errorf("Description = %q, want doc tag value", got)
	}
}

func TestReflectSchema_GenericInstantiation(t *testing.T) {
	type Box[T any] struct {
		Value T `json:"value"`
	}
	type Concrete struct {
		Ints Box[int]    `json:"ints"`
		Strs Box[string] `json:"strs"`
	}
	_, out := schema30(t, Concrete{})
	comp := out["Concrete"]
	if comp == nil {
		t.Fatal("Concrete component missing")
	}
	ints := comp.Properties["ints"]
	if ints == nil {
		t.Fatal("ints property missing")
	}
	// ints is a $ref to a simplified component name: package
	// qualifiers and local-type markers drop, brackets become
	// underscores (Box[int] -> Box_int).
	if ints.Ref != "#/components/schemas/Box_int" {
		t.Errorf("ints.Ref = %q, want #/components/schemas/Box_int", ints.Ref)
	}
	boxInt := out["Box_int"]
	if boxInt == nil {
		t.Fatal("Box_int component missing")
	}
	v := boxInt.Properties["value"]
	if v == nil || v.Type != "integer" {
		t.Errorf("Box_int.value.Type = %v, want integer", v)
	}
}

func TestReflectSchema_31NullableAsTypeArray(t *testing.T) {
	type T struct {
		Name *string `json:"name"`
	}
	s, out := schema31(t, T{})
	comp := out["T"]
	name := comp.Properties["name"]
	if name == nil {
		t.Fatal("name missing")
	}
	// In 3.1.0, the model keeps Type="string" + Nullable=true; the
	// emitter renders the type array at serialization time.
	if name.Type != "string" {
		t.Errorf("name.Type = %q, want string (preserved in 3.1 for type-array rendering)", name.Type)
	}
	if !name.Nullable {
		t.Errorf("name.Nullable = false, want true (preserved in 3.1)")
	}
	_ = s
}

func TestReflectSchema_ComponentsAreJSONSerializable(t *testing.T) {
	type T struct {
		Field string `json:"field"`
	}
	_ = T{}
	_, out := schema30(t, T{})
	for name, s := range out {
		b, err := json.Marshal(s) //nolint:musttag // Schema is a runtime type, not a wire DTO
		if err != nil {
			t.Errorf("marshal %q: %v", name, err)
		}
		if len(b) == 0 {
			t.Errorf("marshal %q produced empty bytes", name)
		}
	}
}

func TestReflectSchema_NilValue(t *testing.T) {
	s, _ := schema30(t, nil)
	if s == nil {
		t.Fatal("schema for nil must not be nil")
	}
}

func TestReflectSchema_PointerToStructIsRef(t *testing.T) {
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		I *Inner `json:"i"`
	}
	_, out := schema30(t, Outer{})
	comp := out["Outer"]
	if comp == nil {
		t.Fatal("Outer component missing")
	}
	i := comp.Properties["i"]
	if i == nil {
		t.Fatal("i property missing")
	}
	if i.Type != "object" && i.Ref == "" {
		// It's nullable, so it should still be a $ref but with
		// the nullable flag set.
		t.Errorf("i = %+v, want object or ref", i)
	}
}

// TestReflectSchema_31NullableStructKeepsStructure guards the
// regression where a nullable struct field in 3.1 mode produced a
// bare `{"type":["object","null"]}` with no properties, because
// the reflector mutated Type to "" and TypeArray, and the 3.1
// emitter gated structural emission on s.Type == "object".
//
// For named structs, the model emits a $ref to a shared component
// (not an inlined object). The use-site nullability is wrapped at
// serialization time (allOf for 3.0, anyOf for 3.1) — see Phase 4's
// follow-up work for showstopper 8. Here we just confirm the
// *model* is consistent.
func TestReflectSchema_31NullableStructKeepsStructure(t *testing.T) {
	type Inner struct {
		X int    `json:"x"`
		Y string `json:"y"`
	}
	type Outer struct {
		I *Inner `json:"i"`
	}
	s, out := schema31(t, Outer{})
	if s.Ref == "" {
		t.Fatalf("Outer should be a $ref, got %+v", s)
	}
	comp := out["Outer"]
	if comp == nil {
		t.Fatal("Outer component missing")
	}
	i := comp.Properties["i"]
	if i == nil {
		t.Fatal("i property missing")
	}
	// Pointer-to-named-struct: emit a $ref. The model must NOT
	// bake nullability into the shared Inner component.
	inner := out["Inner"]
	if inner == nil {
		t.Fatal("Inner component missing")
	}
	if inner.Nullable {
		t.Errorf("Inner component should NOT be nullable; use-site handles nullability")
	}
	// The use site is a $ref. The emitter is responsible for
	// wrapping it in anyOf/nullable at serialization time
	// (handled in the emitter pass).
	if i.Ref != "#/components/schemas/Inner" {
		t.Errorf("i.Ref = %q, want #/components/schemas/Inner", i.Ref)
	}
	_ = i.Nullable // intentionally not asserted: the model
	// preserves Nullable=true on the use-site ref, and the
	// emitter serializes it as the wrapper form.
}

// TestReflectSchema_31NullableSliceKeepsItems guards the same bug
// for slices: a `[]int` (or `*[]int`) field in 3.1 mode must keep
// its `items` schema.
func TestReflectSchema_31NullableSliceKeepsItems(t *testing.T) {
	type Outer struct {
		IDs *[]int `json:"ids"`
	}
	_, out := schema31(t, Outer{})
	comp := out["Outer"]
	if comp == nil {
		t.Fatal("Outer component missing")
	}
	ids := comp.Properties["ids"]
	if ids == nil {
		t.Fatal("ids property missing")
	}
	if ids.Type != "array" {
		t.Errorf("ids.Type = %q, want array (preserved in 3.1 for type-array rendering)", ids.Type)
	}
	if !ids.Nullable {
		t.Errorf("ids.Nullable = false, want true")
	}
	if ids.Items == nil {
		t.Fatal("ids.Items missing — the bug cleared Type and the emitter dropped the items branch")
	}
	if ids.Items.Type != "integer" {
		t.Errorf("ids.Items.Type = %q, want integer", ids.Items.Type)
	}
}

// TestReflectSchema_31NullableMapKeepsValue guards the same bug
// for maps: a `*map[string]int` field in 3.1 mode must keep its
// additionalProperties schema.
func TestReflectSchema_31NullableMapKeepsValue(t *testing.T) {
	type Outer struct {
		Bag *map[string]int `json:"bag"`
	}
	_, out := schema31(t, Outer{})
	comp := out["Outer"]
	if comp == nil {
		t.Fatal("Outer component missing")
	}
	bag := comp.Properties["bag"]
	if bag == nil {
		t.Fatal("bag property missing")
	}
	if bag.Type != "object" {
		t.Errorf("bag.Type = %q, want object (preserved in 3.1)", bag.Type)
	}
	if !bag.Nullable {
		t.Errorf("bag.Nullable = false, want true")
	}
	if bag.AdditionalProperties == nil {
		t.Fatal("bag.AdditionalProperties missing — the bug cleared Type and the emitter dropped the additionalProperties branch")
	}
	if bag.AdditionalProperties.Type != "integer" {
		t.Errorf("bag.AdditionalProperties.Type = %q, want integer", bag.AdditionalProperties.Type)
	}
}

// TestReflectSchema_ComponentNameCollision guards showstopper 9:
// when the *requested* name is already taken in r.out, the
// reflector must pick a suffixed name rather than overwriting the
// existing component.
func TestReflectSchema_ComponentNameCollision(t *testing.T) {
	// The reflector chooses a name by: try t.Name() (or the
	// sanitized form for generic types), then walk "User",
	// "User_2", "User_3"... until a free slot is found. We
	// exercise that walk directly via the reservation helper.
	r := NewReflector()
	if got := r.reserveName("User"); got != "User" {
		t.Errorf("first reserveName(User) = %q, want User", got)
	}
	r.out["User"] = &Schema{Type: "object"}
	if got := r.reserveName("User"); got != "User_2" {
		t.Errorf("second reserveName(User) = %q, want User_2", got)
	}
	r.out["User_2"] = &Schema{Type: "object"}
	if got := r.reserveName("User"); got != "User_3" {
		t.Errorf("third reserveName(User) = %q, want User_3", got)
	}
}

// TestIsValidComponentName guards the OpenAPI 3.x pointer-fragment
// character-class rule: [a-zA-Z0-9._-]. Anything else (notably
// "[" and "]" from generic instantiations) must trigger the
// sanitization path.
func TestIsValidComponentName(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"User", true},
		{"Page_int_", true},
		{"User_2", true},
		{"pkg.Type", true},
		{"a-b-c", true},
		{"Box[int]", false},
		{"Page[T]", false},
		{"", false},
		{"foo bar", false},
		{"foo/bar", false},
		{"foo:bar", false},
	} {
		if got := isValidComponentName(tc.in); got != tc.want {
			t.Errorf("isValidComponentName(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestSanitizeComponentName verifies the character replacements.
func TestSanitizeComponentName(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		// Each illegal char becomes a single underscore; runs are
		// collapsed and trailing underscores trimmed.
		{"Page[Foo]", "Page_Foo"},
		{"Page[pkg.User]", "Page_pkg_User"},
		{"pkg/with/slash.Type", "pkg_with_slash_Type"},
		{"", "Schema"},
		{"  ", "Schema"},
		{"[", "Schema"},
		{"foo bar baz", "foo_bar_baz"},
		{"  foo  ", "foo"},
		{"123", "Schema_123"}, // leading digit is illegal as a JSON
		// pointer-fragment start; "Schema_" prefix makes it valid.
	} {
		if got := sanitizeComponentName(tc.in); got != tc.want {
			t.Errorf("sanitizeComponentName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestReflectSchema_MapWithComplexValue(t *testing.T) {
	type Item struct {
		Tag string `json:"tag"`
	}
	s, out := schema30(t, map[string]Item{})
	if s.Type != "object" {
		t.Errorf("Type = %q, want object", s.Type)
	}
	if s.AdditionalProperties == nil {
		t.Fatal("AdditionalProperties missing")
	}
	if s.AdditionalProperties.Ref != "#/components/schemas/Item" {
		t.Errorf("AdditionalProperties.Ref = %q, want #/components/schemas/Item", s.AdditionalProperties.Ref)
	}
	if _, ok := out["Item"]; !ok {
		t.Errorf("Item component missing")
	}
}

func TestReflectSchema_NestedSlicesAndMaps(t *testing.T) {
	s, _ := schema30(t, map[string][]int{})
	if s.Type != "object" {
		t.Errorf("Type = %q, want object", s.Type)
	}
	if s.AdditionalProperties == nil || s.AdditionalProperties.Type != "array" {
		t.Errorf("AdditionalProperties = %+v", s.AdditionalProperties)
	}
	if s.AdditionalProperties.Items == nil || s.AdditionalProperties.Items.Type != "integer" {
		t.Errorf("AdditionalProperties.Items = %+v", s.AdditionalProperties.Items)
	}
}

func TestReflectSchema_ComplexNumberHasExtension(t *testing.T) {
	s, _ := schema30(t, complex64(0))
	if s.Type != "string" {
		t.Errorf("Type = %q, want string (complex has no JSON form)", s.Type)
	}
	if s.Extensions["x-stdocs-type"] != "complex" {
		t.Errorf("Extensions[x-stdocs-type] = %v, want complex", s.Extensions["x-stdocs-type"])
	}
}

func TestReflectSchema_JsonMarshalStable(t *testing.T) {
	type T struct {
		Field string `json:"field"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	// Marshal twice; the output should be the same.
	first := mustMarshal(t, comp)
	second := mustMarshal(t, comp)
	if first != second {
		t.Errorf("non-deterministic marshal: %s vs %s", first, second)
	}
	// Sanity-check the field is present.
	if !contains(first, `"field"`) {
		t.Errorf("expected `field` in %s", first)
	}
}

// contains reports whether substr is in s.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mapKeys returns the keys of a string-keyed map.
func mapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestReflectSchema_TypeIdentity(t *testing.T) {
	// Two zero values of the same named type should produce the same Ref.
	type T struct {
		A int `json:"a"`
	}
	s1, _ := schema30(t, T{})
	s2, _ := schema30(t, T{})
	if s1.Ref != s2.Ref || s1.Ref == "" {
		t.Errorf("expected stable Ref for same type, got %q and %q", s1.Ref, s2.Ref)
	}
}

func TestReflectSchema_EmptyStruct(t *testing.T) {
	type T struct{}
	s, out := schema30(t, T{})
	if s.Ref != "#/components/schemas/T" {
		t.Errorf("Ref = %q", s.Ref)
	}
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}
	if len(comp.Properties) != 0 {
		t.Errorf("Properties = %v, want empty", comp.Properties)
	}
}

// Reflecting on a non-struct with no value (a nil typed pointer) should not
// panic.
func TestReflectSchema_NoPanicOnNilTypedPointer(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic: %v", r)
		}
	}()
	var p *int
	schema30(t, p)
}

func TestReflectSchema_RecursiveResolution(t *testing.T) {
	// Recursive type: Tree has Children []*Tree.
	// Reflection depth should be limited by the seen map.
	type Tree struct {
		Label    string  `json:"label"`
		Children []*Tree `json:"children"`
	}
	s, out := schema30(t, Tree{})
	comp := out["Tree"]
	if comp == nil {
		t.Fatal("Tree component missing")
	}
	children := comp.Properties["children"]
	if children == nil || children.Type != "array" {
		t.Fatalf("children = %+v", children)
	}
	if children.Items.Ref != "#/components/schemas/Tree" {
		t.Errorf("children.Items.Ref = %q, want #/components/schemas/Tree", children.Items.Ref)
	}
	_ = s
}

func TestReflectSchema_ReusedAcrossStructs(t *testing.T) {
	// Two structs share a sub-struct; it should appear once in components.
	type Common struct {
		ID string `json:"id"`
	}
	type A struct {
		Common Common `json:"common"`
		Name   string `json:"name"`
	}
	type B struct {
		Common Common `json:"common"`
		Other  string `json:"other"`
	}
	_, out := schema30(t, A{})
	if _, ok := out["Common"]; !ok {
		t.Errorf("Common missing from components")
	}
	// B is also reflected, so both should be in components.
	_, out2 := schema30(t, B{})
	if _, ok := out2["Common"]; !ok {
		t.Errorf("Common missing after reflecting B")
	}
}

func TestReflectSchema_30NullableField(t *testing.T) {
	type T struct {
		Name *string `json:"name"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	name := comp.Properties["name"]
	if name.Nullable != true {
		t.Errorf("Nullable = false, want true in 3.0.3")
	}
	if name.Type != "string" {
		t.Errorf("Type = %q, want string in 3.0.3", name.Type)
	}
}

// Suppress unused-import warning on the reflect import.
var _ = reflect.TypeOf

// Typed example parsing: example tags must be converted to the field's
// schema type so the emitted example does not violate its own schema.
func TestReflectSchema_TypedExamples(t *testing.T) {
	type T struct {
		Count  int     `json:"count" example:"42"`
		Ratio  float64 `json:"ratio" example:"0.75"`
		Active bool    `json:"active" example:"true"`
		Label  string  `json:"label" example:"hello"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}
	if got := comp.Properties["count"].Example; got != int64(42) {
		t.Errorf("count example = %#v, want int64(42)", got)
	}
	if got := comp.Properties["ratio"].Example; got != 0.75 {
		t.Errorf("ratio example = %#v, want 0.75", got)
	}
	if got := comp.Properties["active"].Example; got != true {
		t.Errorf("active example = %#v, want true", got)
	}
	if got := comp.Properties["label"].Example; got != "hello" {
		t.Errorf("label example = %#v, want hello", got)
	}
}

func TestReflectSchema_InvalidExamplePanics(t *testing.T) {
	type T struct {
		Count int `json:"count" example:"forty-two"`
	}
	defer func() {
		if recover() == nil {
			t.Errorf("unparseable example on an integer field should panic")
		}
	}()
	_, _ = ReflectSchema(T{})
}

// The constraint tag vocabulary: parsed per type, applied to the
// field's own schema.
func TestReflectSchema_ConstraintTags(t *testing.T) {
	type T struct {
		Title    string  `json:"title" minLength:"1" maxLength:"200" pattern:"^[a-z].*$"`
		Priority int     `json:"priority" minimum:"1" maximum:"5" default:"3"`
		Ratio    float64 `json:"ratio" exclusiveMinimum:"0" exclusiveMaximum:"1.5"`
		Status   string  `json:"status" enum:"pending,active,done" default:"pending"`
		Codes    []int   `json:"codes" minItems:"1" maxItems:"10" uniqueItems:"true"`
		Email    string  `json:"email" format:"email"`
		Level    int     `json:"level" enum:"1,2,3"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}
	title := comp.Properties["title"]
	if *title.MinLength != 1 || *title.MaxLength != 200 || title.Pattern != "^[a-z].*$" {
		t.Errorf("title constraints = %v/%v/%q", title.MinLength, title.MaxLength, title.Pattern)
	}
	prio := comp.Properties["priority"]
	if prio.Minimum != "1" || prio.Maximum != "5" || prio.Default != int64(3) {
		t.Errorf("priority constraints = %q/%q/%#v", prio.Minimum, prio.Maximum, prio.Default)
	}
	ratio := comp.Properties["ratio"]
	if ratio.ExclusiveMinimum != "0" || ratio.ExclusiveMaximum != "1.5" {
		t.Errorf("ratio exclusive bounds = %q/%q", ratio.ExclusiveMinimum, ratio.ExclusiveMaximum)
	}
	status := comp.Properties["status"]
	if len(status.Enum) != 3 || status.Enum[0] != "pending" || status.Default != "pending" {
		t.Errorf("status enum/default = %#v / %#v", status.Enum, status.Default)
	}
	codes := comp.Properties["codes"]
	if *codes.MinItems != 1 || *codes.MaxItems != 10 || !codes.UniqueItems {
		t.Errorf("codes constraints = %v/%v/%v", codes.MinItems, codes.MaxItems, codes.UniqueItems)
	}
	if comp.Properties["email"].Format != "email" {
		t.Errorf("email format = %q, want email", comp.Properties["email"].Format)
	}
	level := comp.Properties["level"]
	if len(level.Enum) != 3 || level.Enum[0] != int64(1) {
		t.Errorf("integer enum = %#v, want typed int64 members", level.Enum)
	}
}

// v0.9.0 (#116): on a slice or array field the scalar constraints
// describe the ELEMENTS, so they land on Items; the array-length tags
// keep describing the array itself.
func TestReflectSchema_ItemConstraintTags(t *testing.T) {
	type T struct {
		Sev    []string  `json:"sev" enum:"info,low,high"`
		Codes  []int     `json:"codes" enum:"1,2,3"`
		Mails  []string  `json:"mails" format:"email"`
		Names  []string  `json:"names" minLength:"2" maxLength:"8" pattern:"^[a-z]+$"`
		Ratios []float64 `json:"ratios" minimum:"0" maximum:"1"`
		Ports  []uint16  `json:"ports" exclusiveMinimum:"0"`
		Tags   []string  `json:"tags" minItems:"1" maxItems:"5" uniqueItems:"true"`
		Fixed  [3]int    `json:"fixed" minimum:"1"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}

	sev := comp.Properties["sev"]
	if sev.Type != "array" || len(sev.Enum) != 0 {
		t.Errorf("the enum must not land on the array itself: %+v", sev)
	}
	if len(sev.Items.Enum) != 3 || sev.Items.Enum[0] != "info" {
		t.Errorf("sev items enum = %#v", sev.Items.Enum)
	}
	// The element type — not "array" — must reach parseScalar, or an int
	// slice's members would silently emit as JSON strings.
	if got := comp.Properties["codes"].Items.Enum; len(got) != 3 || got[0] != int64(1) {
		t.Errorf("codes items enum = %#v, want typed int64 members", got)
	}
	if got := comp.Properties["mails"].Items.Format; got != "email" {
		t.Errorf("mails items format = %q", got)
	}
	names := comp.Properties["names"].Items
	if *names.MinLength != 2 || *names.MaxLength != 8 || names.Pattern != "^[a-z]+$" {
		t.Errorf("names items constraints = %v/%v/%q", names.MinLength, names.MaxLength, names.Pattern)
	}
	ratios := comp.Properties["ratios"].Items
	if ratios.Minimum != "0" || ratios.Maximum != "1" {
		t.Errorf("ratios items bounds = %q/%q", ratios.Minimum, ratios.Maximum)
	}
	// The unsigned auto-minimum lives on the element schema too, so an
	// exclusive bound displaces it there.
	ports := comp.Properties["ports"].Items
	if ports.ExclusiveMinimum != "0" || ports.Minimum != "" {
		t.Errorf("ports items = exclusiveMinimum %q, minimum %q; want the auto-minimum displaced",
			ports.ExclusiveMinimum, ports.Minimum)
	}

	// The array-length tags stay on the array.
	tags := comp.Properties["tags"]
	if *tags.MinItems != 1 || *tags.MaxItems != 5 || !tags.UniqueItems {
		t.Errorf("tags array constraints = %v/%v/%v", tags.MinItems, tags.MaxItems, tags.UniqueItems)
	}
	if tags.Items.MinLength != nil {
		t.Errorf("minItems must not leak onto the elements: %+v", tags.Items)
	}
	// A Go array's elements count too.
	if got := comp.Properties["fixed"].Items.Minimum; got != "1" {
		t.Errorf("fixed items minimum = %q", got)
	}
}

// An array-length tag on a slice of structs carries no element
// constraint, so the element guard must not fire on it.
func TestReflectSchema_ArrayLengthOnStructSlice(t *testing.T) {
	type Inner struct {
		X string `json:"x"`
	}
	type T struct {
		Items []Inner `json:"items" maxItems:"3"`
	}
	_, out := schema30(t, T{})
	got := out["T"].Properties["items"]
	if got.MaxItems == nil || *got.MaxItems != 3 {
		t.Errorf("maxItems on a struct slice = %v, want 3", got.MaxItems)
	}
}

// Misapplied or unparseable constraint tags panic at build time.
func TestReflectSchema_ConstraintTagPanics(t *testing.T) {
	cases := []struct {
		name    string
		reflect func()
	}{
		{"minLength on int", func() {
			type T struct {
				N int `json:"n" minLength:"1"`
			}
			ReflectSchema(T{})
		}},
		{"minimum on string", func() {
			type T struct {
				S string `json:"s" minimum:"1"`
			}
			ReflectSchema(T{})
		}},
		{"minItems on string", func() {
			type T struct {
				S string `json:"s" minItems:"1"`
			}
			ReflectSchema(T{})
		}},
		{"uniqueItems on int", func() {
			type T struct {
				N int `json:"n" uniqueItems:"true"`
			}
			ReflectSchema(T{})
		}},
		{"unparseable minimum", func() {
			type T struct {
				N int `json:"n" minimum:"low"`
			}
			ReflectSchema(T{})
		}},
		{"negative minLength", func() {
			type T struct {
				S string `json:"s" minLength:"-1"`
			}
			ReflectSchema(T{})
		}},
		{"unparseable enum member", func() {
			type T struct {
				N int `json:"n" enum:"1,two,3"`
			}
			ReflectSchema(T{})
		}},
		{"unparseable default", func() {
			type T struct {
				N int `json:"n" default:"none"`
			}
			ReflectSchema(T{})
		}},
		{"minimum and exclusiveMinimum together", func() {
			type T struct {
				N int `json:"n" minimum:"0" exclusiveMinimum:"0"`
			}
			ReflectSchema(T{})
		}},
		{"constraint on struct-typed field", func() {
			type Inner struct {
				X string `json:"x"`
			}
			type T struct {
				I Inner `json:"i" minimum:"1"`
			}
			ReflectSchema(T{})
		}},
		// v0.9.0 (#116): scalar constraints retarget to a slice's
		// elements, so the rejections move to the elements that cannot
		// carry them — and to the two value tags that cannot say which
		// level they mean.
		{"default on slice is ambiguous", func() {
			type T struct {
				V []string `json:"v" default:"a"`
			}
			ReflectSchema(T{})
		}},
		{"example on slice is ambiguous", func() {
			type T struct {
				V []string `json:"v" example:"a"`
			}
			ReflectSchema(T{})
		}},
		{"enum on slice of structs", func() {
			type Inner struct {
				X string `json:"x"`
			}
			type T struct {
				V []Inner `json:"v" enum:"a,b"`
			}
			ReflectSchema(T{})
		}},
		{"enum on slice of slices", func() {
			type T struct {
				V [][]string `json:"v" enum:"a,b"`
			}
			ReflectSchema(T{})
		}},
		{"format on slice of maps", func() {
			type T struct {
				V []map[string]int `json:"v" format:"email"`
			}
			ReflectSchema(T{})
		}},
		// A nil element schema must not be dereferenced.
		{"enum on slice of funcs", func() {
			type T struct {
				V []func() `json:"v" enum:"a,b"`
			}
			ReflectSchema(T{})
		}},
		{"minLength on int elements", func() {
			type T struct {
				V []int `json:"v" minLength:"1"`
			}
			ReflectSchema(T{})
		}},
		{"pattern on int elements", func() {
			type T struct {
				V []int `json:"v" pattern:"^a$"`
			}
			ReflectSchema(T{})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expected panic")
				}
			}()
			tc.reflect()
		})
	}
}

type renamedTask struct {
	ID string `json:"id"`
}

func (renamedTask) SchemaName() string { return "Task" }

type ptrNamed struct {
	X string `json:"x"`
}

func (*ptrNamed) SchemaName() string { return "Named via pointer!" }

// Component-name control: SchemaName overrides, generic
// instantiations simplify, collisions still suffix.
func TestComponentNaming(t *testing.T) {
	r := NewReflector()
	s := r.Reflect(renamedTask{})
	if s.Ref != "#/components/schemas/Task" {
		t.Errorf("SchemaName override: ref = %q", s.Ref)
	}
	if _, ok := r.Components()["Task"]; !ok {
		t.Errorf("component should register under the override name")
	}

	// Pointer-receiver namers work too, and the result is sanitized.
	r2 := NewReflector()
	s2 := r2.Reflect(ptrNamed{})
	if s2.Ref != "#/components/schemas/Named_via_pointer" {
		t.Errorf("pointer-receiver SchemaName: ref = %q", s2.Ref)
	}
}

func TestGenericComponentNaming(t *testing.T) {
	type Task struct {
		ID string `json:"id"`
	}
	type Page[T any] struct {
		Items []T `json:"items"`
	}
	r := NewReflector()
	s := r.Reflect(Page[Task]{})
	if s.Ref != "#/components/schemas/Page_Task" {
		t.Errorf("generic ref = %q, want Page_Task", s.Ref)
	}

	// Nested instantiation.
	type List[T any] struct {
		All []T `json:"all"`
	}
	r2 := NewReflector()
	s3 := r2.Reflect(Page[List[Task]]{})
	if s3.Ref != "#/components/schemas/Page_List_Task" {
		t.Errorf("nested generic ref = %q, want Page_List_Task", s3.Ref)
	}
}

func TestSimplifyTypeExpr(t *testing.T) {
	cases := map[string]string{
		"Task":                       "Task",
		"main.Task":                  "Task",
		"github.com/x/pkg.Task":      "Task",
		"Page[main.Task]":            "Page_Task",
		"Page[main.List[main.Task]]": "Page_List_Task",
		"Pair[main.A,main.B]":        "Pair_A_B",
		"Pair[main.A, other.B]":      "Pair_A_B",
	}
	for in, want := range cases {
		if got := simplifyTypeExpr(in); got != want {
			t.Errorf("simplifyTypeExpr(%q) = %q, want %q", in, got, want)
		}
	}
}

// The openapi tag overrides a field's reflected schema or excludes
// the field; constraints and docs still compose on top.
func TestOpenAPIFieldOverride(t *testing.T) {
	type Custom struct {
		Inner string `json:"inner"`
	}
	type T struct {
		At      Custom           `json:"at" openapi:"type=string,format=date-time" doc:"RFC 3339"`
		Skipped string           `json:"secret" openapi:"-"`
		Bounded Custom           `json:"bounded" openapi:"type=integer" minimum:"1"`
		Ptr     *Custom          `json:"ptr" openapi:"type=string"`
		Raw     json.RawMessage  `json:"raw" openapi:"schema=json-schema"`
		RawPtr  *json.RawMessage `json:"raw_ptr" openapi:"schema=json-schema"`
		RawDoc  json.RawMessage  `json:"raw_doc" openapi:"schema=json-schema" doc:"Backend form schema"`
		RawEx   json.RawMessage  `json:"raw_ex" openapi:"schema=json-schema" example:"{\"type\":\"object\",\"properties\":{\"host\":{\"type\":\"string\"}}}"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp == nil {
		t.Fatal("T component missing")
	}
	at := comp.Properties["at"]
	if at.Type != "string" || at.Format != "date-time" || at.Description != "RFC 3339" {
		t.Errorf("at = %+v", at)
	}
	if at.Ref != "" {
		t.Errorf("override must replace the $ref entirely")
	}
	if _, ok := comp.Properties["secret"]; ok {
		t.Errorf(`openapi:"-" field must be excluded`)
	}
	if comp.Properties["bounded"].Minimum != "1" {
		t.Errorf("constraints must compose on overrides")
	}
	if !comp.Properties["ptr"].Nullable {
		t.Errorf("pointer overrides keep nullability")
	}
	// The overridden struct type must not leak a component.
	if _, ok := out["Custom"]; ok {
		t.Errorf("overridden struct field registered a phantom Custom component")
	}
	// schema=json-schema documents a json.RawMessage as an open object.
	if raw := comp.Properties["raw"]; raw.Type != "object" || raw.AdditionalProperties == nil ||
		raw.Description != "A JSON Schema document." || raw.Extensions["x-stdocs-type"] != "json-schema" {
		t.Errorf("raw schema=json-schema = %+v", raw)
	}
	if !comp.Properties["raw_ptr"].Nullable {
		t.Errorf("*json.RawMessage schema=json-schema must stay nullable")
	}
	if got := comp.Properties["raw_doc"].Description; got != "Backend form schema" {
		t.Errorf("doc: must override the default JSON-Schema-document description, got %q", got)
	}
	// An author-supplied example: on a json-schema field is parsed as a
	// JSON literal (not a scalar), so it can be a whole JSON Schema.
	rawEx, ok := comp.Properties["raw_ex"].Example.(map[string]any)
	if !ok {
		t.Fatalf("raw_ex example must be a JSON object, got %T", comp.Properties["raw_ex"].Example)
	}
	if rawEx["type"] != "object" || rawEx["properties"] == nil {
		t.Errorf("raw_ex example did not round-trip as a JSON Schema: %+v", rawEx)
	}
}

func TestOpenAPIFieldOverridePanics(t *testing.T) {
	cases := []struct {
		name string
		f    func()
	}{
		{"unknown key", func() {
			type T struct {
				X string `json:"x" openapi:"kind=string"`
			}
			ReflectSchema(T{})
		}},
		{"missing type", func() {
			type T struct {
				X string `json:"x" openapi:"format=uuid"`
			}
			ReflectSchema(T{})
		}},
		{"non-scalar type", func() {
			type T struct {
				X string `json:"x" openapi:"type=object"`
			}
			ReflectSchema(T{})
		}},
		{"bare value", func() {
			type T struct {
				X string `json:"x" openapi:"string"`
			}
			ReflectSchema(T{})
		}},
		{"schema unknown value", func() {
			type T struct {
				X json.RawMessage `json:"x" openapi:"schema=foo"`
			}
			ReflectSchema(T{})
		}},
		{"schema combined with type", func() {
			type T struct {
				X json.RawMessage `json:"x" openapi:"schema=json-schema,type=string"`
			}
			ReflectSchema(T{})
		}},
		{"constraint on json-schema object", func() {
			type T struct {
				X json.RawMessage `json:"x" openapi:"schema=json-schema" minLength:"1"`
			}
			ReflectSchema(T{})
		}},
		{"invalid json example on json-schema field", func() {
			type T struct {
				X json.RawMessage `json:"x" openapi:"schema=json-schema" example:"{not json"`
			}
			ReflectSchema(T{})
		}},
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

type namedBase struct {
	B string `json:"b"`
}

func (namedBase) SchemaName() string { return "BaseSchema" }

type derivedFromNamed struct {
	namedBase
	D string `json:"d"`
}

type clobberInner struct {
	X string `json:"x"`
}

type clobberWrapper struct {
	W clobberInner `json:"w"`
}

func (clobberWrapper) SchemaName() string { return "clobberInner" }

// A name reserved for a parent must not be clobbered by a same-named
// child reflected during the parent's own build, and a SchemaName
// promoted from an embedded field names the embedded type only.
func TestComponentNameClobbering(t *testing.T) {
	// Promoted SchemaName: Derived embeds namedBase and inherits its
	// method; Derived must NOT be named BaseSchema.
	r := NewReflector()
	s := r.Reflect(derivedFromNamed{})
	if s.Ref != "#/components/schemas/derivedFromNamed" {
		t.Errorf("derived ref = %q; promoted SchemaName must not rename the outer type", s.Ref)
	}
	r.Reflect(namedBase{})
	if _, ok := r.Components()["BaseSchema"]; !ok {
		t.Errorf("the embedded type keeps its own SchemaName")
	}
	base := r.Components()["BaseSchema"]
	if _, hasD := base.Properties["d"]; hasD {
		t.Errorf("BaseSchema must describe namedBase, not the derived type")
	}

	// Explicit SchemaName colliding with a contained type: both
	// components must exist, distinctly.
	r2 := NewReflector()
	s2 := r2.Reflect(clobberWrapper{})
	if s2.Ref != "#/components/schemas/clobberInner" {
		t.Errorf("wrapper ref = %q", s2.Ref)
	}
	comps := r2.Components()
	if len(comps) != 2 {
		t.Fatalf("components = %v, want wrapper and suffixed inner", mapKeys(comps))
	}
	inner, ok := comps["clobberInner_2"]
	if !ok {
		t.Fatalf("inner type should take the collision suffix; got %v", mapKeys(comps))
	}
	if _, hasX := inner.Properties["x"]; !hasX {
		t.Errorf("suffixed component must hold the inner type's schema")
	}
	wrapper := comps["clobberInner"]
	if wrapper.Properties["w"].Ref != "#/components/schemas/clobberInner_2" {
		t.Errorf("wrapper field must ref the suffixed inner component, got %q (a self-ref means the schema was clobbered)", wrapper.Properties["w"].Ref)
	}
	if !r2.Renamed()["clobberInner_2"] {
		t.Errorf("Renamed() should record the genuine rename")
	}
}

// The openapi override wins over the json ",string" rewrite, params
// structs honor the tag, values are trimmed, and embedded type
// overrides panic.
func TestOpenAPIOverrideInteractions(t *testing.T) {
	type T struct {
		N int64  `json:"n,string" openapi:"type=integer,format=int64"`
		W string `json:"w" openapi:"type=string,format= date-time "`
	}
	_, out := schema30(t, T{})
	n := out["T"].Properties["n"]
	if n.Type != "integer" || n.Format != "int64" {
		t.Errorf("override must beat the ,string rewrite: %+v", n)
	}
	if out["T"].Properties["w"].Format != "date-time" {
		t.Errorf("override values must be trimmed: %q", out["T"].Properties["w"].Format)
	}

	type P struct {
		At      clobberInner `query:"at" openapi:"type=string,format=date-time"`
		Skipped string       `query:"skip" openapi:"-"`
	}
	fields := ParamFields(P{})
	if len(fields) != 1 {
		t.Fatalf("params = %d, want 1 (openapi:\"-\" skips)", len(fields))
	}
	if fields[0].Schema.Type != "string" || fields[0].Schema.Format != "date-time" {
		t.Errorf("params override lost: %+v", fields[0].Schema)
	}

	defer func() {
		if recover() == nil {
			t.Errorf("embedded type override should panic")
		}
	}()
	type E struct {
		ExportedEmbed `openapi:"type=string"`
	}
	ReflectSchema(E{})
}

// ExportedEmbed exists to test openapi overrides on exported embedded
// fields (unexported embeds take the promoted-inline path instead).
type ExportedEmbed struct {
	Inner string `json:"inner"`
}

// v0.4.1: unsigned kinds document minimum 0; required tags work on
// body/response structs (including required-but-nullable).
func TestUnsignedMinimumAndRequiredTag(t *testing.T) {
	type T struct {
		Count    uint    `json:"count"`
		Small    uint8   `json:"small"`
		Explicit uint    `json:"explicit" minimum:"5"`
		Excl     uint    `json:"excl" exclusiveMinimum:"0"`
		Items    *[]int  `json:"items" required:"true"`
		Loose    string  `json:"loose" required:"false"`
		Plain    float64 `json:"plain"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	if comp.Properties["count"].Minimum != "0" || comp.Properties["small"].Minimum != "0" {
		t.Errorf("unsigned fields must document minimum 0: count=%q small=%q",
			comp.Properties["count"].Minimum, comp.Properties["small"].Minimum)
	}
	if comp.Properties["explicit"].Minimum != "5" {
		t.Errorf("explicit minimum overrides the auto bound: %q", comp.Properties["explicit"].Minimum)
	}
	if comp.Properties["excl"].ExclusiveMinimum != "0" || comp.Properties["excl"].Minimum != "" {
		t.Errorf("explicit exclusive bound displaces the auto minimum: %+v", comp.Properties["excl"])
	}
	if comp.Properties["plain"].Minimum != "" {
		t.Errorf("signed/float fields gain no auto minimum")
	}
	req := map[string]bool{}
	for _, n := range comp.Required {
		req[n] = true
	}
	if !req["items"] {
		t.Errorf("required:\"true\" must force the pointer field into required; got %v", comp.Required)
	}
	if !comp.Properties["items"].Nullable {
		t.Errorf("required:\"true\" composes with nullability")
	}
	if req["loose"] {
		t.Errorf("required:\"false\" must exclude the field; got %v", comp.Required)
	}

	defer func() {
		if recover() == nil {
			t.Errorf("invalid required tag should panic")
		}
	}()
	type Bad struct {
		X string `json:"x" required:"yep"`
	}
	ReflectSchema(Bad{})
}

// v0.4.2: the composable openapi:"nullable" entry.
func TestOpenAPINullableEntry(t *testing.T) {
	type Custom struct {
		X string `json:"x"`
	}
	type T struct {
		Note string `json:"note" openapi:"nullable" minLength:"1" doc:"May be null"`
		At   Custom `json:"at" openapi:"type=string,format=date-time,nullable"`
		Both *[]int `json:"both" openapi:"nullable"` // pointer + entry: still just nullable
		Req  string `json:"req" openapi:"nullable" required:"true"`
	}
	_, out := schema30(t, T{})
	comp := out["T"]
	note := comp.Properties["note"]
	if !note.Nullable || note.Type != "string" || *note.MinLength != 1 || note.Description != "May be null" {
		t.Errorf("bare nullable must stack with reflection and tags: %+v", note)
	}
	at := comp.Properties["at"]
	if !at.Nullable || at.Type != "string" || at.Format != "date-time" {
		t.Errorf("override + nullable: %+v", at)
	}
	if !comp.Properties["both"].Nullable {
		t.Errorf("pointer + nullable entry stays nullable")
	}
	req := map[string]bool{}
	for _, n := range comp.Required {
		req[n] = true
	}
	if !req["req"] || !comp.Properties["req"].Nullable {
		t.Errorf("required-but-nullable without pointers: required=%v nullable=%v", req["req"], comp.Properties["req"].Nullable)
	}

	for _, f := range []func(){
		func() {
			type Bad struct {
				X string `json:"x" openapi:"nullable,nullable"`
			}
			ReflectSchema(Bad{})
		},
		func() {
			type P struct {
				X string `query:"x" openapi:"nullable"`
			}
			ParamFields(P{})
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

// v0.5.1 (#70): embedding dominance follows encoding/json exactly —
// every case is asserted against json.Marshal's actual output, so
// agreement is the test.
func TestEmbeddingDominance(t *testing.T) {
	resolve := func(v any) *Schema {
		root, comps := ReflectSchema(v)
		s := root
		if s != nil && s.Ref != "" {
			s = comps[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
		}
		if s == nil {
			t.Fatalf("no schema for %T", v)
		}
		return s
	}
	jsonKeys := func(v any) map[string]bool {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %T: %v", v, err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal %T: %v", v, err)
		}
		keys := make(map[string]bool, len(m))
		for k := range m {
			keys[k] = true
		}
		return keys
	}
	agree := func(name string, v any) *Schema {
		t.Helper()
		s := resolve(v)
		keys := jsonKeys(v)
		for k := range s.Properties {
			if !keys[k] {
				t.Errorf("%s: schema documents %q but json.Marshal does not emit it (wire keys %v)", name, k, keys)
			}
		}
		for k := range keys {
			if _, ok := s.Properties[k]; !ok {
				t.Errorf("%s: json.Marshal emits %q but the schema omits it", name, k)
			}
		}
		return s
	}

	type emA struct {
		X string `json:"x"`
		Y string `json:"y"`
	}
	type emB struct {
		X int `json:"x"`
	}
	// The colliding embeds are assembled with reflect.StructOf: vet's
	// structtag check (rightly) rejects declaring this shape, and the
	// collision is exactly what is under test.
	conflictT := reflect.StructOf([]reflect.StructField{
		{Name: "EmA", Type: reflect.TypeOf(emA{}), Anonymous: true},
		{Name: "EmB", Type: reflect.TypeOf(emB{}), Anonymous: true},
	})
	conflictV := reflect.New(conflictT).Elem()
	conflictV.Field(0).Set(reflect.ValueOf(emA{X: "a", Y: "y"}))
	conflictV.Field(1).Set(reflect.ValueOf(emB{X: 1}))
	s := agree("equal-depth conflict", conflictV.Interface())
	if slices.Contains(s.Required, "x") {
		t.Errorf("a dropped field must not be required: %v", s.Required)
	}

	type emC struct{ V string }
	type emD struct {
		V int `json:"V"`
	}
	type tagWins struct {
		emC
		emD
	}
	if s := agree("tagged beats untagged", tagWins{emC{"s"}, emD{9}}); s.Properties["V"] == nil || s.Properties["V"].Type != "integer" {
		t.Errorf("the tagged int must win: %+v", s.Properties["V"])
	}

	type hidden struct {
		Sec string `json:"s"`
	}
	type holder struct {
		*hidden
		P string `json:"p"`
	}
	if s := agree("unexported pointer embed", holder{&hidden{"sec"}, "p"}); s.Properties["s"] == nil {
		t.Errorf("exported fields of an unexported pointer embed must promote")
	}

	type core struct {
		ID string `json:"id"`
	}
	type viaB struct{ core }
	type viaC struct{ core }
	type diamond struct {
		viaB
		viaC
	}
	agree("diamond drop", diamond{viaB{core{"a"}}, viaC{core{"b"}}})

	type crossDepth struct {
		viaB // id at depth 2
		emB  // x at depth 1... and another id at depth 1 wins below
	}
	_ = crossDepth{}
	type shallowID struct {
		ID int `json:"id"`
	}
	type depthRace struct {
		viaB // -> core -> id (depth 2)
		shallowID
	}
	if s := agree("shallower wins across branches", depthRace{viaB{core{"a"}}, shallowID{7}}); s.Properties["id"] == nil || s.Properties["id"].Type != "integer" {
		t.Errorf("the depth-1 int id must beat the depth-2 string id: %+v", s.Properties["id"])
	}

	type shadowReq struct {
		core         // id required at depth 1
		ID   *string `json:"id,omitempty"` // optional at depth 0 wins
	}
	outer := "y"
	if s := agree("shadowed required must not leak", shadowReq{core{"x"}, &outer}); slices.Contains(s.Required, "id") {
		t.Errorf("the winning optional field must not inherit the loser's required-ness: %v", s.Required)
	}
}

// v0.5.1 verification: the shapes the adversarial round surfaced —
// each pinned against json.Marshal, the oracle for every flattening
// question.
func TestEmbeddingDominanceEdges(t *testing.T) {
	resolve := func(v any) *Schema {
		root, comps := ReflectSchema(v)
		s := root
		if s != nil && s.Ref != "" {
			s = comps[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
		}
		if s == nil {
			t.Fatalf("no schema for %T", v)
		}
		return s
	}
	props := func(v any) map[string]bool {
		s := resolve(v)
		out := make(map[string]bool, len(s.Properties))
		for k := range s.Properties {
			out[k] = true
		}
		return out
	}
	wire := func(v any) map[string]bool {
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal %T: %v", v, err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal %T: %v", v, err)
		}
		out := make(map[string]bool, len(m))
		for k := range m {
			out[k] = true
		}
		return out
	}
	same := func(name string, v any) {
		t.Helper()
		p, w := props(v), wire(v)
		if !maps.Equal(p, w) {
			t.Errorf("%s: schema %v != wire %v", name, p, w)
		}
	}

	// The shared-intermediate diamond: json loses embed multiplicity
	// at the queueing step, so the field below the join point
	// survives while the duplicated type's own leaves annihilate.
	type leafE struct {
		Z int `json:"z"`
	}
	type mid struct {
		leafE
		L int `json:"l"`
	}
	type d1 struct{ mid }
	type d2 struct{ mid }
	type sharedDiamond struct {
		d1
		d2
		X int `json:"x"`
	}
	same("shared-intermediate diamond", sharedDiamond{d1{mid{leafE{11}, 1}}, d2{mid{leafE{22}, 2}}, 3})

	// Embeds with no JSON-visible fields flatten into nothing instead
	// of inventing a required type-named property.
	type guarded struct {
		sync.Mutex
		X int `json:"x"`
	}
	same("sync.Mutex embed", guarded{X: 7})
	type marker struct{}
	type marked struct {
		marker
		X int `json:"x"`
	}
	same("empty marker embed", marked{marker{}, 1})

	// Recursive pointer embeds terminate without a phantom property.
	type recA struct {
		Next *recA `json:"-"`
		X    int   `json:"x"`
	}
	_ = recA{}
	same("self-recursive pointer embed", struct {
		*selfRec
		X int `json:"x"`
	}{&selfRec{nil, 5}, 1})

	// A tag-named unexported struct embed is a leaf field on the wire.
	same("tagged unexported embed", taggedUnexported{hiddenLeaf{"v"}, 2})

	// Marshaler embeds whose promotion is blocked flatten their
	// exported fields, exactly as typeFields does.
	same("blocked marshaler embeds", struct {
		mj1
		mj2
		X int `json:"x"`
	}{mj1{1}, mj2{2}, 3})

	// json:"-," names the key "-"; only the bare "-" tag skips.
	same("dash-named field", struct {
		A int `json:"-,"` //nolint:staticcheck // naming a key "-" is the case under test
		X int `json:"x"`
	}{1, 2})

	// An embedded named scalar is a regular wire field; an openapi
	// override on it must apply, not panic.
	if s := resolve(scalarEmbedOverride{7, 1}); s.Properties["MyScalar"] == nil || s.Properties["MyScalar"].Type != "string" {
		t.Errorf("scalar embed override must apply: %+v", s.Properties["MyScalar"])
	}

	// openapi:"-" hides a field from the document but it still exists
	// on the wire, so a collision json drops must not resurface the
	// rival.
	hiddenCollisionT := reflect.StructOf([]reflect.StructField{
		{Name: "EmHideA", Type: reflect.TypeOf(emHideA{}), Anonymous: true},
		{Name: "EmHideB", Type: reflect.TypeOf(emHideB{}), Anonymous: true},
		{Name: "N", Type: reflect.TypeOf(0), Tag: `json:"n"`},
	})
	hv := reflect.New(hiddenCollisionT).Elem().Interface()
	if p := props(hv); p["v"] {
		t.Errorf("an openapi:\"-\" candidate must still annihilate its rival: %v", p)
	}
}

type selfRec struct {
	*selfRec
	S int `json:"s"`
}

type hiddenLeaf struct {
	V string `json:"v"`
}

type taggedUnexported struct {
	hiddenLeaf `json:"h"`
	X          int `json:"x"`
}

type mj1 struct {
	A1 int `json:"a1"`
}

func (mj1) MarshalJSON() ([]byte, error) { return []byte(`"m1"`), nil }

type mj2 struct {
	B2 int `json:"b2"`
}

func (mj2) MarshalJSON() ([]byte, error) { return []byte(`"m2"`), nil }

type MyScalar int

type scalarEmbedOverride struct {
	MyScalar `openapi:"type=string"`
	X        int `json:"x"`
}

type emHideA struct {
	V string `json:"v" openapi:"-"`
}

type emHideB struct {
	V int `json:"v"`
}

// v0.6.0 verification: invalid json tag names fall back to the Go
// field name, exactly as encoding/json's isValidTag does — the
// document must describe the keys json.Marshal actually emits.
func TestInvalidJSONTagNames(t *testing.T) {
	type T struct {
		Emoji string `json:"🚀"`
		Ctrl  string `json:"a\tb"`
		Cafe  string `json:"café"`
		Punct string `json:"ok$_-"`
	}
	root, comps := ReflectSchema(T{})
	s := root
	if s.Ref != "" {
		s = comps[strings.TrimPrefix(s.Ref, "#/components/schemas/")]
	}
	raw, err := json.Marshal(T{"e", "c", "k", "p"})
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	for k := range wire {
		if s.Properties[k] == nil {
			t.Errorf("wire key %q missing from the schema (props %v)", k, slices.Sorted(maps.Keys(s.Properties)))
		}
	}
	for k := range s.Properties {
		if _, ok := wire[k]; !ok {
			t.Errorf("schema documents %q but the wire never carries it", k)
		}
	}
}

// v0.6.2 (#83): a declared default must satisfy its own constraints —
// a default that is not a valid value is a self-contradictory spec.
func TestDefaultMustSatisfyConstraints(t *testing.T) {
	panics := func(name string, v any) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected a panic, got none", name)
			} else if !strings.Contains(r.(string), "default") {
				t.Errorf("%s: panic %q does not mention the default", name, r)
			}
		}()
		ReflectSchema(v)
	}
	ok := func(name string, v any) {
		t.Helper()
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("%s: unexpected panic %q on a valid default", name, r)
			}
		}()
		ReflectSchema(v)
	}

	panics("default outside enum", struct {
		S string `json:"s" enum:"a,b,c" default:"z"`
	}{})
	panics("int default outside enum", struct {
		N int `json:"n" enum:"1,2,3" default:"9"`
	}{})
	panics("default below minimum", struct {
		N int `json:"n" minimum:"1" maximum:"5" default:"0"`
	}{})
	panics("default above maximum", struct {
		N int `json:"n" minimum:"1" maximum:"5" default:"9"`
	}{})
	panics("default violates exclusiveMinimum", struct {
		R float64 `json:"r" exclusiveMinimum:"0" default:"0"`
	}{})
	panics("default shorter than minLength", struct {
		S string `json:"s" minLength:"3" default:"hi"`
	}{})
	panics("default longer than maxLength", struct {
		S string `json:"s" maxLength:"2" default:"long"`
	}{})
	panics("default fails pattern", struct {
		S string `json:"s" pattern:"^[a-z]+$" default:"ABC"`
	}{})

	ok("default in enum", struct {
		S string `json:"s" enum:"a,b,c" default:"b"`
	}{})
	ok("default within bounds", struct {
		N int `json:"n" minimum:"1" maximum:"5" default:"3"`
	}{})
	ok("default within length and pattern", struct {
		S string `json:"s" minLength:"1" maxLength:"5" pattern:"^[a-z]+$" default:"ok"`
	}{})
	ok("no default", struct {
		N int `json:"n" minimum:"1"`
	}{})
}

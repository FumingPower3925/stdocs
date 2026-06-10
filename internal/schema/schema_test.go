package schema

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/FumingPower3925/stdocs/internal/version"
)

// Helper: build a schema and assert it has the expected JSON when serialized
// to OpenAPI 3.0.3.
func schema30(t *testing.T, value any) (*Schema, map[string]*Schema) {
	t.Helper()
	return ReflectSchema(value, version.OpenAPI30)
}

func schema31(t *testing.T, value any) (*Schema, map[string]*Schema) {
	t.Helper()
	return ReflectSchema(value, version.OpenAPI31)
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
		{int(0), "int32"},
		{int8(0), "int32"},
		{int16(0), "int32"},
		{int32(0), "int32"},
		{int64(0), "int64"},
		{uint(0), "int32"},
		{uint8(0), "int32"},
		{uint16(0), "int32"},
		{uint32(0), "int32"},
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
	// In 3.0.3, Nullable stays as a separate field; in 3.1.0, it becomes
	// a type array.
	s31, _ := schema31(t, v)
	if len(s31.TypeArray) != 2 || s31.TypeArray[0] != "string" || s31.TypeArray[1] != "null" {
		t.Errorf("TypeArray = %v, want [string null]", s31.TypeArray)
	}
	if s31.Type != "" {
		t.Errorf("Type = %q, want empty in 3.1", s31.Type)
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
	// any(nil) is an untyped nil; reflect.TypeOf gives nil. We test the
	// interface path with a typed nil interface value.
	var v any = (*int)(nil) // this is an interface holding a typed nil
	_ = v
	// Use a fresh variable to make the type clear.
	var w any = any(nil)
	_ = w
	type Any = any
	var x Any = nil
	_ = x

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
		Exported   string `json:"exp"`
		unexported string
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
	// ints is a $ref to a Box[int] component. The component itself has
	// a value field typed as integer.
	if ints.Ref != "#/components/schemas/Box[int]" {
		t.Errorf("ints.Ref = %q, want #/components/schemas/Box[int]", ints.Ref)
	}
	boxInt := out["Box[int]"]
	if boxInt == nil {
		t.Fatal("Box[int] component missing")
	}
	v := boxInt.Properties["value"]
	if v == nil || v.Type != "integer" {
		t.Errorf("Box[int].value.Type = %v, want integer", v)
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
	// In 3.1.0, the Type field is cleared and TypeArray is populated.
	if name.Type != "" {
		t.Errorf("name.Type = %q, want empty in 3.1", name.Type)
	}
	if len(name.TypeArray) != 2 || name.TypeArray[0] != "string" || name.TypeArray[1] != "null" {
		t.Errorf("name.TypeArray = %v, want [string null]", name.TypeArray)
	}
	_ = s
}

func TestReflectSchema_ComponentsAreJSONSerializable(t *testing.T) {
	type T struct {
		Field string `json:"field"`
	}
	_, out := schema30(t, T{})
	for name, s := range out {
		b, err := json.Marshal(s)
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

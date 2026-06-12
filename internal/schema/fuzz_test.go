package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// fieldKinds are the field types the fuzzer composes structs from.
var fieldKinds = []reflect.Type{
	reflect.TypeOf(""),
	reflect.TypeOf(0),
	reflect.TypeOf(uint(0)),
	reflect.TypeOf(int32(0)),
	reflect.TypeOf(uint64(0)),
	reflect.TypeOf(0.0),
	reflect.TypeOf(false),
	reflect.TypeOf([]string{}),
	reflect.TypeOf([]int{}),
	reflect.TypeOf(map[string]int{}),
	reflect.TypeOf(json.RawMessage{}),
	reflect.TypeOf([]byte{}),
}

// fuzzTags are tag fragments the fuzzer mixes onto fields — a blend of
// valid, type-mismatched, and unparseable constraint declarations.
var fuzzTags = []string{
	``,
	`doc:"d"`,
	`minimum:"1"`,
	`maximum:"5"`,
	`exclusiveMinimum:"0"`,
	`minLength:"1"`,
	`maxLength:"10"`,
	`pattern:"^a"`,
	`enum:"a,b"`,
	`enum:"1,2"`,
	`default:"3"`,
	`default:"x"`,
	`example:"2"`,
	`format:"email"`,
	`minItems:"1"`,
	`uniqueItems:"true"`,
	`required:"true"`,
	`required:"false"`,
	`minimum:".5"`,
	`minimum:"NaN"`,
	`openapi:"-"`,
	`openapi:"type=string,format=date-time"`,
	`openapi:"kind=bad"`,
	`json:",omitempty"`,
	`json:",string"`,
	`json:"-"`,
}

// buildFuzzType derives a struct type from fuzz bytes: each byte pair
// picks a field type and a tag.
func buildFuzzType(data []byte) (reflect.Type, bool) {
	if len(data) < 2 || len(data) > 24 {
		return nil, false
	}
	var fields []reflect.StructField
	for i := 0; i+1 < len(data); i += 2 {
		ft := fieldKinds[int(data[i])%len(fieldKinds)]
		tag := fuzzTags[int(data[i+1])%len(fuzzTags)]
		name := fmt.Sprintf("F%d", i/2)
		jsonName := fmt.Sprintf("f%d", i/2)
		full := `json:"` + jsonName + `"`
		if strings.HasPrefix(tag, "json:") {
			full = strings.Replace(tag, `json:"`, `json:"`+jsonName, 1)
		} else if tag != "" {
			full += " " + tag
		}
		fields = append(fields, reflect.StructField{
			Name: name,
			Type: ft,
			Tag:  reflect.StructTag(full),
		})
	}
	t, err := safeStructOf(fields)
	if err != nil {
		return nil, false
	}
	return t, true
}

func safeStructOf(fields []reflect.StructField) (t reflect.Type, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("StructOf: %v", r)
		}
	}()
	return reflect.StructOf(fields), nil
}

// FuzzReflectSchema asserts the reflector's contract over arbitrary
// struct shapes and tag mixes: the only allowed panics are the
// documented fail-fast ones (prefixed "stdocs: " and naming a field),
// successful reflections marshal to valid JSON, and reflection is
// deterministic.
func FuzzReflectSchema(f *testing.F) {
	f.Add([]byte{0, 0})
	f.Add([]byte{1, 2, 3, 4})
	f.Add([]byte{7, 8, 9, 10, 11, 12})
	f.Add([]byte{2, 18, 5, 19})  // unsigned + bad numeric literals
	f.Add([]byte{10, 21, 0, 22}) // RawMessage + openapi overrides
	f.Fuzz(func(t *testing.T, data []byte) {
		typ, ok := buildFuzzType(data)
		if !ok {
			t.Skip()
		}
		reflectOnce := func() (s *Schema, comps map[string]*Schema, panicked string) {
			defer func() {
				if r := recover(); r != nil {
					panicked = fmt.Sprint(r)
				}
			}()
			s, comps = ReflectSchema(reflect.New(typ).Elem().Interface())
			return
		}
		s1, c1, p1 := reflectOnce()
		s2, _, p2 := reflectOnce()
		if p1 != p2 {
			t.Fatalf("non-deterministic panic behavior: %q vs %q", p1, p2)
		}
		if p1 != "" {
			if !strings.HasPrefix(p1, "stdocs: ") || !strings.Contains(p1, "field F") {
				t.Fatalf("non-fail-fast panic: %q", p1)
			}
			return
		}
		// json.Number constraint values must always marshal —
		// unmarshalable literals are exactly the bug class the
		// validation exists to prevent. StructOf types are anonymous
		// and inline (no components), so walk the root schema tree.
		var walk func(where string, p *Schema)
		walk = func(where string, p *Schema) {
			if p == nil {
				return
			}
			for _, n := range []json.Number{p.Minimum, p.Maximum, p.ExclusiveMinimum, p.ExclusiveMaximum} {
				if n == "" {
					continue
				}
				if _, err := json.Marshal(n); err != nil {
					t.Fatalf("unmarshalable numeric literal %q at %s: %v", n, where, err)
				}
			}
			for _, v := range []any{p.Default, p.Example} {
				if v == nil {
					continue
				}
				if _, err := json.Marshal(v); err != nil {
					t.Fatalf("unmarshalable value %#v at %s: %v", v, where, err)
				}
			}
			for fname, fp := range p.Properties {
				walk(where+"."+fname, fp)
			}
			walk(where+"[]", p.Items)
			walk(where+"{}", p.AdditionalProperties)
		}
		walk("root", s1)
		for name, comp := range c1 {
			walk(name, comp)
		}
		if !reflect.DeepEqual(s1, s2) {
			t.Fatalf("non-deterministic reflection (root)")
		}
	})
}

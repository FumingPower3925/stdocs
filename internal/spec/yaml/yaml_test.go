package yaml_test

import (
	"strings"
	"testing"

	"github.com/FumingPower3925/stdocs/internal/spec/yaml"
)

func TestYAMLFromJSON_Simple(t *testing.T) {
	in := []byte(`{"openapi":"3.0.3","info":{"title":"T","version":"1.0"}}`)
	y, err := yaml.FromJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(y)
	if !strings.Contains(s, `openapi: "3.0.3"`) {
		t.Errorf("expected openapi line, got %s", s)
	}
	if !strings.Contains(s, `title: "T"`) {
		t.Errorf("expected title line, got %s", s)
	}
}

func TestYAMLFromJSON_Arrays(t *testing.T) {
	in := []byte(`{"tags":["a","b","c"]}`)
	y, _ := yaml.FromJSON(in)
	if !strings.Contains(string(y), `- "a"`) {
		t.Errorf("expected array items, got %s", y)
	}
}

func TestYAMLFromJSON_Nested(t *testing.T) {
	in := []byte(`{"paths":{"/x":{"get":{"summary":"x"}}}}`)
	y, _ := yaml.FromJSON(in)
	s := string(y)
	if !strings.Contains(s, "paths:") {
		t.Errorf("expected paths, got %s", s)
	}
	if !strings.Contains(s, `summary: "x"`) {
		t.Errorf("expected summary, got %s", s)
	}
}

func TestYAMLFromJSON_EmptyObject(t *testing.T) {
	y, _ := yaml.FromJSON([]byte(`{}`))
	if string(y) != "{}\n" {
		t.Errorf("empty object = %q, want {} with trailing newline", y)
	}
}

func TestYAMLFromJSON_NullAndBool(t *testing.T) {
	y, _ := yaml.FromJSON([]byte(`{"a":null,"b":true,"c":false,"d":42,"e":3.14}`))
	s := string(y)
	if !strings.Contains(s, "a: null") {
		t.Errorf("null missing: %s", s)
	}
	if !strings.Contains(s, "b: true") {
		t.Errorf("true missing: %s", s)
	}
	if !strings.Contains(s, "c: false") {
		t.Errorf("false missing: %s", s)
	}
	if !strings.Contains(s, "d: 42") {
		t.Errorf("int missing: %s", s)
	}
}

// TestYAMLFromJSON_EmptyCollectionSpace guards the regression where
// empty maps/arrays emitted "key:{}" / "key:[]" in block context —
// invalid per the YAML 1.2 spec, which requires ":" to be followed by
// a space or newline. The fix routes empty collections through the
// same "value on the same line" path as scalars, so a real YAML
// parser would accept the output.
func TestYAMLFromJSON_EmptyCollectionSpace(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"empty map", `{"a":{}}`, "a: {}"},
		{"empty array", `{"a":[]}`, "a: []"},
		{"nested empty", `{"a":{"b":{}}}`, "a:" + "\n  b: {}"},
		{"mixed", `{"a":{},"b":[],"c":1}`, "a: {}" + "\nb: []" + "\nc: 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := yaml.FromJSON([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			s := string(got)
			if !strings.Contains(s, tc.want) {
				t.Errorf("expected %q in:\n%s", tc.want, s)
			}
			// The original bug was an empty-collection emit with no
			// space after the colon. A space (or newline) is required.
			// Check that no occurrence of ":{}" or ":[]" is preceded
			// by a word character.
			for _, needle := range []string{":{}", ":[]"} {
				idx := 0
				for {
					i := strings.Index(s[idx:], needle)
					if i < 0 {
						break
					}
					abs := idx + i
					if abs > 0 {
						prev := s[abs-1]
						if prev != ' ' && prev != '\n' && prev != '\t' {
							t.Errorf("empty-collection-without-space separator at offset %d in:\n%s", abs, s)
							break
						}
					}
					idx = abs + len(needle)
				}
			}
		})
	}
}

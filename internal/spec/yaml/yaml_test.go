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
	if string(y) != "{}" {
		t.Errorf("empty object = %q, want {}", y)
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

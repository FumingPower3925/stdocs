package uiopt

import "testing"

func TestApplyEmpty(t *testing.T) {
	if got := Apply(nil); got.Config != nil {
		t.Errorf("Apply(nil).Config = %v, want nil", got.Config)
	}
}

func TestApplyConfiguration(t *testing.T) {
	s := Apply([]Option{Configuration(map[string]any{"theme": "purple"})})
	if s.Config["theme"] != "purple" {
		t.Errorf("Config[theme] = %v, want purple", s.Config["theme"])
	}
}

func TestConfigurationLastWins(t *testing.T) {
	s := Apply([]Option{
		Configuration(map[string]any{"a": 1}),
		Configuration(map[string]any{"b": 2}),
	})
	if _, ok := s.Config["a"]; ok {
		t.Errorf("expected the last Configuration to replace the first, got %v", s.Config)
	}
	if s.Config["b"] != 2 {
		t.Errorf("Config[b] = %v, want 2", s.Config["b"])
	}
}

func TestMerge(t *testing.T) {
	if got := Merge(nil, nil); got != nil {
		t.Errorf("Merge(nil,nil) = %v, want nil", got)
	}
	base := map[string]any{"a": 1, "b": 2}
	got := Merge(base, map[string]any{"b": 3, "c": 4})
	if got["a"] != 1 || got["b"] != 3 || got["c"] != 4 {
		t.Errorf("Merge over = %v, want a:1 b:3 c:4", got)
	}
	// base must not be mutated
	if base["b"] != 2 {
		t.Errorf("Merge mutated base: b = %v, want 2", base["b"])
	}
	// only base, or only over
	if got := Merge(map[string]any{"x": 1}, nil); got["x"] != 1 {
		t.Errorf("Merge(base,nil) = %v", got)
	}
	if got := Merge(nil, map[string]any{"y": 1}); got["y"] != 1 {
		t.Errorf("Merge(nil,over) = %v", got)
	}
}

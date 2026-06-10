package stdocs

import (
	"encoding/json"
	"testing"
)

// jx is a small JSON-extract helper for tests. It unmarshals into a
// generic map and returns it.
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

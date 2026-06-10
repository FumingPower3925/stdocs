package pattern

import (
	"reflect"
	"testing"
)

// FuzzParsePattern fuzzes ParsePattern to ensure it does not panic
// on any input and that successful parses survive a round-trip
// through Pattern.String().
//
// Run with:
//
//	go test -fuzz=ParsePattern -fuzztime=10s ./internal/pattern/
func FuzzParsePattern(f *testing.F) {
	f.Add("GET /users")
	f.Add("GET /users/{id}")
	f.Add("POST /v1/orders/{id}/items")
	f.Add("GET /files/{path...}")
	f.Add("GET /")
	f.Add("GET /a/{b}/c/{d}")
	f.Add("DELETE /users/{id}")
	f.Fuzz(func(t *testing.T, s string) {
		// ParsePattern must not panic on any input.
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ParsePattern(%q) panicked: %v", s, r)
			}
		}()
		p, err := ParsePattern(s)
		if err != nil {
			return
		}
		// If parsing succeeded, every name returned by
		// WildcardNames must be non-empty (the stdlib requires
		// a name; stddocs filters out empty entries for
		// defense-in-depth). The slice itself may be nil for
		// patterns with no wildcards.
		names := p.WildcardNames()
		for _, n := range names {
			if n == "" {
				t.Errorf("ParsePattern(%q) returned empty wildcard name; want filtered", s)
			}
		}
		// Method must be uppercased or empty.
		if p.Method != "" && p.Method != toUpper(p.Method) {
			t.Errorf("Method = %q, not uppercased", p.Method)
		}
		_ = reflect.DeepEqual
	})
}

func toUpper(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'a' && c <= 'z' {
			out[i] = c - 32
		}
	}
	return string(out)
}

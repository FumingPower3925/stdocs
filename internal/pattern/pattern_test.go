package pattern

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestParsePattern_Root(t *testing.T) {
	p := MustParsePattern("/")
	if p.Path() != "/" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/")
	}
	if p.HasMethod() {
		t.Errorf("HasMethod() = true, want false")
	}
	if p.Host != "" {
		t.Errorf("Host = %q, want \"\"", p.Host)
	}
	// The root "/" is internally a single implicit Multi("") segment that
	// represents a prefix match. The emitted path is "/" regardless.
	if !p.IsPrefix {
		t.Errorf("IsPrefix = false, want true (root is a prefix match)")
	}
}

func TestParsePattern_MethodAndPath(t *testing.T) {
	p := MustParsePattern("GET /users")
	if p.Method != http.MethodGet {
		t.Errorf("Method = %q, want %q", p.Method, http.MethodGet)
	}
	if p.Path() != "/users" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/users")
	}
	if len(p.Segments) != 1 || p.Segments[0].Kind != KindLiteral || p.Segments[0].Value != "users" {
		t.Errorf("Segments = %+v, want [users]", p.Segments)
	}
}

func TestParsePattern_AcceptsLowercaseMethod(t *testing.T) {
	p := MustParsePattern("get /users")
	if p.Method != http.MethodGet {
		t.Errorf("Method = %q, want %q", p.Method, http.MethodGet)
	}
}

func TestParsePattern_TabSeparatorBetweenMethodAndPath(t *testing.T) {
	p := MustParsePattern("POST\t/users")
	if p.Method != http.MethodPost {
		t.Errorf("Method = %q, want %q", p.Method, http.MethodPost)
	}
	if p.Path() != "/users" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/users")
	}
}

func TestParsePattern_Host(t *testing.T) {
	p := MustParsePattern("example.com/posts")
	if p.Host != "example.com" {
		t.Errorf("Host = %q, want %q", p.Host, "example.com")
	}
	if p.Path() != "/posts" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/posts")
	}
}

func TestParsePattern_MethodAndHost(t *testing.T) {
	p := MustParsePattern("GET example.com/users/{id}")
	if p.Method != http.MethodGet {
		t.Errorf("Method = %q, want %q", p.Method, http.MethodGet)
	}
	if p.Host != "example.com" {
		t.Errorf("Host = %q, want %q", p.Host, "example.com")
	}
	if p.Path() != "/users/{id}" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/users/{id}")
	}
}

func TestParsePattern_SingleWildcard(t *testing.T) {
	p := MustParsePattern("GET /users/{id}")
	if len(p.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2", len(p.Segments))
	}
	if p.Segments[1].Kind != KindWildcard {
		t.Errorf("Segments[1].Kind = %d, want KindWildcard", p.Segments[1].Kind)
	}
	if p.Segments[1].Value != "id" {
		t.Errorf("Segments[1].Value = %q, want %q", p.Segments[1].Value, "id")
	}
	if got, want := p.WildcardNames(), []string{"id"}; !reflect.DeepEqual(got, want) {
		t.Errorf("WildcardNames() = %v, want %v", got, want)
	}
}

func TestParsePattern_MultiWildcard(t *testing.T) {
	p := MustParsePattern("/files/{path...}")
	if len(p.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2", len(p.Segments))
	}
	if p.Segments[1].Kind != KindMulti {
		t.Errorf("Segments[1].Kind = %d, want KindMulti", p.Segments[1].Kind)
	}
	if p.Segments[1].Value != "path" {
		t.Errorf("Segments[1].Value = %q, want %q", p.Segments[1].Value, "path")
	}
	// The "..." suffix is collapsed in the OpenAPI path representation.
	if p.Path() != "/files/{path}" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/files/{path}")
	}
	if got, want := p.WildcardNames(), []string{"path"}; !reflect.DeepEqual(got, want) {
		t.Errorf("WildcardNames() = %v, want %v", got, want)
	}
}

func TestParsePattern_TrailingAnchor(t *testing.T) {
	p := MustParsePattern("/posts/{$}")
	if len(p.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2", len(p.Segments))
	}
	if p.Segments[1].Kind != KindTrailing {
		t.Errorf("Segments[1].Kind = %d, want KindTrailing", p.Segments[1].Kind)
	}
	if p.IsPrefix {
		t.Errorf("IsPrefix = true, want false")
	}
	// The OpenAPI path collapses {$} to a literal slash.
	if p.Path() != "/posts/" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/posts/")
	}
}

func TestParsePattern_TrailingSlashPrefix(t *testing.T) {
	p := MustParsePattern("/posts/")
	if !p.IsPrefix {
		t.Errorf("IsPrefix = false, want true")
	}
	// Internally represented as a literal "posts" segment + anonymous Multi("").
	if len(p.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2", len(p.Segments))
	}
	if p.Segments[0].Kind != KindLiteral || p.Segments[0].Value != "posts" {
		t.Errorf("Segments[0] = %+v, want Literal(posts)", p.Segments[0])
	}
	if p.Segments[1].Kind != KindMulti {
		t.Errorf("Segments[1].Kind = %d, want KindMulti", p.Segments[1].Kind)
	}
	// Emitted path is "/posts/" (the anonymous multi collapses to a slash).
	if p.Path() != "/posts/" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/posts/")
	}
	// Guard: the anonymous Multi("") must not be reported as a
	// named path parameter. An empty parameter name in the emitted
	// spec is invalid (rejected by Spectral and OpenAPI validators).
	if names := p.WildcardNames(); len(names) != 0 {
		t.Errorf("WildcardNames() = %v, want [] (anonymous wildcard must be filtered)", names)
	}
}

// TestWildcardNames_FiltersEmpty guards the regression where the
// implicit anonymous multi from trailing slashes leaked an empty-name
// path parameter into the OpenAPI spec, which is rejected by every
// validator.
func TestWildcardNames_FiltersEmpty(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"/", nil},
		{"/posts/", nil},
		{"/static/", nil},
		{"GET /", nil},
		{"GET /posts/", nil},
		{"/users/{id}", []string{"id"}},
		{"/users/{id}/posts/", []string{"id"}},
		{"/files/{path...}", []string{"path"}},
		{"/{a}/{b}/", []string{"a", "b"}},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			p := MustParsePattern(c.in)
			got := p.WildcardNames()
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("WildcardNames() = %v, want %v", got, c.want)
			}
			// Additionally, no name in the result may be empty.
			for _, n := range got {
				if n == "" {
					t.Errorf("empty name in WildcardNames(): %v", got)
				}
			}
		})
	}
}

func TestParsePattern_MultipleWildcards(t *testing.T) {
	p := MustParsePattern("/b/{bucket}/o/{objectname...}")
	if got, want := p.WildcardNames(), []string{"bucket", "objectname"}; !reflect.DeepEqual(got, want) {
		t.Errorf("WildcardNames() = %v, want %v", got, want)
	}
	if p.Path() != "/b/{bucket}/o/{objectname}" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/b/{bucket}/o/{objectname}")
	}
}

func TestParsePattern_EscapedLiteralIsUnescaped(t *testing.T) {
	// stdlib unescapes literal segments on parse. We do the same so the emitted
	// spec is human-readable.
	p := MustParsePattern("/files/%61%62%63")
	if len(p.Segments) != 2 {
		t.Fatalf("len(Segments) = %d, want 2", len(p.Segments))
	}
	if p.Segments[1].Value != "abc" {
		t.Errorf("Segments[1].Value = %q, want %q", p.Segments[1].Value, "abc")
	}
}

func TestParsePattern_InvalidPatterns(t *testing.T) {
	tests := []string{
		"",                          // empty
		"users",                     // no slash
		http.MethodGet,              // no path
		"GET example.com",           // host but no path
		"GET /users/{",              // unterminated wildcard
		"GET /users/}",              // orphan closing brace
		"GET /a{x}",                 // brace in middle of segment
		"GET /{a}/{b}/{a}",          // duplicate wildcard name
		"GET /{1abc}",               // wildcard name starts with digit
		"GET /{a-b}",                // hyphen in wildcard name
		"GET /posts/{$}/more",       // {$} not at end
		"GET /files/{rest...}/more", // multi not at end
		"example{com",               // brace in host
	}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			if _, err := ParsePattern(s); err == nil {
				t.Errorf("ParsePattern(%q) = nil error, want error", s)
			}
		})
	}
}

// "GET /{$}" is technically accepted: it parses to a Trailing segment at root.
// The emitted path is "/". Some users will use this to anchor a route at root.
func TestParsePattern_TrailingAnchorAtRoot(t *testing.T) {
	p := MustParsePattern("GET /{$}")
	if p.Path() != "/" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/")
	}
	if !p.HasMethod() {
		t.Errorf("HasMethod() = false, want true")
	}
}

func TestParsePattern_OriginalPreserved(t *testing.T) {
	cases := []string{
		"GET /users/{id}",
		"/files/{path...}",
		"example.com/",
		"/",
		"POST /v1/orders/{id}/items/{item_id}",
	}
	for _, s := range cases {
		p := MustParsePattern(s)
		if p.Original != s {
			t.Errorf("Original = %q, want %q", p.Original, s)
		}
	}
}

func TestParsePattern_CustomMethod(t *testing.T) {
	// stdlib accepts custom method tokens (anything that isn't a control char).
	// We mirror that.
	p := MustParsePattern("PURGE /cache")
	if p.Method != "PURGE" {
		t.Errorf("Method = %q, want %q", p.Method, "PURGE")
	}
}

func TestPattern_PathForEmptySegments(t *testing.T) {
	p := MustParsePattern("/")
	if got := p.Path(); got != "/" {
		t.Errorf("Path() = %q, want %q", got, "/")
	}
}

func TestParsePattern_OnlyHost(t *testing.T) {
	// "example.com/" parses to a host with a prefix-only path.
	p := MustParsePattern("example.com/")
	if p.Host != "example.com" {
		t.Errorf("Host = %q, want %q", p.Host, "example.com")
	}
	if p.Path() != "/" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/")
	}
	if !p.IsPrefix {
		t.Errorf("IsPrefix = false, want true")
	}
}

func TestParsePattern_HostWithWildcardInPath(t *testing.T) {
	p := MustParsePattern("api.example.com/v1/users/{id}/posts/{pid}")
	if p.Path() != "/v1/users/{id}/posts/{pid}" {
		t.Errorf("Path() = %q, want %q", p.Path(), "/v1/users/{id}/posts/{pid}")
	}
}

func TestParsePattern_DoesNotPanicOnRandom(t *testing.T) {
	// Defensive: the parser should never panic, only return errors.
	for _, s := range []string{
		"{",
		"}",
		"{}",
		"{...}",
		"GET {",
		"GET {abc",
		"GET abc}",
		"GET /{",
		"GET /{a",
		"GET /a}",
		"GET /a{}b",
		"GET //",
		"GET ///a",
	} {
		_, _ = ParsePattern(s) // must not panic
	}
}

func TestIsValidWildcardName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"id", true},
		{"user_id", true},
		{"_id", true},
		{"Id", true},
		{"a1", true},
		{"a_b_c", true},
		{"", false},
		{"1abc", false},
		{"a-b", false},
		{"a.b", false},
		{"a b", false},
		{"a{}", false},
	}
	for _, c := range cases {
		if got := isValidWildcardName(c.in); got != c.want {
			t.Errorf("isValidWildcardName(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsValidMethod(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{http.MethodGet, true},
		{http.MethodPost, true},
		{"PUT", true},
		{"DELETE", true},
		{"HEAD", true},
		{"OPTIONS", true},
		{"PATCH", true},
		{"CONNECT", true},
		{"TRACE", true},
		{"PURGE", true}, // custom method
		{"", false},
		{"GE T", false}, // contains space
		{"GET\n", false},
	}
	for _, c := range cases {
		if got := isValidMethod(c.in); got != c.want {
			t.Errorf("isValidMethod(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// Quick sanity: Path() never returns a string without a leading "/".
func TestPattern_PathAlwaysStartsWithSlash(t *testing.T) {
	patterns := []string{
		"/",
		"GET /",
		"/users",
		"GET /users/{id}",
		"/files/{path...}",
		"/posts/{$}",
		"example.com/posts",
	}
	for _, s := range patterns {
		p := MustParsePattern(s)
		got := p.Path()
		if !strings.HasPrefix(got, "/") {
			t.Errorf("Path() for %q = %q, missing leading '/'", s, got)
		}
	}
}

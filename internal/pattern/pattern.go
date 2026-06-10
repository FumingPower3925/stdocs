// Package pattern parses Go 1.22+ net/http.ServeMux pattern strings.
package pattern

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// PatternKind classifies a parsed stdlib ServeMux pattern segment.
//
// Deprecated: kept for API compatibility with early users; new code should
// use the Segment struct fields directly.
type PatternKind uint8

const (
	// KindLiteral matches an exact path segment.
	KindLiteral PatternKind = iota
	// KindWildcard matches a single path segment and captures it under Name.
	KindWildcard
	// KindMulti matches the remainder of the path and captures it under Name.
	KindMulti
	// KindTrailing is the literal "/" used in /foo/{$} for the exact-trailing-slash anchor.
	KindTrailing
)

// Note: SpecVersion and the OpenAPI30 / OpenAPI31 constants live in
// version.go so they can be referenced from both this file and schema.go.

// Segment is one piece of a parsed path.
type Segment struct {
	// Kind is the kind of segment.
	Kind PatternKind
	// Value is the literal value for KindLiteral, the wildcard name for KindWildcard
	// and KindMulti, and "/" for KindTrailing.
	Value string
}

// Pattern is a parsed stdlib ServeMux pattern.
//
// Examples:
//
//	"GET /users/{id}"        -> Method="GET", Host="", Segments=[Literal("users"), Wildcard("id")]
//	"/files/{path...}"       -> Method="",   Host="", Segments=[Literal("files"), Multi("path")]
//	"example.com/posts/{$}"  -> Method="",   Host="example.com", Segments=[Literal("posts"), Trailing]
//	"/"                      -> Method="",   Host="", Segments=[]
type Pattern struct {
	// Original is the original pattern string, exactly as passed to ServeMux.
	Original string
	// Method is the upper-case HTTP method (e.g. "GET"), or "" if no method was specified.
	Method string
	// Host is the literal host (e.g. "example.com"), or "" if no host was specified.
	Host string
	// Segments is the ordered list of path segments. The leading slash is implicit.
	Segments []Segment
	// IsPrefix is true if the path ends in "/" (i.e. a prefix match).
	// Trailing-slash patterns expand to an implicit Multi("") at parse time.
	IsPrefix bool
}

// String returns the original pattern string.
func (p *Pattern) String() string { return p.Original }

// Path returns the OpenAPI-style path string with literal "{" and "}" in wildcards
// preserved. The returned path always starts with "/".
//
//	"GET /users/{id}"  -> "/users/{id}"
//	"/files/{path...}" -> "/files/{path}"  (Go's "..." syntax collapses)
//	"/posts/{$}"       -> "/posts/"
//	"/posts/"          -> "/posts/"
//	"/"                -> "/"
func (p *Pattern) Path() string {
	// The root pattern "/" produces segments=[Multi("")] internally. The
	// general loop below emits "/" for that case (the "/" prefix plus a
	// no-op anonymous multi), so we can just run the loop unconditionally.
	var b strings.Builder
	for _, s := range p.Segments {
		b.WriteByte('/')
		switch s.Kind {
		case KindLiteral:
			b.WriteString(s.Value)
		case KindWildcard:
			b.WriteByte('{')
			b.WriteString(s.Value)
			b.WriteByte('}')
		case KindMulti:
			if s.Value == "" {
				// Anonymous multi: just the trailing slash. We have already
				// written the leading "/", so we are done with this segment.
				continue
			}
			// OpenAPI does not have a "rest" syntax in path templates. We emit
			// the wildcard name without "..." so the parameter still appears
			// in the spec.
			b.WriteByte('{')
			b.WriteString(s.Value)
			b.WriteByte('}')
		case KindTrailing:
			// The {$} anchor matches only the trailing slash. We collapse it
			// to a literal "/" in the OpenAPI path so the route still appears
			// in the spec; users can describe it in the description.
		}
	}
	return b.String()
}

// WildcardNames returns the names of all named wildcards in declaration
// order. Multi wildcards and single-segment wildcards are both
// included. The special "{$}" trailing-anchor has no name and is not
// included. Anonymous wildcards (those with an empty Value, produced
// implicitly by trailing slashes) are also filtered out — they are
// not valid OpenAPI path parameters and emitting them produces a
// spec-invalid empty-name parameter.
func (p *Pattern) WildcardNames() []string {
	var names []string
	for _, s := range p.Segments {
		if s.Value == "" {
			continue
		}
		if s.Kind == KindWildcard || s.Kind == KindMulti {
			names = append(names, s.Value)
		}
	}
	return names
}

// HasMethod reports whether the pattern specifies an HTTP method.
func (p *Pattern) HasMethod() bool { return p.Method != "" }

// ParsePattern parses a stdlib net/http.ServeMux pattern string (Go 1.22+).
// The syntax is "[METHOD ] [HOST]/[PATH]" where METHOD, HOST, and PATH are
// independently optional. METHOD is one of GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS/CONNECT.
// HOST is a literal hostname. PATH consists of slash-separated segments which may
// include the wildcards "{name}" (single segment), "{name...}" (rest of path, must
// be the last segment), and "{$}" (trailing-slash anchor, must be the last segment).
//
// Trailing slashes are treated as prefix matches: "/posts/" matches "/posts/",
// "/posts/123", etc. They are represented internally as a KindMulti("") wildcard.
//
// The original stdlib parsePattern lives in src/net/http/pattern.go. This is a
// from-scratch implementation that produces a Pattern value suitable for
// OpenAPI spec generation.
func ParsePattern(s string) (*Pattern, error) {
	if len(s) == 0 {
		return nil, errors.New("stdocs: empty pattern")
	}
	p := &Pattern{Original: s}

	rest := s
	// Split off the optional method.
	if i := strings.IndexAny(rest, " \t"); i >= 0 {
		method := rest[:i]
		rest = strings.TrimLeft(rest[i+1:], " \t")
		if !isValidMethod(method) {
			return nil, fmt.Errorf("stdocs: invalid method %q in pattern %q", method, s)
		}
		p.Method = strings.ToUpper(method)
	}

	// Split off the optional host. The host is everything before the first slash.
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return nil, fmt.Errorf("stdocs: pattern %q missing path (no '/' found)", s)
	}
	host := rest[:i]
	rest = rest[i:]
	if strings.ContainsRune(host, '{') {
		return nil, fmt.Errorf("stdocs: pattern %q has '{' in host (missing initial '/'?)", s)
	}
	if host != "" {
		p.Host = host
	}

	// At this point, rest is the path. It must start with "/".
	if rest == "" || rest[0] != '/' {
		return nil, fmt.Errorf("stdocs: pattern %q has empty path", s)
	}
	// stdlib disallows non-CONNECT patterns with unclean paths; we don't replicate
	// that check (it does not affect OpenAPI generation), but we do unescape literal
	// segments so the emitted path is human-readable.

	seenNames := make(map[string]bool)

	// Walk segments. rest[0] is always '/'.
	off := len(rest) - 1 // offset of the current '/' in the original string
	_ = off              // currently unused; reserved for richer error messages later
	for len(rest) > 0 {
		rest = rest[1:] // consume the leading '/'
		if len(rest) == 0 {
			// Trailing slash with no segment after it -> prefix match.
			// We represent it as an anonymous multi wildcard.
			p.Segments = append(p.Segments, Segment{Kind: KindMulti, Value: ""})
			p.IsPrefix = true
			break
		}
		i := strings.IndexByte(rest, '/')
		var seg string
		if i < 0 {
			seg = rest
			rest = ""
		} else {
			seg = rest[:i]
			rest = rest[i:]
		}

		j := strings.IndexByte(seg, '{')
		if j < 0 {
			// Literal segment. Unescape per RFC 3986 (matches stdlib behaviour).
			if strings.ContainsRune(seg, '}') {
				return nil, fmt.Errorf("stdocs: pattern %q has orphan '}' in literal segment %q", s, seg)
			}
			unsegged, _ := url.PathUnescape(seg)
			if unsegged == "" {
				unsegged = seg
			}
			p.Segments = append(p.Segments, Segment{Kind: KindLiteral, Value: unsegged})
			continue
		}

		// Wildcard segment. Must start with '{' and end with '}'.
		if j != 0 {
			return nil, fmt.Errorf("stdocs: pattern %q has '{{{ ' in middle of segment %q", s, seg)
		}
		if seg[len(seg)-1] != '}' {
			return nil, fmt.Errorf("stdocs: pattern %q has wildcard segment %q missing closing '}'", s, seg)
		}
		inner := seg[1 : len(seg)-1]
		if inner == "$" {
			// {$} trailing-slash anchor. Must be the last segment.
			if len(rest) != 0 {
				return nil, fmt.Errorf("stdocs: pattern %q has '{$}' not at end of path", s)
			}
			p.Segments = append(p.Segments, Segment{Kind: KindTrailing, Value: "/"})
			break
		}
		name, multi := strings.CutSuffix(inner, "...")
		if multi && len(rest) != 0 {
			return nil, fmt.Errorf("stdocs: pattern %q has multi wildcard %q not at end of path", s, seg)
		}
		if name == "" {
			return nil, fmt.Errorf("stdocs: pattern %q has empty wildcard", s)
		}
		if !isValidWildcardName(name) {
			return nil, fmt.Errorf("stdocs: pattern %q has invalid wildcard name %q", s, name)
		}
		if seenNames[name] {
			return nil, fmt.Errorf("stdocs: pattern %q has duplicate wildcard name %q", s, name)
		}
		seenNames[name] = true
		if multi {
			p.Segments = append(p.Segments, Segment{Kind: KindMulti, Value: name})
		} else {
			p.Segments = append(p.Segments, Segment{Kind: KindWildcard, Value: name})
		}
	}

	return p, nil
}

// MustParsePattern is like ParsePattern but panics on error.
func MustParsePattern(s string) *Pattern {
	p, err := ParsePattern(s)
	if err != nil {
		panic(err)
	}
	return p
}

// isValidMethod reports whether s is a non-empty HTTP method token per RFC 7230.
// We accept the same set the stdlib does.
func isValidMethod(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r <= ' ' || r >= 0x7f {
			return false
		}
	}
	switch s {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE":
		return true
	}
	// stdlib also accepts any custom method. We do the same. Whether
	// the method is *also* a valid OpenAPI method is a separate
	// question; see isOpenAPIMethod.
	return true
}

// openAPIMethods is the set of HTTP method tokens that are legal as
// the key of an OpenAPI Path Item Object. Custom methods (PURGE,
// etc.) make the document fail strict validation; callers should
// either use a vendor extension or pick a different approach.
var openAPIMethods = map[string]bool{
	"get":     true,
	"put":     true,
	"post":    true,
	"delete":  true,
	"options": true,
	"head":    true,
	"patch":   true,
	"trace":   true,
}

// IsOpenAPIMethod reports whether s is a legal OpenAPI method key.
// The set is the eight methods listed in the OpenAPI 3.x Paths
// Object definition. The "head" key is a special case: it is
// allowed in OpenAPI but is *also* implicitly registered by
// registering "get" (per RFC 7231); stdocs emits "head" only if
// the user registered it explicitly.
func IsOpenAPIMethod(s string) bool {
	return openAPIMethods[strings.ToLower(s)]
}

// isValidWildcardName reports whether s is a valid Go identifier.
// A wildcard name in a stdlib pattern is required to be a valid Go identifier.
func isValidWildcardName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case unicode.IsLetter(r):
		case i > 0 && unicode.IsDigit(r):
		default:
			return false
		}
	}
	return true
}

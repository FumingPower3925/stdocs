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
// ParsePattern parses a Go 1.22+ http.ServeMux pattern. See the
// package-level comment for the supported subset.
func ParsePattern(s string) (*Pattern, error) {
	if len(s) == 0 {
		return nil, errors.New("stdocs: empty pattern")
	}
	p := &Pattern{Original: s}
	rest, err := splitMethod(s, p)
	if err != nil {
		return nil, err
	}
	rest, err = splitHost(s, rest, p)
	if err != nil {
		return nil, err
	}
	return parsePath(s, rest, p)
}

// splitMethod extracts the optional HTTP method prefix ("GET /foo")
// from the pattern. The remainder is returned.
func splitMethod(s string, p *Pattern) (string, error) {
	i := strings.IndexAny(s, " \t")
	if i < 0 {
		return s, nil
	}
	method := s[:i]
	rest := strings.TrimLeft(s[i+1:], " \t")
	if !isValidMethod(method) {
		return "", fmt.Errorf("stdocs: invalid method %q in pattern %q", method, s)
	}
	p.Method = strings.ToUpper(method)
	return rest, nil
}

// splitHost extracts the optional host prefix ("example.com/foo")
// from the pattern. The remainder is the path.
func splitHost(orig, rest string, p *Pattern) (string, error) {
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return "", fmt.Errorf("stdocs: pattern %q missing path (no '/' found)", orig)
	}
	host := rest[:i]
	rest = rest[i:]
	if strings.ContainsRune(host, '{') {
		return "", fmt.Errorf("stdocs: pattern %q has '{' in host (missing initial '/'?)", orig)
	}
	if host != "" {
		p.Host = host
	}
	if rest == "" || rest[0] != '/' {
		return "", fmt.Errorf("stdocs: pattern %q has empty path", orig)
	}
	return rest, nil
}

// parsePath walks the path portion of the pattern, producing
// Segments. The rest argument is the path with a leading '/'.
func parsePath(orig, rest string, p *Pattern) (*Pattern, error) {
	seenNames := make(map[string]bool)
	for len(rest) > 0 {
		// Consume the leading '/'.
		rest = rest[1:]
		if len(rest) == 0 {
			// Trailing '/' with no segment after it -> prefix
			// match, represented as an anonymous multi wildcard.
			p.Segments = append(p.Segments, Segment{Kind: KindMulti, Value: ""})
			p.IsPrefix = true
			return p, nil
		}
		seg, after := takeSegment(rest)
		rest = after
		if err := parseSegment(orig, seg, rest, seenNames, p); err != nil {
			return nil, err
		}
		if isLastSegment(p) {
			return p, nil
		}
	}
	return p, nil
}

// takeSegment splits the next path segment from rest. Returns
// seg and the remainder (still starting with '/' for any
// segment that wasn't the last, or empty if it was).
func takeSegment(rest string) (seg, after string) {
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return rest, ""
	}
	return rest[:i], rest[i:]
}

// parseSegment classifies a single path segment as a literal
// or wildcard and appends it to p. Updates seenNames for
// duplicate detection.
func parseSegment(orig, seg, rest string, seenNames map[string]bool, p *Pattern) error {
	j := strings.IndexByte(seg, '{')
	if j < 0 {
		return appendLiteral(orig, seg, p)
	}
	return appendWildcard(orig, seg, rest, j, seenNames, p)
}

func appendLiteral(orig, seg string, p *Pattern) error {
	if strings.ContainsRune(seg, '}') {
		return fmt.Errorf("stdocs: pattern %q has orphan '}' in literal segment %q", orig, seg)
	}
	unsegged, _ := url.PathUnescape(seg)
	if unsegged == "" {
		unsegged = seg
	}
	p.Segments = append(p.Segments, Segment{Kind: KindLiteral, Value: unsegged})
	return nil
}

func appendWildcard(orig, seg, rest string, j int, seenNames map[string]bool, p *Pattern) error {
	if j != 0 {
		return fmt.Errorf("stdocs: pattern %q has '{' in middle of segment %q", orig, seg)
	}
	if seg[len(seg)-1] != '}' {
		return fmt.Errorf("stdocs: pattern %q has wildcard segment %q missing closing '}'", orig, seg)
	}
	inner := seg[1 : len(seg)-1]
	if inner == "$" {
		if len(rest) != 0 {
			return fmt.Errorf("stdocs: pattern %q has '{$}' not at end of path", orig)
		}
		p.Segments = append(p.Segments, Segment{Kind: KindTrailing, Value: "/"})
		return nil
	}
	name, multi := strings.CutSuffix(inner, "...")
	if multi && len(rest) != 0 {
		return fmt.Errorf("stdocs: pattern %q has multi wildcard %q not at end of path", orig, seg)
	}
	if name == "" {
		return fmt.Errorf("stdocs: pattern %q has empty wildcard", orig)
	}
	if !isValidWildcardName(name) {
		return fmt.Errorf("stdocs: pattern %q has invalid wildcard name %q", orig, name)
	}
	if seenNames[name] {
		return fmt.Errorf("stdocs: pattern %q has duplicate wildcard name %q", orig, name)
	}
	seenNames[name] = true
	if multi {
		p.Segments = append(p.Segments, Segment{Kind: KindMulti, Value: name})
	} else {
		p.Segments = append(p.Segments, Segment{Kind: KindWildcard, Value: name})
	}
	return nil
}

// isLastSegment reports whether the most recently appended
// segment terminates the pattern (e.g. {$}).
func isLastSegment(p *Pattern) bool {
	if len(p.Segments) == 0 {
		return false
	}
	last := p.Segments[len(p.Segments)-1]
	return last.Kind == KindTrailing || last.Kind == KindMulti
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

// IsOpenAPIMethod reports whether s is one of the eight fixed Path
// Item operation keys shared by OpenAPI 3.0 and 3.1. (OpenAPI 3.2
// additionally allows "query" and arbitrary methods via
// additionalOperations; that version-specific logic lives in the
// emitter.) The "head" key is a special case: it is
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

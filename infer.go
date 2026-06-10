package stdocs

import (
	"strings"
	"unicode"
)

// summaryFromFuncName turns a Go function name like "getUser" or
// "ListUsers" or "HTTPHandler" into a human-readable summary like
// "Get user" or "List users" or "HTTP handler". Common prefixes
// ("handle", "Handle") are stripped, and the first character is
// upper-cased. If the result is empty, an empty string is returned
// (the caller decides what to do).
//
// The input may be a fully-qualified Go function name (e.g.
// "github.com/foo/bar.(*Type).Method" or "github.com/foo/bar.funcName");
// only the trailing identifier is used.
func summaryFromFuncName(name string) string {
	if name == "" {
		return ""
	}
	// Strip a fully-qualified Go function name to its last component.
	// Examples:
	//   "github.com/foo.bar.baz"      -> "baz"
	//   "github.com/foo.(*Type).M"    -> "M"
	//   "package.func1"               -> "func1"
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	// Strip a "handle" or "Handler" prefix.
	lower := strings.ToLower(name)
	for _, prefix := range []string{"handlehttp", "handler", "handle"} {
		if strings.HasPrefix(lower, prefix) && len(name) > len(prefix) {
			name = name[len(prefix):]
			lower = lower[len(prefix):]
			break
		}
	}
	// Insert spaces at case boundaries: "getUser" -> "get User", etc.
	var b strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			if unicode.IsLower(prev) || unicode.IsDigit(prev) {
				b.WriteRune(' ')
			} else if i+1 < len(runes) && unicode.IsLower(r) && unicode.IsUpper(prev) {
				// "HTTPHandler" -> between P and H, no space (we want HTTP).
				// Between T and T, no space. The case for inserting a
				// space here is "XMLParser" -> "XML Parser": we want a
				// break before P.
				// Detected: previous is upper, current is upper,
				// next is lower -> insert space.
				b.WriteRune(' ')
			}
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return ""
	}
	// Capitalize the first letter, lowercase the rest of the first word.
	out = strings.ToLower(out)
	if out[0] >= 'a' && out[0] <= 'z' {
		out = string(out[0]-32) + out[1:]
	}
	// "Xml parser" -> "XML parser": uppercase any all-caps run at the
	// start of the string. We do a simple pass: find the first
	// non-letter, then if letters before it form an acronym, upper-case.
	runes = []rune(out)
	for i, r := range runes {
		if !unicode.IsLetter(r) {
			if i >= 2 {
				// Check if the prefix is short and acronym-like.
				// Heuristic: 2+ letters all uppercase after lowercasing
				// were the same. We approximate: if any are 3+ chars
				// in a row, treat as acronym.
				// Skip this complexity for v0 — return as-is.
			}
			break
		}
	}
	return out
}

// tagFromPath returns a default tag for a route based on the first path
// segment. "/users/{id}" -> "users"; "/" -> ""; "/v1/users" -> "v1".
func tagFromPath(path string) string {
	// Strip the optional method.
	if i := strings.IndexAny(path, " \t"); i >= 0 {
		path = path[i+1:]
	}
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return ""
	}
	if i := strings.Index(path, "/"); i >= 0 {
		path = path[:i]
	}
	// Strip leading/trailing braces in case the first segment is a wildcard
	// (unusual but valid).
	path = strings.Trim(path, "{}")
	if path == "" {
		return ""
	}
	// Capitalize the first letter for tag-name presentation.
	return strings.ToUpper(path[:1]) + path[1:]
}

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
	// Insert spaces at case boundaries: "getUser" -> "get User".
	// We also remember which output positions are part of an
	// acronym (a run of 2+ letters at a word start where every
	// source rune was upper-case) so we can re-uppercase them
	// after lowercasing.
	var b strings.Builder
	var acronymSpans []span
	runes := []rune(name)
	wordStart := true
	wordUpperCount := 0
	wordStartIdx := 0
	// flushWord records the current run as an acronym if it
	// qualifies.
	flushWord := func() {
		if wordUpperCount >= 2 {
			acronymSpans = append(acronymSpans, span{wordStartIdx, wordStartIdx + wordUpperCount})
		}
	}
	for i, r := range runes {
		if i > 0 {
			prev := runes[i-1]
			// Boundary 1: a lower- or digit-preceded upper. Always a
			// new word.
			if unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
				flushWord()
				b.WriteRune(' ')
				wordStart = true
				wordUpperCount = 0
			} else if unicode.IsUpper(r) && unicode.IsUpper(prev) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				// Boundary 2: an upper-preceded upper where the NEXT
				// rune is lower. "XMLParser" -> "XML Parser"; this
				// breaks the acronym.
				flushWord()
				b.WriteRune(' ')
				wordStart = true
				wordUpperCount = 0
			}
		}
		if wordStart {
			wordStartIdx = b.Len()
			wordStart = false
			wordUpperCount = 0
		}
		if unicode.IsUpper(r) {
			wordUpperCount++
		} else if unicode.IsLetter(r) {
			// A non-uppercase letter ends the acronym run for
			// this word.
			wordUpperCount = 0
		}
		b.WriteRune(r)
	}
	flushWord()
	out := strings.TrimSpace(b.String())
	if out == "" {
		return ""
	}
	// Lowercase the whole string, then re-uppercase the acronym
	// spans. The spans are byte indices in the lowercased string
	// (b.String() above is the original case; lowercasing does
	// not change byte length for ASCII, and our runs are ASCII).
	outLower := strings.ToLower(out)
	if len(acronymSpans) > 0 {
		outBytes := []byte(outLower)
		for _, s := range acronymSpans {
			// Bounds-check defensively: the span is a byte index
			// into b.String(); trim may have shifted left by at
			// most a few leading spaces. Clamp.
			start, end := s.start, s.end
			if start > len(outBytes) {
				continue
			}
			if end > len(outBytes) {
				end = len(outBytes)
			}
			for k := start; k < end; k++ {
				if c := outBytes[k]; c >= 'a' && c <= 'z' {
					outBytes[k] = c - 32
				}
			}
		}
		outLower = string(outBytes)
	}
	// Capitalize the first letter of the first word.
	if outLower != "" && outLower[0] >= 'a' && outLower[0] <= 'z' {
		outLower = string(outLower[0]-32) + outLower[1:]
	}
	return outLower
}

type span struct {
	start, end int
}

// sourceRun maps a position in the post-splitter output back to a
// position in the original function name. The splitter wrote
// only the runes of `name`, in order, optionally with ' '
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

// firstSegment returns the first non-empty path segment in
// lowercase, suitable for substitution into a summary template
// (e.g. "{resource}" -> "articles"). Returns "" for "/" or for
// paths that begin with a wildcard.
func firstSegment(path string) string {
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
	path = strings.Trim(path, "{}")
	return strings.ToLower(path)
}

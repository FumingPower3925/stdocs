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
	// Method values get a "-fm" suffix from the runtime
	// ("pkg.(*svc).GetUser-fm"); strip it before extracting the
	// trailing identifier.
	name = strings.TrimSuffix(name, "-fm")
	name = stripPackageQualifier(name)
	if isAnonymousFuncName(name) {
		// Closures are named func1, func2, ... by the runtime;
		// "Func1" is not a useful summary. Return "" so the caller
		// falls through to WithDefaultSummary or a blank summary.
		return ""
	}
	name = stripHandlerPrefix(name)
	if name == "" {
		return ""
	}
	out, spans := splitOnCaseBoundaries(name)
	out = restoreAcronyms(out, spans)
	return capitalizeFirst(out)
}

// isAnonymousFuncName reports whether name is a runtime-generated
// anonymous function identifier: "func" followed by digits ("func1"),
// or — for nested closures, whose qualified name ends in ".2" — a
// bare digit run.
func isAnonymousFuncName(name string) bool {
	rest := strings.TrimPrefix(name, "func")
	if rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// stripPackageQualifier returns the trailing identifier of a
// fully-qualified Go function name.
//
//	"github.com/foo.bar.baz"   -> "baz"
//	"github.com/foo.(*Type).M" -> "M"
//	"package.func1"            -> "func1"
func stripPackageQualifier(name string) string {
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// stripHandlerPrefix removes a "Handle", "Handler", or "HandleHTTP"
// prefix from a name (case-insensitive). Returns the name unchanged
// if the prefix is not present or the remainder would be empty.
func stripHandlerPrefix(name string) string {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"handlehttp", "handler", "handle"} {
		if strings.HasPrefix(lower, prefix) && len(name) > len(prefix) {
			return name[len(prefix):]
		}
	}
	return name
}

// splitOnCaseBoundaries inserts spaces at case boundaries and
// returns the result plus a list of byte ranges that were
// acronyms in the source (so they can be re-uppercased later).
//
//	"getUser"     -> ("get User", [])
//	"HTTPHandler" -> ("HTTPHandler", [{0,4}])  // 4-letter run
//	"XMLParser"   -> ("XML Parser", [{0,3}])
func splitOnCaseBoundaries(name string) (string, []span) {
	var b strings.Builder
	var acronymSpans []span
	runes := []rune(name)
	state := wordState{atStart: true, runStart: 0}
	flush := func() {
		if state.runLen >= 2 {
			acronymSpans = append(acronymSpans, span{state.runStart, state.runStart + state.runLen})
		}
	}
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			lowerPrev := unicode.IsLower(prev) || unicode.IsDigit(prev)
			upperNext := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if lowerPrev || (unicode.IsUpper(prev) && upperNext) {
				flush()
				b.WriteRune(' ')
				state.atStart = true
				state.runLen = 0
			}
		}
		if state.atStart {
			state.runStart = b.Len()
			state.atStart = false
			state.runLen = 0
		}
		if unicode.IsUpper(r) {
			state.runLen++
		} else if unicode.IsLetter(r) {
			// A non-uppercase letter ends the run.
			state.runLen = 0
		}
		b.WriteRune(r)
	}
	flush()
	return b.String(), acronymSpans
}

// wordState tracks the case-splitter's progress through the source.
type wordState struct {
	atStart  bool // next rune starts a new word
	runStart int  // byte index in the output of the current upper-run
	runLen   int  // length of the current upper-run
}

// restoreAcronyms uppercases the bytes in spans within s. Spans
// are byte indices computed against s; the function is
// bounds-checked so out-of-range entries are clamped or dropped.
func restoreAcronyms(s string, spans []span) string {
	if len(spans) == 0 {
		return strings.ToLower(s)
	}
	out := []byte(strings.ToLower(s))
	for _, sp := range spans {
		start, end := sp.start, sp.end
		if start >= len(out) {
			continue
		}
		if end > len(out) {
			end = len(out)
		}
		for k := start; k < end; k++ {
			if c := out[k]; c >= 'a' && c <= 'z' {
				out[k] = c - 32
			}
		}
	}
	return string(out)
}

// capitalizeFirst uppercases the first ASCII letter of s.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// span is a half-open byte range in a string.
type span struct {
	start, end int
}

// tagFromPath returns a default tag for a route based on the first path
// segment. "/users/{id}" -> "users"; "/" -> ""; "/v1/users" -> "v1".
func tagFromPath(path string) string {
	seg := firstSegment(path)
	if isVersionSegment(seg) {
		// /v1/tasks should group by Tasks, not by a useless V1 shared
		// across every route.
		rest := strings.TrimPrefix(path, "/"+seg)
		seg = firstSegment(rest)
	}
	if seg == "" {
		return ""
	}
	// Capitalize the first letter for tag-name presentation.
	return strings.ToUpper(seg[:1]) + seg[1:]
}

// isVersionSegment reports whether seg is a conventional version
// path segment: "v" followed by digits (v1, v2, v10).
func isVersionSegment(seg string) bool {
	if len(seg) < 2 || (seg[0] != 'v' && seg[0] != 'V') {
		return false
	}
	for _, r := range seg[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
	// Strip leading/trailing braces in case the first segment is a
	// wildcard (unusual but valid).
	return strings.ToLower(strings.Trim(path, "{}"))
}

// Package yaml converts JSON documents (the form produced by the
// OpenAPI emitters) to YAML. It is a minimal, hand-rolled
// converter that supports only the values the emitters produce:
// objects, arrays, strings, numbers, booleans, and null.
//
// For round-trip verification against a real YAML parser, see the
// separate test module at internal/spec/yaml/roundtrip_test (kept
// out of the main module so gopkg.in/yaml.v3 never appears in the
// dependency graph).
package yaml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// FromJSON converts JSON bytes to YAML bytes. It supports only the
// subset of values that the OpenAPI emitters produce: objects,
// arrays, strings, numbers, booleans, and null.
//
// The input JSON is decoded with json.Number preserved, so integers
// of any magnitude survive the conversion verbatim. Keys in objects
// are sorted alphabetically for stable output (matching the key
// order encoding/json produces for the JSON endpoint).
//
// This is a deliberately minimal implementation: there is no support for
// anchors, custom tags, or other advanced YAML features. It is sufficient
// for emitting OpenAPI specs as YAML.
func FromJSON(jsonBytes []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	// Start at depth -1 so top-level keys sit in column zero (each
	// nesting level writes its children at indent+1).
	if err := emitYAML(&buf, v, -1); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	// Trim the leading newline and end with exactly one trailing
	// newline, as POSIX text tools expect.
	if len(b) > 0 && b[0] == '\n' {
		b = b[1:]
	}
	if len(b) > 0 && b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	return b, nil
}

func emitYAML(buf *bytes.Buffer, v any, indent int) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case json.Number:
		// A JSON number literal is also a valid YAML scalar; emit it
		// verbatim so large integers keep full precision.
		buf.WriteString(x.String())
	case float64:
		// Defensive: FromJSON decodes numbers as json.Number, but a
		// caller-constructed value may still carry float64.
		if x == float64(int64(x)) && x >= -1e15 && x <= 1e15 {
			buf.WriteString(strconv.FormatInt(int64(x), 10))
		} else {
			buf.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
		}
	case string:
		emitYAMLString(buf, x)
	case []any:
		if len(x) == 0 {
			buf.WriteString("[]")
			return nil
		}
		for _, item := range x {
			buf.WriteByte('\n')
			writeIndent(buf, indent+1)
			if isCollection(item) && !isEmptyCollection(item) {
				// The nested collection starts with its own newline, so
				// emit a bare "-" to avoid trailing whitespace.
				buf.WriteByte('-')
				if err := emitYAML(buf, item, indent+1); err != nil {
					return err
				}
				continue
			}
			buf.WriteString("- ")
			if err := emitYAML(buf, item, 0); err != nil {
				return err
			}
		}
	case map[string]any:
		if len(x) == 0 {
			buf.WriteString("{}")
			return nil
		}
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vv := x[k]
			buf.WriteByte('\n')
			writeIndent(buf, indent+1)
			emitYAMLKey(buf, k)
			buf.WriteByte(':')
			if isCollection(vv) && !isEmptyCollection(vv) {
				if err := emitYAML(buf, vv, indent+1); err != nil {
					return err
				}
			} else {
				// Scalars and empty collections both go on the same
				// line as the key. The space is required by the YAML
				// spec (":" must be followed by space or newline) and
				// matters most for empty maps/arrays, which would
				// otherwise emit "key:{}" / "key:[]" — invalid in block
				// context.
				buf.WriteByte(' ')
				if err := emitYAML(buf, vv, 0); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("yaml: unsupported type %T", v)
	}
	return nil
}

func writeIndent(buf *bytes.Buffer, n int) {
	for range n {
		buf.WriteString("  ")
	}
}

func isCollection(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

// isEmptyCollection reports whether v is a map[string]any or []any with
// zero elements. Used to decide whether to emit the value on the same
// line as the key (with a space separator) or on a new line.
func isEmptyCollection(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		return len(x) == 0
	case []any:
		return len(x) == 0
	}
	return false
}

// plainSafeKey reports whether k can be written as a plain (unquoted)
// YAML mapping key without changing its meaning. We are deliberately
// conservative: only identifier-like keys that no YAML 1.1 or 1.2
// resolver would read as anything but a string qualify. Everything
// else — response-status keys like "200" (which would resolve to an
// integer), paths, keys with punctuation, and boolean-ish words like
// "on"/"yes" — is double-quoted. JSON keys are always strings, and
// the OpenAPI spec requires YAML mapping keys to stay strings.
func plainSafeKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
		case r >= '0' && r <= '9' || r == '-':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	switch strings.ToLower(k) {
	case "true", "false", "null", "yes", "no", "on", "off", "y", "n":
		return false
	}
	return true
}

// emitYAMLKey writes a mapping key, quoting it unless it is plainly
// safe as an unquoted string scalar.
func emitYAMLKey(buf *bytes.Buffer, k string) {
	if plainSafeKey(k) {
		buf.WriteString(k)
		return
	}
	emitYAMLString(buf, k)
}

// emitYAMLString writes a string as a double-quoted YAML scalar with
// all characters that are forbidden or ambiguous inside double quotes
// escaped: the quote and backslash themselves, every C0 control
// character, DEL, and the C1 range including U+0085 NEL (a YAML line
// break that would otherwise silently fold into a space).
func emitYAMLString(buf *bytes.Buffer, s string) {
	buf.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			buf.WriteString(`\"`)
		case r == '\\':
			buf.WriteString(`\\`)
		case r == '\n':
			buf.WriteString(`\n`)
		case r == '\t':
			buf.WriteString(`\t`)
		case r == '\r':
			buf.WriteString(`\r`)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f),
			r == 0x2028, r == 0x2029:
			fmt.Fprintf(buf, `\u%04X`, r)
		default:
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
}

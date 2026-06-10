package yaml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// yamlFromJSON converts JSON bytes to YAML bytes. It supports only the
// subset of values that the OpenAPI emitters produce: objects, arrays,
// strings, numbers, booleans, and null.
//
// The input JSON is unmarshalled into interface{} and re-emitted. Keys
// in objects are sorted alphabetically for stable output.
//
// This is a deliberately minimal implementation: there is no support for
// anchors, custom tags, or other advanced YAML features. It is sufficient
// for emitting OpenAPI specs as YAML.
func FromJSON(jsonBytes []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(jsonBytes, &v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := emitYAML(&buf, v, 0); err != nil {
		return nil, err
	}
	b := buf.Bytes()
	// Trim a leading newline if present.
	if len(b) > 0 && b[0] == '\n' {
		b = b[1:]
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
	case float64:
		// JSON numbers come in as float64. Emit them as integers
		// when they have no fractional part.
		if x == float64(int64(x)) {
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
			buf.WriteString("- ")
			if isCollection(item) {
				if err := emitYAML(buf, item, indent+1); err != nil {
					return err
				}
			} else {
				if err := emitYAML(buf, item, 0); err != nil {
					return err
				}
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
			buf.WriteString(k)
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
	for i := 0; i < n; i++ {
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

// emitYAMLString writes a YAML string, choosing the simplest valid form
// (block, single-quoted, or double-quoted) for the content. We default
// to double-quoted with necessary escapes; the result is correct for
// all inputs.
func emitYAMLString(buf *bytes.Buffer, s string) {
	// Decide: needs quoting?
	// Always quote for safety; the result is always valid.
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(r)
		}
	}
	buf.WriteByte('"')
}

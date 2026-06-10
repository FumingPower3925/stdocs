package spec

import "encoding/json"

// MarshalJSON serializes v to JSON bytes.
func MarshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalJSON parses JSON bytes into v.
func UnmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

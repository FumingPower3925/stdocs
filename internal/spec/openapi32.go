package spec

import (
	"encoding/json"

	"github.com/FumingPower3925/stdocs/internal/schema"
	"github.com/FumingPower3925/stdocs/internal/version"
)

// EmitOpenAPI32 produces the OpenAPI 3.2.0 JSON bytes for the given input.
//
// OpenAPI 3.2 (released 2025-09-19) is a minor version on top of 3.1.
// The schema body and the per-object structure are the same as 3.1
// for the fields stdocs emits. The differences that matter for us are:
//
//   - The "openapi" field is "3.2.0".
//   - A "$self" field MAY be set to the canonical URI of the document
//     (passed via the selfURL argument, set with WithSelfURL on the
//     mux). When non-empty, it is emitted at the root of the spec.
//   - OAuth "implicit" flow was removed in 3.2. stdocs does not emit
//     a specific OAuth flow type today (it forwards the user's
//     SecurityScheme as-is), so this is a no-op for us.
//   - Streaming media type flags, $dynamicRef, multi-content arrays
//     are 3.2 features we do not yet emit. They are additive and can
//     land in a follow-up without breaking 3.1 or 3.0.
//
// For the parts of the spec that stdocs emits today, a 3.2 document
// is wire-compatible with 3.1 plus the optional "$self" field. The
// user-visible constant is OpenAPI32; the body shape is the same as
// 3.1.
func EmitOpenAPI32(in SpecInput, selfURL string) ([]byte, error) {
	root := BuildRoot32(in, selfURL)
	return json.Marshal(root)
}

// BuildRoot32 builds the top-level OpenAPI 3.2.0 object.
func BuildRoot32(in SpecInput, selfURL string) map[string]any {
	e := &emitter{openapi: string(version.OpenAPI32), buildSchema: buildSchema32}
	return e.buildRoot32(in, selfURL)
}

// buildSchema32 converts a *schema.Schema into the map[string]any
// form for 3.2. The body shape is identical to 3.1 for the fields
// stdocs emits today; 3.2-only schema keywords (e.g. $dynamicRef)
// are not yet emitted and are out of scope for this commit.
func buildSchema32(s *schema.Schema) map[string]any {
	return buildSchema31(s)
}

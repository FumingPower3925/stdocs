// Package tsgen emits TypeScript declarations for a documented mux —
// the API contract as types, generated from the same version-agnostic
// model the OpenAPI emitters consume, so the dialect questions
// external generators face (3.0 nullable:true vs 3.1 anyOf-null)
// never arise.
//
// The output is one self-contained module: an exported interface or
// type alias per component schema, a components interface gluing them
// together, an operations interface keyed by the document's stable
// operationIds, and a webhooks interface when webhooks are declared.
// Field documentation and constraint tags become JSDoc. No runtime
// code is emitted — no client, no fetch wrapper, no npm package,
// permanently: types are the part of an SDK nobody can maintain by
// hand, and the transport is the part every application wants to own.
//
// The natural wiring is a tiny generator next to the mux constructor
// the golden-file workflow already uses:
//
//	//go:generate go run ./cmd/gentypes
//	func main() {
//	    src, err := tsgen.Generate(buildMux())
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    os.WriteFile("api.ts", src, 0o644)
//	}
//
// Output is deterministic — sorted declarations, stable names — so
// api.ts works as a committed artifact exactly like the spec bytes:
// regenerating on an stdocs upgrade is a contract change, review the
// diff. Generation reads the document model, not the served JSON, so
// edits made by WithOpenAPI hooks are invisible here; the escape
// hatch operates downstream of what these types describe.
//
// Supported TypeScript: current releases under default compiler
// settings plus --strict (the emitted file is checked against the
// pinned typescript release in CI). Strictness flags beyond --strict
// (exactOptionalPropertyTypes and friends) are untested territory.
package tsgen

import (
	"fmt"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/internal/tsbridge"
)

// Generate returns the TypeScript declarations for the mux's
// documented API. It builds the document first, so the same fail-fast
// panics and validation errors as [stdocs.Mux.JSON] apply; like JSON,
// it rebuilds automatically when routes were registered since the
// last build.
func Generate(m *stdocs.Mux) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("tsgen: nil mux")
	}
	in, err := tsbridge.SpecInput(m)
	if err != nil {
		return nil, err
	}
	return emit(in), nil
}

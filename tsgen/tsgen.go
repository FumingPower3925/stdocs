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
// Each operations entry holds parameters grouped by location (path,
// query, header, cookie — a group is optional when every member is),
// a requestBody when one is declared, and responses keyed by status:
// the body type per status, undefined for body-less entries, string
// for raw bodies (the JSDoc carries @contentType for non-JSON media
// types and @header for declared response headers). Field
// documentation and constraint tags become JSDoc. No runtime code is
// emitted — no client, no fetch wrapper, no npm package,
// permanently: types are the part of an SDK nobody can maintain by
// hand, and the transport is the part every application wants to own.
//
// Reading the generated types, two conventions follow the document
// rather than the wire's habits. A secured route's automatic 401 is
// body-less and types as undefined; when middleware really writes
// your error envelope on 401, a mux-level WithDefaultResponse(401,
// envelope) supplies the body everywhere, automatic 401 included.
// And a pointer field without omitempty types as optional and
// nullable ("due_at?: string | null") because the document says
// nullable-not-required; json.Marshal in fact always emits the key,
// so required:"true" on the field pins it for consumers.
//
// The natural wiring is a small generator program calling an
// exported mux constructor — the same constructor the golden-file
// workflow uses; a server's package main cannot be imported, so the
// constructor lives in a library package:
//
//	// cmd/gentypes/main.go
//	func main() {
//	    src, err := tsgen.Generate(api.NewMux())
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    if err := os.WriteFile("api.ts", src, 0o644); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// driven by a directive in a module-root file (go generate resolves
// the command relative to the file holding the directive):
//
//	//go:generate go run ./cmd/gentypes
//
// Output is deterministic — sorted declarations, stable names — so
// api.ts works as a committed artifact exactly like the spec bytes:
// regenerating on an stdocs upgrade is a contract change, review the
// diff. Distribute the file as you would any committed types
// artifact — vendor it in a monorepo, or publish it as your own
// types package for separate frontend consumers; tsgen stops at the
// bytes. Generation reads the document model, not the served JSON, so
// edits made by WithOpenAPI hooks are invisible here, and a mux
// configured with WithSpec serves its hand-written document while
// Generate still describes the registered routes — both escape
// hatches operate downstream of what these types describe. Webhooks
// appear for 3.1 and 3.2 muxes only, matching the served document.
//
// Supported TypeScript: current releases under default compiler
// settings plus --strict (the emitted file is checked against the
// pinned typescript release in CI). Strictness flags beyond --strict
// (exactOptionalPropertyTypes and friends) are untested territory.
package tsgen

import (
	"fmt"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/internal/spec"
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
	var nameErr error
	out, err := tsbridge.Generate(m, func(in spec.SpecInput) []byte {
		// Emission runs under the build lock: the model holds live
		// operation pointers that concurrent rebuilds mutate.
		if nameErr = checkNames(in); nameErr != nil {
			return nil
		}
		return emit(in)
	})
	if err != nil {
		return nil, err
	}
	return out, nameErr
}

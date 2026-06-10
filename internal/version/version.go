// Package version defines the OpenAPI spec-version identifiers
// used by the emitters. The constants are re-exported at the root
// stdocs package as OpenAPI30 / OpenAPI31 / OpenAPI32.
//
// stdocs supports the latest patch of each minor of OpenAPI 3.
// Older patches (3.0.3, 3.1.0, etc.) are not exposed as constants —
// wire compatibility means 3.0.3 documents are valid 3.0.4 documents,
// so users who wrote `OpenAPI30` previously are silently upgraded
// to the latest patch of their chosen minor.
package version

// SpecVersion identifies which OpenAPI version a spec is emitted as.
type SpecVersion string

const (
	// OpenAPI30 is the latest 3.0.x patch (currently 3.0.4).
	OpenAPI30 SpecVersion = "3.0.4"
	// OpenAPI31 is the latest 3.1.x patch (currently 3.1.2).
	OpenAPI31 SpecVersion = "3.1.2"
	// OpenAPI32 is OpenAPI 3.2.0.
	OpenAPI32 SpecVersion = "3.2.0"
)

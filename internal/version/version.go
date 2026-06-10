package version

// SpecVersion identifies which OpenAPI version a spec is emitted as.
//
// stdocs supports both OpenAPI 3.0.3 and 3.1.0 from v0. Choose with
// stdocs.WithVersion(stdocs.OpenAPI30) (the default) or
// stdocs.WithVersion(stdocs.OpenAPI31). Both versions are fully emitted
// and tested; the choice is global per stdocs.Handler.
type SpecVersion string

const (
	// OpenAPI30 is the 3.0.3 version string.
	OpenAPI30 SpecVersion = "3.0.3"
	// OpenAPI31 is the 3.1.0 version string.
	OpenAPI31 SpecVersion = "3.1.0"
)

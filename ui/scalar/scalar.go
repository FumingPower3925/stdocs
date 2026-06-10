// Package scalar provides a Scalar UI for stdocs.
//
// Scalar is a modern OpenAPI viewer loaded from a CDN. To use it,
// import this sub-package and pass scalar.WithUI() to stdocs.New or
// stdocs.DocsHandler:
//
//	import (
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/scalar"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), scalar.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Mount()  // or mount mux.Docs() on a parent mux
//
// This sub-package adds the Scalar HTML to the docs handler. The
// Scalar JavaScript and CSS are loaded from cdn.jsdelivr.net at page
// load time, so an internet connection is required.
//
// The CDN URL is pinned to a specific version (1.59.2) and points at
// the verbatim dist/browser/standalone.js file from the npm package,
// so its bytes are deterministic and the sha384 SRI hash below is
// pinned in the <script> tag. Bumping the pinned version requires
// re-computing the hash. For an air-gapped build, use the
// ui/scalaremb sub-package instead — it vendors the bundle in-repo.
package scalar

import (
	"fmt"

	"github.com/FumingPower3925/stdocs"
)

// scalarVersion is the version of @scalar/api-reference this
// package is pinned to. Bumping this requires updating the SRI
// hash below and re-vendoring the bundle in ui/scalaremb.
const scalarVersion = "1.59.2"

// scalarSRIHash is the sha384 SRI hash of dist/browser/standalone.js
// at the pinned version. Re-compute with:
//
//	curl -fsSL "https://cdn.jsdelivr.net/npm/@scalar/api-reference@<ver>/dist/browser/standalone.js" \
//	    | openssl dgst -sha384 -binary | openssl base64 -A
const scalarSRIHash = "sha384-qdTNFfkRv/L0BHDvwW9XzQxu3rtN4r41Oun6L7siNlsqDTGlEKX1MYgNfNoRZ4Qg"

// WithUI returns a stdocs.Option that replaces the default docs
// page with the Scalar UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = scalarHTML
	}
}

// scalarHTML is the docs page served when the scalar sub-package is in
// use. The Scalar reference web component is loaded from cdn.jsdelivr.net
// and configured to fetch the spec from the same origin's openapi.json.
//
// Scalar expects the spec URL in the `data-url` attribute. The previous
// form (the URL as element content of a <script type="application/json">)
// made Scalar treat the URL as the document and fail with "Invalid
// YAML object".
var scalarHTML = fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0}</style>
</head>
<body>
<script id="api-reference" data-url="{{.SpecURL}}"></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@%s/dist/browser/standalone.js"
        integrity="%s"
        crossorigin="anonymous"></script>
</body>
</html>`, scalarVersion, scalarSRIHash)

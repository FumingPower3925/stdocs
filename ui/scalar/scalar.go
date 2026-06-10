// Package scalar provides a Scalar UI for stdocs.
//
// Scalar is a modern OpenAPI viewer loaded from a CDN. To use it,
// import this sub-package and pass scalar.WithUI() to stdocs.New or
// stdocs.Mount:
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
// The CDN URL is pinned to a specific version (1.59.2). SRI
// integrity is NOT pinned because jsDelivr's @scalar/api-reference
// URL generates the bundle on the fly; the file's bytes differ
// between requests. For an air-gapped build with SRI, use the
// ui/scalaremb sub-package instead — it vendors the bundle
// in-repo and the hash is stable.
package scalar

import "github.com/FumingPower3925/stdocs"

// scalarVersion is the version of @scalar/api-reference this
// package is pinned to. Bumping this requires updating the
// constant below and running the bundled-scalar test in
// ui/scalaremb.
const scalarVersion = "1.59.2"

// WithUI returns a stdocs.Option that replaces the default zero-JS
// docs page with the Scalar UI.
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
const scalarHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0}</style>
</head>
<body>
<script id="api-reference" data-url="{{.SpecURL}}"></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.59.2"></script>
</body>
</html>`

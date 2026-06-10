// Package stoplight provides a Stoplight Elements UI for stdocs.
//
// Stoplight Elements is a modern OpenAPI viewer that supports both
// OpenAPI 3.0.3 and 3.1.0, loaded from a CDN. To use it, import this
// sub-package and pass stoplight.WithUI() to stdocs.New or
// stdocs.Mount:
//
//	import (
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/stoplight"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), stoplight.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Mount()
//
// This sub-package adds the Stoplight HTML to the docs handler. The
// Stoplight web components are loaded from cdn.jsdelivr.net at page
// load time, so an internet connection is required.
//
// The CDN URL is pinned to a specific version (9.0.22). SRI
// integrity is NOT pinned because jsDelivr's @stoplight/elements
// web-components.min.js is generated on the fly; the file's bytes
// differ between requests. For an air-gapped build with SRI,
// this package would need a vendored variant (we do not currently
// provide one).
package stoplight

import "github.com/FumingPower3925/stdocs"

// stoplightVersion is the version of @stoplight/elements this
// package is pinned to. Bumping this requires updating the
// constant below.
const stoplightVersion = "9.0.22"

// WithUI returns a stdocs.Option that replaces the default zero-JS
// docs page with Stoplight Elements.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = stoplightHTML
	}
}

const stoplightHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0;padding:0}</style>
<script src="https://cdn.jsdelivr.net/npm/@stoplight/elements@9.0.22/web-components.min.js"></script>
</head>
<body>
<elements-api apiDescriptionUrl="{{.SpecURL}}" router="hash" layout="sidebar"></elements-api>
</body>
</html>`

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
package scalar

import "github.com/FumingPower3925/stdocs"

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
const scalarHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0}</style>
</head>
<body>
<script id="api-reference" type="application/json">{{.SpecURL}}</script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`

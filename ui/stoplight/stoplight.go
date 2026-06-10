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
package stoplight

import "github.com/FumingPower3925/stdocs"

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
<script src="https://cdn.jsdelivr.net/npm/@stoplight/elements/web-components.min.js"></script>
</head>
<body>
<elements-api apiDescriptionUrl="{{.SpecURL}}" router="hash" layout="sidebar"></elements-api>
</body>
</html>`

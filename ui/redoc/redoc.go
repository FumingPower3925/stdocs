// Package redoc provides a Redoc UI for stdocs.
//
// Redoc is a clean, three-pane OpenAPI viewer, loaded from a CDN. To
// use it, import this sub-package and pass redoc.WithUI() to
// stdocs.New or stdocs.Mount:
//
//	import (
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/redoc"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), redoc.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Mount()
//
// This sub-package adds the Redoc HTML to the docs handler. The Redoc
// JavaScript and CSS are loaded from cdn.jsdelivr.net at page load
// time, so an internet connection is required.
package redoc

import "github.com/FumingPower3925/stdocs"

// WithUI returns a stdocs.Option that replaces the default zero-JS
// docs page with Redoc.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = redocHTML
	}
}

const redocHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0;padding:0}</style>
</head>
<body>
<redoc spec-url='{{.SpecURL}}'></redoc>
<script src="https://cdn.jsdelivr.net/npm/redoc/bundles/redoc.standalone.js"></script>
</body>
</html>`

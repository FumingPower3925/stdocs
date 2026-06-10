// Package redoc provides a Redoc UI for stdocs.
//
// Redoc is a clean, three-pane OpenAPI viewer, loaded from a CDN. To use
// it, import this sub-package and pass redoc.WithUI() to stdocs.New or
// stdocs.Mount:
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
//
// The CDN URL is pinned to a specific version (2.5.3, the current
// latest 2.x). Integrity hashes (sha384) are pre-computed and pinned.
// Bumping the pinned version requires re-computing the hashes; see
// CONTRIBUTING.md for the procedure.
package redoc

import "github.com/FumingPower3925/stdocs"

// redocVersion is the version of redoc this package is pinned to.
const redocVersion = "2.5.3"

// redocSRIHash is the sha384 SRI hash of redoc.standalone.js at
// the pinned version. Re-compute with:
//
//	curl -sL "https://cdn.jsdelivr.net/npm/redoc@<ver>/bundles/redoc.standalone.js" \
//	    | openssl dgst -sha384 -binary | openssl base64 -A
const redocSRIHash = "sha384-xiEssMQFSpSfLbzRZCGfxxIM5QDb2DTrU6vyoZdp2sV1L6pmOMy6MpTtUoLbpC96"

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
<script src="https://cdn.jsdelivr.net/npm/redoc@2.5.3/bundles/redoc.standalone.js"
        integrity="sha384-xiEssMQFSpSfLbzRZCGfxxIM5QDb2DTrU6vyoZdp2sV1L6pmOMy6MpTtUoLbpC96"
        crossorigin="anonymous"></script>
</body>
</html>`

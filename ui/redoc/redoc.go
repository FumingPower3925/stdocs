// Package redoc provides a Redoc UI for stdocs.
//
// Redoc is a clean, three-pane OpenAPI viewer, loaded from a CDN. To use
// it, import this sub-package and pass redoc.WithUI() to stdocs.New or
// stdocs.DocsHandler:
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
// Bumping the pinned version requires re-computing the hash (the
// recipe is inlined above the hash constant below). For an air-gapped
// build, use the ui/redocemb sub-package instead — it vendors the
// bundle in-repo.
package redoc

import (
	"fmt"

	"github.com/FumingPower3925/stdocs"
)

// redocVersion is the version of redoc this package is pinned to.
// Bumping this requires updating the SRI hash below and
// re-vendoring the bundle in ui/redocemb.
const redocVersion = "2.5.3"

// redocSRIHash is the sha384 SRI hash of redoc.standalone.js at
// the pinned version. Re-compute with:
//
//	curl -fsSL "https://cdn.jsdelivr.net/npm/redoc@<ver>/bundles/redoc.standalone.js" \
//	    | openssl dgst -sha384 -binary | openssl base64 -A
const redocSRIHash = "sha384-xiEssMQFSpSfLbzRZCGfxxIM5QDb2DTrU6vyoZdp2sV1L6pmOMy6MpTtUoLbpC96"

// WithUI returns a stdocs.Option that replaces the default docs
// page with Redoc.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = redocHTML
		c.UICSP = cspPolicy
	}
}

// cspPolicy is the Content-Security-Policy served with the Redoc docs
// page. The bundle loads from jsdelivr; style-src keeps 'unsafe-inline'
// for Redoc's runtime style injection, while script-src has no
// 'unsafe-inline'. Redoc renders in a Web Worker, so worker-src blob:
// is allowed. The external Redoc logo (cdn.redoc.ly) is not allowed, so
// the page makes no third-party connections. Browser-verified by the
// uismoke CSP test; override with stdocs.WithCSP.
const cspPolicy = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; img-src 'self' data:; font-src 'self' data:; " +
	"connect-src 'self'; worker-src blob:; " +
	"style-src https://cdn.jsdelivr.net 'unsafe-inline'; " +
	"script-src https://cdn.jsdelivr.net"

var redocHTML = fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0;padding:0}</style>
</head>
<body>
<redoc spec-url='{{.SpecURL}}'></redoc>
<script src="https://cdn.jsdelivr.net/npm/redoc@%s/bundles/redoc.standalone.js"
        integrity="%s"
        crossorigin="anonymous"></script>
</body>
</html>`, redocVersion, redocSRIHash)

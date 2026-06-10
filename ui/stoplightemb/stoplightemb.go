// Package stoplightemb provides an embedded (air-gapped) Stoplight
// Elements UI for stdocs.
//
// Unlike the sibling ui/stoplight package, which loads Stoplight
// from a CDN at page-load time, ui/stoplight/emb vendors the
// Stoplight web-component bundle in your binary so the docs UI
// works without an internet connection.
//
// The vendored bundle is pinned to @stoplight/elements@9.0.22.
//
// To use it:
//
//	import (
//	    "net/http"
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/stoplightemb"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), stoplightemb.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Handle("GET /docs/_assets/", http.StripPrefix(
//	    "/docs/_assets/", stoplightemb.AssetHandler()))
//
// The asset handler adds about 2.4 MB to your binary and is only
// included if you import this sub-package.
package stoplightemb

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/FumingPower3925/stdocs"
)

//go:embed assets*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// WithUI returns a stdocs.Option that replaces the default zero-JS
// docs page with the embedded Stoplight UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = html
	}
}

// AssetHandler returns an http.Handler that serves the embedded
// Stoplight web components at the root. Mount it on your mux with
// a path strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", stoplightemb.AssetHandler()))
func AssetHandler() http.Handler {
	return http.FileServer(http.FS(assetsSubFS))
}

const html = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0;padding:0}</style>
<link rel="stylesheet" href="/docs/_assets/styles.min.css">
<script src="/docs/_assets/web-components.min.js"></script>
</head>
<body>
<elements-api apiDescriptionUrl="{{.SpecURL}}" router="hash" layout="sidebar"></elements-api>
</body>
</html>`

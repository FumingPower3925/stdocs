// Package scalaremb provides an embedded (air-gapped) Scalar UI for
// stdocs.
//
// Unlike the sibling ui/scalar package, which loads Scalar from a
// CDN at page-load time, ui/scalaremb vendors the Scalar JavaScript
// bundle in your binary so the docs UI works without an internet
// connection.
//
// To use it:
//
//  1. Run `go generate ./...` in your module to download the Scalar
//     bundle. This populates ui/scalaremb/assets/standalone.js.
//
//  2. Import this sub-package and pass scalaremb.WithUI() to
//     stdocs.New or stdocs.Mount:
//
//     import (
//     "net/http"
//     "github.com/FumingPower3925/stdocs"
//     "github.com/FumingPower3925/stdocs/ui/scalaremb"
//     )
//
//     mux := stdocs.New(stdocs.WithTitle("My API"), scalaremb.WithUI())
//     mux.HandleFunc("GET /x", h)
//     mux.Mount()
//     mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
//
// The asset handler is small (~3.6 MB) and increases your binary size
// accordingly. It is only included if you import this sub-package.
package scalaremb

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/FumingPower3925/stdocs"
)

//go:generate bash -c "curl -sL https://cdn.jsdelivr.net/npm/@scalar/api-reference@latest/dist/browser/standalone.js -o assets/standalone.js"

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// WithUI returns a stdocs.Option that replaces the default zero-JS
// docs page with the embedded Scalar UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = html
	}
}

// AssetHandler returns an http.Handler that serves the embedded
// Scalar JavaScript bundle at the root. Mount it on your mux with
// a path strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
func AssetHandler() http.Handler {
	return http.FileServer(http.FS(assetsSubFS))
}

const html = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0}</style>
</head>
<body>
<script id="api-reference" type="application/json">{{.SpecURL}}</script>
<script src="/docs/_assets/standalone.js"></script>
</body>
</html>`

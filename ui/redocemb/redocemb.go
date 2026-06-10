// Package redocemb provides an embedded (air-gapped) Redoc UI for
// stdocs.
//
// Unlike the sibling ui/redoc package, which loads Redoc from a
// CDN at page-load time, ui/redoc/emb vendors the Redoc JavaScript
// bundle in your binary so the docs UI works without an internet
// connection.
//
// The vendored bundle is pinned to redoc@2.5.3 and its sha384 SRI
// hash is set in the sibling ui/redoc package.
//
// To use it:
//
//	import (
//	    "net/http"
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/redocemb"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), redocemb.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Handle("GET /docs/_assets/", http.StripPrefix(
//	    "/docs/_assets/", redocemb.AssetHandler()))
//
// The asset handler adds about 1.1 MB to your binary and is only
// included if you import this sub-package.
package redocemb

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
// docs page with the embedded Redoc UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = html
	}
}

// AssetHandler returns an http.Handler that serves the embedded
// Redoc JavaScript bundle at the root. Mount it on your mux with
// a path strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", redocemb.AssetHandler()))
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
</head>
<body>
<redoc spec-url='{{.SpecURL}}'></redoc>
<script src="/docs/_assets/redoc.standalone.js"></script>
</body>
</html>`

// Package swaggeruiemb provides an embedded (air-gapped) Swagger UI
// for stdocs.
//
// Unlike the sibling ui/swaggerui package, which loads Swagger UI
// from a CDN at page-load time, ui/swaggerui/emb vendors the Swagger
// UI JavaScript and CSS in your binary so the docs UI works
// without an internet connection.
//
// The vendored bundle is pinned to swagger-ui-dist@5.32.6 and its
// sha384 SRI hash is set in the sibling ui/swaggerui package.
//
// To use it:
//
//	import (
//	    "net/http"
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/swaggeruiemb"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), swaggeruiemb.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Handle("GET /docs/_assets/", http.StripPrefix(
//	    "/docs/_assets/", swaggeruiemb.AssetHandler()))
//
// The asset handler adds about 1.7 MB to your binary and is only
// included if you import this sub-package.
package swaggeruiemb

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/FumingPower3925/stdocs"
)

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// WithUI returns a stdocs.Option that replaces the default zero-JS
// docs page with the embedded Swagger UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = html
	}
}

// AssetHandler returns an http.Handler that serves the embedded
// Swagger UI bundle at the root. Mount it on your mux with a path
// strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", swaggeruiemb.AssetHandler()))
func AssetHandler() http.Handler {
	return http.FileServer(http.FS(assetsSubFS))
}

const html = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="/docs/_assets/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="/docs/_assets/swagger-ui-bundle.js" crossorigin></script>
<script>
window.onload = () => {
  SwaggerUIBundle({
    url: '{{.SpecURL}}',
    dom_id: '#swagger-ui',
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIBundle.SwaggerUIStandalonePreset
    ],
    layout: 'BaseLayout',
    deepLinking: true,
  });
};
</script>
</body>
</html>`

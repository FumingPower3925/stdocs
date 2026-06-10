// Package swaggeruiemb provides an embedded (air-gapped) Swagger UI
// for stdocs.
//
// Unlike the sibling ui/swaggerui package, which loads Swagger UI
// from a CDN at page-load time, ui/swaggeruiemb vendors the Swagger
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
//	mux.Mount()
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
	"path"
	"strings"

	"github.com/FumingPower3925/stdocs"
)

// Maintainer-only: re-vendors the pinned Swagger UI bundle into
// assets/. Consumers never need to run this; the bundle ships
// in-repo (and `go generate` cannot run inside the module cache
// anyway). Bumping the version requires updating swaggerUIVersion,
// the URLs below, the SRI hashes in ui/swaggerui, and the hash pins
// in the tests.
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.32.6/swagger-ui-bundle.js -o assets/swagger-ui-bundle.js"
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.32.6/swagger-ui.css -o assets/swagger-ui.css"

// swaggerUIVersion is the version of swagger-ui-dist vendored under
// assets/. It must match the devDependencies entry in the repo-root
// package.json.
const swaggerUIVersion = "5.32.6"

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// WithUI returns a stdocs.Option that replaces the default docs
// page with the embedded Swagger UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = html
	}
}

// AssetHandler returns an http.Handler that serves the embedded
// Swagger UI bundle at the root. File responses carry an immutable
// Cache-Control header; directory requests return 404. Mount it on
// your mux with a path strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", swaggeruiemb.AssetHandler()))
func AssetHandler() http.Handler {
	fileServer := http.FileServer(http.FS(assetsSubFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		info, err := fs.Stat(assetsSubFS, name)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, r)
	})
}

// html uses relative asset URLs so the page works under any docs
// prefix (stdocs.WithDocsPrefix) or reverse proxy: the docs page is
// always served at <prefix>/, so "_assets/..." resolves to
// <prefix>/_assets/... in the browser.
const html = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="_assets/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="_assets/swagger-ui-bundle.js"></script>
<script>
window.onload = () => {
  SwaggerUIBundle({
    url: '{{.SpecURL}}',
    dom_id: '#swagger-ui',
    presets: [SwaggerUIBundle.presets.apis],
    layout: 'BaseLayout',
    deepLinking: true,
  });
};
</script>
</body>
</html>`

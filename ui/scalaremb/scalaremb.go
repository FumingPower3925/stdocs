// Package scalaremb provides an embedded (air-gapped) Scalar UI for
// stdocs.
//
// Unlike the sibling ui/scalar package, which loads Scalar from a
// CDN at page-load time, ui/scalaremb vendors the Scalar JavaScript
// bundle in your binary so the docs UI works without an internet
// connection.
//
// The vendored bundle is pinned to @scalar/api-reference@1.59.3 and
// ships in-repo, so importing this package is all you need; the
// //go:generate directive below is a maintainer-only convenience for
// re-vendoring the bundle on a version bump.
//
// To use it:
//
//	import (
//	    "net/http"
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/scalaremb"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), scalaremb.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Mount() // registers the docs AND the embedded asset route
//
// Mount registers the asset route automatically (and tolerates a
// pre-existing manual registration). Only a manually mounted docs
// handler needs its own asset registration:
//
//	mux.ServeMux.Handle("GET /docs/", mux.Docs())
//	mux.ServeMux.Handle("GET /docs/_assets/", http.StripPrefix(
//	    "/docs/_assets/", scalaremb.AssetHandler()))
//
// The asset handler adds about 3.6 MB to your binary and is only
// included if you import this sub-package.
package scalaremb

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/FumingPower3925/stdocs"
)

// Maintainer-only: re-vendors the pinned Scalar bundle into assets/.
// Consumers never need to run this; the bundle ships in-repo (and
// `go generate` cannot run inside the module cache anyway). Bumping
// the version requires updating scalarVersion, the URL below, the
// SRI hash in ui/scalar, and the hash pin in the tests.
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.59.3/dist/browser/standalone.js -o assets/standalone.js"

// scalarVersion is the version of @scalar/api-reference vendored
// under assets/. It must match the devDependencies entry in the
// repo-root package.json.
const scalarVersion = "1.59.3"

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// WithUI returns a stdocs.Option that replaces the default docs
// page with the embedded Scalar UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = html
		c.Assets = AssetHandler()
	}
}

// AssetHandler returns an http.Handler that serves the embedded
// Scalar JavaScript bundle at the root. File responses carry an
// immutable Cache-Control header; directory requests return 404.
// Mount it on your mux with a path strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
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
<style>body{margin:0}</style>
</head>
<body>
<script id="api-reference" data-url="{{.SpecURL}}"></script>
<script src="_assets/standalone.js"></script>
</body>
</html>`

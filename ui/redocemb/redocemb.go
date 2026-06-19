// Package redocemb provides an embedded (air-gapped) Redoc UI for
// stdocs.
//
// Unlike the sibling ui/redoc package, which loads Redoc from a
// CDN at page-load time, ui/redocemb vendors the Redoc JavaScript
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
//	mux.Mount() // registers the docs AND the embedded asset route
//
// Mount registers the asset route automatically (and tolerates a
// pre-existing manual registration). Only a manually mounted docs
// handler needs its own asset registration:
//
//	mux.ServeMux.Handle("GET /docs/", mux.Docs())
//	mux.ServeMux.Handle("GET /docs/_assets/", http.StripPrefix(
//	    "/docs/_assets/", redocemb.AssetHandler()))
//
// The asset handler adds about 1.1 MB to your binary and is only
// included if you import this sub-package.
package redocemb

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/internal/uiopt"
)

// Maintainer-only: re-vendors the pinned Redoc bundle into assets/.
// Consumers never need to run this; the bundle ships in-repo (and
// `go generate` cannot run inside the module cache anyway). Bumping
// the version requires updating redocVersion, the URL below, the
// SRI hash in ui/redoc, and the hash pin in the tests.
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/redoc@2.5.3/bundles/redoc.standalone.js -o assets/redoc.standalone.js"

// redocVersion is the version of redoc vendored under assets/. It
// must match the devDependencies entry in the repo-root
// package.json.
const redocVersion = "2.5.3"

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// UIOption configures the embedded Redoc UI installed by WithUI.
type UIOption = uiopt.Option

// WithConfiguration passes Redoc options to the docs page. The map is
// passed as the options object to Redoc.init, so its keys are Redoc's
// options — for example a "theme" object, "disableSearch", or
// "hideDownloadButtons". The spec URL is the first argument to Redoc.init
// and is set by stdocs. See the Redoc options reference for the current
// list: https://redocly.com/docs/redoc/config
func WithConfiguration(cfg map[string]any) UIOption {
	return uiopt.Configuration(cfg)
}

// WithUI returns a stdocs.Option that replaces the default docs
// page with the embedded Redoc. Pass WithConfiguration to forward
// Redoc-native options.
func WithUI(opts ...UIOption) stdocs.Option {
	s := uiopt.Apply(opts)
	return func(c *stdocs.Config) {
		c.UIDoc = html
		c.Assets = AssetHandler()
		c.UICSP = cspPolicy
		c.UIConfig = s.Config
	}
}

// cspPolicy is the Content-Security-Policy served with the embedded
// Redoc docs page. Every asset is same-origin ('self'); style-src keeps
// 'unsafe-inline' for Redoc's runtime style injection, while script-src
// has no 'unsafe-inline'. Redoc is started by an inline initializer that
// calls Redoc.init, pinned by sha256 hash (recomputed from the served
// page by the parity test, so it cannot drift). Redoc renders in a Web
// Worker, so worker-src blob: is allowed. The external Redoc logo
// (cdn.redoc.ly) is not allowed, so the embedded page makes no network
// calls off the origin. Browser-verified by the uismoke CSP test;
// override with stdocs.WithCSP.
const cspPolicy = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; img-src 'self' data:; font-src 'self' data:; " +
	"connect-src 'self'; worker-src blob:; " +
	"style-src 'self' 'unsafe-inline'; " +
	"script-src 'self' 'sha256-kRi52mjhMEGeaWrXVdIBpI5Asm1Mag/mrCt/TlEPDT8='"

// AssetHandler returns an http.Handler that serves the embedded
// Redoc JavaScript bundle at the root. File responses carry an
// immutable Cache-Control header; directory requests return 404.
// Mount it on your mux with a path strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", redocemb.AssetHandler()))
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
<style>body{margin:0;padding:0}</style>
</head>
<body>
<div id="redoc"></div>
{{if .ConfigJSON}}<script id="redoc-config" type="application/json">{{.ConfigJSON}}</script>
{{end}}<script src="_assets/redoc.standalone.js"></script>
<script>
window.onload = () => {
  var el = document.getElementById('redoc-config');
  Redoc.init('{{.SpecURL}}', el ? JSON.parse(el.textContent) : {}, document.getElementById('redoc'));
};
</script>
</body>
</html>`

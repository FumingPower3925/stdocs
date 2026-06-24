// Package stoplightemb provides an embedded (air-gapped) Stoplight
// Elements UI for stdocs.
//
// Unlike the sibling ui/stoplight package, which loads Stoplight
// from a CDN at page-load time, ui/stoplightemb vendors the
// Stoplight web-component bundle in your binary so the docs UI
// works without an internet connection.
//
// The vendored bundle is pinned to @stoplight/elements@9.0.23.
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
//	mux.Mount() // registers the docs AND the embedded asset route
//
// Mount registers the asset route automatically (and tolerates a
// pre-existing manual registration). Only a manually mounted docs
// handler needs its own asset registration:
//
//	mux.ServeMux.Handle("GET /docs/", mux.Docs())
//	mux.ServeMux.Handle("GET /docs/_assets/", http.StripPrefix(
//	    "/docs/_assets/", stoplightemb.AssetHandler()))
//
// The asset handler adds about 2.4 MB to your binary and is only
// included if you import this sub-package.
package stoplightemb

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/internal/uiopt"
)

// Maintainer-only: re-vendors the pinned Stoplight Elements bundle
// into assets/. Consumers never need to run this; the bundle ships
// in-repo (and `go generate` cannot run inside the module cache
// anyway). Bumping the version requires updating stoplightVersion,
// the URLs below, the SRI hashes in ui/stoplight, and the hash pins
// in the tests.
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/@stoplight/elements@9.0.23/web-components.min.js -o assets/web-components.min.js"
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/@stoplight/elements@9.0.23/styles.min.css -o assets/styles.min.css"

// stoplightVersion is the version of @stoplight/elements vendored
// under assets/. It must match the devDependencies entry in the
// repo-root package.json.
const stoplightVersion = "9.0.23"

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// UIOption configures the embedded Stoplight Elements UI installed by
// WithUI.
type UIOption = uiopt.Option

// WithConfiguration passes Stoplight Elements configuration to the docs
// page. Stoplight Elements has no JSON configuration object — it is
// configured through attributes on its <elements-api> element — so the
// map's keys are rendered as element attributes and its values must be
// strings, booleans, or numbers. Keys are Stoplight attribute names, for
// example "hideTryItPanel", "hideSchemas", "tryItCredentialsPolicy", or
// "logo". apiDescriptionUrl, router, and layout are set by stdocs and
// cannot be overridden. See the Stoplight Elements configuration
// reference: https://github.com/stoplightio/elements/blob/main/docs/getting-started/elements/elements-options.md
func WithConfiguration(cfg map[string]any) UIOption {
	return uiopt.Configuration(cfg)
}

// WithUI returns a stdocs.Option that replaces the default docs
// page with the embedded Stoplight UI. Pass WithConfiguration to
// forward Stoplight-native options as element attributes.
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
// Stoplight Elements docs page. Every asset is same-origin ('self');
// style-src keeps 'unsafe-inline' for the runtime style injection, while
// script-src has no 'unsafe-inline'. The embedded page makes no network
// calls off the origin. Browser-verified by the uismoke CSP test;
// override with stdocs.WithCSP.
const cspPolicy = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; img-src 'self' data:; font-src 'self' data:; " +
	"connect-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'"

// AssetHandler returns an http.Handler that serves the embedded
// Stoplight web components at the root. File responses carry an
// immutable Cache-Control header; directory requests return 404.
// Mount it on your mux with a path strip, e.g.:
//
//	mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", stoplightemb.AssetHandler()))
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
<link rel="stylesheet" href="_assets/styles.min.css">
<script src="_assets/web-components.min.js"></script>
</head>
<body>
<elements-api apiDescriptionUrl="{{.SpecURL}}" router="hash" layout="sidebar"{{if .ConfigAttrs}} {{.ConfigAttrs}}{{end}}></elements-api>
</body>
</html>`

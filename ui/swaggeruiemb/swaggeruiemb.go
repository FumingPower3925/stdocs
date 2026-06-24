// Package swaggeruiemb provides an embedded (air-gapped) Swagger UI
// for stdocs.
//
// Unlike the sibling ui/swaggerui package, which loads Swagger UI
// from a CDN at page-load time, ui/swaggeruiemb vendors the Swagger
// UI JavaScript and CSS in your binary so the docs UI works
// without an internet connection.
//
// The vendored bundle is pinned to swagger-ui-dist@5.32.8 and its
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
//	mux.Mount() // registers the docs AND the embedded asset route
//
// Mount registers the asset route automatically (and tolerates a
// pre-existing manual registration). Only a manually mounted docs
// handler needs its own asset registration:
//
//	mux.ServeMux.Handle("GET /docs/", mux.Docs())
//	mux.ServeMux.Handle("GET /docs/_assets/", http.StripPrefix(
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
	"github.com/FumingPower3925/stdocs/internal/uiopt"
)

// Maintainer-only: re-vendors the pinned Swagger UI bundle into
// assets/. Consumers never need to run this; the bundle ships
// in-repo (and `go generate` cannot run inside the module cache
// anyway). Bumping the version requires updating swaggerUIVersion,
// the URLs below, the SRI hashes in ui/swaggerui, and the hash pins
// in the tests.
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.32.8/swagger-ui-bundle.js -o assets/swagger-ui-bundle.js"
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.32.8/swagger-ui.css -o assets/swagger-ui.css"

// swaggerUIVersion is the version of swagger-ui-dist vendored under
// assets/. It must match the devDependencies entry in the repo-root
// package.json.
const swaggerUIVersion = "5.32.8"

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// UIOption configures the embedded Swagger UI installed by WithUI.
type UIOption = uiopt.Option

// WithConfiguration passes Swagger UI configuration to the docs page.
// The map is merged over Swagger UI's SwaggerUIBundle({...}) options, so
// its keys are Swagger UI's parameters — for example "docExpansion",
// "filter", "defaultModelsExpandDepth", "tryItOutEnabled", or
// "displayRequestDuration". The "url" and "dom_id" are set by stdocs and
// always win. See the Swagger UI configuration reference:
// https://swagger.io/docs/open-source-tools/swagger-ui/usage/configuration/
func WithConfiguration(cfg map[string]any) UIOption {
	return uiopt.Configuration(cfg)
}

// defaultConfig disables the one Swagger UI feature that cannot work
// under the strict default Content-Security-Policy: the spec validator
// badge, an <img> loaded from validator.swagger.io (Swagger UI passes
// the spec URL so the validator can fetch and check it), blocked by
// img-src 'self'. It is a plain default a caller's WithConfiguration
// overrides — set
// WithConfiguration(map[string]any{"validatorUrl": "https://validator.swagger.io/validator"})
// to restore it (and relax the CSP with stdocs.WithDocsSecurityHeaders(false)
// or stdocs.WithCSP so it can be reached).
func defaultConfig() map[string]any {
	return map[string]any{"validatorUrl": nil}
}

// WithUI returns a stdocs.Option that replaces the default docs
// page with the embedded Swagger UI. Pass WithConfiguration to forward
// Swagger UI-native options; they override the CSP-safe defaults.
func WithUI(opts ...UIOption) stdocs.Option {
	s := uiopt.Apply(opts)
	return func(c *stdocs.Config) {
		c.UIDoc = html
		c.Assets = AssetHandler()
		c.UICSP = cspPolicy
		c.UIConfig = uiopt.Merge(defaultConfig(), s.Config)
	}
}

// cspPolicy is the Content-Security-Policy served with the embedded
// Swagger UI docs page. Every asset is same-origin ('self'); style-src
// keeps 'unsafe-inline' for Swagger UI's runtime style injection, while
// script-src has no 'unsafe-inline': the inline SwaggerUIBundle init
// script is pinned by sha256 hash, recomputed from the served page by
// the parity test so it cannot drift. Browser-verified by the uismoke
// CSP test; override with stdocs.WithCSP.
const cspPolicy = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; img-src 'self' data:; font-src 'self' data:; " +
	"connect-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"script-src 'self' 'sha256-/hiYqyotivZTycRdrHOvvzeU3mmFj2BujPaMeU4hReg='"

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
{{if .ConfigJSON}}<script id="swagger-config" type="application/json">{{.ConfigJSON}}</script>
{{end}}<script src="_assets/swagger-ui-bundle.js"></script>
<script>
window.onload = () => {
  var el = document.getElementById('swagger-config');
  var extra = el ? JSON.parse(el.textContent) : {};
  SwaggerUIBundle(Object.assign(
    {presets: [SwaggerUIBundle.presets.apis], layout: 'BaseLayout', deepLinking: true},
    extra,
    {url: '{{.SpecURL}}', dom_id: '#swagger-ui'}));
};
</script>
</body>
</html>`

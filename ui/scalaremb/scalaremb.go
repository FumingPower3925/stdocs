// Package scalaremb provides an embedded (air-gapped) Scalar UI for
// stdocs.
//
// Unlike the sibling ui/scalar package, which loads Scalar from a
// CDN at page-load time, ui/scalaremb vendors the Scalar JavaScript
// bundle in your binary so the docs UI works without an internet
// connection.
//
// The vendored bundle is pinned to @scalar/api-reference@1.62.1 and
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
	"github.com/FumingPower3925/stdocs/internal/uiopt"
)

// Maintainer-only: re-vendors the pinned Scalar bundle into assets/.
// Consumers never need to run this; the bundle ships in-repo (and
// `go generate` cannot run inside the module cache anyway). Bumping
// the version requires updating scalarVersion, the URL below, the
// SRI hash in ui/scalar, and the hash pin in the tests.
//go:generate bash -c "curl -fsSL https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.62.1/dist/browser/standalone.js -o assets/standalone.js"

// scalarVersion is the version of @scalar/api-reference vendored
// under assets/. It must match the devDependencies entry in the
// repo-root package.json.
const scalarVersion = "1.62.1"

//go:embed assets/*
var assetsFS embed.FS

// assetsSubFS is the assets/ subdirectory as a rooted fs.FS.
var assetsSubFS, _ = fs.Sub(assetsFS, "assets")

// UIOption configures the embedded Scalar UI installed by WithUI.
type UIOption = uiopt.Option

// WithConfiguration passes Scalar configuration to the docs page. The
// map is serialized to JSON and placed in Scalar's data-configuration
// attribute, so its keys are Scalar's configuration options — for
// example "theme", "layout", "hideModels", "hideSearch", or
// "documentDownloadType". The spec source is set by stdocs via data-url;
// keys that also point at a spec are the caller's responsibility. Keys
// override the CSP-safe defaults (see defaultConfig) at the TOP LEVEL
// only: a nested object such as "agent" replaces the default wholesale,
// so re-state any sub-key you want to keep. See the Scalar configuration
// reference:
// https://github.com/scalar/scalar/blob/main/documentation/configuration.md
func WithConfiguration(cfg map[string]any) UIOption {
	return uiopt.Configuration(cfg)
}

// defaultConfig disables the Scalar features that cannot work under the
// strict default Content-Security-Policy, so the page has no dead chrome:
// the "Ask AI" agent and "Generate MCP" button call scalar.com (blocked
// by connect-src 'self'), and the default web fonts come from
// fonts.scalar.com (blocked by font-src). The developer tools are hidden
// too (they only appear on localhost by default). Every key is a plain
// default that a caller's WithConfiguration overrides — pass, for
// example, WithConfiguration(map[string]any{"agent": map[string]any{
// "disabled": false}}) to bring "Ask AI" back (and relax the CSP with
// stdocs.WithDocsSecurityHeaders(false) or stdocs.WithCSP so it can
// reach scalar.com).
func defaultConfig() map[string]any {
	return map[string]any{
		"showDeveloperTools": "never",
		"agent":              map[string]any{"disabled": true},
		"mcp":                map[string]any{"disabled": true},
		"withDefaultFonts":   false,
	}
}

// WithUI returns a stdocs.Option that replaces the default docs
// page with the embedded Scalar UI. Pass WithConfiguration to forward
// Scalar-native options; they override the CSP-safe defaults.
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
// Scalar docs page. Every asset is same-origin ('self'); style-src
// keeps 'unsafe-inline' for Scalar's runtime style injection, while
// script-src has no 'unsafe-inline'. External fonts (fonts.scalar.com)
// and the Scalar registry API (api.scalar.com) are not allowed, so the
// embedded page is fully self-contained and makes no network calls off
// the origin. Browser-verified by the uismoke CSP test; override with
// stdocs.WithCSP.
const cspPolicy = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; img-src 'self' data:; font-src 'self' data:; " +
	"connect-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'"

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
<script id="api-reference" data-url="{{.SpecURL}}"{{if .ConfigAttr}} data-configuration="{{.ConfigAttr}}"{{end}}></script>
<script src="_assets/standalone.js"></script>
</body>
</html>`

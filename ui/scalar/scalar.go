// Package scalar provides a Scalar UI for stdocs.
//
// Scalar is a modern OpenAPI viewer loaded from a CDN. To use it,
// import this sub-package and pass scalar.WithUI() to stdocs.New or
// stdocs.DocsHandler:
//
//	import (
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/scalar"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), scalar.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Mount()  // or mount mux.Docs() on a parent mux
//
// This sub-package adds the Scalar HTML to the docs handler. The
// Scalar JavaScript and CSS are loaded from cdn.jsdelivr.net at page
// load time, so an internet connection is required.
//
// The CDN URL is pinned to a specific version (1.61.0) and points at
// the verbatim dist/browser/standalone.js file from the npm package,
// so its bytes are deterministic and the sha384 SRI hash below is
// pinned in the <script> tag. Bumping the pinned version requires
// re-computing the hash. For an air-gapped build, use the
// ui/scalaremb sub-package instead — it vendors the bundle in-repo.
package scalar

import (
	"fmt"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/internal/uiopt"
)

// scalarVersion is the version of @scalar/api-reference this
// package is pinned to. Bumping this requires updating the SRI
// hash below and re-vendoring the bundle in ui/scalaremb.
const scalarVersion = "1.61.0"

// scalarSRIHash is the sha384 SRI hash of dist/browser/standalone.js
// at the pinned version. Re-compute with:
//
//	curl -fsSL "https://cdn.jsdelivr.net/npm/@scalar/api-reference@<ver>/dist/browser/standalone.js" \
//	    | openssl dgst -sha384 -binary | openssl base64 -A
const scalarSRIHash = "sha384-Xs/iJWxWjlZkAyFsV9D9nI5WP5iKFnNNbnZVXMsTxD5dseQbCaaaYIb0YUtR9Ckl"

// UIOption configures the Scalar UI installed by WithUI.
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
// page with the Scalar UI. Pass WithConfiguration to forward
// Scalar-native options; they override the CSP-safe defaults.
func WithUI(opts ...UIOption) stdocs.Option {
	s := uiopt.Apply(opts)
	return func(c *stdocs.Config) {
		c.UIDoc = scalarHTML
		c.UICSP = cspPolicy
		c.UIConfig = uiopt.Merge(defaultConfig(), s.Config)
	}
}

// cspPolicy is the Content-Security-Policy served with the Scalar docs
// page. The bundle loads from jsdelivr; style-src keeps 'unsafe-inline'
// because Scalar injects styles at runtime, but script-src has no
// 'unsafe-inline'. External fonts (fonts.scalar.com) and the Scalar
// registry API (api.scalar.com) are deliberately not allowed, so the
// page makes no third-party connections and Scalar falls back to system
// fonts. Browser-verified by the uismoke CSP test; override with
// stdocs.WithCSP.
const cspPolicy = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; img-src 'self' data:; font-src 'self' data:; " +
	"connect-src 'self'; style-src https://cdn.jsdelivr.net 'unsafe-inline'; " +
	"script-src https://cdn.jsdelivr.net"

// scalarHTML is the docs page served when the scalar sub-package is in
// use. The Scalar reference web component is loaded from cdn.jsdelivr.net
// and configured to fetch the spec from the same origin's openapi.json.
//
// Scalar expects the spec URL in the `data-url` attribute. The previous
// form (the URL as element content of a <script type="application/json">)
// made Scalar treat the URL as the document and fail with "Invalid
// YAML object".
var scalarHTML = fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0}</style>
</head>
<body>
<script id="api-reference" data-url="{{.SpecURL}}"{{if .ConfigAttr}} data-configuration="{{.ConfigAttr}}"{{end}}></script>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@%s/dist/browser/standalone.js"
        integrity="%s"
        crossorigin="anonymous"></script>
</body>
</html>`, scalarVersion, scalarSRIHash)

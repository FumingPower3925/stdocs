// Package stoplight provides a Stoplight Elements UI for stdocs.
//
// Stoplight Elements is a modern OpenAPI viewer that supports
// OpenAPI 3.0.x and 3.1.x, loaded from a CDN. To use it, import this
// sub-package and pass stoplight.WithUI() to stdocs.New or
// stdocs.DocsHandler:
//
//	import (
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/stoplight"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), stoplight.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Mount()
//
// This sub-package adds the Stoplight HTML to the docs handler. The
// Stoplight web components and stylesheet are loaded from
// cdn.jsdelivr.net at page load time, so an internet connection is
// required.
//
// The CDN URLs are pinned to a specific version (9.0.23) and point
// at the verbatim web-components.min.js and styles.min.css files
// from the npm package, so their bytes are deterministic and the
// sha384 SRI hashes below are pinned in the <script>/<link> tags.
// Bumping the pinned version requires re-computing the hashes. For
// an air-gapped build, use the ui/stoplightemb sub-package instead —
// it vendors the bundle in-repo.
package stoplight

import (
	"fmt"

	"github.com/FumingPower3925/stdocs"
	"github.com/FumingPower3925/stdocs/internal/uiopt"
)

// stoplightVersion is the version of @stoplight/elements this
// package is pinned to. Bumping this requires updating the SRI
// hashes below and re-vendoring the bundle in ui/stoplightemb.
const stoplightVersion = "9.0.23"

// SRI hashes (sha384) for the pinned Stoplight Elements assets.
// Re-compute with:
//
//	curl -fsSL "https://cdn.jsdelivr.net/npm/@stoplight/elements@<ver>/web-components.min.js" \
//	    | openssl dgst -sha384 -binary | openssl base64 -A
//
// (and the same for styles.min.css).
const (
	stoplightJSHash  = "sha384-Kx8v0VsAmmNDqBDAOnY3pQFLUNZNwhakX114rKqExXeXBbDgXHBvasXBU8QxWSMB"
	stoplightCSSHash = "sha384-iVQBHadsD+eV0M5+ubRCEVXrXEBj+BqcuwjUwPoVJc0Pb1fmrhYSAhL+BFProHdV"
)

// UIOption configures the Stoplight Elements UI installed by WithUI.
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
// page with Stoplight Elements. Pass WithConfiguration to forward
// Stoplight-native options as element attributes.
func WithUI(opts ...UIOption) stdocs.Option {
	s := uiopt.Apply(opts)
	return func(c *stdocs.Config) {
		c.UIDoc = stoplightHTML
		c.UICSP = cspPolicy
		c.UIConfig = s.Config
	}
}

// cspPolicy is the Content-Security-Policy served with the Stoplight
// Elements docs page. The web-components bundle and stylesheet load
// from jsdelivr; style-src keeps 'unsafe-inline' for the runtime style
// injection, while script-src has no 'unsafe-inline'. The page makes no
// third-party connections. Browser-verified by the uismoke CSP test;
// override with stdocs.WithCSP.
const cspPolicy = "default-src 'none'; base-uri 'none'; form-action 'none'; " +
	"frame-ancestors 'self'; img-src 'self' data:; font-src 'self' data:; " +
	"connect-src 'self'; style-src https://cdn.jsdelivr.net 'unsafe-inline'; " +
	"script-src https://cdn.jsdelivr.net"

var stoplightHTML = fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>body{margin:0;padding:0}</style>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@stoplight/elements@%[1]s/styles.min.css"
      integrity="%[3]s"
      crossorigin="anonymous">
<script src="https://cdn.jsdelivr.net/npm/@stoplight/elements@%[1]s/web-components.min.js"
        integrity="%[2]s"
        crossorigin="anonymous"></script>
</head>
<body>
<elements-api apiDescriptionUrl="{{.SpecURL}}" router="hash" layout="sidebar"{{if .ConfigAttrs}} {{.ConfigAttrs}}{{end}}></elements-api>
</body>
</html>`, stoplightVersion, stoplightJSHash, stoplightCSSHash)

// Package swaggerui provides a Swagger UI for stdocs.
//
// Swagger UI is the classic OpenAPI viewer, loaded from a CDN. To use
// it, import this sub-package and pass swaggerui.WithUI() to
// stdocs.New or stdocs.DocsHandler:
//
//	import (
//	    "github.com/FumingPower3925/stdocs"
//	    "github.com/FumingPower3925/stdocs/ui/swaggerui"
//	)
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), swaggerui.WithUI())
//	mux.HandleFunc("GET /x", h)
//	mux.Mount()
//
// This sub-package adds the Swagger UI HTML to the docs handler. The
// Swagger UI JavaScript and CSS are loaded from cdn.jsdelivr.net at
// page load time, so an internet connection is required.
//
// The CDN URLs are pinned to a specific version (5.32.6, the
// current latest 5.x). Integrity hashes are pre-computed
// (sha384) and pinned in the <link>/<script> tags. Bumping the
// pinned version requires re-computing the hashes; see
// CONTRIBUTING.md for the procedure.
package swaggerui

import (
	"fmt"

	"github.com/FumingPower3925/stdocs"
)

// swaggerUIVersion is the version of swagger-ui-dist this package
// is pinned to. Bumping this requires updating the integrity
// hashes below and re-vendoring the bundle in ui/swaggeruiemb.
const swaggerUIVersion = "5.32.6"

// SRI hashes (sha384) for the pinned Swagger UI assets. These
// were computed from the published jsDelivr release (verified
// byte-identical to the npm tarball) and pinned in the
// <link>/<script> tags. If you change swaggerUIVersion,
// re-compute these with:
//
//	curl -fsSL "https://cdn.jsdelivr.net/npm/swagger-ui-dist@<ver>/swagger-ui-bundle.js" \
//	    | openssl dgst -sha384 -binary | openssl base64 -A
//
// (and the same for swagger-ui.css).
const (
	swaggerUIBundleHash = "sha384-EYdOaiRwn44zNjrw+Tfs06qYz9BGQVo2f4/pLY5i7VorbjnZNhdplAbTBk8FXHUJ"
	swaggerUICSSHash    = "sha384-9Q2fpS+xeS4ffJy6CagnwoUl+4ldAYhOs9pgZuEKxypVModhmZFzeMlvVsAjf7uT"
)

// WithUI returns a stdocs.Option that replaces the default docs
// page with Swagger UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = swaggerHTML
	}
}

var swaggerHTML = fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@%[1]s/swagger-ui.css"
      integrity="%[3]s"
      crossorigin="anonymous">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@%[1]s/swagger-ui-bundle.js"
        integrity="%[2]s"
        crossorigin="anonymous"></script>
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
</html>`, swaggerUIVersion, swaggerUIBundleHash, swaggerUICSSHash)

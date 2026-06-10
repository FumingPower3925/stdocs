// Package swaggerui provides a Swagger UI for stdocs.
//
// Swagger UI is the classic OpenAPI viewer, loaded from a CDN. To use
// it, import this sub-package and pass swaggerui.WithUI() to
// stdocs.New or stdocs.Mount:
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
package swaggerui

import "github.com/FumingPower3925/stdocs"

// WithUI returns a stdocs.Option that replaces the default zero-JS
// docs page with Swagger UI.
func WithUI() stdocs.Option {
	return func(c *stdocs.Config) {
		c.UIDoc = swaggerHTML
	}
}

const swaggerHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}}</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
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

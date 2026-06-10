# stdocs

Zero-dependency OpenAPI 3.0.3 and 3.1.0 documentation generation for a stdlib
`net/http.ServeMux` (Go 1.22+; tested on 1.26).

```go
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users/{id}", getUser)
http.Handle("/api/", mux)
http.Handle("/docs/", mux.Docs())  // docs UI at /docs/
http.ListenAndServe(":8080", nil)
```

That's it. `stdocs` walks your registered routes, parses the Go 1.22
pattern syntax, generates an OpenAPI spec from your Go types, and serves
a docs UI at `/docs` (configurable).

## Features

- **Zero runtime dependencies.** Only the Go standard library. (Test
  deps include `gopkg.in/yaml.v3` for round-trip verification, never
  imported by the runtime code.)
- **Two OpenAPI versions.** Both 3.0.3 and 3.1.0 are emitted and tested.
  Choose with `stdocs.WithVersion(stdocs.OpenAPI31)`.
- **Type-to-schema reflection.** Pass a Go value to `stdocs.WithResponse`
  or `stdocs.WithBody` and get a JSON Schema automatically, including
  pointers (nullable), slices, maps, `time.Time`, recursive types via
  `$ref`, embedded structs, generic-instantiated types, and `json` tag
  handling.
- **Smart defaults.** Handler function names are turned into summaries
  (`getUser` → "Get user"). The first path segment becomes the tag
  (`/users/...` → tag "users"). Path parameters are auto-included.
- **Security schemes.** First-class support for HTTP bearer/basic,
  API keys, and OAuth 2.0 with scopes. Register once, attach per-route
  with `WithSecurity`. Unregistered scheme names are reported as
  errors at JSON/YAML emission time.
- **Operation examples.** `WithExample(zeroValue)` emits an OpenAPI
  `example` field on the request body or response.
- **Webhooks** (3.1 only). Register out-of-band callbacks the API
  emits.
- **Five UI flavors.** Zero-JS raw HTML is the default. Scalar, Swagger
  UI, Redoc, and Stoplight Elements are available as opt-in
  sub-packages. Scalar also has an air-gapped embedded variant.
- **XSS-safe.** The docs HTML is rendered through `html/template`;
  titles and spec URLs are escaped.
- **OpenAPI-compliant output.** Valid for all major UIs (3.0.3) and
  for Scalar / Stoplight (3.1.0).
- **Escape hatch.** `WithOpenAPI(func(*Config))` gives you full access
  to the spec document for any feature stdocs does not expose directly.

## Install

```bash
go get github.com/FumingPower3925/stdocs
```

Requires Go 1.22 or later (for the `net/http` pattern syntax).

## Quick start

```go
package main

import (
    "net/http"
    "github.com/FumingPower3925/stdocs"
)

func main() {
    mux := stdocs.New(
        stdocs.WithTitle("My API"),
        stdocs.WithAPIVersion("1.0.0"),
    )
    mux.HandleFunc("GET /users/{id}", getUser)
    mux.HandleFunc("POST /users", createUser)
    http.Handle("/api/", mux)
    http.Handle("/docs/", mux.Docs())  // serves /docs/ and /docs/openapi.json
    http.ListenAndServe(":8080", nil)
}
```

Visit `http://localhost:8080/docs/` to see the docs. Visit
`http://localhost:8080/docs/openapi.json` to get the raw spec.

## Tiers

### Tier 1 — Zero-config

Routes registered with no documentation opts get a summary inferred from
the handler function name and a tag inferred from the first path segment.
Every route is auto-documented with at least a `200 OK` response.

```go
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users", listUsers)   // -> summary "List users", tag "Users"
mux.HandleFunc("GET /health", health)      // -> summary "Health", tag "Health"
http.Handle("/docs/", mux.Docs())
```

### Tier 2 — Rich metadata

Pass `stdocs` route options to attach summaries, tags, request body
types, and response types. The reflection in `WithResponse` and
`WithBody` produces JSON Schemas from your Go types automatically.

```go
type User struct {
    ID    string `json:"id" doc:"Unique ID"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
}

type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

type APIError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

mux.HandleFunc("GET /users/{id}", getUser,
    stdocs.Summary("Get a user by ID"),
    stdocs.Tags("users"),
    stdocs.WithResponse(200, User{}),
    stdocs.WithResponse(404, APIError{}),
)

mux.HandleFunc("POST /users", createUser,
    stdocs.Summary("Create a user"),
    stdocs.Tags("users"),
    stdocs.WithBody(CreateUserRequest{}),
    stdocs.WithResponse(201, User{}),
    stdocs.WithResponse(400, APIError{}),
)
```

### Security

Register schemes at the mux level, require them on routes:

```go
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithBearerAuth("bearerAuth", "JWT"),
    stdocs.WithGlobalSecurity("bearerAuth"),
)

mux.HandleFunc("GET /public", healthCheck, stdocs.WithNoSecurity())
mux.HandleFunc("GET /me", getCurrentUser)  // uses the global bearerAuth

mux.HandleFunc("POST /posts", createPost,
    stdocs.WithSecurity("bearerAuth", "write:posts", "read:posts"),
)
```

Supported scheme types: `WithBearerAuth`, `WithBasicAuth`,
`WithAPIKeyAuth` (header/query/cookie), `WithOAuth2Auth`, or a fully
custom `WithSecurityScheme`.

### Operation examples

```go
mux.HandleFunc("POST /users", createUser,
    stdocs.WithBody(CreateUserRequest{}),
    stdocs.WithResponse(201, User{}),
    stdocs.WithExample(CreateUserRequest{Title: "Buy milk"}),
    stdocs.WithResponseExample(201, User{ID: "u-1", Name: "Alice"}),
)
```

### Webhooks (3.1 only)

```go
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithVersion(stdocs.OpenAPI31),
    stdocs.WithWebhooks(map[string]stdocs.Webhook{
        "newUser": {
            Method:  "POST",
            Summary: "New user created",
            RequestBody: &stdocs.RequestBody{Schema: mustSchema(User{})},
            Responses: map[string]*stdocs.Response{
                "200": {Description: "OK"},
            },
        },
    }),
)
```

### Escape hatch

`WithOpenAPI` gives you a callback that runs after the spec is built,
with the document as a `map[string]any`. Use it for features stdocs
does not expose directly:

```go
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithOpenAPI(func(doc map[string]any) {
        doc["x-internal-api"] = true
        doc["info"].(map[string]any)["x-logo"] = map[string]any{
            "url": "https://example.com/logo.png",
        }
    }),
)
```

The callback runs once per spec build; subsequent reads use the cache.
Call `mux.Refresh()` to force a re-run.

### Tier 1 with a plain `*http.ServeMux`

If you want to expose a docs UI at a configurable prefix but your
routes are served by a stock `*http.ServeMux`, use
`stdocs.DocsHandler`. The returned handler serves the docs UI and
an empty placeholder spec. It does not introspect your mux; for
route enumeration, use `*stdocs.Mux`.

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /users", listUsers)
mux.Handle("/docs/", stdocs.DocsHandler(stdocs.WithTitle("My API")))
```

## OpenAPI versions

```go
// Default: 3.0.3
mux := stdocs.New(stdocs.WithTitle("My API"))

// Opt-in: 3.1.0
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithVersion(stdocs.OpenAPI31),
)
```

The choice is per-mux. 3.0.3 is rendered by all major UIs (Swagger UI,
Redoc, Scalar, Stoplight). 3.1.0 is rendered by Scalar and Stoplight.

## UI flavors

The default UI is a tiny zero-JS HTML page that fetches
`/docs/openapi.json` and renders a list of routes. No CDN, no extra
dependencies, ~3KB. To use a richer UI, import a sub-package:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("My API"), scalar.WithUI())
```

Available UIs:

| Sub-package | UI | Source | Notes |
|---|---|---|---|
| (none) | Raw HTML | embedded | zero-JS, zero-dependency, ~3KB |
| `ui/scalar` | Scalar | CDN | modern, pretty, requires internet |
| `ui/scalaremb` | Scalar | embedded | air-gapped, +3.6 MB binary, run `go generate` first |
| `ui/swaggerui` | Swagger UI | CDN | classic |
| `ui/redoc` | Redoc | CDN | clean three-pane |
| `ui/stoplight` | Stoplight Elements | CDN | works for both 3.0.3 and 3.1.0 |

The sub-package pattern is `oaswrap/spec-ui`-style: each sub-package
exposes a `WithUI()` `stdocs.Option` that swaps the embedded HTML.
Sub-packages are tree-shaken by the linker if not imported.

For the embedded Scalar (`ui/scalaremb`):

```bash
go generate ./...                       # fetches standalone.js into assets/
mux.Handle("GET /docs/_assets/", http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

## Configuration

### Mux-level (Options to `stdocs.New` / `stdocs.DocsHandler`)

| Option | Default | Description |
|---|---|---|
| `WithTitle(s)` | `"API"` | API title (in OpenAPI `info.title`) |
| `WithAPIVersion(s)` | `"0.0.0"` | API version (in OpenAPI `info.version`) |
| `WithVersion(s)` | `"3.0.3"` | OpenAPI spec version: `"3.0.3"` or `"3.1.0"` |
| `WithDescription(s)` | empty | Markdown description |
| `WithServer(url, desc)` | `{"/", ""}` | OpenAPI `servers` entry |
| `WithContact(name, email, url)` | none | OpenAPI `contact` object |
| `WithLicense(name, url)` | none | OpenAPI `license` object |
| `WithTag(name, desc)` | none | Declare a top-level tag |
| `WithDocsPrefix(p)` | `"/docs"` | URL prefix for the docs UI and spec |
| `WithDefaultSummary(t)` | empty | Fallback summary template |
| `WithGlobalSecurity(name, scopes...)` | none | Default security on every operation |
| `WithOpenAPI(fn)` | none | Post-build spec mutation callback |
| `WithWebhooks(map)` | none | (3.1 only) Webhook definitions |
| `WithBearerAuth(name, format)` | none | HTTP bearer scheme |
| `WithBasicAuth(name)` | none | HTTP basic scheme |
| `WithAPIKeyAuth(name, in, paramName)` | none | API key scheme |
| `WithOAuth2Auth(name, flows)` | none | OAuth 2.0 scheme |
| `WithSecurityScheme(name, scheme)` | none | Fully custom security scheme |

### Per-route (RouteOpts to `mux.HandleFunc`)

| Opt | Description |
|---|---|
| `Summary(s)` | Operation summary |
| `Description(s)` | Operation description (Markdown) |
| `Tags(...s)` | Operation tags (multiple calls accumulate) |
| `Deprecated()` | Mark as deprecated |
| `OperationID(s)` | Override the auto-derived operationId |
| `WithBody(value)` | Reflect `value` as the request body schema |
| `Optional()` | Mark the request body as not required (after `WithBody`) |
| `BodyContentType(ct)` | Override the request body content type (default `application/json`) |
| `WithResponse(status, body)` | Add a response. `body == nil` for no body (e.g. 204) |
| `ResponseDescription(status, desc)` | Override the default description for a response |
| `ResponseHeader(status, name, type, desc)` | Document a response header (e.g. rate-limit) |
| `WithExample(value)` | Add an example to the most recent body/response |
| `WithResponseExample(status, value)` | Add an example to a specific response |
| `WithParam(name, in, typ, desc)` | Add a parameter (path/query/header/cookie) |
| `QueryParam(name, typ, desc)` | Shorthand for `WithParam` with `in="query"` |
| `HeaderParam(name, typ, desc)` | Shorthand with `in="header"` |
| `CookieParam(name, typ, desc)` | Shorthand with `in="cookie"` |
| `WithSecurity(name, scopes...)` | Require a security scheme on this operation |
| `WithNoSecurity()` | Clear security on this operation (emits `security: []`) |

## How it works

Go 1.22's `net/http.ServeMux` supports method+path patterns like
`"GET /users/{id}"`, but it does not expose its registered patterns
publicly. stdocs works around this by wrapping the mux: when you call
`stdocs.New()`, you get a `*stdocs.Mux` that embeds `*http.ServeMux`
and intercepts `Handle`/`HandleFunc` calls to record pattern +
metadata. On the first call to `/docs/openapi.json`, the registry is
walked, patterns are parsed, and the OpenAPI spec is built and cached.

This means **no comments, no code generation, no `unsafe` trick** — the
pattern string itself is the documentation.

## Demo

A complete demo lives in [`cmd/demo`](./cmd/demo). Run it:

```bash
go run ./cmd/demo
# open http://localhost:8080/docs/
```

It implements a tiny task tracker with five endpoints and a recursive
`Task` type (parent-child), demonstrating most of the stdocs features.

## License

Apache-2.0. See [LICENSE](./LICENSE).

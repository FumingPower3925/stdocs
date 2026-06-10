# stdocs

**Languages:** [English](README.md) (canonical) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

stdocs turns a standard library `net/http.ServeMux` into a self-documenting API: register routes as usual, and it serves interactive documentation for them — Scalar, Swagger UI, Redoc, or Stoplight Elements at `/docs` — backed by a generated OpenAPI 3.0/3.1/3.2 document. Zero dependencies, no code generation: the patterns you already write are the source of truth.

```go
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users/{id}", getUser)
mux.Mount() // docs UI at /docs/, spec at /docs/openapi.json
log.Fatal(http.ListenAndServe(":8080", mux))
```

That's it. `stdocs` walks your registered routes, generates an OpenAPI spec from your Go types, and serves a docs UI at `/docs/`.

![The four rich UIs — Scalar, Swagger UI, Redoc, and Stoplight Elements — rendering the same generated spec](.github/uis.png)

The same generated document, rendered by each of the four bundled rich UIs — every one available CDN-pinned or fully embedded for air-gapped builds.

## Table of contents

- [Features](#features)
- [Install](#install)
- [Usage](#usage)
- [UIs](#uis)
- [How it works](#how-it-works)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Five UIs** — a tiny dependency-free default (~1.6 KB, inline JS only), plus Scalar, Swagger UI, Redoc, and Stoplight Elements — each available as a CDN sub-package (version-pinned with SRI integrity hashes) or an air-gapped embedded sub-package.
- **Three OpenAPI versions** — 3.0.4 (default), 3.1.2, and 3.2.0, all tested.
- **Reflection** — Go types become JSON Schemas: pointers, slices, maps, generics, embedded structs, recursive types via `$ref`, `json` tags (including `omitempty`, `omitzero`, and `,string`), `json.Marshaler`/`encoding.TextMarshaler` awareness.
- **Smart defaults** — function names become summaries, the first path segment becomes the tag, path params are auto-included.
- **Security** — bearer, basic, API key, OAuth 2.0 (including the 3.2 device flow). Unregistered scheme names are reported as errors.
- **Environment toggling** — `mux.Mount(enabled)`/`mux.Docs(enabled)` and `WithDisabled(true)` turn the docs on or off per environment, and `Hidden()`/`Internal()` + `WithInternal(show)` control per-route visibility — all without changing registered routes.
- **XSS-safe** — the docs HTML is rendered through `html/template`.
- **Zero deps** — only the Go standard library at runtime.

## Install

```bash
go get github.com/FumingPower3925/stdocs
```

Requires Go 1.26.4 or later (the module's `go` directive). The route patterns stdocs documents (`"GET /users/{id}"`) are the method+path syntax that `net/http.ServeMux` gained in Go 1.22.

## Usage

Routes are documented automatically from the pattern and the registered function name:

```go
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users", listUsers)        // summary "List users", tag "Users"
mux.HandleFunc("GET /health", healthCheck)     // summary "Health check", tag "Health"
```

Pass route options to attach bodies, responses, tags, and security:

```go
type User struct {
    ID    string `json:"id" doc:"Unique ID"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
}

type CreateUserRequest struct {
    Name string `json:"name"`
}

type APIError struct {
    Message string `json:"message"`
}

mux.HandleFunc("GET /users/{id}", getUser,
    stdocs.Summary("Get a user by ID"),
    stdocs.WithResponse(200, User{}),
    stdocs.WithResponse(404, APIError{}),
)

mux.HandleFunc("POST /users", createUser,
    stdocs.WithBody(CreateUserRequest{}),
    stdocs.WithResponse(201, User{}),
)
```

For features stdocs does not expose directly, use the escape hatch:

```go
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithOpenAPI(func(doc map[string]any) {
        doc["info"].(map[string]any)["x-logo"] = map[string]any{
            "url": "https://example.com/logo.png",
        }
    }),
)
```

To pin the spec to a specific OpenAPI version, use `WithVersion`:

```go
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithVersion(stdocs.OpenAPI32),  // 3.2.0
)
```

`stdocs` ships the latest patch of each minor (`OpenAPI30` = 3.0.4, `OpenAPI31` = 3.1.2, `OpenAPI32` = 3.2.0). For 3.2 you can additionally set the document's canonical URI:

```go
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithVersion(stdocs.OpenAPI32),
    stdocs.WithSelfURL("https://example.com/openapi.json"),
)
```

The full option list lives on [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs).

### Mounting and disabling the docs

`mux.Mount()` is shorthand for registering the handler returned by `mux.Docs()` on the mux itself at the configured docs prefix — there is one docs handler, two ways to place it. Use `Mount()` unless you need the handler directly (to wrap it in auth middleware or mount it on another mux). Both accept the same optional bool with the same rule: an explicit per-call value wins over `WithDisabled` in both directions.

The docs UI and the spec endpoints (`openapi.json`, `openapi.yaml`) can be turned off without unregistering routes. The decision is taken when `Mount()`/`Docs()` is called (wrap the handler yourself if you need a per-request switch):

```go
// 1) Per-mux: WithDisabled(true) makes Mount a no-op and Docs return
//    a 404 handler everywhere.
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithDisabled(os.Getenv("ENV") == "prod"),
)
mux.HandleFunc("GET /users", listUsers)
mux.Mount() // registers nothing when disabled
```

```go
// 2) Per-call: pass the condition to Mount (or to Docs when
//    mounting manually).
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users", listUsers)
mux.Mount(os.Getenv("ENV") != "prod")
```

When disabled, every request under the docs prefix gets a 404. The spec is still buildable via `mux.JSON()` and `mux.YAML()` — disabling the UI does not stop spec generation. Routes registered under the docs prefix (the docs page itself, asset handlers) never appear in the generated spec.

### Hiding individual routes

Per-route visibility composes with the switches above: `Hidden()` excludes a route from the document everywhere, and `Internal()` excludes it unless the mux was built with `WithInternal(true)` (when shown, the operation carries `x-internal: true`, the extension spec-filtering tools understand). A complete environment setup:

```go
env := os.Getenv("ENV")
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithDisabled(env == "prod"), // prod: no docs at all
    stdocs.WithInternal(env == "dev"),  // dev: full docs; elsewhere internal routes are hidden
)
mux.HandleFunc("GET /users", listUsers)                           // always documented
mux.HandleFunc("POST /admin/keys", rotateKeys, stdocs.Internal()) // documented only in dev
mux.HandleFunc("GET /healthz", healthCheck, stdocs.Hidden())      // never documented
mux.Mount()
```

Excluded routes leave no trace in the document — no paths, no schemas, no operation ids. Visibility only shapes the published documentation: hidden and internal routes **still serve traffic in every environment**. It is not access control; protect sensitive endpoints with real authentication.

If you have a hand-written OpenAPI document instead of generated routes, serve it with `DocsHandler` + `WithSpec`:

```go
spec, _ := os.ReadFile("openapi.json")
http.Handle("GET /docs/", stdocs.DocsHandler(
    stdocs.WithTitle("My API"),
    stdocs.WithSpec(spec),
))
```

## UIs

The default UI is a tiny dependency-free HTML page (~1.6 KB, inline JS only, no external assets). To use a richer UI, import a sub-package and pass its `WithUI()` option:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("My API"), scalar.WithUI())
```

For an air-gapped build (no CDN), import the matching `*emb` sub-package and mount its `AssetHandler()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalaremb"

mux := stdocs.New(stdocs.WithTitle("My API"), scalaremb.WithUI())
mux.Mount()
mux.Handle("GET /docs/_assets/",
    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

Each rich UI comes in two flavors:

| UI                                | CDN sub-package                        | Embedded sub-package                        |
| --------------------------------- | -------------------------------------- | ------------------------------------------- |
| _(default)_ (built-in, ~1.6 KB)   | —                                      | —                                           |
| Scalar                            | `ui/scalar` (~3.6 MB from the CDN)     | `ui/scalaremb` (~3.6 MB in your binary)     |
| Swagger UI                        | `ui/swaggerui` (~1.7 MB from the CDN)  | `ui/swaggeruiemb` (~1.7 MB in your binary)  |
| Redoc                             | `ui/redoc` (~1.1 MB from the CDN)      | `ui/redocemb` (~1.1 MB in your binary)      |
| Stoplight                         | `ui/stoplight` (~2.4 MB from the CDN)  | `ui/stoplightemb` (~2.4 MB in your binary)  |

All CDN URLs are pinned to exact versions with sha384 SRI integrity hashes. Sub-packages are not linked into your binary unless imported.

## How it works

Go 1.22's `net/http.ServeMux` supports method+path patterns but does not expose them publicly. `stdocs.New()` returns a `*stdocs.Mux` that embeds `*http.ServeMux` and intercepts `Handle`/`HandleFunc` calls to record pattern + metadata. On the first request to `/docs/openapi.json`, the registry is walked and the spec is built and cached (call `mux.Refresh()` to rebuild).

No comments, no code generation, no `unsafe` — the pattern string is the documentation.

A runnable demo lives in [`cmd/demo`](./cmd/demo):

```bash
go run ./cmd/demo
# open http://localhost:8080/docs/
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Translations are community-maintained; see the "Translations" section there to add or update one.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## License

Apache-2.0. See [LICENSE](LICENSE).

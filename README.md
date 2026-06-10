# stdocs

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

OpenAPI 3.0.4, 3.1.2, and 3.2.0 generation for the Go 1.22+ stdlib `net/http.ServeMux`. No runtime dependencies.

```go
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users/{id}", getUser)
http.Handle("/api/", mux)
http.Handle("/docs/", mux.Docs())
log.Fatal(http.ListenAndServe(":8080", nil))
```

That's it. `stdocs` walks your registered routes, generates an OpenAPI spec from your Go types, and serves a docs UI at `/docs/`.

## Table of contents

- [Features](#features)
- [Install](#install)
- [Usage](#usage)
- [UIs](#uis)
- [How it works](#how-it-works)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Zero deps** — only the Go standard library at runtime.
- **Three OpenAPI versions** — 3.0.4 (default), 3.1.2, and 3.2.0, all tested. Older patches (3.0.3, 3.1.0) are not exposed as constants: per the OpenAPI spec, tooling should accept any 3.0.* / 3.1.*, so a single "latest patch" per minor is the right default.
- **Reflection** — Go types become JSON Schemas: pointers, slices, maps, generics, embedded structs, recursive types via `$ref`, `json` tags.
- **Smart defaults** — function names become summaries, first path segment becomes the tag, path params are auto-included.
- **Security** — bearer, basic, API key, OAuth 2.0. Unregistered scheme names are reported as errors.
- **Eight UIs** — zero-JS HTML by default; Scalar, Swagger UI, Redoc, Stoplight as opt-in CDN sub-packages, with matching air-gapped (vendored) variants.
- **Runtime toggling** — `mux.Docs(false)` and `WithDisabled(true)` let you turn the docs UI on or off per environment, per call, or behind a feature flag, without changing registered routes.
- **XSS-safe** — the docs HTML is rendered through `html/template`.

## Install

```bash
go get github.com/FumingPower3925/stdocs
```

Requires Go 1.22 or later. Tested on Go 1.26.

## Usage

Routes are documented automatically from the pattern and the registered function name:

```go
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users", listUsers)        // summary "List users", tag "Users"
mux.HandleFunc("GET /health", healthCheck)     // summary "Health", tag "Health"
```

Pass route options to attach bodies, responses, tags, and security:

```go
type User struct {
    ID    string `json:"id" doc:"Unique ID"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
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

The full option list lives in [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs).

### Disabling the docs UI

The docs UI and the spec endpoints (`openapi.json`, `openapi.yaml`) can be turned off without unregistering routes or changing the call site. Two patterns are supported:

```go
// 1) Per-call: pass false to mux.Docs at the call site.
mux := stdocs.New(stdocs.WithTitle("My API"))
mux.HandleFunc("GET /users", listUsers)

if os.Getenv("ENV") == "prod" {
    mux.Handle("GET /docs/", mux.Docs(false))  // serves 404
} else {
    mux.Handle("GET /docs/", mux.Docs())        // serves the UI
}
```

```go
// 2) Per-mux: WithDisabled() makes Mount and Docs a no-op everywhere.
mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithDisabled(os.Getenv("ENV") == "prod"),
)
mux.HandleFunc("GET /users", listUsers)
mux.Mount()  // registers nothing when disabled
```

Both patterns return `http.NotFoundHandler()` for every request to the docs prefix. The spec is still buildable via `mux.JSON()` and `mux.YAML()` — disabling the UI does not stop spec generation. `DocsHandler` (the Tier-1 placeholder) respects `WithDisabled` the same way.

## UIs

The default UI is a tiny zero-JS HTML page (~3 KB). To use a richer UI, import a sub-package and pass its `WithUI()` option:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("My API"), scalar.WithUI())
```

For an air-gapped build (no CDN), import the matching `*emb` sub-package and mount its `AssetHandler()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalaremb"

mux := stdocs.New(stdocs.WithTitle("My API"), scalaremb.WithUI())
mux.Handle("GET /docs/_assets/",
    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

Each UI comes in two flavors:

| UI           | CDN sub-package  | Embedded sub-package | Embedded size |
| ------------ | ---------------- | -------------------- | ------------- |
| _(default)_  | —                | —                    | 3 KB          |
| Scalar       | `ui/scalar`      | `ui/scalaremb`       | ~3.6 MB       |
| Swagger UI   | `ui/swaggerui`   | `ui/swaggeruiemb`    | ~1.7 MB       |
| Redoc        | `ui/redoc`       | `ui/redocemb`        | ~1.1 MB       |
| Stoplight    | `ui/stoplight`   | `ui/stoplightemb`    | ~2.4 MB       |

CDN URLs are pinned to a specific version with sha384 SRI integrity hashes (except Scalar and Stoplight, whose jsDelivr bundles are generated on the fly and cannot be SRI-pinned; use the embedded variants for SRI). Sub-packages are tree-shaken if not imported.

## How it works

Go 1.22's `net/http.ServeMux` supports method+path patterns but does not expose them publicly. `stdocs.New()` returns a `*stdocs.Mux` that embeds `*http.ServeMux` and intercepts `Handle`/`HandleFunc` calls to record pattern + metadata. On the first request to `/docs/openapi.json`, the registry is walked and the spec is built and cached.

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

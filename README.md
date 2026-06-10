# stdocs

**Languages:** [English](README.md) (canonical) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

OpenAPI 3.0.3 and 3.1.0 generation for the Go 1.22+ stdlib `net/http.ServeMux`. No runtime dependencies.

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
- **Two OpenAPI versions** — 3.0.3 (default) and 3.1.0, both tested.
- **Reflection** — Go types become JSON Schemas: pointers, slices, maps, generics, embedded structs, recursive types via `$ref`, `json` tags.
- **Smart defaults** — function names become summaries, first path segment becomes the tag, path params are auto-included.
- **Security** — bearer, basic, API key, OAuth 2.0. Unregistered scheme names are reported as errors.
- **Five UIs** — zero-JS HTML by default; Scalar, Swagger UI, Redoc, Stoplight as opt-in sub-packages.
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

The full option list lives in [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs).

## UIs

The default UI is a tiny zero-JS HTML page (~3 KB). To use a richer UI, import a sub-package and pass its `WithUI()` option:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("My API"), scalar.WithUI())
```

Available UIs: `ui/scalar` (CDN), `ui/scalaremb` (air-gapped, ~3.6 MB), `ui/swaggerui`, `ui/redoc`, `ui/stoplight`. Sub-packages are tree-shaken if not imported.

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

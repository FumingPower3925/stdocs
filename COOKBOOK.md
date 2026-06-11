# Cookbook

The three patterns every API hits immediately, each as a complete,
compilable recipe. The [package
reference](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
documents every option used here.

## Pagination

One params struct documents the query parameters with their
constraints and defaults; one envelope type documents the page shape.
Declare both once, reuse them on every list endpoint via an opt
bundle.

```go
type ListParams struct {
    Cursor string `query:"cursor" doc:"Opaque pagination cursor"`
    Limit  int    `query:"limit" doc:"Page size" default:"20" minimum:"1" maximum:"100"`
}

type TaskPage struct {
    Items      []Task `json:"items"`
    NextCursor string `json:"next_cursor,omitempty" doc:"Cursor for the next page"`
}

var paginated = stdocs.Opts(stdocs.WithParams(ListParams{}))

mux.HandleFunc("GET /tasks", listTasks, paginated,
    stdocs.WithResponse(200, TaskPage{}),
)
```

The rendered operation documents `cursor` and `limit` with their
bounds and default, and the response schema shows the envelope with
`items` required and `next_cursor` optional (it carries `omitempty`).
The handler still parses and clamps the values itself — the document
describes the contract, `stdocs.DriftWarn` warns in development when
the two diverge.

## Auth and errors

Declare the scheme and the error envelope once at the mux level.
Secured routes document their 401 automatically; everything else
shares the 500.

```go
type APIError struct {
    Message string `json:"message" doc:"Human-readable error"`
    Code    string `json:"code,omitempty" doc:"Machine-readable error code"`
}

mux := stdocs.New(
    stdocs.WithTitle("My API"),
    stdocs.WithBearerAuth("bearerAuth", "JWT"),
    stdocs.WithDefaultResponse(500, APIError{}),
    stdocs.WithDefaultResponse(401, APIError{}), // gives the auto-401 a body
)

mux.HandleFunc("GET /me", getProfile,
    stdocs.WithSecurity("bearerAuth"),
    stdocs.WithResponse(200, Profile{}),
)
```

Every operation now documents the 500 envelope; `GET /me` documents
the security requirement and a bodied 401. Enforcement is your
middleware's job — wrap the mux as usual; the handler chain is plain
`net/http`. To keep try-it console traffic from mutating real data,
gate writes with `stdocs.FromDocs` (see the reference's "Try-it
requests and drift" section).

## Mixing generated and hand-written documents

A team with an existing, hand-written OpenAPI document serves it
through the same UIs (Tier 1, `DocsHandler`); services with generated
documents use the mux (Tier 2). Both can live in one process:

```go
// Tier 2: the service's own routes, documented by reflection.
api := stdocs.New(stdocs.WithTitle("Tasks API"))
api.HandleFunc("GET /tasks", listTasks)
api.Mount() // docs at /docs/

// Tier 1: a legacy contract served as-is under /legacy-docs/.
legacy, _ := os.ReadFile("legacy-openapi.json")
api.Handle("GET /legacy-docs/", http.StripPrefix("/legacy-docs",
    stdocs.DocsHandler(
        stdocs.WithTitle("Legacy API"),
        stdocs.WithSpec(legacy),
    )),
    stdocs.Hidden(), // the docs mount itself is not part of the API contract
)

log.Fatal(http.ListenAndServe(":8080", api))
```

Both documentation sets render with whichever UI sub-package you
import. When the legacy contract's routes migrate into the mux, their
documentation moves from the file to the registration — one route at
a time, with the golden-file test (reference: "Using the spec
downstream") diffing each step.

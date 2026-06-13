# Migrating to stdocs

Four guides for the most common starting points. Each is a mapping
table plus the workflow differences worth knowing — the [package
reference](https://pkg.go.dev/github.com/FumingPower3925/stdocs) has
the full detail on every option.

## Coming from swaggo/swag

stdocs replaces the annotation comments and the `swag init` step: the
route registration itself is the source of truth, the spec is
generated at runtime (or exported with `mux.JSON()`), and nothing is
committed or regenerated.

| swag annotation | stdocs equivalent |
| --- | --- |
| `@Summary Get a user` | `stdocs.Summary("Get a user")` — or nothing: the handler's function name infers one |
| `@Description ...` | `stdocs.Description("...")` |
| `@Tags users` | `stdocs.Tags("users")` — or nothing: the first non-version path segment infers one (mux-wide style: `WithTagFunc`) |
| `@ID get-user` | `stdocs.OperationID("get-user")` (mux-wide style: `WithOperationIDFunc`) |
| `@Param id path string true "User ID"` | nothing — path params come from the `{id}` pattern wildcard |
| `@Param limit query int false "Page size" minimum(1) maximum(100) default(20)` | `stdocs.QueryParam("limit", "integer", "Page size", stdocs.ParamMinimum(1), stdocs.ParamMaximum(100), stdocs.ParamDefault(20))` — or a `WithParams` struct with `minimum:"1" maximum:"100" default:"20"` tags |
| `@Accept json` | the default; `stdocs.WithBodyContentType` overrides |
| `@Produce json` | the default; `stdocs.WithResponseContentType` overrides |
| `@Success 200 {object} model.User` | `stdocs.WithResponse(200, User{})` |
| `@Failure 404 {object} model.APIError` | `stdocs.WithResponse(404, APIError{})` — shared errors once via `stdocs.WithDefaultResponse` |
| `@Router /users/{id} [get]` | the registration: `mux.HandleFunc("GET /users/{id}", ...)` — it cannot drift |
| `@Security BearerAuth` | `stdocs.WithSecurity("bearerAuth")` |
| `@securityDefinitions.apikey` block | `stdocs.WithBearerAuth` / `WithBasicAuth` / `WithAPIKeyAuth` / `WithOAuth2Auth` |
| `@title`, `@version`, `@contact.*`, `@license.*` | `stdocs.WithTitle`, `WithAPIVersion`, `WithContact`, `WithLicense` / `WithSPDXLicense` |
| `example:"..."` struct tags | the same tag, parsed per field type |
| `swaggertype:"string"` | `openapi:"type=string"` |
| `swaggerignore:"true"` | `openapi:"-"` |
| `format:"email"` | the same tag |

Workflow differences: there is no CLI and no generated `docs/`
package — delete both; the spec serves at `/docs/openapi.json` at
runtime; for a committed artifact (PR diffing, client generation),
use the golden-file pattern from the package reference's "Using the
spec downstream" section. Output is OpenAPI 3.0.4/3.1.2/3.2.0
(selectable), not Swagger 2.0.

## Coming from FastAPI

The `/docs` experience carries over: register routes, get interactive
documentation. What FastAPI derives from type hints at runtime,
stdocs reads from struct tags and route opts — and validation stays
your handler's job (see "Scope and non-goals" in the reference).

| FastAPI | stdocs |
| --- | --- |
| `@app.get("/tasks/{id}")` | `mux.HandleFunc("GET /tasks/{id}", getTask)` |
| `response_model=Task` | `stdocs.WithResponse(200, Task{})` |
| `status_code=201` | `stdocs.WithResponse(201, Task{})` |
| `Field(ge=1, le=5, default=3)` | `minimum:"1" maximum:"5" default:"3"` struct tags |
| `Field(min_length=1, pattern=r"...")` | `minLength:"1" pattern:"..."` |
| `Literal["a", "b"]` / `Enum` | `enum:"a,b"` |
| `Query(default=20, ge=1)` parameters | a `WithParams` struct: `Limit int \`query:"limit" default:"20" minimum:"1"\`` |
| `Header()` / `Cookie()` parameters | `header:"X-Trace-Id"` / `cookie:"session"` tags in the same struct |
| `tags=["tasks"]`, router prefixes | `stdocs.Tags(...)`; reusable bundles via `stdocs.Opts(...)`; public prefixes via `stdocs.WithPathPrefix` |
| `responses={500: {"model": Error}}` shared errors | `stdocs.WithDefaultResponse(500, APIError{})`, once per mux |
| `Depends(oauth2_scheme)` | `stdocs.WithBearerAuth(...)` + `stdocs.WithSecurity(...)` — enforcement is your middleware |
| automatic 422 documentation | not generated: stdocs documents what you declare; the automatic 401 on secured routes is the analogous nicety |
| Swagger UI at `/docs` | the built-in page, or `ui/swaggerui` / `ui/scalar` / `ui/redoc` / `ui/stoplight` |

What stays manual in Go: request body decoding (`json.NewDecoder`),
validation, and auth middleware. `stdocs.DriftWarn` helps notice when
the handlers and the document disagree during development.

## Coming from huma or another typed-handler framework

Teams usually move this direction to get back to plain
`http.HandlerFunc` and zero dependencies. The trade is explicit:
typed-handler frameworks *enforce* the contract at runtime; stdocs
*documents* it and leaves enforcement to your code.

| Typed-handler concept | stdocs equivalent |
| --- | --- |
| operation registration structs | `RouteOpt` values on `HandleFunc` |
| input struct with validation tags | the same struct shapes: constraint tags document (but do not enforce) the rules |
| output struct → response schema | `stdocs.WithResponse(status, T{})` |
| error model (RFC 7807 etc.) | your own error type + `stdocs.WithDefaultResponse` |
| middleware reading the matched operation | no equivalent — the mux is a plain `*http.ServeMux` and exposes no per-request operation metadata; `stdocs.FromDocs` covers only the narrower case of identifying docs-console traffic |
| `$ref` naming control | a `SchemaName() string` method on the type |

Keep the framework's validation semantics in your handlers (or a
validation library) — the constraint tags describe the same rules to
consumers, and `DriftWarn` plus the golden-file test keep the
document honest while you migrate route by route: both stacks can
serve side by side under one `http.ServeMux` during the transition.

## Retrofitting an existing API

Adding stdocs to a grown codebase is an additive diff: handlers and
middleware keep their signatures, and the stdocs-referencing lines
concentrate where routes are registered. The patterns that carry a
real retrofit:

**Doc-only mirror types.** Handlers that decode anonymous structs or
build `map[string]any` responses need named types for `WithBody`/
`WithResponse`. Collect them in a `docs.go` next to the registration
— zero runtime effect, one reviewable file. The mirror can drift from
the handler the same way any documentation can; `stdocs.DriftWarn`
catches status and content-type divergence in development, and the
golden-file test (reference: "Using the spec downstream") makes any
spec change reviewable in PRs.

**Upgrading from v0.4.0 with an embedded UI.** `Mount` now registers
the `_assets` route itself; the manual
`mux.Handle("GET /docs/_assets/", ...)` line from the old example is
redundant and can be deleted (Mount tolerates it either way).

**Upgrading to v0.6.1 — clean output is now the default.** The
generated document no longer carries the `Generated from Go type ...`
descriptions or the `x-stdocs-type`/`x-stdocs-warning` annotation
extensions (`x-stdocs-additionalOperations` stays — it is how
custom-method routes appear on 3.0/3.1). If you commit the spec as a
golden file, expect a one-time diff dropping those; if you relied on
the annotations (e.g. reading `x-stdocs-type` to see which Go types
were untypeable), add `WithCleanOutput(false)` to keep them.

**Upgrading DriftWarn to v0.5.0.** Drift logs get more accurate, so
expect them to change on upgrade: body sampling now looks one level
into list rows (`orders[].fee_cents`) and into array bodies, statuses
covered only by a `default` entry get its media-type contract
checked, and a body of the wrong top-level JSON kind — the classic
literal-null 200 — warns once as `body-kind-mismatch` instead of
producing a false missing-field warning per key. One class of
warnings disappears: the ServeMux's own canonicalization redirects
(`/sub` hitting a `GET /sub/` registration) are no longer attributed
to the route, since its handler never ran. New warnings on upgrade
mean the divergence was already being served; `stdocs.DriftNotify`
turns the same findings into structured input for a CI gate.

**Docs behind auth middleware.** `Mount` registers the docs on the
mux itself, so blanket auth middleware guards the docs page too. Skip
the docs prefix in the middleware when the docs should stay open —
the reference's "Mounting and toggling" section shows the pattern.

**Raw and file responses.** CSV exports, plain-text errors, and
downloads document in one opt:

```go
stdocs.WithRawResponse(200, "text/csv"),
stdocs.WithResponseHeader(200, "Content-Disposition", "string", "attachment; filename=export.csv"),
```

**List-row subsets.** When summaries repeat half of a canonical model
(`StationSummary` next to `Station`), share the common fields through
an embedded core instead of re-declaring them — the reference's
"Composing view types" passage (under Field tags) shows the pattern
and why a doc-only subset helper deliberately does not exist.

**Generic envelopes.** A `ListPage[T]` wrapper documents naturally;
give each instantiation a deliberate component name with a
`SchemaName` method when the simplified automatic one
(`ListPage_Shipment`) is not what your consumers should see.

**Inconsistent error shapes.** Different handler eras with different
error bodies document honestly: declare the dominant shape once with
`stdocs.WithDefaultResponse`, and give each era its own
`stdocs.WithFallbackResponse` inside an `stdocs.Opts` bundle — route
fallbacks beat the mux default, explicit declarations beat both. A
plain-text era bundles the same way with
`stdocs.WithFallbackRawResponse(404, "text/plain; charset=utf-8")`.
Include a status-0 fallback in each bundle (`WithFallbackResponse(0,
EraError{})` or its raw twin): without it, the era's routes carry the
mux-wide default entry as their documented catch-all — the wrong
era's shape.

**Same-named types across packages.** `handlers.Stats` and
`store.Stats` collide on the component name; the second takes a
`Stats_2` suffix and `Lint` flags it (`name-collision`). Two one-line
`SchemaName` methods give both deliberate names.

**Keeping it honest in CI.** The golden-file test plus a
`Lint` gate (allow-listing accepted findings by `Warning.Code`)
turns the retrofit's documentation debt into a visible, reviewable
list instead of silent rot.

# Changelog

All notable changes to stdocs are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

Nothing yet.

## [0.4.0] - 2026-06-11

### Added

- `WithCleanOutput(true)` strips the stdocs annotation extensions
  (`x-stdocs-type`, `x-stdocs-warning`) and the auto-generated
  "Generated from Go type ..." descriptions for documents published
  as contracts. `x-stdocs-additionalOperations` survives — it is the
  only 3.0/3.1 representation of custom-method operations.
- Component-name control: a `SchemaName() string` method names a
  type's component (value or pointer receiver), and generic
  instantiations simplify to readable identifiers
  (`Page[main.Task]` → `Page_Task`).
- The `openapi` field tag: `openapi:"-"` excludes a field from the
  document; `openapi:"type=string,format=date-time"` replaces the
  reflected schema when reflection cannot infer the wire format.
  Constraint and doc tags compose on top.
- `WithResponseContentType(status, ct)` — the response-side
  counterpart of `WithBodyContentType`; `DriftWarn` treats declared
  non-JSON content types as the contract.
- `WithMultipartBody` + `FilePart`/`FieldPart` document
  multipart/form-data file uploads.
- Spec richness: `WithExternalDocs` (document), `WithTagExternalDocs`
  (tags, order-independent with `WithTag`), the `ExternalDocs` route
  opt (operations), `WithSPDXLicense` (3.1+ `identifier`, degrading
  to name-only on 3.0), and `WithOperationIDFunc` for operationId
  style control.
- `Mux.Lint()` reports advisory consumability findings: operations
  without error responses or summaries, untyped schema fields,
  collision-suffixed component names, custom-method extension
  carriers, and vendor extensions in non-clean output.
- `MIGRATING.md`: migration guides from swaggo/swag, FastAPI, and
  typed-handler frameworks, with mapping tables, linked from the
  READMEs.

### Changed

- Nullable scalars on 3.1/3.2 emit the `anyOf` form instead of a
  `type` array: both are valid JSON Schema 2020-12, but real-world
  generators digest `anyOf` more reliably (ogen rejects the array
  form), and nullable `$ref` use sites already emitted it. Verified:
  ogen now generates a typed client from a stdocs 3.1 document, and
  CI gains that consumability gate.
- Generic component names changed from fully-qualified sanitizations
  (`main_Page_main_Task`) to the simplified form (`Page_Task`).
  Spec-affecting for documents containing generic types.

## [0.3.0] - 2026-06-11

### Added

- Schema constraint tags on struct fields: `minimum`, `maximum`,
  `exclusiveMinimum`, `exclusiveMaximum`, `minLength`, `maxLength`,
  `pattern`, `minItems`, `maxItems`, `uniqueItems`, `enum`,
  `default`, and `format`. Values are parsed per the field type and
  validated against it; misapplied or unparseable constraints panic
  at document-build time. Exclusive bounds emit the boolean form on
  3.0 and the numeric 2020-12 keywords on 3.1/3.2.
- Typed parameter declaration, two surfaces: `ParamOpt` modifiers on
  `WithParam`/`QueryParam`/`HeaderParam`/`CookieParam`
  (`ParamRequired`, `ParamDefault`, `ParamExample`, `ParamEnum`,
  `ParamMinimum`, `ParamMaximum`, `ParamMinLength`, `ParamMaxLength`,
  `ParamPattern` — values validated against the declared type), and
  `WithParams(struct)`, which reflects a struct with
  `query:`/`header:`/`cookie:` location tags, the body fields' tag
  vocabulary, and `required:"true"`.
- `Opts(...)` combines route opts into reusable bundles.
- `WithDefaultResponse(status, body)` documents a response on every
  operation that does not declare the status itself — the shared
  error envelope declared once. Status 0 means the OpenAPI `default`
  response.
- Secured operations (per-route `WithSecurity` or inherited
  `WithGlobalSecurity`) automatically document a 401; a per-route
  401 or a `WithDefaultResponse(401, body)` wins, and
  `WithAutoUnauthorized(false)` suppresses it. Spec-affecting.
- `WithPathPrefix(prefix)` prepends a documentation-only prefix to
  every emitted path, for muxes mounted behind `http.StripPrefix` or
  a stripping reverse proxy.
- `DriftWarn(mux, logf)`, a development aid that warns once per
  route and finding when a handler returns an undocumented status or
  writes a non-JSON Content-Type for a JSON-documented response.

### Changed

- `WithParam` and its shorthands fail fast: an unknown type string
  (previously a silent empty schema), an unknown `in` location, or an
  empty name now panics at registration. Duplicate (name, location)
  parameter pairs on one operation panic at document build, numeric
  constraint values must satisfy the JSON number grammar, enum tag
  members are trimmed (empty members panic), and nullable fields
  with an enum list `null` so the published contract matches
  `encoding/json`.
- YAML output reformats exponent-form numbers (`1e3` → `1.0e+3`) so
  YAML 1.1 parsers type them as numbers.
- CI gains a spec-validation job: generated 3.0.4/3.1.2 documents
  run through openapi-spec-validator and 3.2.0 validates against the
  official OpenAPI 3.2 JSON Schema on every push.
- The package documentation on pkg.go.dev is now the canonical
  reference, organized by topic with runnable examples; the READMEs
  (en/es/ca) are slimmed to hero, features, one worked example, the
  UI table, and per-topic links into the reference.

## [0.2.0] - 2026-06-11

### Added

- README documentation for the spec-as-artifact workflow (golden-file
  test, PR contract diffing, linting, client generation), the
  `doc:`/`description:`/`example:` field tags, the
  `WithResponse(0, ...)` default-response convention, and an explicit
  scope-and-non-goals section.
- `FromDocs(r, docsPrefix)` and `Mux.FromDocs(r)` report whether a
  request appears to originate from the docs UI's try-it consoles
  (best-effort, Referer-based), so teams can apply their own policy —
  block writes, divert to scratch storage, tag for observability.
  Documented with a guard-middleware example and an explicit
  not-a-security-control caveat: the signal is client-controlled and
  must only ever gate restrictions.

### Fixed

- Go `int`, `uint`, and `uint32` now reflect as `format: int64`
  (previously `int32`). Go `int`/`uint` are 64-bit on every supported
  platform and `uint32` exceeds the int32 range — clients generated
  from the old mapping mis-typed these fields. Spec-affecting.
- The `example` struct tag is parsed according to the field type:
  `example:"42"` on an integer field now emits the number 42 instead
  of a string that violated its own schema. Unparseable values panic
  at document-build time.

## [0.1.1] - 2026-06-10

### Added

- `Mux.Mount` accepts the same optional bool as `Mux.Docs`, with the
  same rule: an explicit per-call value wins over `WithDisabled` in
  both directions (`mux.Mount(env != "prod")`).
- Per-route visibility: the `Hidden()` route opt excludes a route
  from the generated document everywhere; `Internal()` excludes it
  unless the mux is configured with the new `WithInternal(true)`
  option (default false — internal routes never leak by accident).
  Shown internal operations carry the conventional `x-internal:
  true` extension. Excluded routes leave no trace in the document
  (no paths, schemas, or operation-id effects) and still serve
  traffic — visibility is documentation shaping, not access
  control.

### Changed

- Go support now follows the Go project's release policy: the two
  most recent Go releases (currently 1.25 and 1.26; `go` directive
  1.25.0, down from 1.26.4). CI computes every supported patch
  release from the go.mod floor and runs build, vet, the race-enabled
  test suite, and the YAML round-trip module on each.

## [0.1.0] - 2026-06-10

Initial release.

### Added

- OpenAPI document generation for routes registered on a wrapped
  `net/http.ServeMux` (`stdocs.New` + `HandleFunc`/`Handle` with the
  Go 1.22+ method+path pattern syntax). Specs are served at
  `<prefix>/openapi.json` and `<prefix>/openapi.yaml` together with a
  docs UI page; the prefix defaults to `/docs` (`WithDocsPrefix`).
- Three OpenAPI versions — 3.0.4 (default), 3.1.2, and 3.2.0: the
  latest patch of each 3.x minor, selected with `WithVersion`. All
  three outputs validate against external validators
  (openapi-spec-validator for 3.0/3.1, the official 2025-09-17 JSON
  Schema for 3.2). 3.2 extras: `$self` via `WithSelfURL` (validated as
  a fragment-free URI reference), first-class `query` operations,
  custom HTTP methods under `additionalOperations` (an
  `x-stdocs-additionalOperations` extension carries them on 3.0/3.1),
  and the `deviceAuthorization` OAuth flow.
- Type-to-schema reflection that follows the `encoding/json`
  contract: pointers (nullable), slices, maps, arrays, generics,
  recursive types via `$ref`, embedded structs (tagged ones nest,
  unexported ones promote), `json.RawMessage` /
  `json.Marshaler` / `encoding.TextMarshaler` awareness, and the
  `omitempty`, `omitzero`, `,string`, and `-` tag options. Component
  names are unique document-wide; same-named types from different
  packages get numeric suffixes with consistent `$ref`s.
- Smart defaults: summaries inferred from handler function names
  (closures and method values excluded), tags from the first path
  segment (matching the casing of `WithTag` declarations), wildcard
  path parameters at the path-item level, auto-200 responses, and
  document-unique operation ids that stay stable across rebuilds.
- Route opts: `Summary`, `Description`, `Tags`, `Deprecated`,
  `OperationID`, `Optional`, `WithBody`, `WithBodyContentType`,
  `WithResponse`, `WithResponseDescription`, `WithResponseHeader`,
  `WithResponseExample`, `WithExample`, `WithParam` (+ `QueryParam`,
  `HeaderParam`, `CookieParam`), `WithSecurity`, `WithNoSecurity`.
  Response/body decoration opts are order-independent.
- Security schemes: `WithBearerAuth`, `WithBasicAuth`,
  `WithAPIKeyAuth`, `WithOAuth2Auth`, `WithSecurityScheme`, and
  `WithGlobalSecurity`. Requirements referencing unregistered scheme
  names are reported as errors from `JSON()`/`YAML()`.
- Webhooks for 3.1/3.2 via `WithWebhooks`, with payload schemas
  reflected from `BodyValue` like route bodies.
- Five UIs: a dependency-free default page (~1.6 KB, inline JS only),
  plus Scalar, Swagger UI, Redoc, and Stoplight Elements — each as a
  CDN sub-package (exact versions, sha384 SRI on every script and
  stylesheet) and an air-gapped embedded sub-package (vendored npm
  bundle bytes, integrity-tested, immutable cache headers).
- Docs toggling: `Mux.Docs(enabled)` per call site and
  `WithDisabled(bool)` per mux; an explicit per-call value wins.
  Routes under the docs prefix never appear in the generated spec.
- Tier 1: `DocsHandler` + `WithSpec` serve a hand-written OpenAPI
  document behind any of the bundled UIs without wrapping a mux.
- Escape hatch: `WithOpenAPI(func(map[string]any))` mutates the built
  document before caching; `Refresh()` forces a rebuild.
- XSS-safe docs page rendered through `html/template`; relative spec
  and asset URLs work under any prefix or reverse proxy.
- Spanish and Catalan README translations with a documented
  translation policy (`CONTRIBUTING.md`, `TRANSLATORS.md`).
- Tooling: GitHub Actions CI (build, gofmt, race tests, fuzz, vet,
  golangci-lint, YAML round-trip against gopkg.in/yaml.v3, coverage),
  Dependabot for gomod/actions/npm with per-package version-parity
  tests, and a runnable demo (`cmd/demo`).

[Unreleased]: https://github.com/FumingPower3925/stdocs/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/FumingPower3925/stdocs/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/FumingPower3925/stdocs/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/FumingPower3925/stdocs/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/FumingPower3925/stdocs/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/FumingPower3925/stdocs/releases/tag/v0.1.0

# Changelog

All notable changes to stdocs are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/FumingPower3925/stdocs/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/FumingPower3925/stdocs/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/FumingPower3925/stdocs/releases/tag/v0.1.0

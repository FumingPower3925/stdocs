# Changelog

All notable changes to stdocs are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

Everything below is part of the upcoming first public release; the
project has not been tagged before.

### Added

- OpenAPI 3.0.4, 3.1.2, and 3.2.0 emitters — the latest patch of each
  3.x minor. The retired 3.0.3/3.1.0 version strings are no longer
  accepted (their documents are wire-compatible with the new patches).
- `WithSelfURL(url)` sets the OpenAPI 3.2 `$self` field; the value is
  validated as a fragment-free RFC 3986 URI reference.
- OpenAPI 3.2 `query` operations are emitted as first-class Path Item
  keys, and custom HTTP methods (PURGE, ...) are emitted under 3.2's
  `additionalOperations`. On 3.0/3.1 custom methods go under the
  always-legal `x-stdocs-additionalOperations` extension instead of an
  invalid Path Item key.
- OAuth 2.0 `deviceAuthorization` flow (OpenAPI 3.2) via
  `OAuthFlows.DeviceAuthorization` / `OAuthFlow.DeviceAuthorizationURL`.
- Embedded (air-gapped) UI sub-packages for all four rich UIs:
  `ui/scalaremb`, `ui/swaggeruiemb`, `ui/redocemb`, `ui/stoplightemb`,
  each vendoring the exact npm bundle bytes with sha384 integrity
  tests and immutable cache headers on the asset handler.
- CDN UI sub-packages (`ui/scalar`, `ui/swaggerui`, `ui/redoc`,
  `ui/stoplight`) pin exact versions with sha384 SRI integrity hashes
  on every script and stylesheet.
- `DocsHandler` + `WithSpec(json)` serve a hand-written OpenAPI
  document behind any of the bundled UIs (Tier 1). Without `WithSpec`
  a minimal valid placeholder document is served.
- `Mux.Docs(enabled ...bool)` and `WithDisabled(bool)` toggle the docs
  endpoints per call site or per mux; an explicit per-call value wins
  over the config in both directions.
- `WithResponseDescription`, `WithResponseHeader`, and
  `WithBodyContentType` route opts — order-independent: each creates
  its response/body entry when needed, and `WithResponse` merges into
  existing entries instead of replacing them.
- `WithDefaultSummary` templates substitute `{resource}` with the
  first path segment.
- Spanish and Catalan README translations with a translation policy
  (`CONTRIBUTING.md`, `TRANSLATORS.md`, `.github/CODEOWNERS`).
- Godoc `Example` functions for the core APIs.
- GitHub Actions CI (build, gofmt, race tests, fuzz, vet, lint,
  YAML round-trip against gopkg.in/yaml.v3, coverage) and a Dependabot
  config covering gomod, github-actions, and the npm manifest that
  tracks UI bundle versions; per-package parity tests fail CI when the
  manifest and the Go pins drift apart.
- Fuzz test for `pattern.ParsePattern` and a three-version parity test
  (3.0/3.1/3.2 documents agree on paths, operations, parameters,
  response statuses, operationIds, and component names).

### Fixed

- Component names are unique document-wide: all routes and webhooks
  share one schema reflector per build, so two same-named types from
  different packages get distinct components (`User`, `User_2`) with
  matching `$ref`s, instead of silently sharing one schema.
- Nullability is preserved on every use of a named type (previously
  only the first `*T` use site got the `allOf`/`anyOf` null wrapper).
- Schema reflection follows the `encoding/json` contract:
  `json.RawMessage`, `json.Marshaler`, and `encoding.TextMarshaler`
  implementors are no longer reflected structurally; `[N]byte` arrays
  are number arrays (only slices are base64); the `,string` and
  `omitzero` tag options are honored; embedded structs with a JSON tag
  name nest instead of flattening; exported fields of embedded
  unexported structs are promoted.
- 3.0/3.1 nullable types keep structural info (items, properties,
  additionalProperties).
- `doc:`/`example` struct tags on fields of named struct types are no
  longer dropped: 3.0 wraps the `$ref` in `allOf` with the siblings,
  3.1/3.2 emit them next to the `$ref`.
- Webhook request/response bodies declared via `BodyValue` are
  reflected into real schemas (previously emitted `"schema": null`,
  invalid per the official 3.1/3.2 JSON Schemas).
- Operation-ID disambiguation can no longer collide with explicit ids
  and is stable across `Refresh()` rebuilds.
- `WithGlobalSecurity` typos are caught by the same validation as
  per-route security requirements.
- Wildcard-derived path parameters are emitted once, at the path-item
  level (previously duplicated at both path and operation level).
- YAML emitter: mapping keys are quoted when needed (response status
  keys stay strings per the OpenAPI spec), control characters are
  escaped, large integers keep full precision via `json.Number`,
  top-level keys sit in column zero, and the document ends with a
  newline. Verified by round-trip tests against gopkg.in/yaml.v3,
  including a yaml.Node key-tag check.
- Summary inference returns nothing for closures (`func1`) and strips
  the `-fm` method-value suffix instead of producing garbage
  summaries; acronyms are restored in any position.
- Inferred tags adopt the casing of a matching `WithTag` declaration,
  so described tag groups no longer split from operations.
- `Mount()` registers exact patterns for the docs page and spec
  endpoints (user wildcard routes can no longer shadow them) and is
  idempotent; routes registered under the docs prefix are excluded
  from the generated spec.
- `WithDocsPrefix` normalizes its input and rejects `"/"` with a clear
  panic; `WithVersion` panics on unknown versions; `Docs` panics when
  given more than one bool.
- The docs page is rendered once at construction through
  `html/template` (escaping the title), the bare-prefix request
  redirects to the slash-terminated form, and embedded UI assets use
  relative URLs so any docs prefix and reverse-proxy setup works.
- `ui/stoplight` loads the required `styles.min.css` (the CDN page
  previously rendered unstyled).
- The default UI whitelists operation keys (the path-level
  `parameters` array no longer renders as a phantom method row) and
  builds all rows with `textContent`.
- `WithNoSecurity` emits `security: []` as the spec requires;
  unregistered scheme names error from `JSON()`/`YAML()`.
- `info.contact`/`info.license` omit empty subfields; the `required`
  array is deduplicated; the demo's ID counter is atomic.

### Changed

- Public API consolidation ahead of the freeze: `Mount(mux, ...)` →
  `DocsHandler(...)`; `ResponseDescription`/`ResponseHeader`/
  `BodyContentType` → `WithResponseDescription`/`WithResponseHeader`/
  `WithBodyContentType`; `schema` reflection moved to a shared
  per-document `Reflector`.
- The three emitters share a single `emitter` struct; the per-version
  files are thin wrappers that pick the right schema builder.
- Module floor pinned at Go 1.26.4 (`go 1.26.4` in both modules);
  CI derives its toolchain from `go.mod`.

[Unreleased]: https://github.com/FumingPower3925/stdocs/commits/main

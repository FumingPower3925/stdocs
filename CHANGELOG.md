# Changelog

All notable changes to stdocs are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed
- 3.0/3.1 nullable types now keep structural info (items,
  properties, additionalProperties) — was previously lost when
  `applyVersion` cleared `Schema.Type`.
- Generic Go types now get sanitized component names with
  numeric-suffix collision handling. Previously two
  `List[User]` instantiations produced the same component name.
- `WithNoSecurity` now emits `security: []` (was a silent
  no-op). Per the OpenAPI spec, an empty array is required to
  override a globally-applied scheme.
- Unregistered security scheme names are reported as errors
  from `JSON()`/`YAML()`.
- Wildcards with no name are filtered out of operation
  parameters (defense in depth at both registry and
  wildcard-names levels).
- Custom HTTP methods (PURGE, etc.) are flagged with an
  `x-stdocs-warning` extension rather than silently producing
  an invalid spec.
- Operation-IDs are document-unique: collisions get a numeric
  suffix (`do` -> `do`, `do_2`).
- `required` list is deduplicated after embedded struct
  flattening, with orphan entries stripped.
- Contact/license subfields are emitted only when set; no more
  `"name":""` in the spec.
- Scalar and embedded-scalar UIs use the `data-url` form for
  the API reference element.
- Tier-1 `Mount` was a placeholder that took an unused
  `*http.ServeMux` and never read it; replaced with
  `DocsHandler` that returns a real `http.Handler`.
- YAML emission of empty collections no longer has a missing
  space separator.
- `summaryFromFuncName` now correctly restores acronyms in any
  position (`parseXML` -> "Parse XML", `HTTPHandler` ->
  "HTTP handler"). The previous version had a no-op acronym
  loop.
- The docs HTML template now uses `html/template`, escaping
  `Title` and `SpecURL`; previously a `Title` of
  `<script>alert(1)</script>` would have rendered unescaped.
- The demo's `nextID` counter is now `atomic.Int64` (was a
  data race under concurrent POSTs).
- The docs prefix is normalized: leading slash added if
  missing, trailing slash stripped.
- The spec URL embedded in the docs HTML is now relative
  (`openapi.json` instead of `/docs/openapi.json`); works
  under any reverse-proxy prefix.

### Added
- `ResponseDescription(status, description)` route opt.
- `ResponseHeader(status, name, type, description)` route opt.
- `BodyContentType(ct)` route opt.
- `WithDefaultSummary` template now substitutes `{resource}`
  with the first path segment.
- `Mux.Docs(enabled ...bool)` for per-call toggling of the
  docs UI: pass `false` (or use `WithDisabled(true)`) to get
  a 404 handler at the docs prefix. Useful for environment-
  based or feature-flag activation.
- `gopkg.in/yaml.v3` test-only dependency, used to verify the
  hand-rolled YAML emitter round-trips. Lives in its own
  submodule (`internal/spec/yaml/roundtrip_test/`) so it does
  not appear in the main module's `go.mod`.
- Fuzz test for `pattern.ParsePattern` to ensure no input
  panics and well-formed invariants on success.
- 3.0/3.1 parity test.
- GitHub Actions CI workflow.

### Changed
- OpenAPI 3.0.3 and 3.1.0 emitters share a common
  `emitter` struct; the per-version files are now thin
  wrappers that pick the right schema builder.
- The docs handler for *Mux and DocsHandler share a
  `docsCore`; both render the UI via `html/template`.
- `WithVersion` panics on unknown versions (was silent
  fallback to 3.0.3).

## [0.1.0] - 2026-01-01

Initial release.

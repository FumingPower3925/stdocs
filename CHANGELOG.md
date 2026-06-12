# Changelog

All notable changes to stdocs are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

Nothing yet.

## [0.5.1] - 2026-06-12

### Fixed

- Embedded-field flattening now follows `encoding/json`'s dominance
  rules exactly: a shallower field hides deeper ones, a lone tagged
  field beats untagged same-depth rivals, any other same-depth name
  collision drops the field entirely (including diamond embedding —
  while a field below a shared join point survives, since
  `encoding/json` loses embed multiplicity there), unexported pointer
  embeds promote their exported fields, and a shadowed embed's
  required-ness no longer leaks onto the winning field. Documents
  change only where they disagreed with what `json.Marshal` actually
  serves — schemas with colliding embeds stop claiming fields that
  never reach the wire, so golden files over such shapes will show a
  diff worth reading.
- Flattening is decided by struct kind, as `encoding/json` decides
  it, which retires a family of phantom properties: an embedded
  struct with no JSON-visible fields (`sync.Mutex`, marker structs)
  no longer documents a required property named after the type,
  recursive pointer embeds no longer document a self-referential
  phantom, and marshaler embeds whose method promotion is blocked
  flatten their exported fields like the wire does. Tag-named
  unexported struct embeds — which `json.Marshal` serves as nested
  objects — are documented now, `json:"-,"` names a key `-` instead
  of dropping the field, an `openapi:"-"` field keeps participating
  in name dominance (hiding a field cannot resurface a rival the
  wire drops), and an `openapi` override on an embedded named scalar
  applies instead of panicking about a flattening that never
  happens.
- The DriftWarn reference told users to place the wrapper around
  their middleware, which its signature makes impossible; it now
  states the real limitation — responses written by surrounding
  middleware are invisible to it.

## [0.5.0] - 2026-06-12

DriftWarn graduates from log lines to a CI-gateable contract checker,
and raw responses get the same default/fallback treatment JSON bodies
have.

### Added

- `DriftNotify(fn)`: every drift warning is also delivered as a
  structured `DriftFinding` with a stable `Code` (`build-failed`,
  `undeclared-status`, `content-type-mismatch`, `body-kind-mismatch`,
  `missing-required-field`, `undocumented-fields`) — allow-list by
  Code in a test that replays traffic and drift becomes a CI gate,
  the same discipline as `Warning.Code`.
- `DriftSampleBodies` now looks one level into rows: elements of
  array-of-object properties (`orders[].fee_cents`) and of array
  bodies (`[].id`) have their keys compared against the documented
  row schema, accumulated per field so warn volume stays bounded by
  the schema, never the row count.
- `WithFallbackRawResponse(status, contentType)` and
  `WithDefaultRawResponse(status, contentType)`: raw string-typed
  default responses at both scopes, completing the
  default/fallback x JSON/raw grid — a plain-text error era bundles
  the way a JSON one does. Precedence is unchanged, and a
  `WithResponseContentType` on the route survives a raw fallback.
- A documented pattern for list-row subsets: share the canonical
  model's common fields through an embedded core (reflection
  flattens embedding the way `encoding/json` promotes fields). There
  is deliberately no doc-only subset helper — a document trimmed
  below what the handler writes is what `DriftWarn` exists to catch.

### Changed

- Drift logs get more accurate on upgrade: statuses covered only by
  a `default` entry now have its declared media type checked (a CSV
  default served as plain text warns like an explicit status would),
  and new row-level warnings surface divergence that was already
  being served. A sampled body of the wrong top-level JSON kind —
  the classic literal-null 200 against an object schema — warns once
  as `body-kind-mismatch` instead of counting every required field
  missing. One class of warnings disappears: the ServeMux's own
  canonicalization redirects (`/sub` hitting a `GET /sub/`
  registration, path cleaning) are no longer attributed to the
  route — its handler never ran, so there is no contract to compare.
- The generator notes were re-verified against current releases:
  ogen v1.17.0 fixed the nullable anyOf-with-facets rejection, and
  the `nullable-facet-generators` advisory now says so precisely
  (the Code is unchanged; exclusive bounds, the webhook-security
  bug, and the oapi-codegen nullable-enum constant still stand).
  CI generates with ogen v1.20.3.

### Fixed

- `DriftWarn`'s route snapshot records its registration generation
  before building, closing a window where a route registered
  mid-build could be snapshotted unfinalized and never revisited.
- Bodies streamed via `ReadFrom` (sendfile) flow through the
  sampling capture buffer instead of bypassing the body check.

## [0.4.2] - 2026-06-12

### Added

- `WithFallbackResponse(status, body)`: route-scoped default
  responses, built for `Opts` bundles — one fallback per error-shape
  era. Explicit declarations win, then route fallbacks, then the
  mux-level `WithDefaultResponse`.
- `WithRawResponse(status, contentType)`: raw and file responses
  (CSV, plain text, downloads) in one opt, replacing the inferred
  `WithResponse(status, "") + WithResponseContentType` idiom (which
  keeps working).
- The `openapi` tag accepts a composable `nullable` entry: bare
  `openapi:"nullable"` stacks with reflection (constraints and doc
  tags keep composing), decoupling wire-level null from Go pointers;
  combined with `required:"true"` it expresses required-but-nullable
  without changing the Go type. Also composes inside type overrides.
- `DriftSampleBodies()`: opt-in DriftWarn body sampling that compares
  response bodies' top-level keys against the documented object
  schema — each missing required key warns once per route, status,
  and field; undocumented extras once per route and status (64 KB
  cap, development aid).

## [0.4.1] - 2026-06-12

A bugs-polish-and-deep-testing release: every known defect from the
adversarial verification backlog and a five-persona user-simulation
study, fixed and pinned.

### Fixed

- Embedded UIs no longer render a silent blank page when mounted the
  documented way: `Mount` registers the `*emb` packages' asset route
  automatically through the new `Config.Assets` field, tolerating a
  pre-existing manual registration — upgrading code that followed the
  old two-line example keeps working, and the manual line can simply
  be deleted. Manual `Docs()` mounting keeps the explicit
  `AssetHandler` registration.
- Routes registered after a build now appear on the next read — the
  spec cache tracks a registration generation, so `JSON`, `YAML`,
  the served endpoints, `Lint`, and `DriftWarn` stop serving stale
  documents and the "register everything first" caveats are gone.
  Registration is synchronized with serving (matching the embedded
  `ServeMux`'s own guarantees), late registrations after the first
  build validate their schemas eagerly at `HandleFunc`, and the
  published operation ids depend only on the current route set —
  never on when intermediate builds happened.
- `Mount` builds the document eagerly: fail-fast tag panics fire at
  startup instead of inside the first docs request.
- `DriftWarn` is snapshot-based: no more race against
  `Refresh`/finalize, late registrations are picked up, and a
  JSON-documented `default` response served with a non-JSON
  Content-Type now warns (the text/plain straggler that previously
  slipped through).
- Webhook operations no longer inherit document-level security —
  they emit an explicit `security: []` override (or the new
  `Webhook.Security`), so generated clients compile again.
- Host-scoped patterns are handled honestly: a deterministic
  survivor per (method, path) — hostless wins — with an
  `x-stdocs-warning` on hosted survivors, no dangling operationId
  suffixes, host-free tag/summary inference, and a `shadowed-route`
  Lint finding for the registrations the document cannot express.
- `required:"true"` now works on body/response structs (previously a
  silent no-op outside `WithParams`); with a pointer field it
  documents required-but-nullable, and `required:"false"` opts out.
- Unsigned integer fields document `minimum: 0`, yielding to
  explicit bound tags. Spec-affecting.
- Default operationIds normalize hyphenated path segments
  (`get_internal_reconcile_status`). Spec-affecting.
- `Optional()` is order-independent with `WithBody`.
- Tag inference skips version segments (`/v1/tasks` groups under
  `Tasks`, not `V1`); `WithTagFunc` overrides the inference for
  other conventions. Spec-affecting for version-prefixed APIs.

### Added

- `Lint` findings carry a stable `Warning.Code` for CI allow-lists,
  plus new advisories: `required-with-default`, `auto-descriptions`,
  `dangling-id-suffix`, and `shadowed-route`; `exclusive-bounds`
  warns that current Go generators reject the numeric 3.1/3.2 form.
- The `ParamOpt` vocabulary is complete: `ParamFormat`,
  `ParamExclusiveMinimum`/`Maximum`, `ParamMinItems`/`MaxItems`/
  `UniqueItems`, and `ParamItems` for typed array elements.
- Reflector fuzzing (`reflect.StructOf` over arbitrary shapes and
  tag mixes) and a 500-case emitter property test pinning the
  per-version dialect rules.
- Headless rendering smoke tests for all nine bundled UIs (build tag
  `uismoke`; manual-dispatch CI job), through a shadow-DOM-piercing
  harness.
- CI gates: Spectral style errors on the 3.0.4 corpus document and
  an 80% statement-coverage floor.
- `MIGRATING.md` gains a retrofit guide (mirror types, docs behind
  auth middleware, raw responses, generic envelopes, error-shape
  eras); the reference explains the default response plainly,
  documents the required-tag override, map reflection, the raw
  download idiom, embedded-asset auto-registration, and extends the
  generator notes (oapi-codegen nullable-enum caveat, TypeScript
  generators).

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
  form), and nullable `$ref` use sites already emitted it. Type-gated
  facets (bounds, lengths, pattern, array facets, the enum with a
  null member) hoist onto the wrapper so doc UIs render them.
  Verified: ogen generates typed clients from stdocs documents, and
  CI gains that consumability gate (3.0.4 full-corpus plus a 3.1
  document; numeric 3.1 exclusive bounds are rejected by current Go
  generators — `Lint()` warns).
- 3.1/3.2 schema objects emit the `examples` array instead of the
  dialect-deprecated singular `example` (3.0 unchanged).
- `WithTag` merges into an existing declaration instead of appending
  a duplicate, making `WithTagExternalDocs` order-independence true
  in both directions.
- Component-name reservation accounts for types reflected during
  another type's build, and a `SchemaName` method promoted from an
  embedded field no longer renames the embedding type — both
  previously produced silently shared (clobbered) components.
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

[Unreleased]: https://github.com/FumingPower3925/stdocs/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/FumingPower3925/stdocs/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/FumingPower3925/stdocs/compare/v0.4.2...v0.5.0
[0.4.2]: https://github.com/FumingPower3925/stdocs/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/FumingPower3925/stdocs/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/FumingPower3925/stdocs/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/FumingPower3925/stdocs/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/FumingPower3925/stdocs/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/FumingPower3925/stdocs/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/FumingPower3925/stdocs/releases/tag/v0.1.0

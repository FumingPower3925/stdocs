// Package stdocs turns a standard library [net/http.ServeMux] into a
// self-documenting API: routes registered on the wrapped mux are
// served as interactive documentation — a /docs UI rendered by
// Scalar, Swagger UI, Redoc, Stoplight Elements, or a built-in page —
// backed by a generated OpenAPI 3.0, 3.1, or 3.2 document.
//
// The pattern syntax it documents ("GET /users/{id}") is the
// method+path routing introduced in Go 1.22. The module supports the
// two most recent Go releases, matching the Go project's release
// policy. There are no dependencies beyond the standard library and
// no code generation: the patterns you already write are the source
// of truth.
//
// # Two ways to use it
//
//   - [New] returns a [Mux] — an *http.ServeMux wrapper that records
//     route metadata as you register handlers and generates the
//     OpenAPI document from it. This is the recommended way to use
//     stdocs.
//
//   - [DocsHandler] serves a docs UI for a hand-written OpenAPI
//     document supplied via [WithSpec], without introspecting any
//     mux.
//
// # Example
//
//	type User struct {
//	    ID   string `json:"id" doc:"Unique ID"`
//	    Name string `json:"name" minLength:"1" maxLength:"100"`
//	}
//
//	func getUser(w http.ResponseWriter, r *http.Request) { /* ... */ }
//
//	func main() {
//	    mux := stdocs.New(
//	        stdocs.WithTitle("My API"),
//	        stdocs.WithVersion(stdocs.OpenAPI31),
//	        stdocs.WithBearerAuth("bearerAuth", "JWT"),
//	    )
//	    mux.HandleFunc("GET /users/{id}", getUser,
//	        stdocs.Summary("Get user by id"),
//	        stdocs.WithResponse(200, User{}),
//	        stdocs.WithSecurity("bearerAuth"),
//	    )
//	    mux.Mount() // docs UI at /docs/, spec at /docs/openapi.json
//	    log.Fatal(http.ListenAndServe(":8080", mux))
//	}
//
// Serve the mux itself — it is the http.Handler for your routes.
//
// # Options and route opts
//
// Mux-level configuration uses [Option] values passed to [New] or
// [DocsHandler] (all named With*, e.g. [WithTitle], [WithVersion],
// [WithDocsPrefix], [WithDisabled]). Per-route documentation uses
// [RouteOpt] values passed to [Mux.HandleFunc] / [Mux.Handle]:
// bare-named opts set simple operation metadata ([Summary],
// [Description], [Tags], [Deprecated], [OperationID], [Optional],
// [Hidden], [Internal]), while opts that attach bodies, responses,
// parameters, or security are named With* ([WithBody],
// [WithResponse], [WithParam], [WithSecurity], ...). [Opts] combines
// route opts into reusable bundles. Parameter declarations take
// [ParamOpt] refinements ([ParamRequired], [ParamDefault],
// [ParamMinimum], ...).
//
// # Smart defaults
//
// Undocumented routes still document themselves: the summary is
// inferred from the handler's function name (getUser → "Get user";
// closures excluded), the tag from the first non-version path
// segment (/v1/tasks groups under Tasks; [WithTagFunc] overrides the
// inference, and a matching [WithTag] declaration's casing is
// adopted),
// path parameters from the pattern's wildcards, a 200 response when
// none is declared, and a document-unique operationId from the
// method and path (get_users_by_id) that stays stable across
// rebuilds. Operations carrying a security requirement also document
// a 401 — see [WithAutoUnauthorized].
//
// Invalid documentation input — an unknown parameter type, a
// constraint tag on the wrong field type, an unparseable example —
// panics at registration or document-build time rather than
// publishing a wrong contract.
//
// # Field tags
//
// Struct fields reflected into schemas (via [WithBody],
// [WithResponse], [WithParams], or webhook payloads) carry
// documentation and constraints in tags:
//
//	type CreateTask struct {
//	    Title    string   `json:"title" doc:"Short title" minLength:"1" maxLength:"200"`
//	    Priority int      `json:"priority" minimum:"1" maximum:"5" default:"3" example:"2"`
//	    Status   string   `json:"status" enum:"pending,active,done"`
//	    Tags     []string `json:"tags" maxItems:"10" uniqueItems:"true"`
//	    Email    string   `json:"email" format:"email"`
//	}
//
// doc: (or description:) sets the schema description. The constraint
// vocabulary is minimum, maximum, exclusiveMinimum, exclusiveMaximum
// (numeric fields), minLength, maxLength, pattern (string fields),
// minItems, maxItems, uniqueItems (slice and array fields), and
// enum, default, example, format (any scalar field). Values are
// parsed according to the field's type — enum:"1,2,3" on an int
// emits numbers — and validated against it; a misapplied or
// unparseable constraint panics at registration ([WithParams]
// structs) or document-build time (bodies, responses, webhooks).
// Exclusive
// bounds render per version: the boolean draft-4 form on 3.0,
// numeric 2020-12 keywords on 3.1/3.2.
//
// Required-ness follows the encoding/json contract: every
// non-pointer field without omitempty/omitzero is required. An
// explicit required tag overrides the contract in both directions —
// required:"true" forces a field into the required list (with a
// pointer field, that documents required-but-nullable), and
// required:"false" keeps it out. Maps document as objects whose
// additionalProperties schema comes from the value type.
//
// When reflection cannot infer a field's wire format, the openapi
// tag is the per-field escape hatch: openapi:"-" excludes the field
// from the document (JSON serialization is unaffected),
// openapi:"type=string,format=date-time" replaces the reflected
// schema entirely — constraint and doc tags still compose on top —
// and a bare openapi:"nullable" stacks with reflection, decoupling
// wire-level null from Go pointers (with required:"true", that is
// required-but-nullable without changing the Go type).
//
// Composing view types: when a list endpoint returns a subset of a
// canonical model (an OrderSummary next to Order), share the common
// fields through an embedded core instead of re-declaring them —
// reflection flattens embedded structs the way encoding/json
// promotes their fields, so the documented shape and the served
// JSON stay in agreement (keep cores collision-free: same-named
// fields across sibling embeds are where the two can diverge):
//
//	type OrderCore struct {
//	    ID     string `json:"id" required:"true"`
//	    Status string `json:"status" enum:"open,paid,refunded"`
//	}
//	type Order struct {
//	    OrderCore
//	    Items []Item `json:"items"`
//	}
//	type OrderSummary struct{ OrderCore }
//
// The embedded core appears as its own component schema; that is
// expected, not a leak. There is deliberately no doc-only subset
// helper: encoding/json has no way to drop a promoted field at
// serialization time, so a document trimmed below what the handler
// writes would be precisely the divergence [DriftWarn] exists to
// catch.
//
// # Parameters
//
// Path parameters come from the pattern's wildcards automatically.
// Query, header, and cookie parameters are declared either inline —
// [QueryParam], [HeaderParam], [CookieParam], or the general
// [WithParam] — refined with [ParamOpt] values:
//
//	stdocs.QueryParam("limit", "integer", "Page size",
//	    stdocs.ParamDefault(20), stdocs.ParamMinimum(1), stdocs.ParamMaximum(100))
//
// or as a struct via [WithParams], sharing the field-tag vocabulary:
//
//	type ListParams struct {
//	    Cursor string `query:"cursor" doc:"Opaque pagination cursor"`
//	    Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
//	    Trace  string `header:"X-Trace-Id" required:"true"`
//	}
//
// # Responses
//
// [WithResponse] declares a response per status; response-decorating
// opts ([WithResponseDescription], [WithResponseHeader],
// [WithResponseExample], [WithResponseContentType]) are
// order-independent. Status 0 declares the OpenAPI "default"
// response — the catch-all consumers fall back to for undeclared
// statuses; in plain terms, "any status not listed here: expect this
// shape", conventionally the shared error body. A response with a
// string body and a non-JSON content type documents raw downloads:
// WithResponse(200, "") + WithResponseContentType(200, "text/csv"). [WithDefaultResponse] declares a response once at the
// mux level (typically the shared error envelope) and documents it
// on every operation that does not declare the status itself.
// [WithMultipartBody] documents multipart/form-data file uploads
// from [FilePart] and [FieldPart] declarations. [WithRawResponse]
// documents raw responses (CSV, plain text, downloads) as a
// string-typed body under a given content type in one opt, and
// [WithFallbackResponse] is the route-scoped counterpart of
// [WithDefaultResponse] — built for [Opts] bundles, so codebases
// with several error-shape eras declare one fallback per era
// (explicit declarations win, then route fallbacks, then mux
// defaults). Both forms have raw twins, [WithFallbackRawResponse]
// and [WithDefaultRawResponse], so a plain-text error era bundles
// the same way a JSON one does; precedence is unchanged, and a
// WithResponseContentType already on the route survives a raw
// fallback's content type.
//
// # Visibility
//
// [Hidden] excludes a route from the document everywhere; [Internal]
// excludes it unless the mux was built with [WithInternal](true), in
// which case the operation carries the conventional x-internal: true
// extension. Excluded routes leave no trace — no paths, schemas, or
// operation-id effects — and still serve traffic: visibility shapes
// documentation, it is not access control.
//
// # Mounting and toggling
//
// [Mux.Mount] registers the docs handler on the mux itself under the
// docs prefix (default /docs, configurable with [WithDocsPrefix]);
// [Mux.Docs] returns the same handler for mounting elsewhere or
// wrapping in middleware. Both accept an optional bool —
// mux.Mount(env != "prod") — and an explicit per-call value wins
// over [WithDisabled] in both directions. Disabled docs serve 404;
// the spec stays buildable via [Mux.JSON] and [Mux.YAML].
//
// Mount registers the docs on the mux itself, so blanket auth
// middleware wrapped around the mux guards the docs page too —
// often surprising in dev. Skip the docs prefix in the middleware
// when the docs should stay open:
//
//	func auth(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        if r.URL.Path == "/docs" || strings.HasPrefix(r.URL.Path, "/docs/") {
//	            next.ServeHTTP(w, r) // docs stay public
//	            return
//	        }
//	        // ... verify credentials ...
//	        next.ServeHTTP(w, r)
//	    })
//	}
//
// When the mux is mounted under a prefix the application never sees
// — [net/http.StripPrefix] or a stripping reverse proxy —
// [WithPathPrefix] prepends the public prefix to every documented
// path.
//
// # Docs UIs
//
// The default docs page is a dependency-free ~1.6 KB route list.
// Four rich UIs ship as sub-packages, each in two flavors: a CDN
// variant pinning exact versions with sha384 subresource-integrity
// hashes, and an embedded variant vendoring the npm bundle bytes for
// air-gapped builds:
//
//	import "github.com/FumingPower3925/stdocs/ui/scalar"     // CDN, SRI-pinned
//	import "github.com/FumingPower3925/stdocs/ui/scalaremb"  // embedded
//
//	mux := stdocs.New(stdocs.WithTitle("My API"), scalar.WithUI())
//
// The packages are ui/scalar, ui/swaggerui, ui/redoc, and
// ui/stoplight, plus their *emb twins. SRI means subresource
// integrity: the CDN script tags carry sha384 hashes browsers verify
// before executing the fetched assets. The embedded twins serve
// their bundles from the binary — [Mux.Mount] registers their asset
// route automatically; only when mounting the handler manually via
// [Mux.Docs] does the asset route need its own registration:
//
//	mux.ServeMux.Handle("GET /docs/", mux.Docs())
//	mux.ServeMux.Handle("GET /docs/_assets/",
//	    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
//
// # Try-it requests and drift
//
// The rich UIs' try-it consoles send real requests. [FromDocs]
// identifies them (best-effort, Referer-based) so middleware can
// restrict what they may do — never use it to grant access.
// [DriftWarn] wraps the mux in a development aid that warns when a
// handler returns a status the document does not declare or serves a
// response with a content type contradicting the declared body; with
// [DriftSampleBodies] it additionally compares response bodies'
// keys — top-level and one level of list rows — against the
// documented schema. [DriftNotify] delivers every finding as a
// [DriftFinding] with a stable Code, turning replayed traffic into a
// CI gate.
//
// # Component names
//
// Schema component names come from the Go type name; same-named
// types from different packages get numeric suffixes, and generic
// instantiations simplify to readable identifiers
// (Page[main.Task] → Page_Task). A type can name itself — useful
// because component names become class names in generated clients —
// by implementing:
//
//	func (TaskPage) SchemaName() string { return "TaskPage" }
//
// The override wins over every automatic rule; collisions are still
// suffixed.
//
// # Using the spec downstream
//
// [Mux.JSON] and [Mux.YAML] return the exact bytes served at the
// spec endpoints. Output is deterministic — sorted keys, stable
// operationIds and component names across rebuilds — so the spec
// works as a committed artifact: golden-file tests, contract diffing
// in PRs, linting, and client generation. For documents published as
// contracts, [WithCleanOutput] strips the stdocs annotation
// extensions and auto-generated descriptions, and [Mux.Lint] reports
// advisory consumability findings (operations without error
// responses, untyped fields, collision-suffixed names). Determinism
// holds within a release; upgrading stdocs itself may legitimately
// change the emitted bytes, so regenerate golden files when bumping
// the dependency and review the diff like any contract change.
//
// Generator notes: current Go client generators (ogen, oapi-codegen)
// reject the numeric exclusive-bound form that 3.1/3.2 correctly
// emit — generate from the 3.0.4 document when exclusiveMinimum or
// exclusiveMaximum tags are in play ([Mux.Lint] warns about this),
// and oapi-codegen consumes 3.0 documents only. oapi-codegen also
// generates a spurious "<nil>" constant from nullable enums (the
// null member 3.0 legally requires; an upstream bug) — prefer
// non-nullable enum fields (no pointer and no openapi:"nullable")
// in generator-facing contracts. On 3.1/3.2, nullability combined
// with a default, uniqueItems, or byte format produces anyOf forms
// that ogen releases before v1.17.0 reject — and oapi-codegen,
// consuming 3.0 only, never sees ([Mux.Lint] warns:
// nullable-facet-generators); the 3.0.4 document handles them all.
// An explicit Webhook.Security requirement trips the same ogen
// webhook-codegen bug that motivated the security: [] default —
// prefer documenting webhook auth in the description until upstream
// fixes it. openapi-typescript and similar TypeScript generators consume
// either version directly; enums become string-literal unions and
// SchemaName methods control the generated type names. A
// document-level default response ([WithDefaultResponse] with
// status 0) enables ogen's typed convenient-error handling;
// [DriftWarn] still checks the default entry's body and content-type
// contracts, but a default entry covers every status, so
// undeclared-status findings are off the table — weigh that against
// the generator convenience when drift gating matters. [WithOpenAPI]
// registers a hook that may mutate the document before caching, as
// an escape hatch for anything stdocs does not expose; [Mux.Refresh]
// forces a rebuild.
//
// # OpenAPI versions
//
// stdocs emits the latest patch of each supported minor: [OpenAPI30]
// (3.0.4, the default), [OpenAPI31] (3.1.2), and [OpenAPI32]
// (3.2.0). Select one with [WithVersion]. For 3.2, [WithSelfURL]
// sets the document's canonical URI ($self — the URL at which the
// published document itself lives). All three outputs are
// validated against external validators in CI.
//
// # Scope and non-goals
//
// stdocs documents stdlib ServeMux applications; it does not
// integrate with third-party routers, does not validate requests or
// enforce the documented contract at runtime, and uses no code
// generation, comment annotations, or dependencies — permanently, by
// design. The document describes intent; keeping handlers honest is
// the application's job ([DriftWarn] helps notice when they are
// not).
package stdocs

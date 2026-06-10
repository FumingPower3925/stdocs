// Package stdocs builds an OpenAPI 3.0, 3.1, or 3.2 specification
// from routes registered on a standard library [net/http.ServeMux]
// and serves the spec (and an optional docs UI) over HTTP.
//
// The pattern syntax it documents ("GET /users/{id}") is the
// method+path routing introduced in Go 1.22; the module itself
// requires Go 1.26 or later.
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
//	    ID   string `json:"id"`
//	    Name string `json:"name"`
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
// Serve the mux itself (it is the http.Handler for your routes). To
// mount it under a sub-path, wrap it with [net/http.StripPrefix] —
// note the generated spec paths will not include the stripped
// prefix.
//
// # Options and route opts
//
// Mux-level configuration uses [Option] values passed to [New] or
// [DocsHandler] (all named With*, e.g. [WithTitle], [WithVersion],
// [WithDocsPrefix], [WithDisabled]). Per-route documentation uses
// [RouteOpt] values passed to [Mux.HandleFunc] / [Mux.Handle]:
// bare-named opts set simple operation metadata ([Summary],
// [Description], [Tags], [Deprecated], [OperationID], [Optional]),
// while opts that attach bodies, responses, parameters, or security
// are named With* ([WithBody], [WithResponse], [WithParam],
// [WithSecurity], ...).
//
// # OpenAPI versions
//
// stdocs emits the latest patch of each supported minor: [OpenAPI30]
// (3.0.4, the default), [OpenAPI31] (3.1.2), and [OpenAPI32]
// (3.2.0). Select one with [WithVersion]. For 3.2, [WithSelfURL]
// sets the document's canonical URI ($self).
package stdocs

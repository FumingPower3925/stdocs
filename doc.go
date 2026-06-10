// Package stdocs builds an OpenAPI 3.0 / 3.1 spec from routes
// registered on a Go 1.22+ http.ServeMux and serves the spec (and
// an optional docs UI) over HTTP.
//
// # Tiers
//
// There are two usage tiers:
//
//   - Tier 1 (DocsHandler): expose a docs UI and a placeholder
//     spec at a configurable URL prefix. The spec is empty; this
//     tier is for users who already have a hand-written spec or
//     who don't need route enumeration.
//
//   - Tier 2 (*Mux): register Go handlers with .HandleFunc / .Handle
//     and stdocs reflects them into a populated OpenAPI document.
//     This is the recommended way to use stdocs.
//
// # Example
//
//	mux := stdocs.New(
//	    stdocs.WithTitle("My API"),
//	    stdocs.WithVersion(stdocs.OpenAPI31),
//	    stdocs.WithBearerAuth("bearerAuth"),
//	)
//
//	mux.HandleFunc("GET /users/{id}", getUser,
//	    stdocs.Summary("Get user by id"),
//	    stdocs.Response(200, User{}),
//	)
//
//	http.Handle("/api/", mux)
//	http.Handle("/docs/", mux.Docs())
//
// # Stable v0.1.x API
//
// The 0.1 line is intentionally minimal: route registration, a
// small set of route opts (Summary, Description, Tags, WithBody,
// WithResponse, WithExample, WithParam, WithSecurity, etc.), and
// JSON/YAML emission. See the README for the full option list.
package stdocs

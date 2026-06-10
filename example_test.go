package stdocs_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/FumingPower3925/stdocs"
)

// The simplest possible setup: wrap the mux, register routes, mount
// the docs, serve. The generated document is available programmatically
// through JSON() too.
func ExampleNew() {
	mux := stdocs.New(stdocs.WithTitle("My API"))
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {})
	mux.Mount() // docs UI at /docs/, spec at /docs/openapi.json

	b, _ := mux.JSON()
	var doc struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title string `json:"title"`
		} `json:"info"`
	}
	_ = json.Unmarshal(b, &doc)
	fmt.Println(doc.OpenAPI, doc.Info.Title)
	// Output: 3.0.4 My API
}

// Selecting the OpenAPI version. stdocs ships the latest patch of
// each supported minor: 3.0.4 (default), 3.1.2, and 3.2.0.
func ExampleWithVersion() {
	mux := stdocs.New(
		stdocs.WithTitle("My API"),
		stdocs.WithVersion(stdocs.OpenAPI32),
		stdocs.WithSelfURL("https://api.example.com/openapi.json"),
	)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {})

	b, _ := mux.JSON()
	var doc struct {
		OpenAPI string `json:"openapi"`
		Self    string `json:"$self"`
	}
	_ = json.Unmarshal(b, &doc)
	fmt.Println(doc.OpenAPI, doc.Self)
	// Output: 3.2.0 https://api.example.com/openapi.json
}

// Documenting request and response bodies through reflection.
func ExampleWithResponse() {
	type User struct {
		ID   string `json:"id" doc:"Unique ID"`
		Name string `json:"name"`
	}
	type CreateUserRequest struct {
		Name string `json:"name"`
	}

	mux := stdocs.New(stdocs.WithTitle("My API"))
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.Summary("Create a user"),
		stdocs.WithBody(CreateUserRequest{}),
		stdocs.WithResponse(201, User{}),
		stdocs.WithResponseDescription(201, "The newly created user"),
	)

	b, _ := mux.JSON()
	var doc struct {
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	_ = json.Unmarshal(b, &doc)
	for name := range doc.Components.Schemas {
		if name == "User" {
			fmt.Println("components.schemas has User")
		}
	}
	// Output: components.schemas has User
}

// Registering a security scheme and requiring it on a route.
func ExampleWithBearerAuth() {
	mux := stdocs.New(
		stdocs.WithTitle("My API"),
		stdocs.WithBearerAuth("bearerAuth", "JWT"),
		stdocs.WithGlobalSecurity("bearerAuth"),
	)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.WithNoSecurity(), // public route opts out of the global scheme
	)

	b, _ := mux.JSON()
	var doc struct {
		Paths map[string]map[string]struct {
			Security []map[string][]string `json:"security"`
		} `json:"paths"`
	}
	_ = json.Unmarshal(b, &doc)
	fmt.Println(len(doc.Paths["/health"]["get"].Security))
	// Output: 0
}

// Serving a docs UI for a hand-written spec (Tier 1) — no route
// introspection involved.
func ExampleDocsHandler() {
	spec := []byte(`{"openapi":"3.0.4","info":{"title":"Hand-written","version":"1.0.0"},"paths":{}}`)

	mux := http.NewServeMux()
	mux.Handle("GET /docs/", stdocs.DocsHandler(
		stdocs.WithTitle("Hand-written"),
		stdocs.WithSpec(spec),
	))
	fmt.Println("docs mounted")
	// Output: docs mounted
}

// Turning the docs off per environment without touching routes.
func ExampleMux_Docs() {
	prod := true
	mux := stdocs.New(stdocs.WithTitle("My API"))
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {})

	// An explicit per-call value wins over WithDisabled in both
	// directions; mux.Docs(false) serves 404 for every docs request.
	mux.ServeMux.Handle("GET /docs/", mux.Docs(!prod))
	fmt.Println("docs enabled:", !prod)
	// Output: docs enabled: false
}

// Restricting what try-it consoles may do: the docs page's "Try it
// out" buttons send real requests, identified (best-effort) by their
// Referer. FromDocs gates a restriction — never use it to grant
// access, since the header is client-controlled.
func ExampleFromDocs() {
	mux := stdocs.New(stdocs.WithTitle("My API"))
	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {})
	mux.Mount()

	guard := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && mux.FromDocs(r) {
				http.Error(w, "try-it requests cannot modify data", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	_ = guard // pass guard(mux) to http.ListenAndServe

	r, _ := http.NewRequest(http.MethodPost, "/users", nil)
	r.Header.Set("Referer", "https://api.example.com/docs/")
	fmt.Println("from docs:", mux.FromDocs(r))
	// Output: from docs: true
}

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

// A json.RawMessage field that carries a JSON Schema document is
// documented with openapi:"schema=json-schema": a free-form object
// stdocs does not itself validate. On such a field the example: tag
// takes a JSON literal, so an author can show a representative schema.
func Example_jsonSchemaDocumentField() {
	type PlatformResponse struct {
		Name         string          `json:"name"`
		ConfigSchema json.RawMessage `json:"config_schema" openapi:"schema=json-schema" example:"{\"type\":\"object\",\"properties\":{\"host\":{\"type\":\"string\"}},\"required\":[\"host\"]}"`
	}

	mux := stdocs.New(stdocs.WithTitle("Platform API"))
	mux.HandleFunc("GET /platforms/{id}", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.Summary("Get a platform"),
		stdocs.WithResponse(200, PlatformResponse{}),
	)

	b, _ := mux.JSON()
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]struct {
					Type        string `json:"type"`
					Description string `json:"description"`
				} `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	_ = json.Unmarshal(b, &doc)
	cs := doc.Components.Schemas["PlatformResponse"].Properties["config_schema"]
	fmt.Printf("config_schema: %s — %s\n", cs.Type, cs.Description)
	// Output: config_schema: object — A JSON Schema document.
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

// Constraint tags and typed parameters: the struct documents its own
// validation rules, and WithParams reflects query parameters from a
// struct with the same vocabulary.
func ExampleWithParams() {
	type ListParams struct {
		Cursor string `query:"cursor" doc:"Opaque pagination cursor"`
		Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
	}
	mux := stdocs.New(stdocs.WithTitle("My API"))
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.WithParams(ListParams{}),
	)
	raw, _ := mux.JSON()
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `json:"name"`
				Schema struct {
					Type    string          `json:"type"`
					Default json.RawMessage `json:"default"`
					Maximum json.RawMessage `json:"maximum"`
				} `json:"schema"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	json.Unmarshal(raw, &doc)
	for _, p := range doc.Paths["/tasks"]["get"].Parameters {
		fmt.Printf("%s (%s) default=%s max=%s\n",
			p.Name, p.Schema.Type, p.Schema.Default, p.Schema.Maximum)
	}
	// Output:
	// cursor (string) default= max=
	// limit (integer) default=20 max=100
}

// A repeated query filter — ?severity=high&severity=low — documents as
// an array parameter whose elements carry the enum. Item options nest
// inside ParamItems, which owns the element schema; the same contract
// comes from a WithParams struct field tagged
// `query:"severity" enum:"info,low,high"`.
func ExampleParamItems() {
	mux := stdocs.New(stdocs.WithTitle("My API"))
	mux.HandleFunc("GET /resources", func(w http.ResponseWriter, r *http.Request) {},
		stdocs.QueryParam("severity", "array", "Repeated severity filter",
			stdocs.ParamItems("string", stdocs.ItemEnum("info", "low", "high")),
			stdocs.ParamUniqueItems(),
		),
	)
	raw, _ := mux.JSON()
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name   string `json:"name"`
				Schema struct {
					Type        string `json:"type"`
					UniqueItems bool   `json:"uniqueItems"`
					Items       struct {
						Type string   `json:"type"`
						Enum []string `json:"enum"`
					} `json:"items"`
				} `json:"schema"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	json.Unmarshal(raw, &doc)
	p := doc.Paths["/resources"]["get"].Parameters[0]
	fmt.Printf("%s (%s of %s) enum=%v uniqueItems=%v\n",
		p.Name, p.Schema.Type, p.Schema.Items.Type, p.Schema.Items.Enum, p.Schema.UniqueItems)
	// Output:
	// severity (array of string) enum=[info low high] uniqueItems=true
}

// The shared error envelope is declared once at the mux level; every
// operation documents it unless it declares the status itself.
func ExampleWithDefaultResponse() {
	type APIError struct {
		Message string `json:"message"`
	}
	mux := stdocs.New(
		stdocs.WithTitle("My API"),
		stdocs.WithDefaultResponse(500, APIError{}),
	)
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {})
	raw, _ := mux.JSON()
	var doc struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Description string `json:"description"`
			} `json:"responses"`
		} `json:"paths"`
	}
	json.Unmarshal(raw, &doc)
	resps := doc.Paths["/tasks"]["get"].Responses
	for _, code := range []string{"200", "500"} {
		fmt.Printf("%s: %s\n", code, resps[code].Description)
	}
	// Output:
	// 200: OK
	// 500: Internal Server Error
}

// Route-opt bundles: declare a preset once, reuse it across routes.
func ExampleOpts() {
	paginated := stdocs.Opts(
		stdocs.QueryParam("cursor", "string", "Opaque cursor"),
		stdocs.QueryParam("limit", "integer", "Page size", stdocs.ParamDefault(20)),
	)
	mux := stdocs.New(stdocs.WithTitle("My API"))
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {}, paginated)
	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {}, paginated)
	raw, _ := mux.JSON()
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name string `json:"name"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	json.Unmarshal(raw, &doc)
	for _, path := range []string{"/tasks", "/users"} {
		names := make([]string, 0, 2)
		for _, p := range doc.Paths[path]["get"].Parameters {
			names = append(names, p.Name)
		}
		fmt.Println(path, names)
	}
	// Output:
	// /tasks [cursor limit]
	// /users [cursor limit]
}

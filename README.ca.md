> ⚠️ **Aquesta és una traducció comunitària del [README en anglès](README.md).** La versió anglesa és la de referència. Aquesta traducció pot estar desfasada; en cas de dubte, consulta la versió anglesa.
>
> Per proposar correccions, vegeu [`CONTRIBUTING.md`](CONTRIBUTING.md) → "Translations".

# stdocs

**Idiomes:** [English](README.md) (canònic) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Generació d'OpenAPI 3.0.4, 3.1.2 i 3.2.0 per al `net/http.ServeMux` de la biblioteca estàndard de Go 1.22+. Sense dependències en temps d'execució.

```go
mux := stdocs.New(stdocs.WithTitle("La meva API"))
mux.HandleFunc("GET /users/{id}", getUser)
http.Handle("/api/", mux)
http.Handle("/docs/", mux.Docs())
log.Fatal(http.ListenAndServe(":8080", nil))
```

Això és tot. `stdocs` recorre les rutes registrades, genera un document OpenAPI a partir dels teus tipus Go i serveix una interfície de documentació a `/docs/`.

## Taula de continguts

- [Característiques](#característiques)
- [Instal·lació](#instal·lació)
- [Ús](#ús)
- [Interfícies de documentació](#interfícies-de-documentació)
- [Com funciona](#com-funciona)
- [Contribuir](#contribuir)
- [Llicència](#llicència)

## Característiques

- **Sense dependències** — només la biblioteca estàndard de Go en temps d'execució.
- **Tres versions d'OpenAPI** — 3.0.4 (per defecte), 3.1.2 i 3.2.0, totes tres provades. No s'exposen constants de pedaçs antics (3.0.3, 3.1.0): segons l'especificació d'OpenAPI, el tooling ha d'acceptar qualsevol 3.0.* / 3.1.*, per la qual cosa un sol "últim pedaç" per menor és el valor per defecte correcte.
- **Reflexió** — els tipus Go es converteixen en JSON Schemas: punters, slices, mapes, genèrics, structs incrustats, tipus recursius via `$ref`, etiquetes `json`.
- **Valors per defecte intel·ligents** — els noms de les funcions es converteixen en resums, el primer segment de la ruta es converteix en el tag, els paràmetres de path s'inclouen automàticament.
- **Seguretat** — bearer, basic, API key, OAuth 2.0. Els noms d'esquemes no registrats es reporten com a errors.
- **Vuit interfícies** — HTML sense JS per defecte; Scalar, Swagger UI, Redoc, Stoplight com a sub-paquets opcionals amb CDN, cadascuna amb una variant "air-gapped" (incrustada) en un sub-paquet separat.
- **Segur davant XSS** — l'HTML de la documentació es renderitza amb `html/template`.

## Instal·lació

```bash
go get github.com/FumingPower3925/stdocs
```

Requereix Go 1.22 o superior. Provat amb Go 1.26.

## Ús

Les rutes es documenten automàticament a partir del patró i del nom de la funció registrada:

```go
mux := stdocs.New(stdocs.WithTitle("La meva API"))
mux.HandleFunc("GET /users", listUsers)        // resum "List users", tag "Users"
mux.HandleFunc("GET /health", healthCheck)     // resum "Health", tag "Health"
```

Passa opcions de ruta per afegir cossos, respostes, tags i seguretat:

```go
type User struct {
    ID    string `json:"id" doc:"Identificador únic"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
}

mux.HandleFunc("GET /users/{id}", getUser,
    stdocs.Summary("Obtenir un usuari per ID"),
    stdocs.WithResponse(200, User{}),
    stdocs.WithResponse(404, APIError{}),
)

mux.HandleFunc("POST /users", createUser,
    stdocs.WithBody(CreateUserRequest{}),
    stdocs.WithResponse(201, User{}),
)
```

Per a funcionalitats que `stdocs` no exposa directament, fes servir la sortida d'emergència:

```go
mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithOpenAPI(func(doc map[string]any) {
        doc["info"].(map[string]any)["x-logo"] = map[string]any{
            "url": "https://example.com/logo.png",
        }
    }),
)
```

Per fixar el spec a una versió específica d'OpenAPI, fes servir `WithVersion`:

```go
mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithVersion(stdocs.OpenAPI32),  // 3.2.0
)
```

`stdocs` inclou l'últim pedaç de cada menor (`OpenAPI30` = 3.0.4, `OpenAPI31` = 3.1.2, `OpenAPI32` = 3.2.0). Per a 3.2 també pots fixar l'URI canònic del document:

```go
mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithVersion(stdocs.OpenAPI32),
    stdocs.WithSelfURL("https://example.com/openapi.json"),
)
```

La llista completa d'opcions és a [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs).

## Interfícies de documentació

La interfície per defecte és una pàgina HTML mínima sense JavaScript (~3 KB). Per fer servir una interfície més rica, importa un sub-paquet i passa la seva opció `WithUI()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("La meva API"), scalar.WithUI())
```

Per a una compilació sense connexió a internet, importa el sub-paquet `*emb` corresponent i munta el seu `AssetHandler()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalaremb"

mux := stdocs.New(stdocs.WithTitle("La meva API"), scalaremb.WithUI())
mux.Handle("GET /docs/_assets/",
    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

Cada interfície ve en dues variants:

| Interfície       | Sub-paquet CDN      | Sub-paquet incrustat   | Mida incrustada |
| ---------------- | ------------------- | --------------------- | --------------- |
| _(per defecte)_  | —                   | —                     | 3 KB           |
| Scalar           | `ui/scalar`         | `ui/scalaremb`        | ~3.6 MB        |
| Swagger UI       | `ui/swaggerui`      | `ui/swaggeruiemb`     | ~1.7 MB        |
| Redoc            | `ui/redoc`          | `ui/redocemb`         | ~1.1 MB        |
| Stoplight        | `ui/stoplight`      | `ui/stoplightemb`     | ~2.4 MB        |

Les URLs del CDN estan fixades a una versió específica amb hashes d'integritat sha384 (excepte Scalar i Stoplight, els bundles de jsDelivr dels quals es generen al vol i no admeten SRI; feu servir les variants incrustades per tenir SRI). Els sub-paquets s'eliminen en el tree-shaking si no s'importen.

## Com funciona

El `net/http.ServeMux` de Go 1.22 admet patrons de mètode i ruta, però no els exposa públicament. `stdocs.New()` retorna un `*stdocs.Mux` que embolcalla `*http.ServeMux` i intercepta les crides a `Handle`/`HandleFunc` per registrar el patró i les metadades. A la primera petició a `/docs/openapi.json`, es recorre el registre i es construeix i desa el document a la memòria cau.

Sense comentaris, sense generació de codi, sense `unsafe` — el patró mateix és la documentació.

Hi ha un demo executable a [`cmd/demo`](cmd/demo):

```bash
go run ./cmd/demo
# obre http://localhost:8080/docs/
```

## Contribuir

Vegeu [`CONTRIBUTING.md`](CONTRIBUTING.md). Les traduccions les manté la comunitat; consulteu la secció "Translations" per afegir-ne o actualitzar-ne una.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## Llicència

Apache-2.0. Vegeu [LICENSE](LICENSE).

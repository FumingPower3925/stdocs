# stdocs

**Languages:** [English](README.md) (canonical) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

stdocs converteix un `net/http.ServeMux` de la biblioteca estàndard en una API autodocumentada: registra les rutes com sempre i stdocs en serveix la documentació interactiva — Scalar, Swagger UI, Redoc o Stoplight Elements a `/docs` — basada en un document OpenAPI 3.0/3.1/3.2 generat. Zero dependències i sense generació de codi: els patrons que ja escrius són la font de veritat.

```go
mux := stdocs.New(stdocs.WithTitle("La meva API"))
mux.HandleFunc("GET /users/{id}", getUser)
mux.Mount() // UI de docs a /docs/, spec a /docs/openapi.json
log.Fatal(http.ListenAndServe(":8080", mux))
```

Això és tot. `stdocs` recorre les teves rutes registrades, genera un spec OpenAPI a partir dels teus tipus Go i serveix una UI de documentació a `/docs/`. Véns de swaggo/swag, FastAPI o un framework de handlers tipats? [MIGRATING.md](MIGRATING.md) tradueix els teus costums a stdocs.

![Les quatre UI completes — Scalar, Swagger UI, Redoc i Stoplight Elements — mostrant el mateix spec generat](.github/uis.png)

El mateix document generat, mostrat per cadascuna de les quatre UI incloses — totes disponibles des de CDN amb versió fixada o totalment incrustades per a builds air-gapped.

## Taula de continguts

- [Característiques](#característiques)
- [Instal·lació](#installació)
- [Ús](#ús)
- [UIs](#uis)
- [Documentació](#documentació)
- [Com funciona](#com-funciona)
- [Abast i non-goals](#abast-i-non-goals)
- [Contribuir](#contribuir)
- [Llicència](#llicència)

## Característiques

- **Cinc UI** — una per defecte, diminuta i sense dependències (~1.6 KB), més Scalar, Swagger UI, Redoc i Stoplight Elements — cadascuna amb versió fixada des de CDN amb hashes d'integritat SRI o totalment incrustada per a builds air-gapped.
- **Tres versions d'OpenAPI** — 3.0.4 (per defecte), 3.1.2 i 3.2.0, totes validades externament.
- **Reflexió** — els tipus Go es converteixen en JSON Schemas seguint el contracte d'`encoding/json`, amb documentació i regles de validació (`minimum`, `maxLength`, `pattern`, `enum`, `default`, …) llegides dels tags de l'struct.
- **Paràmetres tipats** — declara paràmetres query/header/cookie des d'un struct o inline amb modificadors tipats i validats.
- **Valors per defecte intel·ligents** — els noms de funcions es converteixen en resums, els segments de ruta en tags, els paràmetres de ruta i un 200 es documenten sols, les rutes amb seguretat documenten el seu 401 i l'envelope d'error compartit es declara una sola vegada per a tot el mux.
- **Control per entorn** — activa o desactiva els docs segons l'entorn, amaga rutes individuals i detecta el trànsit de les consoles try-it, tot sense tocar les rutes registrades.
- **Honest per defecte** — una documentació mal declarada provoca un panic en lloc de publicar un contracte erroni, i un middleware de desenvolupament opcional avisa quan els handlers es desvien del document.
- **Zero dependències** — només la biblioteca estàndard de Go en temps d'execució.

## Instal·lació

```bash
go get github.com/FumingPower3925/stdocs
```

Requereix Go 1.25 o posterior. La referència completa també és disponible offline quan un fitxer font importa el mòdul (executa `go mod tidy` després d'afegir l'import): `go doc github.com/FumingPower3925/stdocs`. stdocs segueix la mateixa política de suport que el projecte Go — les dues releases més recents, actualment 1.25 i 1.26 — i la CI executa la suite completa de tests a cada patch release de totes dues. Els patrons de ruta que stdocs documenta (`"GET /users/{id}"`) són la sintaxi mètode+ruta que `net/http.ServeMux` va incorporar a Go 1.22.

## Ús

Les rutes es documenten soles a partir del patró i el nom del handler; els tags de l'struct i les route opts afegeixen la resta:

```go
type CreateTask struct {
    Title    string `json:"title" doc:"Títol curt" minLength:"1" maxLength:"200"`
    Priority int    `json:"priority" minimum:"1" maximum:"5" default:"3"`
}

type Task struct {
    ID string `json:"id" doc:"ID únic"`
}

type ListParams struct {
    Cursor string `query:"cursor" doc:"Cursor opac de paginació"`
    Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
}

type APIError struct {
    Message string `json:"message"`
}

mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithBearerAuth("bearerAuth", "JWT"),
    stdocs.WithDefaultResponse(500, APIError{}), // l'envelope d'error, una sola vegada
)

mux.HandleFunc("GET /tasks", listTasks, stdocs.WithParams(ListParams{}))

mux.HandleFunc("POST /tasks", createTask,
    stdocs.WithBody(CreateTask{}),
    stdocs.WithResponse(201, Task{}),
    stdocs.WithSecurity("bearerAuth"), // documenta també el 401
)

mux.Mount(os.Getenv("ENV") != "prod")
```

Una documentació mal declarada — un tipus de paràmetre amb un typo, un `minLength` en un `int`, un `example` que no es pot parsejar — provoca un panic o es nega a construir el document, en lloc de publicar un contracte erroni.

## UIs

Importa un subpaquet i passa-li la seva opció `WithUI()`; els bessons `*emb` incrusten el bundle per a builds air-gapped:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("La meva API"), scalar.WithUI())
```

| UI                               | Subpaquet CDN                          | Subpaquet incrustat                         |
| -------------------------------- | --------------------------------------- | -------------------------------------------- |
| _(per defecte)_ (integrada, ~1.6 KB) | —                                      | —                                            |
| Scalar                           | `ui/scalar` (~3.6 MB des del CDN)        | `ui/scalaremb` (~3.6 MB al teu binari)       |
| Swagger UI                       | `ui/swaggerui` (~1.7 MB des del CDN)     | `ui/swaggeruiemb` (~1.7 MB al teu binari)    |
| Redoc                            | `ui/redoc` (~1.1 MB des del CDN)         | `ui/redocemb` (~1.1 MB al teu binari)        |
| Stoplight                        | `ui/stoplight` (~2.4 MB des del CDN)     | `ui/stoplightemb` (~2.4 MB al teu binari)    |

Les URL del CDN estan fixades a versions exactes amb hashes SRI sha384; els subpaquets no s'enllacen al teu binari si no els importes. Detalls de configuració de la variant incrustada: [Docs UIs](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Docs_UIs).

## Documentació

La referència completa viu a [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs), organitzada per temes:

- [Field tags](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Field_tags) — `doc:`, `example:` i el vocabulari de constraints.
- [Parameters](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Parameters) — structs de `WithParams` i modificadors `ParamOpt`.
- [Responses](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Responses) — declaracions per status, la resposta `default` i sobres d'error a nivell de mux.
- [Visibility](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Visibility) — `Hidden`, `Internal` i `WithInternal(show)`.
- [Mounting and toggling](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Mounting_and_toggling) — `Mount`/`Docs`, activació per entorn i prefixos de ruta darrere d'un proxy.
- [Try-it requests and drift](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Try_it_requests_and_drift) — detecció amb `FromDocs` i l'ajuda de desenvolupament `DriftWarn`.
- [Using the spec downstream](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Using_the_spec_downstream) — tests de golden file, diffs a les PR i generació de clients.
- [OpenAPI versions](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-OpenAPI_versions) — `WithVersion`, el `$self` de 3.2 i l'escape hatch `WithOpenAPI`.
- [DocsHandler](https://pkg.go.dev/github.com/FumingPower3925/stdocs#DocsHandler) — serveix un spec escrit a mà darrere de qualsevol de les UI incloses.

[MIGRATING.md](MIGRATING.md) complementa la referència amb guies de migració des de swaggo/swag, FastAPI i frameworks de handlers tipats — taules d'equivalències literals incloses.

## Com funciona

`stdocs.New()` retorna un `*stdocs.Mux` que incrusta `*http.ServeMux` i registra patró + metadades a mesura que registres handlers. Amb la primera petició a `/docs/openapi.json`, es recorre el registre i l'spec es construeix i es desa en cache (`mux.Refresh()` el reconstrueix). Sense comentaris, sense generació de codi, sense `unsafe`: el patró és la documentació.

Una demo executable viu a [`cmd/demo`](./cmd/demo):

```bash
go run ./cmd/demo
# obre http://localhost:8080/docs/
```

## Abast i non-goals

stdocs documenta aplicacions de `ServeMux` de la biblioteca estàndard — no s'integra amb altres routers, no valida requests en temps d'execució i no fa servir generació de codi, anotacions en comentaris ni dependències, permanentment i per disseny. El document descriu la intenció; mantenir els handlers honestos és feina de l'aplicació. La declaració completa de límits, inclòs quan encaixa millor una altra eina, és a la [documentació del paquet](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Scope_and_non_goals).

## Contribuir

Consulta [CONTRIBUTING.md](CONTRIBUTING.md). Les traduccions les manté la comunitat; consulta-hi la secció "Translations" per afegir-ne o actualitzar-ne una.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## Llicència

Apache-2.0. Consulta [LICENSE](LICENSE).

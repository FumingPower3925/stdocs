> ⚠️ **This is a community translation of the [English README](README.md).** The English version is canonical. This translation may be out of date; when in doubt, consult the English version.
>
> To propose corrections, see [`CONTRIBUTING.md`](CONTRIBUTING.md) → "Translations".

# stdocs

**Idiomes:** [English](README.md) (canonical) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

stdocs converteix un `net/http.ServeMux` de la biblioteca estàndard en una API autodocumentada: registra les rutes com sempre i stdocs en serveix la documentació interactiva — Scalar, Swagger UI, Redoc o Stoplight Elements a `/docs` — basada en un document OpenAPI 3.0/3.1/3.2 generat. Zero dependències i sense generació de codi: els patrons que ja escrius són la font de la veritat.

```go
mux := stdocs.New(stdocs.WithTitle("La meva API"))
mux.HandleFunc("GET /users/{id}", getUser)
mux.Mount() // UI de docs a /docs/, spec a /docs/openapi.json
log.Fatal(http.ListenAndServe(":8080", mux))
```

Això és tot. `stdocs` recorre les teves rutes registrades, genera un spec OpenAPI a partir dels teus tipus Go i serveix una UI de documentació a `/docs/`.

![Les quatre UI completes — Scalar, Swagger UI, Redoc i Stoplight Elements — mostrant el mateix spec generat](.github/uis.png)

El mateix document generat, mostrat per cadascuna de les quatre UI incloses — totes disponibles des de CDN amb versió fixada o totalment incrustades per a builds air-gapped.

## Taula de continguts

- [Característiques](#característiques)
- [Instal·lació](#installació)
- [Ús](#ús)
- [UIs](#uis)
- [Com funciona](#com-funciona)
- [Contribuir](#contribuir)
- [Llicència](#llicència)

## Característiques

- **Cinc UI** — una per defecte, diminuta i sense dependències (~1.6 KB, només JS inline), més Scalar, Swagger UI, Redoc i Stoplight Elements — cadascuna disponible com a subpaquet CDN (amb versió fixada i hashes d'integritat SRI) o com a subpaquet incrustat air-gapped.
- **Tres versions d'OpenAPI** — 3.0.4 (per defecte), 3.1.2 i 3.2.0, totes provades.
- **Reflexió** — els tipus Go es converteixen en JSON Schemas: punters, slices, mapes, genèrics, structs incrustats, tipus recursius via `$ref`, tags `json` (inclosos `omitempty`, `omitzero` i `,string`), i reconeixement de `json.Marshaler`/`encoding.TextMarshaler`.
- **Valors per defecte intel·ligents** — els noms de funcions es converteixen en resums, el primer segment de la ruta es converteix en el tag i els paràmetres de ruta s'inclouen automàticament.
- **Seguretat** — bearer, basic, API key, OAuth 2.0 (inclòs el device flow de 3.2). Els noms d'esquemes no registrats es reporten com a errors.
- **Activació per entorn** — `mux.Docs(enabled)` i `WithDisabled(true)` activen o desactiven la documentació segons l'entorn sense canviar les rutes registrades.
- **Segur davant XSS** — l'HTML de la documentació es renderitza amb `html/template`.
- **Zero dependències** — només la biblioteca estàndard de Go en temps d'execució.

## Instal·lació

```bash
go get github.com/FumingPower3925/stdocs
```

Requereix Go 1.25 o posterior. stdocs segueix la mateixa política de suport que el projecte Go — les dues releases més recents, actualment 1.25 i 1.26 — i la CI executa la suite completa de tests a cada patch release de totes dues. Els patrons de ruta que stdocs documenta (`"GET /users/{id}"`) són la sintaxi mètode+ruta que `net/http.ServeMux` va incorporar a Go 1.22.

## Ús

Les rutes es documenten automàticament a partir del patró i del nom de la funció registrada:

```go
mux := stdocs.New(stdocs.WithTitle("La meva API"))
mux.HandleFunc("GET /users", listUsers)        // resum "List users", tag "Users"
mux.HandleFunc("GET /health", healthCheck)     // resum "Health check", tag "Health"
```

Passa opcions de ruta per adjuntar cossos, respostes, tags i seguretat:

```go
type User struct {
    ID    string `json:"id" doc:"Identificador únic"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
}

type CreateUserRequest struct {
    Name string `json:"name"`
}

type APIError struct {
    Message string `json:"message"`
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

Per a funcionalitats que stdocs no exposa directament, fes servir l'escape hatch:

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

Per fixar l'spec a una versió concreta d'OpenAPI, fes servir `WithVersion`:

```go
mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithVersion(stdocs.OpenAPI32),  // 3.2.0
)
```

`stdocs` inclou l'últim pedaç de cada versió menor (`OpenAPI30` = 3.0.4, `OpenAPI31` = 3.1.2, `OpenAPI32` = 3.2.0). Per a 3.2 pots fixar a més l'URI canònic del document:

```go
mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithVersion(stdocs.OpenAPI32),
    stdocs.WithSelfURL("https://example.com/openapi.json"),
)
```

La llista completa d'opcions és a [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs).

### Muntar i desactivar la documentació

`mux.Mount()` és una drecera per registrar el handler que retorna `mux.Docs()` al mateix mux, sota el prefix de documentació configurat: hi ha un únic handler de documentació i dues maneres de col·locar-lo. Fes servir `Mount()` tret que necessitis el handler directament (per embolcallar-lo en un middleware d'autenticació o muntar-lo en un altre mux). Tots dos accepten el mateix bool opcional amb la mateixa regla: un valor explícit per crida té prioritat sobre `WithDisabled` en tots dos sentits.

La UI de documentació i els endpoints de l'spec (`openapi.json`, `openapi.yaml`) es poden desactivar sense anul·lar el registre de rutes. La decisió es pren quan es crida `Mount()`/`Docs()` (embolcalla el handler tu mateix si necessites decidir-ho per petició):

```go
// 1) Per mux: WithDisabled(true) fa que Mount no faci res i que
//    Docs retorni un handler 404 a tot arreu.
mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithDisabled(os.Getenv("ENV") == "prod"),
)
mux.HandleFunc("GET /users", listUsers)
mux.Mount() // no registra res quan està desactivat
```

```go
// 2) Per crida: passa la condició a Mount (o a Docs si muntes
//    manualment).
mux := stdocs.New(stdocs.WithTitle("La meva API"))
mux.HandleFunc("GET /users", listUsers)
mux.Mount(os.Getenv("ENV") != "prod")
```

Quan està desactivada, tota petició sota el prefix de documentació rep un 404. L'spec encara es pot construir amb `mux.JSON()` i `mux.YAML()` — desactivar la UI no atura la generació de l'spec. Les rutes registrades sota el prefix de documentació (la mateixa pàgina de docs, els handlers de recursos) no apareixen mai a l'spec generat.

### Amagar rutes individuals

La visibilitat per ruta es combina amb els mecanismes anteriors: `Hidden()` exclou una ruta del document a tot arreu, i `Internal()` l'exclou tret que el mux s'hagi construït amb `WithInternal(true)` (quan es mostra, l'operació porta `x-internal: true`, l'extensió que entenen les eines de filtratge de specs). Una configuració completa per entorn:

```go
env := os.Getenv("ENV")
mux := stdocs.New(
    stdocs.WithTitle("La meva API"),
    stdocs.WithDisabled(env == "prod"), // prod: sense documentació
    stdocs.WithInternal(env == "dev"),  // dev: documentació completa; a la resta, les rutes internes queden amagades
)
mux.HandleFunc("GET /users", listUsers)                           // sempre documentada
mux.HandleFunc("POST /admin/keys", rotateKeys, stdocs.Internal()) // documentada només a dev
mux.HandleFunc("GET /healthz", healthCheck, stdocs.Hidden())      // mai documentada
mux.Mount()
```

Les rutes excloses no deixen rastre al document: ni rutes, ni esquemes, ni operationIds. La visibilitat només dona forma a la documentació publicada: les rutes amagades i internes **continuen servint trànsit a tots els entorns**. No és control d'accés; protegeix els endpoints sensibles amb autenticació real.

Si tens un document OpenAPI escrit a mà en lloc de rutes generades, serveix-lo amb `DocsHandler` + `WithSpec`:

```go
spec, _ := os.ReadFile("openapi.json")
http.Handle("GET /docs/", stdocs.DocsHandler(
    stdocs.WithTitle("La meva API"),
    stdocs.WithSpec(spec),
))
```

## UIs

La UI per defecte és una petita pàgina HTML sense dependències (~1.6 KB, només JS inline, sense recursos externs). Per fer servir una UI més completa, importa un subpaquet i passa la seva opció `WithUI()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("La meva API"), scalar.WithUI())
```

Per a un build air-gapped (sense CDN), importa el subpaquet `*emb` corresponent i munta el seu `AssetHandler()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalaremb"

mux := stdocs.New(stdocs.WithTitle("La meva API"), scalaremb.WithUI())
mux.Mount()
mux.Handle("GET /docs/_assets/",
    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

Cada UI completa ve en dues variants:

| UI                                  | Subpaquet CDN                          | Subpaquet incrustat                          |
| ----------------------------------- | --------------------------------------- | -------------------------------------------- |
| _(per defecte)_ (integrada, ~1.6 KB) | —                                       | —                                            |
| Scalar                              | `ui/scalar` (~3.6 MB des del CDN)       | `ui/scalaremb` (~3.6 MB al teu binari)       |
| Swagger UI                          | `ui/swaggerui` (~1.7 MB des del CDN)    | `ui/swaggeruiemb` (~1.7 MB al teu binari)    |
| Redoc                               | `ui/redoc` (~1.1 MB des del CDN)        | `ui/redocemb` (~1.1 MB al teu binari)        |
| Stoplight                           | `ui/stoplight` (~2.4 MB des del CDN)    | `ui/stoplightemb` (~2.4 MB al teu binari)    |

Totes les URL del CDN estan fixades a versions exactes amb hashes d'integritat SRI sha384. Els subpaquets no s'enllacen al teu binari si no els importes.

## Com funciona

El `net/http.ServeMux` de Go 1.22 admet patrons de mètode+ruta, però no els exposa públicament. `stdocs.New()` retorna un `*stdocs.Mux` que incrusta `*http.ServeMux` i intercepta les crides a `Handle`/`HandleFunc` per registrar el patró i les metadades. A la primera petició a `/docs/openapi.json`, es recorre el registre i l'spec es construeix i es desa a la memòria cau (crida `mux.Refresh()` per reconstruir-lo).

Sense comentaris, sense generació de codi, sense `unsafe` — la cadena del patró és la documentació.

Hi ha una demo executable a [`cmd/demo`](./cmd/demo):

```bash
go run ./cmd/demo
# obre http://localhost:8080/docs/
```

## Contribuir

Consulta [CONTRIBUTING.md](CONTRIBUTING.md). Les traduccions les manté la comunitat; consulta-hi la secció "Translations" per afegir-ne o actualitzar-ne una.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## Llicència

Apache-2.0. Consulta [LICENSE](LICENSE).

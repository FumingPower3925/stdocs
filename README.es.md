# stdocs

**Languages:** [English](README.md) (canonical) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

stdocs convierte un `net/http.ServeMux` de la biblioteca estándar en una API autodocumentada: registra tus rutas como siempre y stdocs sirve su documentación interactiva — Scalar, Swagger UI, Redoc o Stoplight Elements en `/docs` — respaldada por un documento OpenAPI 3.0/3.1/3.2 generado. Cero dependencias y sin generación de código: los patrones que ya escribes son la fuente de verdad.

```go
mux := stdocs.New(stdocs.WithTitle("Mi API"))
mux.HandleFunc("GET /users/{id}", getUser)
mux.Mount() // UI de docs en /docs/, spec en /docs/openapi.json
log.Fatal(http.ListenAndServe(":8080", mux))
```

Eso es todo. `stdocs` recorre tus rutas registradas, genera un spec OpenAPI a partir de tus tipos Go y sirve una UI de documentación en `/docs/`.

![Las cuatro UI completas — Scalar, Swagger UI, Redoc y Stoplight Elements — mostrando el mismo spec generado](.github/uis.png)

El mismo documento generado, mostrado por cada una de las cuatro UI incluidas — todas disponibles desde CDN con versión fijada o totalmente incrustadas para builds air-gapped.

## Tabla de contenidos

- [Características](#características)
- [Instalación](#instalación)
- [Uso](#uso)
- [UIs](#uis)
- [Documentación](#documentación)
- [Cómo funciona](#cómo-funciona)
- [Alcance y non-goals](#alcance-y-non-goals)
- [Contribuir](#contribuir)
- [Licencia](#licencia)

## Características

- **Cinco UI** — una por defecto, diminuta y sin dependencias (~1.6 KB), más Scalar, Swagger UI, Redoc y Stoplight Elements — cada una con versión fijada desde CDN con hashes de integridad SRI o totalmente incrustada para builds air-gapped.
- **Tres versiones de OpenAPI** — 3.0.4 (por defecto), 3.1.2 y 3.2.0, todas validadas externamente.
- **Reflexión** — los tipos Go se convierten en JSON Schemas siguiendo el contrato de `encoding/json`, con documentación y reglas de validación (`minimum`, `maxLength`, `pattern`, `enum`, `default`, …) leídas de los tags del struct.
- **Parámetros tipados** — declara parámetros query/header/cookie desde un struct o inline con modificadores tipados y validados.
- **Valores por defecto inteligentes** — los nombres de funciones se convierten en resúmenes, los segmentos de ruta en tags, los parámetros de ruta y un 200 se documentan solos, las rutas con seguridad documentan su 401 y el envelope de error compartido se declara una sola vez para todo el mux.
- **Control por entorno** — activa o desactiva los docs según el entorno, oculta rutas individuales y detecta el tráfico de las consolas try-it, todo sin tocar las rutas registradas.
- **Honesto por defecto** — una documentación mal declarada provoca un panic en lugar de publicar un contrato erróneo, y un middleware de desarrollo opcional avisa cuando los handlers se desvían del documento.
- **Cero dependencias** — solo la biblioteca estándar de Go en tiempo de ejecución.

## Instalación

```bash
go get github.com/FumingPower3925/stdocs
```

Requiere Go 1.25 o posterior. stdocs sigue la misma política de soporte que el proyecto Go — las dos releases más recientes, actualmente 1.25 y 1.26 — y la CI ejecuta la suite completa de tests en cada patch release de ambas. Los patrones de ruta que stdocs documenta (`"GET /users/{id}"`) son la sintaxis método+ruta que `net/http.ServeMux` incorporó en Go 1.22.

## Uso

Las rutas se documentan solas a partir del patrón y el nombre del handler; los tags del struct y las route opts añaden el resto:

```go
type CreateTask struct {
    Title    string `json:"title" doc:"Título corto" minLength:"1" maxLength:"200"`
    Priority int    `json:"priority" minimum:"1" maximum:"5" default:"3"`
}

type Task struct {
    ID string `json:"id" doc:"ID único"`
}

type ListParams struct {
    Cursor string `query:"cursor" doc:"Cursor opaco de paginación"`
    Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
}

type APIError struct {
    Message string `json:"message"`
}

mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithBearerAuth("bearerAuth", "JWT"),
    stdocs.WithDefaultResponse(500, APIError{}), // el envelope de error, una sola vez
)

mux.HandleFunc("GET /tasks", listTasks, stdocs.WithParams(ListParams{}))

mux.HandleFunc("POST /tasks", createTask,
    stdocs.WithBody(CreateTask{}),
    stdocs.WithResponse(201, Task{}),
    stdocs.WithSecurity("bearerAuth"), // documenta también el 401
)

mux.Mount(os.Getenv("ENV") != "prod")
```

Una documentación mal declarada — un tipo de parámetro con un typo, un `minLength` en un `int`, un `example` que no parsea — provoca un panic al registrar o al construir el documento, en lugar de publicar un contrato erróneo.

## UIs

Importa un subpaquete y pasa su opción `WithUI()`; los gemelos `*emb` incrustan el bundle para builds air-gapped:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("Mi API"), scalar.WithUI())
```

| UI                                 | Subpaquete CDN                          | Subpaquete incrustado                        |
| ---------------------------------- | --------------------------------------- | -------------------------------------------- |
| _(por defecto)_ (integrada, ~1.6 KB) | —                                        | —                                            |
| Scalar                             | `ui/scalar` (~3.6 MB desde el CDN)       | `ui/scalaremb` (~3.6 MB en tu binario)       |
| Swagger UI                         | `ui/swaggerui` (~1.7 MB desde el CDN)    | `ui/swaggeruiemb` (~1.7 MB en tu binario)    |
| Redoc                              | `ui/redoc` (~1.1 MB desde el CDN)        | `ui/redocemb` (~1.1 MB en tu binario)        |
| Stoplight                          | `ui/stoplight` (~2.4 MB desde el CDN)    | `ui/stoplightemb` (~2.4 MB en tu binario)    |

Las URL del CDN están fijadas a versiones exactas con hashes SRI sha384; los subpaquetes no se enlazan en tu binario salvo que los importes. Detalles de la configuración de la variante incrustada: [Docs UIs](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Docs_UIs).

## Documentación

La referencia completa vive en [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs), organizada por temas:

- [Field tags](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Field_tags) — `doc:`, `example:` y el vocabulario de constraints.
- [Parameters](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Parameters) — structs de `WithParams` y modificadores `ParamOpt`.
- [Responses](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Responses) — declaraciones por status, la respuesta `default` y sobres de error a nivel de mux.
- [Visibility](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Visibility) — `Hidden`, `Internal` y `WithInternal(show)`.
- [Mounting and toggling](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Mounting_and_toggling) — `Mount`/`Docs`, activación por entorno y prefijos de ruta tras un proxy.
- [Try-it requests and drift](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Try_it_requests_and_drift) — detección con `FromDocs` y la ayuda de desarrollo `DriftWarn`.
- [Using the spec downstream](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Using_the_spec_downstream) — tests de golden file, diffs en PRs y generación de clientes.
- [OpenAPI versions](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-OpenAPI_versions) — `WithVersion`, el `$self` de 3.2 y el escape hatch `WithOpenAPI`.
- [DocsHandler](https://pkg.go.dev/github.com/FumingPower3925/stdocs#DocsHandler) — sirve un spec escrito a mano detrás de cualquiera de las UI incluidas.

## Cómo funciona

`stdocs.New()` devuelve un `*stdocs.Mux` que incrusta `*http.ServeMux` y registra patrón + metadatos a medida que registras handlers. Con la primera petición a `/docs/openapi.json`, se recorre el registro y el spec se construye y se cachea (`mux.Refresh()` lo reconstruye). Sin comentarios, sin generación de código, sin `unsafe`: el patrón es la documentación.

Una demo ejecutable vive en [`cmd/demo`](./cmd/demo):

```bash
go run ./cmd/demo
# abre http://localhost:8080/docs/
```

## Alcance y non-goals

stdocs documenta aplicaciones de `ServeMux` de la biblioteca estándar — no se integra con otros routers, no valida requests en tiempo de ejecución y no usa generación de código, anotaciones en comentarios ni dependencias, permanentemente y por diseño. El documento describe la intención; mantener los handlers honestos es trabajo de la aplicación. La declaración completa de límites, incluido cuándo encaja mejor otra herramienta, está en la [documentación del paquete](https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Scope_and_non_goals).

## Contribuir

Consulta [CONTRIBUTING.md](CONTRIBUTING.md). Las traducciones las mantiene la comunidad; consulta allí la sección "Translations" para añadir o actualizar una.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## Licencia

Apache-2.0. Consulta [LICENSE](LICENSE).

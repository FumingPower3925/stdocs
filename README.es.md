> ⚠️ **This is a community translation of the [English README](README.md).** The English version is canonical. This translation may be out of date; when in doubt, consult the English version.
>
> To propose corrections, see [`CONTRIBUTING.md`](CONTRIBUTING.md) → "Translations".

# stdocs

**Idiomas:** [English](README.md) (canonical) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Generación de OpenAPI 3.0.4, 3.1.2 y 3.2.0 para el `net/http.ServeMux` de la biblioteca estándar (sintaxis de patrones de Go 1.22+). Sin dependencias en tiempo de ejecución.

```go
mux := stdocs.New(stdocs.WithTitle("Mi API"))
mux.HandleFunc("GET /users/{id}", getUser)
mux.Mount() // interfaz de docs en /docs/, spec en /docs/openapi.json
log.Fatal(http.ListenAndServe(":8080", mux))
```

Eso es todo. `stdocs` recorre tus rutas registradas, genera un spec OpenAPI a partir de tus tipos Go y sirve una interfaz de documentación en `/docs/`.

## Tabla de contenidos

- [Características](#características)
- [Instalación](#instalación)
- [Uso](#uso)
- [Interfaces de documentación](#interfaces-de-documentación)
- [Cómo funciona](#cómo-funciona)
- [Contribuir](#contribuir)
- [Licencia](#licencia)

## Características

- **Cero dependencias** — solo la biblioteca estándar de Go en tiempo de ejecución.
- **Tres versiones de OpenAPI** — 3.0.4 (por defecto), 3.1.2 y 3.2.0, todas probadas.
- **Reflexión** — los tipos Go se convierten en JSON Schemas: punteros, slices, mapas, genéricos, structs embebidos, tipos recursivos vía `$ref`, etiquetas `json` (incluidas `omitempty`, `omitzero` y `,string`), y reconocimiento de `json.Marshaler`/`encoding.TextMarshaler`.
- **Valores por defecto inteligentes** — los nombres de funciones se convierten en resúmenes, el primer segmento de la ruta se convierte en el tag y los parámetros de ruta se incluyen automáticamente.
- **Seguridad** — bearer, basic, API key, OAuth 2.0 (incluido el device flow de 3.2). Los nombres de esquemas no registrados se reportan como errores.
- **Cinco interfaces** — una por defecto, diminuta y sin dependencias (~1.6 KB, solo JS inline), más Scalar, Swagger UI, Redoc y Stoplight Elements — cada una disponible como subpaquete CDN (con versión fijada y hashes de integridad SRI) o como subpaquete embebido aislado de la red.
- **Conmutación por entorno** — `mux.Docs(enabled)` y `WithDisabled(true)` activan o desactivan la documentación según el entorno sin cambiar las rutas registradas.
- **Seguro frente a XSS** — el HTML de la documentación se renderiza con `html/template`.

## Instalación

```bash
go get github.com/FumingPower3925/stdocs
```

Requiere Go 1.26.4 o posterior (la directiva `go` del módulo). Los patrones de ruta que stdocs documenta (`"GET /users/{id}"`) son la sintaxis método+ruta que `net/http.ServeMux` incorporó en Go 1.22.

## Uso

Las rutas se documentan automáticamente a partir del patrón y del nombre de la función registrada:

```go
mux := stdocs.New(stdocs.WithTitle("Mi API"))
mux.HandleFunc("GET /users", listUsers)        // resumen "List users", tag "Users"
mux.HandleFunc("GET /health", healthCheck)     // resumen "Health check", tag "Health"
```

Pasa opciones de ruta para adjuntar cuerpos, respuestas, tags y seguridad:

```go
type User struct {
    ID    string `json:"id" doc:"Identificador único"`
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
    stdocs.Summary("Obtener un usuario por ID"),
    stdocs.WithResponse(200, User{}),
    stdocs.WithResponse(404, APIError{}),
)

mux.HandleFunc("POST /users", createUser,
    stdocs.WithBody(CreateUserRequest{}),
    stdocs.WithResponse(201, User{}),
)
```

Para funcionalidades que stdocs no expone directamente, usa el mecanismo de escape:

```go
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithOpenAPI(func(doc map[string]any) {
        doc["info"].(map[string]any)["x-logo"] = map[string]any{
            "url": "https://example.com/logo.png",
        }
    }),
)
```

Para fijar el spec a una versión concreta de OpenAPI, usa `WithVersion`:

```go
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithVersion(stdocs.OpenAPI32),  // 3.2.0
)
```

`stdocs` incluye el último parche de cada versión menor (`OpenAPI30` = 3.0.4, `OpenAPI31` = 3.1.2, `OpenAPI32` = 3.2.0). Para 3.2 puedes fijar además el URI canónico del documento:

```go
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithVersion(stdocs.OpenAPI32),
    stdocs.WithSelfURL("https://example.com/openapi.json"),
)
```

La lista completa de opciones está en [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs).

### Montar y desactivar la documentación

`mux.Mount()` es una abreviatura de registrar el handler que devuelve `mux.Docs()` en el propio mux, bajo el prefijo de documentación configurado: hay un único handler de documentación y dos formas de colocarlo. Usa `Mount()` salvo que necesites el handler directamente (para envolverlo en un middleware de autenticación o montarlo en otro mux). Ambos aceptan el mismo bool opcional con la misma regla: un valor explícito por llamada gana a `WithDisabled` en ambos sentidos.

La interfaz de documentación y los endpoints del spec (`openapi.json`, `openapi.yaml`) se pueden desactivar sin anular el registro de rutas. La decisión se toma cuando se llama a `Mount()`/`Docs()` (envuelve el handler tú mismo si necesitas un conmutador por petición):

```go
// 1) Por mux: WithDisabled(true) hace que Mount no haga nada y que
//    Docs devuelva un handler 404 en todas partes.
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithDisabled(os.Getenv("ENV") == "prod"),
)
mux.HandleFunc("GET /users", listUsers)
mux.Mount() // no registra nada cuando está desactivado
```

```go
// 2) Por llamada: pasa la condición a Mount (o a Docs si montas
//    manualmente).
mux := stdocs.New(stdocs.WithTitle("Mi API"))
mux.HandleFunc("GET /users", listUsers)
mux.Mount(os.Getenv("ENV") != "prod")
```

Cuando está desactivada, toda petición bajo el prefijo de documentación recibe un 404. El spec sigue pudiéndose construir con `mux.JSON()` y `mux.YAML()` — desactivar la interfaz no detiene la generación del spec. Las rutas registradas bajo el prefijo de documentación (la propia página de docs, los handlers de recursos) nunca aparecen en el spec generado.

Si tienes un documento OpenAPI escrito a mano en lugar de rutas generadas, sírvelo con `DocsHandler` + `WithSpec`:

```go
spec, _ := os.ReadFile("openapi.json")
http.Handle("GET /docs/", stdocs.DocsHandler(
    stdocs.WithTitle("Mi API"),
    stdocs.WithSpec(spec),
))
```

## Interfaces de documentación

La interfaz por defecto es una pequeña página HTML sin dependencias (~1.6 KB, solo JS inline, sin recursos externos). Para usar una interfaz más rica, importa un subpaquete y pasa su opción `WithUI()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("Mi API"), scalar.WithUI())
```

Para una compilación aislada de la red (sin CDN), importa el subpaquete `*emb` correspondiente y monta su `AssetHandler()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalaremb"

mux := stdocs.New(stdocs.WithTitle("Mi API"), scalaremb.WithUI())
mux.Mount()
mux.Handle("GET /docs/_assets/",
    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

Cada interfaz rica viene en dos variantes:

| Interfaz        | Subpaquete CDN | Subpaquete embebido | Tamaño embebido |
| --------------- | -------------- | ------------------- | --------------- |
| _(por defecto)_ | —              | — (inline, ~1.6 KB) | —               |
| Scalar          | `ui/scalar`    | `ui/scalaremb`      | ~3.6 MB         |
| Swagger UI      | `ui/swaggerui` | `ui/swaggeruiemb`   | ~1.7 MB         |
| Redoc           | `ui/redoc`     | `ui/redocemb`       | ~1.1 MB         |
| Stoplight       | `ui/stoplight` | `ui/stoplightemb`   | ~2.4 MB         |

Todas las URL del CDN están fijadas a versiones exactas con hashes de integridad SRI sha384. Los subpaquetes no se enlazan en tu binario a menos que los importes.

## Cómo funciona

El `net/http.ServeMux` de Go 1.22 admite patrones de método+ruta, pero no los expone públicamente. `stdocs.New()` devuelve un `*stdocs.Mux` que embebe `*http.ServeMux` e intercepta las llamadas a `Handle`/`HandleFunc` para registrar el patrón y los metadatos. En la primera petición a `/docs/openapi.json`, se recorre el registro y el spec se construye y se guarda en caché (llama a `mux.Refresh()` para reconstruirlo).

Sin comentarios, sin generación de código, sin `unsafe` — la cadena del patrón es la documentación.

Hay una demo ejecutable en [`cmd/demo`](./cmd/demo):

```bash
go run ./cmd/demo
# abre http://localhost:8080/docs/
```

## Contribuir

Consulta [CONTRIBUTING.md](CONTRIBUTING.md). Las traducciones las mantiene la comunidad; consulta allí la sección "Translations" para añadir o actualizar una.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## Licencia

Apache-2.0. Consulta [LICENSE](LICENSE).

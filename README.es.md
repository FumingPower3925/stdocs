> ⚠️ **Esta es una traducción comunitaria del [README en inglés](README.md).** La versión en inglés es la de referencia. Esta traducción puede estar desactualizada; en caso de duda, consulta la versión en inglés.
>
> Para proponer correcciones, ver [`CONTRIBUTING.md`](CONTRIBUTING.md) → "Translations".

# stdocs

**Idiomas:** [English](README.md) (canónico) · [Español](README.es.md) · [Català](README.ca.md)

[![CI](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml/badge.svg)](https://github.com/FumingPower3925/stdocs/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/FumingPower3925/stdocs)](https://goreportcard.com/report/github.com/FumingPower3925/stdocs)
[![Go Reference](https://pkg.go.dev/badge/github.com/FumingPower3925/stdocs.svg)](https://pkg.go.dev/github.com/FumingPower3925/stdocs)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Generación de OpenAPI 3.0.4, 3.1.2 y 3.2.0 para el `net/http.ServeMux` de la biblioteca estándar de Go 1.22+. Sin dependencias en tiempo de ejecución.

```go
mux := stdocs.New(stdocs.WithTitle("Mi API"))
mux.HandleFunc("GET /users/{id}", getUser)
http.Handle("/api/", mux)
http.Handle("/docs/", mux.Docs())
log.Fatal(http.ListenAndServe(":8080", nil))
```

Eso es todo. `stdocs` recorre las rutas registradas, genera un documento OpenAPI a partir de tus tipos Go y sirve una interfaz de documentación en `/docs/`.

## Tabla de contenidos

- [Características](#características)
- [Instalación](#instalación)
- [Uso](#uso)
- [Interfaces de documentación](#interfaces-de-documentación)
- [Cómo funciona](#cómo-funciona)
- [Contribuir](#contribuir)
- [Licencia](#licencia)

## Características

- **Sin dependencias** — solo la biblioteca estándar de Go en tiempo de ejecución.
- **Tres versiones de OpenAPI** — 3.0.4 (por defecto), 3.1.2 y 3.2.0, todas probadas. No se exponen constantes de parches antiguos (3.0.3, 3.1.0): según la especificación de OpenAPI, el tooling debe aceptar cualquier 3.0.* / 3.1.*, por lo que un único "último parche" por menor es el valor por defecto correcto.
- **Reflexión** — los tipos Go se convierten en JSON Schemas: punteros, slices, mapas, genéricos, structs embebidos, tipos recursivos vía `$ref`, etiquetas `json`.
- **Valores predeterminados inteligentes** — los nombres de las funciones se convierten en resúmenes, el primer segmento de la ruta se convierte en el tag, los parámetros de path se incluyen automáticamente.
- **Seguridad** — bearer, basic, API key, OAuth 2.0. Los nombres de esquemas no registrados se reportan como errores.
- **Ocho interfaces** — HTML sin JS por defecto; Scalar, Swagger UI, Redoc, Stoplight como sub-paquetes opcionales con CDN, cada una con una variante "air-gapped" (embebida) en un sub-paquete separado.
- **Conmutación en tiempo de ejecución** — `mux.Docs(false)` y `WithDisabled(true)` permiten activar o desactivar la interfaz de documentación según el entorno, la llamada o un feature flag, sin cambiar las rutas registradas.
- **Seguro frente a XSS** — el HTML de la documentación se renderiza con `html/template`.

## Instalación

```bash
go get github.com/FumingPower3925/stdocs
```

Requiere Go 1.22 o superior. Probado con Go 1.26.

## Uso

Las rutas se documentan automáticamente a partir del patrón y del nombre de la función registrada:

```go
mux := stdocs.New(stdocs.WithTitle("Mi API"))
mux.HandleFunc("GET /users", listUsers)        // resumen "List users", tag "Users"
mux.HandleFunc("GET /health", healthCheck)     // resumen "Health", tag "Health"
```

Pasa opciones de ruta para añadir cuerpos, respuestas, tags y seguridad:

```go
type User struct {
    ID    string `json:"id" doc:"Identificador único"`
    Name  string `json:"name"`
    Email string `json:"email,omitempty"`
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

Para funcionalidades que `stdocs` no expone directamente, usa la salida de emergencia:

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

Para fijar el spec a una versión específica de OpenAPI, usa `WithVersion`:

```go
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithVersion(stdocs.OpenAPI32),  // 3.2.0
)
```

`stdocs` incluye el último parche de cada menor (`OpenAPI30` = 3.0.4, `OpenAPI31` = 3.1.2, `OpenAPI32` = 3.2.0). Para 3.2 también puedes fijar el URI canónico del documento:

```go
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithVersion(stdocs.OpenAPI32),
    stdocs.WithSelfURL("https://example.com/openapi.json"),
)
```

La lista completa de opciones está en [pkg.go.dev](https://pkg.go.dev/github.com/FumingPower3925/stdocs).

### Desactivar la interfaz de documentación

La interfaz de documentación y los endpoints del spec (`openapi.json`, `openapi.yaml`) se pueden desactivar sin desregistrar rutas ni cambiar el punto de llamada. Se admiten dos patrones:

```go
// 1) Por llamada: pasa false a mux.Docs en el punto de uso.
mux := stdocs.New(stdocs.WithTitle("Mi API"))
mux.HandleFunc("GET /users", listUsers)

if os.Getenv("ENV") == "prod" {
    mux.Handle("GET /docs/", mux.Docs(false))  // responde 404
} else {
    mux.Handle("GET /docs/", mux.Docs())        // sirve la interfaz
}
```

```go
// 2) Por mux: WithDisabled() hace que Mount y Docs no registren nada.
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithDisabled(os.Getenv("ENV") == "prod"),
)
mux.HandleFunc("GET /users", listUsers)
mux.Mount()  // no registra nada cuando está desactivado
```

Ambos patrones devuelven `http.NotFoundHandler()` para cada petición al prefijo de documentación. El spec sigue siendo construible con `mux.JSON()` y `mux.YAML()` — desactivar la interfaz no detiene la generación del spec. `DocsHandler` (el placeholder de Tier 1) respeta `WithDisabled` de la misma forma.

## Interfaces de documentación

La interfaz por defecto es una página HTML mínima sin JavaScript (~3 KB). Para usar una interfaz más rica, importa un sub-paquete y pasa su opción `WithUI()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("Mi API"), scalar.WithUI())
```

Para una compilación sin conexión a internet, importa el sub-paquete `*emb` correspondiente y monta su `AssetHandler()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalaremb"

mux := stdocs.New(stdocs.WithTitle("Mi API"), scalaremb.WithUI())
mux.Handle("GET /docs/_assets/",
    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

Cada interfaz viene en dos variantes:

| Interfaz         | Sub-paquete CDN    | Sub-paquete embebido   | Tamaño embebido |
| ---------------- | ------------------ | --------------------- | --------------- |
| _(por defecto)_  | —                  | —                     | 3 KB           |
| Scalar           | `ui/scalar`        | `ui/scalaremb`        | ~3.6 MB        |
| Swagger UI       | `ui/swaggerui`     | `ui/swaggeruiemb`     | ~1.7 MB        |
| Redoc            | `ui/redoc`         | `ui/redocemb`         | ~1.1 MB        |
| Stoplight        | `ui/stoplight`     | `ui/stoplightemb`     | ~2.4 MB        |

Las URLs del CDN están fijadas a una versión específica con hashes de integridad sha384 (excepto Scalar y Stoplight, cuyos bundles de jsDelivr se generan al vuelo y no admiten SRI; usa las variantes embebidas para tener SRI). Los sub-paquetes se eliminan en el tree-shaking si no se importan.

## Cómo funciona

El `net/http.ServeMux` de Go 1.22 soporta patrones de método y ruta, pero no los expone públicamente. `stdocs.New()` devuelve un `*stdocs.Mux` que envuelve `*http.ServeMux` e intercepta las llamadas a `Handle`/`HandleFunc` para registrar el patrón y los metadatos. En la primera petición a `/docs/openapi.json`, se recorre el registro y se construye y cachea el documento.

Sin comentarios, sin generación de código, sin `unsafe` — el patrón mismo es la documentación.

Hay un demo ejecutable en [`cmd/demo`](cmd/demo):

```bash
go run ./cmd/demo
# abre http://localhost:8080/docs/
```

## Contribuir

Ver [`CONTRIBUTING.md`](CONTRIBUTING.md). Las traducciones las mantiene la comunidad; consulta la sección "Translations" para añadir o actualizar una.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## Licencia

Apache-2.0. Ver [LICENSE](LICENSE).

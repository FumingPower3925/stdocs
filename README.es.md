> ⚠️ **This is a community translation of the [English README](README.md).** The English version is canonical. This translation may be out of date; when in doubt, consult the English version.
>
> To propose corrections, see [`CONTRIBUTING.md`](CONTRIBUTING.md) → "Translations".

# stdocs

**Idiomas:** [English](README.md) (canonical) · [Español](README.es.md) · [Català](README.ca.md)

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

El mismo documento generado, mostrado por cada una de las cuatro UI incluidas — todas disponibles desde CDN con versión fijada o totalmente embebidas para builds air-gapped.

## Tabla de contenidos

- [Características](#características)
- [Instalación](#instalación)
- [Uso](#uso)
- [UIs](#uis)
- [Usar el spec en otras herramientas](#usar-el-spec-en-otras-herramientas)
- [Cómo funciona](#cómo-funciona)
- [Alcance y non-goals](#alcance-y-non-goals)
- [Contribuir](#contribuir)
- [Licencia](#licencia)

## Características

- **Cinco UI** — una por defecto, diminuta y sin dependencias (~1.6 KB, solo JS inline), más Scalar, Swagger UI, Redoc y Stoplight Elements — cada una disponible como subpaquete CDN (con versión fijada y hashes de integridad SRI) o como subpaquete embebido air-gapped.
- **Tres versiones de OpenAPI** — 3.0.4 (por defecto), 3.1.2 y 3.2.0, todas probadas.
- **Reflexión** — los tipos Go se convierten en JSON Schemas: punteros, slices, mapas, genéricos, structs embebidos, tipos recursivos vía `$ref`, tags `json` (incluidos `omitempty`, `omitzero` y `,string`), y reconocimiento de `json.Marshaler`/`encoding.TextMarshaler`.
- **Valores por defecto inteligentes** — los nombres de funciones se convierten en resúmenes, el primer segmento de la ruta se convierte en el tag y los parámetros de ruta se incluyen automáticamente.
- **Seguridad** — bearer, basic, API key, OAuth 2.0 (incluido el device flow de 3.2). Los nombres de esquemas no registrados se reportan como errores.
- **Activación por entorno** — `mux.Docs(enabled)` y `WithDisabled(true)` activan o desactivan la documentación según el entorno sin cambiar las rutas registradas.
- **Detección de try-it** — `FromDocs` identifica las requests que vienen de las consolas de los docs para que tu middleware decida qué pueden hacer.
- **Seguro frente a XSS** — el HTML de la documentación se renderiza con `html/template`.
- **Cero dependencias** — solo la biblioteca estándar de Go en tiempo de ejecución.

## Instalación

```bash
go get github.com/FumingPower3925/stdocs
```

Requiere Go 1.25 o posterior. stdocs sigue la misma política de soporte que el proyecto Go — las dos releases más recientes, actualmente 1.25 y 1.26 — y la CI ejecuta la suite completa de tests en cada patch release de ambas. Los patrones de ruta que stdocs documenta (`"GET /users/{id}"`) son la sintaxis método+ruta que `net/http.ServeMux` incorporó en Go 1.22.

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

### Tags de campo

Los campos de un struct pueden llevar documentación en tags, que se recogen al reflejar el tipo:

| Tag | Efecto |
|---|---|
| `doc:"…"` (o `description:"…"`) | Establece la descripción del campo en el schema |
| `example:"…"` | Establece el ejemplo del campo — se parsea según el tipo del campo, así que `example:"42"` en un `int` emite el número 42 |

```go
type Task struct {
    ID       string `json:"id" doc:"ID único de la tarea"`
    Priority int    `json:"priority" doc:"1 (baja) a 5 (urgente)" example:"3"`
}
```

Un valor de `example` que no se pueda parsear como el tipo del campo provoca un panic al construir el documento.

### La respuesta default

`WithResponse(0, body)` declara la respuesta `default` de OpenAPI — la entrada comodín a la que recurren los consumidores para códigos de estado no declarados, por convención la forma de error compartida:

```go
mux.HandleFunc("GET /tasks/{id}", getTask,
    stdocs.WithResponse(200, Task{}),
    stdocs.WithResponse(0, APIError{}), // "default" en el documento
)
```

Para funcionalidades que stdocs no expone directamente, usa el escape hatch:

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

La UI de documentación y los endpoints del spec (`openapi.json`, `openapi.yaml`) se pueden desactivar sin anular el registro de rutas. La decisión se toma cuando se llama a `Mount()`/`Docs()` (envuelve el handler tú mismo si necesitas decidirlo por petición):

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

Cuando está desactivada, toda petición bajo el prefijo de documentación recibe un 404. El spec sigue pudiéndose construir con `mux.JSON()` y `mux.YAML()` — desactivar la UI no detiene la generación del spec. Las rutas registradas bajo el prefijo de documentación (la propia página de docs, los handlers de recursos) nunca aparecen en el spec generado.

### Ocultar rutas individuales

La visibilidad por ruta se combina con los mecanismos anteriores: `Hidden()` excluye una ruta del documento en todas partes, e `Internal()` la excluye salvo que el mux se haya construido con `WithInternal(true)` (cuando se muestra, la operación lleva `x-internal: true`, la extensión que entienden las herramientas de filtrado de specs). Una configuración completa por entorno:

```go
env := os.Getenv("ENV")
mux := stdocs.New(
    stdocs.WithTitle("Mi API"),
    stdocs.WithDisabled(env == "prod"), // prod: sin documentación
    stdocs.WithInternal(env == "dev"),  // dev: documentación completa; en el resto, las rutas internas quedan ocultas
)
mux.HandleFunc("GET /users", listUsers)                           // siempre documentada
mux.HandleFunc("POST /admin/keys", rotateKeys, stdocs.Internal()) // documentada solo en dev
mux.HandleFunc("GET /healthz", healthCheck, stdocs.Hidden())      // nunca documentada
mux.Mount()
```

Las rutas excluidas no dejan rastro en el documento: ni rutas, ni esquemas, ni operationIds. La visibilidad solo da forma a la documentación publicada: las rutas ocultas e internas **siguen sirviendo tráfico en todos los entornos**. No es control de acceso; protege los endpoints sensibles con autenticación real.

### Detectar requests de prueba

Las consolas "Try it out" / "Test Request" de las UI completas envían **requests reales** a tu backend: en la red son indistinguibles de cualquier otro cliente. `FromDocs` las identifica (de forma aproximada, mediante la cabecera `Referer` que el navegador adjunta a los fetch de la página de docs) para que tu equipo decida la política: bloquear escrituras, desviarlas a un almacenamiento de pruebas, etiquetarlas para observabilidad o lo que prefiera.

```go
guard := func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet && mux.FromDocs(r) {
            http.Error(w, "las peticiones de prueba no pueden modificar datos", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
log.Fatal(http.ListenAndServe(":8080", guard(mux)))
```

`FromDocs` es una protección contra accidentes, **no un control de seguridad**: la cabecera `Referer` la controla el cliente (se puede falsificar) y se puede eliminar (extensiones de privacidad, una `Referrer-Policy` estricta). Además solo funciona cuando la página de docs y la API comparten origen — con una URL absoluta de `WithServer` en otro host, el navegador envía por defecto un `Referer` solo con el origen y la detección devuelve false. Úsala solo para *restringir* lo que puede hacer el tráfico originado en los docs — nunca para conceder acceso ni saltarte la autenticación.

Si tienes un documento OpenAPI escrito a mano en lugar de rutas generadas, sírvelo con `DocsHandler` + `WithSpec`:

```go
spec, _ := os.ReadFile("openapi.json")
http.Handle("GET /docs/", stdocs.DocsHandler(
    stdocs.WithTitle("Mi API"),
    stdocs.WithSpec(spec),
))
```

## UIs

La UI por defecto es una pequeña página HTML sin dependencias (~1.6 KB, solo JS inline, sin recursos externos). Para usar una UI más completa, importa un subpaquete y pasa su opción `WithUI()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalar"

mux := stdocs.New(stdocs.WithTitle("Mi API"), scalar.WithUI())
```

Para un build air-gapped (sin CDN), importa el subpaquete `*emb` correspondiente y monta su `AssetHandler()`:

```go
import "github.com/FumingPower3925/stdocs/ui/scalaremb"

mux := stdocs.New(stdocs.WithTitle("Mi API"), scalaremb.WithUI())
mux.Mount()
mux.Handle("GET /docs/_assets/",
    http.StripPrefix("/docs/_assets/", scalaremb.AssetHandler()))
```

Cada UI completa viene en dos variantes:

| UI                                  | Subpaquete CDN                          | Subpaquete embebido                          |
| ----------------------------------- | --------------------------------------- | -------------------------------------------- |
| _(por defecto)_ (integrada, ~1.6 KB) | —                                       | —                                            |
| Scalar                              | `ui/scalar` (~3.6 MB desde el CDN)      | `ui/scalaremb` (~3.6 MB en tu binario)       |
| Swagger UI                          | `ui/swaggerui` (~1.7 MB desde el CDN)   | `ui/swaggeruiemb` (~1.7 MB en tu binario)    |
| Redoc                               | `ui/redoc` (~1.1 MB desde el CDN)       | `ui/redocemb` (~1.1 MB en tu binario)        |
| Stoplight                           | `ui/stoplight` (~2.4 MB desde el CDN)   | `ui/stoplightemb` (~2.4 MB en tu binario)    |

Todas las URL del CDN están fijadas a versiones exactas con hashes de integridad SRI sha384. Los subpaquetes no se enlazan en tu binario a menos que los importes.

## Usar el spec en otras herramientas

El documento generado no es solo para la página de docs: `mux.JSON()` y `mux.YAML()` te dan exactamente los bytes servidos en los endpoints del spec, y la salida es **determinista por construcción** — las claves van ordenadas y los operationIds y nombres de componentes son estables entre reconstrucciones — así que funciona como artefacto commiteado.

El patrón recomendado es un test de golden file:

```go
var update = flag.Bool("update", false, "rewrite openapi.json")

func TestOpenAPIGolden(t *testing.T) {
    got, err := NewAPI().JSON() // tu constructor del mux
    if err != nil {
        t.Fatal(err)
    }
    const golden = "openapi.json"
    if *update {
        if err := os.WriteFile(golden, got, 0o644); err != nil {
            t.Fatal(err)
        }
    }
    want, err := os.ReadFile(golden)
    if err != nil {
        t.Fatalf("%v (ejecuta: go test -run TestOpenAPIGolden -update)", err)
    }
    if !bytes.Equal(got, want) {
        t.Fatalf("openapi.json está desactualizado; ejecuta: go test -run TestOpenAPIGolden -update")
    }
}
```

Cada cambio en la API aparece ahora como un diff revisable de `openapi.json` en la PR, y el archivo commiteado alimenta el resto de la cadena de herramientas sin ejecutar el servidor:

- **Diff de contrato** — p. ej. `oasdiff breaking old.json openapi.json` en la CI señala cambios que rompen compatibilidad.
- **Linting** — `spectral lint openapi.json` (o Redocly CLI) aplica reglas de estilo de API.
- **Generación de clientes** — apunta `openapi-generator`, `oapi-codegen` o tu pipeline de SDKs al archivo commiteado para producir clientes tipados en cualquier lenguaje.

## Cómo funciona

El `net/http.ServeMux` de Go 1.22 admite patrones de método+ruta, pero no los expone públicamente. `stdocs.New()` devuelve un `*stdocs.Mux` que embebe `*http.ServeMux` e intercepta las llamadas a `Handle`/`HandleFunc` para registrar el patrón y los metadatos. En la primera petición a `/docs/openapi.json`, se recorre el registro y el spec se construye y se guarda en caché (llama a `mux.Refresh()` para reconstruirlo).

Sin comentarios, sin generación de código, sin `unsafe` — la cadena del patrón es la documentación.

Hay una demo ejecutable en [`cmd/demo`](./cmd/demo):

```bash
go run ./cmd/demo
# abre http://localhost:8080/docs/
```

## Alcance y non-goals

stdocs hace una sola cosa: documenta aplicaciones de `net/http.ServeMux` de la biblioteca estándar y sirve el resultado. Conocer los límites de antemano te ahorra una evaluación:

- **Solo biblioteca estándar.** No hay integraciones con gin/echo/chi/fiber y no las habrá — el `ServeMux` envuelto es el diseño, no un primer adaptador.
- **Documentación, no enforcement.** stdocs no valida requests, no hace binding de parámetros ni comprueba que los handlers cumplan el contrato documentado. El documento describe la intención; mantener los handlers honestos es trabajo de la aplicación (el flujo de golden file de arriba hace el drift revisable).
- **Sin generación de código, sin anotaciones en comentarios, sin dependencias.** Permanentemente, por diseño.
- **La UI integrada se mantiene mínima.** La página por defecto es una lista de rutas de ~1.6 KB sin dependencias y sin consola de try-it — esa pequeñez es su característica. Las cuatro UI completas incluyen consolas y están a un import de distancia.

Cuando otra cosa encaja mejor: si el contrato es tu entregable (revisiones de spec entre equipos, clientes en varios lenguajes, conformidad forzada), un generador spec-first como oapi-codegen u ogen es la herramienta correcta; si empiezas de cero y quieres validación forzada desde los tipos, un framework de handlers tipados como huma lo es. stdocs es para el código que ya tienes.

## Contribuir

Consulta [CONTRIBUTING.md](CONTRIBUTING.md). Las traducciones las mantiene la comunidad; consulta allí la sección "Translations" para añadir o actualizar una.

```bash
go test -race -count=1 ./...
golangci-lint run ./...
```

## Licencia

Apache-2.0. Consulta [LICENSE](LICENSE).

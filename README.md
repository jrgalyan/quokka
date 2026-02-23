# quokka

A minimal, production-ready HTTP framework built on top of Go's `net/http`.

```
go get github.com/jrgalyan/quokka
```

## Quick Start

```go
package main

import (
    "net/http"
    "time"

    "github.com/jrgalyan/quokka"
)

func main() {
    r := quokka.New()
    r.Use(quokka.Recover(nil), quokka.Logger(quokka.LoggerConfig{}))

    r.GET("/hello/:name", func(c *quokka.Context) {
        c.JSON(http.StatusOK, map[string]any{"hello": c.Param("name")})
    })

    api := r.Group("/api", quokka.Timeout(5*time.Second))
    api.POST("/items", func(c *quokka.Context) { /* ... */ })

    srv := quokka.NewServer(quokka.ServerConfig{Addr: ":8080"}, r, nil)
    _ = srv.Start()
}
```

## Features

- Trie-based routing with path parameters and wildcards
- Route groups with prefix and per-group middleware
- Functional middleware composition
- Structured logging with request IDs
- Panic recovery, request timeouts, body limits
- CORS, security headers, gzip compression, rate limiting
- JWT authentication (Bearer tokens)
- Static file serving
- Graceful shutdown on SIGINT/SIGTERM
- TLS support

## Routing

Register routes with HTTP method helpers. Each accepts a path, a handler, and optional per-route middleware.

```go
r := quokka.New()

r.GET("/users", listUsers)
r.POST("/users", createUser)
r.PUT("/users/:id", updateUser)
r.DELETE("/users/:id", deleteUser)
r.PATCH("/users/:id", patchUser)
r.OPTIONS("/users", optionsUsers)
r.HEAD("/users", headUsers)

// Generic method registration
r.Handle("GET", "/health", healthCheck)
```

### Path Parameters

Named parameters start with `:` and match a single path segment.

```go
r.GET("/users/:id", func(c *quokka.Context) {
    id := c.Param("id")
    c.JSON(200, map[string]any{"id": id})
})
```

### Wildcards

A `*` segment matches everything after it. The matched value is available as `c.Param("*")`.

```go
r.GET("/files/*", func(c *quokka.Context) {
    filepath := c.Param("*")
    c.Text(200, "requested: "+filepath)
})
```

### Static Files

```go
// Serve a directory under a prefix (GET and HEAD)
r.ServeFiles("/static", http.Dir("./public"))

// Serve a single file at an exact path
r.File("/favicon.ico", "./public/favicon.ico")
```

### Trailing Slash Redirect

When enabled, requests to `/path/` are 301-redirected to `/path` (query string preserved).

```go
r.RedirectTrailingSlash = true
```

### Custom 404 and 405 Handlers

```go
r.NotFound(func(c *quokka.Context) {
    c.JSON(404, map[string]any{"error": "not found"})
})

r.MethodNotAllowed(func(c *quokka.Context) {
    c.JSON(405, map[string]any{"error": "method not allowed"})
})
```

### Error Handler

A unified error handler receives both 404 and 405 cases with a sentinel error (`ErrNotFound` or `ErrMethodNotAllowed`).

```go
r.ErrorHandler = func(c *quokka.Context, status int, err error) {
    c.JSON(status, map[string]any{"error": err.Error()})
}
```

## Route Groups

Groups share a path prefix and middleware. Groups support the same method helpers as the router.

```go
api_v1 := r.Group("/api/v1", authMiddleware)
api_v1.Use(quokka.Timeout(5 * time.Second))

api_v1.GET("/users", listUsers)
api_v1.POST("/users", createUser)
```

## Context

`Context` wraps `http.ResponseWriter` (field `W`) and `*http.Request` (field `R`).

### Input Helpers

```go
c.Param("id")            // path parameter
c.Query("page")          // query string ?page=2
c.Header("X-Request-Id") // request header
c.Form("email")          // form field (parses form on first call)
c.Cookie("session")      // cookie value (returns value, ok)
```

#### JSON Binding

Decodes the request body as JSON. Unknown fields are rejected. Body is limited to `Router.MaxBodySize` (default 10 MB).

```go
var input CreateUserRequest
if err := c.BindJSON(&input); err != nil {
    c.JSON(400, quokka.ErrorResponse{Error: "bad request", Message: err.Error()})
    return
}
```

#### Query and Form Binding

Bind query parameters or form values into a struct using struct tags.

```go
type Filters struct {
    Page  int    `query:"page"`
    Limit int    `query:"limit"`
    Sort  string `query:"sort"`
}

var f Filters
if err := c.BindQuery(&f); err != nil { /* ... */ }
```

```go
type LoginForm struct {
    Email    string `form:"email"`
    Password string `form:"password"`
}

var form LoginForm
if err := c.BindForm(&form); err != nil { /* ... */ }
```

Supported field types: `string`, `int*`, `float*`, `bool`.

#### File Uploads

```go
fh, err := c.FormFile("avatar")         // single file
fhs, err := c.FormFiles("attachments")  // multiple files
err = c.SaveFile(fh, "/uploads/pic.jpg") // save to disk
```

### Output Helpers

```go
c.JSON(200, obj)                  // application/json
c.Text(200, "hello")              // text/plain
c.Bytes(200, data, "image/png")   // arbitrary bytes with content type
c.Status(201)                     // status code only
c.NoContent()                     // 204 No Content
c.Redirect(302, "/login")         // redirect (0 defaults to 302)
c.SetHeader("X-Custom", "value")  // response header
c.SetCookie("name", "value", &http.Cookie{
    Path: "/", HttpOnly: true, Secure: true,
})
```

### Request Context

```go
ctx := c.Context()  // returns c.R.Context()
```

## Middleware

Middleware wraps handlers with cross-cutting concerns. The type signature:

```go
type Handler    func(*Context)
type Middleware func(Handler) Handler
```

Register middleware globally or per-group/route:

```go
r.Use(quokka.Recover(nil), quokka.Logger(quokka.LoggerConfig{}))           // global
api := r.Group("/api", quokka.Timeout(5*time.Second))     // per-group
r.GET("/path", handler, quokka.BodyLimit(1<<20))          // per-route
```

### Logger

Structured access logging via `slog`. Injects a request ID (from `X-Request-Id` header or auto-generated) and logs method, path, status, and duration. Accepts a `LoggerConfig` to set the logger and optional sanitization.

```go
r.Use(quokka.Logger(quokka.LoggerConfig{}))                         // uses slog.Default()
r.Use(quokka.Logger(quokka.LoggerConfig{Logger: myLogger}))         // custom logger
r.Use(quokka.Logger(quokka.LoggerConfig{                            // with sanitization
    Logger: myLogger,
    Sanitize: &quokka.SanitizeConfig{
        Params:  []string{"email"},
        Headers: []string{"Authorization"},
    },
}))
```

`LoggerConfig` fields:

| Field | Description |
|-------|-------------|
| `Logger` | `*slog.Logger` for output (nil uses `slog.Default()`) |
| `Sanitize` | `*SanitizeConfig` for redaction (nil disables) |

Retrieve the request ID downstream:

```go
id, ok := quokka.RequestID(c.Context())
```

### Recover

Catches panics, logs the error and stack trace, and returns a 500 JSON response.

```go
r.Use(quokka.Recover(nil))
```

### Timeout

Sets a context deadline on each request. Handlers should check `c.Context().Err()` for cancellation.

```go
r.Use(quokka.Timeout(10 * time.Second))
```

### CORS

Handles Cross-Origin Resource Sharing with preflight support.

```go
r.Use(quokka.CORS(quokka.DefaultCORSConfig()))
```

`CORSConfig` fields:

| Field | Default |
|-------|---------|
| `AllowOrigins` | `["*"]` |
| `AllowMethods` | GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS |
| `AllowHeaders` | Origin, Content-Type, Accept, Authorization, X-Request-Id |
| `ExposeHeaders` | (empty) |
| `MaxAge` | 86400 (24h) |
| `AllowCredentials` | false |

When `AllowCredentials` is true and origins include `"*"`, the middleware reflects the actual request origin instead of emitting `"*"`.

### Security Headers

Sets common security response headers.

```go
r.Use(quokka.SecurityHeaders(quokka.DefaultSecurityHeadersConfig()))
```

`SecurityHeadersConfig` fields:

| Field | Default |
|-------|---------|
| `HSTSMaxAge` | 63072000 (2 years) |
| `HSTSIncludeSubdomains` | true |
| `HSTSPreload` | false |
| `ContentTypeNosniff` | true |
| `FrameOption` | `"DENY"` |
| `ReferrerPolicy` | `"strict-origin-when-cross-origin"` |

### Gzip

Compresses responses using gzip. Responses smaller than `MinLength` are sent uncompressed. Already-compressed content types (images, archives) are skipped automatically.

```go
r.Use(quokka.Gzip(quokka.GzipConfig{}))  // defaults: level=DefaultCompression, minLength=256
```

`GzipConfig` fields:

| Field | Default |
|-------|---------|
| `Level` | `gzip.DefaultCompression` |
| `MinLength` | 256 bytes |

### Sanitizer

`Sanitizer` is a reusable utility for redacting sensitive path parameters, query parameters, and headers. Create one via `NewSanitizer` and call its methods from any output writer. The `Logger` middleware integrates with it automatically via `LoggerConfig.Sanitize`.

```go
// Standalone usage
san := quokka.NewSanitizer(quokka.SanitizeConfig{
    Params:      []string{"email", "ssn"},
    QueryParams: []string{"token"},
    Headers:     []string{"Authorization", "X-Api-Key"},
    Mask:        "***",  // default
})

path := san.Path(req.URL.Path, params)     // redacted path
query := san.Query(req.URL.RawQuery)       // redacted query string
headers := san.Headers(req.Header)         // cloned headers with redacted values
```

`SanitizeConfig` fields:

| Field | Default |
|-------|---------|
| `Params` | (empty) |
| `QueryParams` | (empty) |
| `Headers` | (empty) |
| `Mask` | `"***"` |

`NewSanitizer` returns nil when all lists are empty (no work). Methods on a nil `*Sanitizer` return inputs unchanged, so callers can skip nil checks.

### Body Limit

Restricts the maximum request body size using `http.MaxBytesReader`.

```go
r.Use(quokka.BodyLimit(1 << 20))  // 1 MB
```

### Rate Limit

Per-client rate limiting using a token bucket algorithm. Exceeded requests receive a 429 response with a `Retry-After` header.

```go
r.Use(quokka.RateLimit(quokka.RateLimitConfig{
    Rate:  10,  // requests per second
    Burst: 20,  // max burst
}))
```

`RateLimitConfig` fields:

| Field | Default |
|-------|---------|
| `Rate` | 10 req/s |
| `Burst` | 20 |
| `CleanupInterval` | 1 minute |
| `StaleAfter` | 5 minutes |
| `KeyFunc` | X-Forwarded-For, then RemoteAddr |

Provide a custom `KeyFunc` to key on something other than client IP:

```go
quokka.RateLimit(quokka.RateLimitConfig{
    Rate: 5,
    KeyFunc: func(c *quokka.Context) string {
        return c.Header("X-API-Key")
    },
})
```

## JWT Authentication

Validates Bearer tokens and injects claims into the request context. Returns RFC 6750 `WWW-Authenticate` headers on failure.

```go
r.Use(quokka.JWTAuth(quokka.JWTConfig{
    Keyfunc: func(token *jwt.Token) (any, error) {
        return []byte("secret"), nil
    },
    Issuer:   "myapp",
    Audience: "api",
}))
```

`JWTConfig` fields:

| Field | Description |
|-------|-------------|
| `Keyfunc` | Resolves the verification key (required) |
| `Issuer` | Expected `iss` claim |
| `Audience` | Expected `aud` claim |
| `Skew` | Clock skew tolerance (default 30s) |
| `Optional` | When true, requests without Authorization pass through |

Supported signing methods: HS256, HS384, HS512, RS256, RS384, RS512, ES256, EdDSA.

Retrieve claims downstream:

```go
claims, ok := quokka.JWTClaims(c.Context())
if ok {
    userID := claims["sub"].(string)
}
```

## Error Handling

Quokka provides a consistent JSON error structure inspired by RFC 9457:

```go
type ErrorResponse struct {
    Error   string            `json:"error"`
    Message string            `json:"message,omitempty"`
    Code    string            `json:"code,omitempty"`
    Details map[string]string `json:"details,omitempty"`
}
```

Sentinel errors for use with `ErrorHandler`:

- `quokka.ErrNotFound` -- route not found (404)
- `quokka.ErrMethodNotAllowed` -- method not allowed (405)

## Server

`Server` wraps `http.Server` with configured timeouts and graceful shutdown.

```go
srv := quokka.NewServer(quokka.ServerConfig{
    Addr:         ":8080",
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  120 * time.Second,
}, router, logger)

if err := srv.Start(); err != nil {
    log.Fatal(err)
}
```

`ServerConfig` defaults:

| Field | Default |
|-------|---------|
| `Addr` | `:8080` |
| `ReadTimeout` | 15s |
| `WriteTimeout` | 30s |
| `IdleTimeout` | 120s |
| `ReadHeaderTimeout` | 5s |
| `TLSConfig` | nil |

On SIGINT or SIGTERM the server drains in-flight requests with a 30-second shutdown timeout.

TLS is enabled by providing a `TLSConfig` with certificates or a `GetCertificate` function.

## Examples

See the [`examples/`](examples/) directory for a complete TODO API server:

```bash
PORT=9090 go run ./examples/todos/main.go
```

## Compatibility

Go 1.23+

## License

Apache-2.0

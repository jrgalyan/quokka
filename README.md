# quokka

A minimal, production‑ready HTTP framework built on top of net/http.

Import it:

```
import "github.com/jrgalyan/quokka"
```

Quick start:

```
package main

import (
    "net/http"
    "time"

    quokka "github.com/jrgalyan/quokka"
)

func main() {
    r := quokka.New()
    r.Use(quokka.Recover(nil), quokka.Logger(nil))

    // Method helpers and params
    r.GET("/hello/:name", func(c *quokka.Context) {
        c.JSON(http.StatusOK, map[string]any{"hello": c.Param("name")})
    })

    // Group routes
    api := r.Group("/api", quokka.Timeout(5*time.Second))
    api.POST("/items", func(c *quokka.Context) { /* ... */ })

    srv := quokka.NewServer(quokka.ServerConfig{Addr: ":8080"}, r, nil)
    _ = srv.Start()
}
```

Demo server is in cmd/quokka.

Compatibility: Go 1.22+.

License: Apache-2.0

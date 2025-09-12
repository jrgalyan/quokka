// Package quokka provides a minimal, production‑ready HTTP framework built on top of net/http.
//
// It focuses on:
//   - Fast, middleware‑driven routing with path params and wildcard support
//   - A small, explicit API that is easy to reason about and test
//   - Structured logging, panic recovery, timeouts, and graceful shutdown
//
// Getting started:
//
//	r := quokka.New()
//	r.Use(quokka.Recover(nil), quokka.Logger(nil))
//	r.Handle(http.MethodGet, "/hello/:name", func(c *quokka.Context) {
//		c.JSON(http.StatusOK, map[string]any{"hello": c.Param("name")})
//	})
//
//	srv := quokka.NewServer(quokka.ServerConfig{Addr: ":8080"}, r, nil)
//	_ = srv.Start()
//
// The package is framework‑agnostic and container‑friendly; import it and wire it in your service.
package quokka

# Quokka Examples

This directory contains runnable examples demonstrating how to build services with the quokka microframework.

## Todos REST Service

A fully-functional, in-memory Todos REST API showing all common HTTP methods and quokka features (routing, middleware, static files, graceful shutdown).

### Build

    go build ./examples/todos

### Run

    go run ./examples/todos

The service listens on port 8080 by default. Set PORT to override:

    PORT=9090 go run ./examples/todos

### API

Base path: /api

- Health: GET /health
- Static file demo: GET /static/LICENSE (served from repository root if available)
- List: GET /api/todos?offset=0&limit=50
- Create: POST /api/todos
  - Body: {"title":"Buy milk"}
- Retrieve: GET /api/todos/:id
- Replace: PUT /api/todos/:id
  - Body: {"title":"New title","completed":true}
- Partial update: PATCH /api/todos/:id
  - Body: {"completed":true}
- Delete: DELETE /api/todos/:id
- Existence: HEAD /api/todos/:id
- Allow discovery: OPTIONS /api/todos and /api/todos/:id (returns Allow header)

Responses are JSON. Errors follow a structured format: {"error":"...","message":"..."}.

### Notes
- Storage is in-memory and resets on restart.
- Middleware in use: Recover, Logger (request id and structured access logs), CORS, and SecurityHeaders.
- Graceful shutdown via `quokka.NewServer` on SIGINT/SIGTERM.

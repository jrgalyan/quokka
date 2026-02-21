/*
 *    Copyright 2025 Jeff Galyan
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package quokka

import (
	"context"
	"net/http"
	"path"
	"strings"
	"sync"
)

// Handler is the framework handler signature.
// It receives a *Context which wraps http primitives and helpers.
type Handler func(*Context)

// Middleware composes a handler with cross-cutting concerns.
type Middleware func(Handler) Handler

// Router provides HTTP method routing with middleware chaining and groups.
type Router struct {
	mu          sync.RWMutex
	root        *node
	mw          []Middleware
	notFound    Handler
	methodNA    Handler
	MaxBodySize int64 // max request body bytes for BindJSON; 0 means 10MB default

	// RedirectTrailingSlash, when true, causes the router to issue a 301
	// redirect when a request path has a trailing slash but the registered
	// route does not (e.g. /api/users/ → /api/users). The query string is
	// preserved across the redirect.
	RedirectTrailingSlash bool

	// ErrorHandler, when set, is called instead of the default notFound and
	// methodNA handlers. It receives the Context, the HTTP status code
	// (404 or 405), and a sentinel error (ErrNotFound or ErrMethodNotAllowed).
	ErrorHandler func(*Context, int, error)
}

type node struct {
	segment  string
	param    bool
	wildcard bool
	children []*node
	handlers map[string]Handler // method -> handler
}

// New creates a new Router.
func New() *Router {
	r := &Router{root: &node{handlers: make(map[string]Handler)}}
	r.notFound = func(c *Context) { c.JSON(http.StatusNotFound, ErrorResponse{Error: "not found"}) }
	r.methodNA = func(c *Context) { c.JSON(http.StatusMethodNotAllowed, ErrorResponse{Error: "method not allowed"}) }
	return r
}

// Use adds router-level middleware.
func (r *Router) Use(mw ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mw = append(r.mw, mw...)
}

// NotFound sets a custom handler for 404 responses. Router-level middleware is applied at request time.
func (r *Router) NotFound(h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notFound = h
}

// MethodNotAllowed sets a custom handler for 405 responses. Router-level middleware is applied at request time.
func (r *Router) MethodNotAllowed(h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.methodNA = h
}

// Handle registers a route handler for method and path.
func (r *Router) Handle(method, p string, h Handler, mw ...Middleware) {
	r.handleWithPrefix("", method, p, h, mw...)
}

func (r *Router) handleWithPrefix(prefix, method, p string, h Handler, mw ...Middleware) {
	if h == nil {
		panic("quokka: nil handler")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p == "" || p[0] != '/' {
		panic("path must start with /")
	}
	if prefix != "" {
		p = path.Join("/", prefix, p)
	}
	parts := splitPath(p)
	n := r.root
	for _, seg := range parts {
		child := matchChild(n, seg)
		if child == nil {
			child = &node{segment: seg, param: strings.HasPrefix(seg, ":"), wildcard: seg == "*", handlers: make(map[string]Handler)}
			n.children = append(n.children, child)
		}
		n = child
	}
	h = chain(mw, h)
	n.handlers[strings.ToUpper(method)] = h
}

// GET registers a handler for GET requests to the given path.
func (r *Router) GET(p string, h Handler, mw ...Middleware) { r.Handle(http.MethodGet, p, h, mw...) }

// POST registers a handler for POST requests to the given path.
func (r *Router) POST(p string, h Handler, mw ...Middleware) {
	r.Handle(http.MethodPost, p, h, mw...)
}

// PUT registers a handler for PUT requests to the given path.
func (r *Router) PUT(p string, h Handler, mw ...Middleware) { r.Handle(http.MethodPut, p, h, mw...) }

// DELETE registers a handler for DELETE requests to the given path.
func (r *Router) DELETE(p string, h Handler, mw ...Middleware) {
	r.Handle(http.MethodDelete, p, h, mw...)
}

// PATCH registers a handler for PATCH requests to the given path.
func (r *Router) PATCH(p string, h Handler, mw ...Middleware) {
	r.Handle(http.MethodPatch, p, h, mw...)
}

// OPTIONS registers a handler for OPTIONS requests to the given path.
func (r *Router) OPTIONS(p string, h Handler, mw ...Middleware) {
	r.Handle(http.MethodOptions, p, h, mw...)
}

// HEAD registers a handler for HEAD requests to the given path.
func (r *Router) HEAD(p string, h Handler, mw ...Middleware) {
	r.Handle(http.MethodHead, p, h, mw...)
}

// Group represents a route group with a common prefix and middleware.
type Group struct {
	r      *Router
	prefix string
	mw     []Middleware
}

// Group creates a new route group.
func (r *Router) Group(prefix string, mw ...Middleware) *Group {
	return &Group{r: r, prefix: strings.Trim(prefix, "/"), mw: mw}
}

// Use adds middleware to group.
func (g *Group) Use(mw ...Middleware) { g.mw = append(g.mw, mw...) }

// Handle registers a handler within the group.
func (g *Group) Handle(method, p string, h Handler, mw ...Middleware) {
	fullMW := append([]Middleware{}, g.mw...)
	fullMW = append(fullMW, mw...)
	g.r.handleWithPrefix(g.prefix, method, p, h, fullMW...)
}

// GET registers a handler for GET requests within the group.
func (g *Group) GET(p string, h Handler, mw ...Middleware) { g.Handle(http.MethodGet, p, h, mw...) }

// POST registers a handler for POST requests within the group.
func (g *Group) POST(p string, h Handler, mw ...Middleware) {
	g.Handle(http.MethodPost, p, h, mw...)
}

// PUT registers a handler for PUT requests within the group.
func (g *Group) PUT(p string, h Handler, mw ...Middleware) { g.Handle(http.MethodPut, p, h, mw...) }

// DELETE registers a handler for DELETE requests within the group.
func (g *Group) DELETE(p string, h Handler, mw ...Middleware) {
	g.Handle(http.MethodDelete, p, h, mw...)
}

// PATCH registers a handler for PATCH requests within the group.
func (g *Group) PATCH(p string, h Handler, mw ...Middleware) {
	g.Handle(http.MethodPatch, p, h, mw...)
}

// OPTIONS registers a handler for OPTIONS requests within the group.
func (g *Group) OPTIONS(p string, h Handler, mw ...Middleware) {
	g.Handle(http.MethodOptions, p, h, mw...)
}

// HEAD registers a handler for HEAD requests within the group.
func (g *Group) HEAD(p string, h Handler, mw ...Middleware) {
	g.Handle(http.MethodHead, p, h, mw...)
}

// ServeFiles serves static files under prefix from provided filesystem (GET and HEAD).
func (r *Router) ServeFiles(prefix string, fs http.FileSystem) {
	fileServer := http.FileServer(fs)
	// Normalize prefix to always start with a single slash and have no trailing slash
	pfx := "/" + strings.Trim(strings.TrimSpace(prefix), "/")
	if pfx == "/" {
		pfx = ""
	} // root
	route := pfx + "/*"
	h := func(c *Context) {
		strip := pfx
		if strip == "" {
			strip = "/"
		}
		http.StripPrefix(strip, fileServer).ServeHTTP(c.W, c.R.Clone(c.R.Context()))
	}
	r.GET(route, h)
	r.HEAD(route, h)
}

// File serves a single file at exact path.
func (r *Router) File(p, fpath string) {
	h := func(c *Context) { http.ServeFile(c.W, c.R, fpath) }
	r.GET(p, h)
	r.HEAD(p, h)
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := newContext(w, req)

	r.mu.RLock()

	// Trailing slash redirect: if enabled and path ends with "/" (but is not
	// the root), redirect to the trimmed path preserving the query string.
	urlPath := req.URL.Path
	if r.RedirectTrailingSlash && len(urlPath) > 1 && strings.HasSuffix(urlPath, "/") {
		target := strings.TrimRight(urlPath, "/")
		if q := req.URL.RawQuery; q != "" {
			target += "?" + q
		}
		r.mu.RUnlock()
		http.Redirect(w, req, target, http.StatusMovedPermanently)
		return
	}

	n, params := r.find(urlPath)
	var h Handler
	if n == nil || len(n.handlers) == 0 {
		h = r.errorHandler(http.StatusNotFound, ErrNotFound)
	} else if handler, ok := n.handlers[strings.ToUpper(req.Method)]; ok {
		c.params = params
		h = handler
	} else if req.Method == http.MethodHead {
		// Auto HEAD: fall back to the GET handler if no explicit HEAD handler exists.
		if getHandler, gok := n.handlers[http.MethodGet]; gok {
			c.params = params
			h = getHandler
		} else {
			h = r.errorHandler(http.StatusMethodNotAllowed, ErrMethodNotAllowed)
		}
	} else {
		h = r.errorHandler(http.StatusMethodNotAllowed, ErrMethodNotAllowed)
	}
	c.maxBodySize = r.MaxBodySize
	mw := r.mw
	r.mu.RUnlock()

	h = chain(mw, h)
	h(c)
}

// errorHandler returns the appropriate handler for the given status/error.
// When a custom ErrorHandler is set it is used; otherwise the default
// notFound/methodNA handlers are returned.
func (r *Router) errorHandler(status int, err error) Handler {
	if r.ErrorHandler != nil {
		eh := r.ErrorHandler
		return func(c *Context) { eh(c, status, err) }
	}
	if status == http.StatusMethodNotAllowed {
		return r.methodNA
	}
	return r.notFound
}

func (r *Router) find(pathStr string) (*node, map[string]string) {
	parts := splitPath(pathStr)
	n := r.root
	params := map[string]string{}
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		var next *node
		for _, ch := range n.children {
			if ch.segment == p {
				next = ch
				break
			}
			if ch.param {
				next = ch
				params[ch.segment[1:]] = p
			}
			if ch.wildcard {
				next = ch
				params["*"] = strings.Join(parts[i:], "/")
				i = len(parts) - 1
			}
		}
		if next == nil {
			return nil, nil
		}
		n = next
	}
	return n, params
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return []string{}
	}
	raw := strings.Split(p, "/")
	parts := raw[:0]
	for _, s := range raw {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

func matchChild(n *node, seg string) *node {
	for _, ch := range n.children {
		if ch.segment == seg {
			return ch
		}
		if seg == "*" && ch.wildcard {
			return ch
		}
	}
	// Detect conflicting param names at the same level (e.g. :id vs :userId).
	if strings.HasPrefix(seg, ":") {
		for _, ch := range n.children {
			if ch.param {
				panic("quokka: conflicting param name " + seg + ", existing " + ch.segment)
			}
		}
	}
	return nil
}

// Context key types to avoid collisions

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request_id"
)

// WithRequestID injects a request id into context
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestID extracts the request correlation ID from ctx.
func RequestID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyRequestID).(string)
	return v, ok
}

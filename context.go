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
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

// Context wraps http primitives and offers helpers for params, JSON, etc.
type Context struct {
	W      http.ResponseWriter
	R      *http.Request
	params map[string]string
	status int
	wrote  bool
}

func newContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{W: w, R: r, params: map[string]string{}}
}

func (c *Context) Param(name string) string { return c.params[name] }

func (c *Context) Query(key string) string { return c.R.URL.Query().Get(key) }

func (c *Context) Form(key string) string {
	_ = c.R.ParseForm()
	return c.R.FormValue(key)
}

func (c *Context) Header(key string) string { return c.R.Header.Get(key) }

func (c *Context) BindJSON(dst any) error {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Debug("error closing body", slog.String("error", err.Error()))
		}
	}(c.R.Body)
	dec := json.NewDecoder(io.LimitReader(c.R.Body, 10<<20)) // 10MB limit
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (c *Context) JSON(code int, v any) {
	if !c.wrote {
		c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	}
	c.status = code
	c.W.WriteHeader(code)
	_ = json.NewEncoder(c.W).Encode(v)
	c.wrote = true
}

// Text writes a plain text response
func (c *Context) Text(code int, s string) {
	if !c.wrote {
		c.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	c.status = code
	c.W.WriteHeader(code)
	_, _ = c.W.Write([]byte(s))
	c.wrote = true
}

// Bytes writes arbitrary bytes with a content type
func (c *Context) Bytes(code int, b []byte, contentType string) {
	if contentType != "" && !c.wrote {
		c.W.Header().Set("Content-Type", contentType)
	}
	c.status = code
	c.W.WriteHeader(code)
	_, _ = c.W.Write(b)
	c.wrote = true
}

// NoContent writes a 204 No Content
func (c *Context) NoContent() { c.Status(http.StatusNoContent) }

// Redirect sends a redirect to location with code (default 302 if code==0)
func (c *Context) Redirect(code int, location string) {
	if code == 0 {
		code = http.StatusFound
	}
	c.W.Header().Set("Location", location)
	c.Status(code)
}

// SetHeader sets a response header value
func (c *Context) SetHeader(k, v string) { c.W.Header().Set(k, v) }

// SetCookie adds a Set-Cookie header for name/value with optional attributes
func (c *Context) SetCookie(name, value string, attrs *http.Cookie) {
	ck := &http.Cookie{Name: name, Value: url.QueryEscape(value)}
	if attrs != nil {
		// copy selected attributes
		ck.Path = attrs.Path
		ck.Domain = attrs.Domain
		ck.Expires = attrs.Expires
		ck.MaxAge = attrs.MaxAge
		ck.Secure = attrs.Secure
		ck.HttpOnly = attrs.HttpOnly
		ck.SameSite = attrs.SameSite
	}
	http.SetCookie(c.W, ck)
}

// Cookie retrieves a cookie value and ok flag
func (c *Context) Cookie(name string) (string, bool) {
	ck, err := c.R.Cookie(name)
	if err != nil {
		return "", false
	}
	v, _ := url.QueryUnescape(ck.Value)
	return v, true
}

// Status writes only the status code
func (c *Context) Status(code int) {
	if c.wrote {
		return
	}
	c.status = code
	c.W.WriteHeader(code)
	c.wrote = true
}

func (c *Context) Context() context.Context { return c.R.Context() }

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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
)

// Context wraps http primitives and offers helpers for params, JSON, etc.
type Context struct {
	W           http.ResponseWriter
	R           *http.Request
	params      map[string]string
	status      int
	wrote       bool
	maxBodySize int64
}

func newContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{W: w, R: r, params: map[string]string{}}
}

// Param returns the value of a path parameter by name (e.g. ":id").
func (c *Context) Param(name string) string { return c.params[name] }

// Query returns a query string parameter value by key.
func (c *Context) Query(key string) string { return c.R.URL.Query().Get(key) }

// Form returns a form field value by key, parsing the form if necessary.
func (c *Context) Form(key string) string {
	if err := c.R.ParseForm(); err != nil {
		slog.Debug("form parse error", slog.Any("err", err))
	}
	return c.R.FormValue(key)
}

// Header returns a request header value by key.
func (c *Context) Header(key string) string { return c.R.Header.Get(key) }

// BindJSON decodes the request body as JSON into dst.
// Unknown fields are rejected and the body is limited to MaxBodySize (default 10 MB).
func (c *Context) BindJSON(dst any) error {
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Debug("error closing body", slog.String("error", err.Error()))
		}
	}(c.R.Body)
	limit := c.maxBodySize
	if limit <= 0 {
		limit = 10 << 20 // 10MB default
	}
	dec := json.NewDecoder(io.LimitReader(c.R.Body, limit))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

// JSON serializes v as JSON and writes it with the given status code.
func (c *Context) JSON(code int, v any) {
	if c.wrote {
		return
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		slog.Error("JSON encoding failed", slog.Any("err", err))
		c.W.WriteHeader(http.StatusInternalServerError)
		c.status = http.StatusInternalServerError
		c.wrote = true
		return
	}
	c.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.status = code
	c.W.WriteHeader(code)
	if _, err := c.W.Write(buf.Bytes()); err != nil {
		slog.Debug("response write error", slog.Any("err", err))
	}
	c.wrote = true
}

// Text writes a plain text response
func (c *Context) Text(code int, s string) {
	if c.wrote {
		return
	}
	c.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.status = code
	c.W.WriteHeader(code)
	if _, err := c.W.Write([]byte(s)); err != nil {
		slog.Debug("response write error", slog.Any("err", err))
	}
	c.wrote = true
}

// Bytes writes arbitrary bytes with a content type
func (c *Context) Bytes(code int, b []byte, contentType string) {
	if c.wrote {
		return
	}
	if contentType != "" {
		c.W.Header().Set("Content-Type", contentType)
	}
	c.status = code
	c.W.WriteHeader(code)
	if _, err := c.W.Write(b); err != nil {
		slog.Debug("response write error", slog.Any("err", err))
	}
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
	ck := &http.Cookie{Name: name, Value: url.PathEscape(value)}
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
	v, err := url.PathUnescape(ck.Value)
	if err != nil {
		return "", false
	}
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

// FormFile returns the first file for the provided form key.
// It parses the multipart form if it has not been parsed yet.
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	limit := c.maxBodySize
	if limit <= 0 {
		limit = 10 << 20
	}
	if err := c.R.ParseMultipartForm(limit); err != nil {
		return nil, err
	}
	f, fh, err := c.R.FormFile(name)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	return fh, nil
}

// FormFiles returns all files for the provided form key.
// It parses the multipart form if it has not been parsed yet.
func (c *Context) FormFiles(name string) ([]*multipart.FileHeader, error) {
	limit := c.maxBodySize
	if limit <= 0 {
		limit = 10 << 20
	}
	if err := c.R.ParseMultipartForm(limit); err != nil {
		return nil, err
	}
	if c.R.MultipartForm == nil || c.R.MultipartForm.File == nil {
		return nil, http.ErrMissingFile
	}
	fhs, ok := c.R.MultipartForm.File[name]
	if !ok || len(fhs) == 0 {
		return nil, http.ErrMissingFile
	}
	return fhs, nil
}

// SaveFile copies an uploaded file to the given destination path on disk.
func (c *Context) SaveFile(fh *multipart.FileHeader, dst string) error {
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, src)
	return err
}

// Context returns the request's context.Context.
func (c *Context) Context() context.Context { return c.R.Context() }

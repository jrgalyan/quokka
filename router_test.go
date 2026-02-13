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

package quokka_test

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

type memFS map[string]string

func (m memFS) Open(name string) (fs.File, error) {
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	if c, ok := m[name]; ok {
		return &memFile{name: name, data: []byte(c)}, nil
	}
	return nil, fs.ErrNotExist
}

type memFile struct {
	name string
	data []byte
	off  int
}

func (f *memFile) Stat() (fs.FileInfo, error) {
	return fileInfo{name: f.name, size: int64(len(f.data))}, nil
}
func (f *memFile) Read(p []byte) (int, error) {
	if f.off >= len(f.data) {
		return 0, io.EOF
	}
	n := copy(p, f.data[f.off:])
	f.off += n
	if f.off >= len(f.data) {
		return n, io.EOF
	}
	return n, nil
}
func (f *memFile) Close() error { return nil }

type fileInfo struct {
	name string
	size int64
}

func (fi fileInfo) Name() string       { return strings.TrimPrefix(fi.name, "/") }
func (fi fileInfo) Size() int64        { return fi.size }
func (fi fileInfo) Mode() fs.FileMode  { return 0444 }
func (fi fileInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (fi fileInfo) IsDir() bool        { return false }
func (fi fileInfo) Sys() any           { return nil }

var _ = Describe("Router", func() {
	It("routes methods and captures params", func() {
		r := q.New()
		r.GET("/hi/:name", func(c *q.Context) { c.Text(http.StatusOK, c.Param("name")) })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/hi/alex", nil)
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("alex"))
	})

	It("returns 404 and 405 appropriately", func() {
		r := q.New()
		r.POST("/things", func(c *q.Context) { c.Status(http.StatusCreated) })

		// 404 for unknown path
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/missing", nil))
		Expect(rr.Code).To(Equal(http.StatusNotFound))

		// 405 for known path with different method
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/things", nil))
		Expect(rr.Code).To(Equal(http.StatusMethodNotAllowed))
	})

	It("supports wildcard segments", func() {
		r := q.New()
		r.GET("/static/*", func(c *q.Context) { c.Text(http.StatusOK, c.Param("*")) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/static/a/b/c.txt", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("a/b/c.txt"))
	})

	It("groups routes with prefix and middleware", func() {
		r := q.New()
		order := []string{}
		r.Use(func(next q.Handler) q.Handler { return func(c *q.Context) { order = append(order, "r"); next(c) } })
		g := r.Group("/api", func(next q.Handler) q.Handler { return func(c *q.Context) { order = append(order, "g"); next(c) } })
		g.GET("/ping", func(c *q.Context) { order = append(order, "h"); c.Text(http.StatusOK, "ok") })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/ping", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(order).To(Equal([]string{"r", "g", "h"}))
	})

	It("serves files from filesystem and single file", func() {
		r := q.New()
		r.ServeFiles("/pub", http.FS(memFS{"/a.txt": "hello"}))
		r.File("/one", "LICENSE")

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/pub/a.txt", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(ContainSubstring("hello"))

		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/one", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		// content of LICENSE should start with Apache header in this repo
		Expect(rr.Body.String()).To(ContainSubstring("Apache License"))
	})

	It("handles concurrent requests safely", func() {
		r := q.New()
		r.GET("/count", func(c *q.Context) { c.Text(http.StatusOK, "ok") })
		r.GET("/user/:id", func(c *q.Context) { c.Text(http.StatusOK, c.Param("id")) })

		var wg sync.WaitGroup
		const n = 100
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				rr := httptest.NewRecorder()
				r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/count", nil))
				Expect(rr.Code).To(Equal(http.StatusOK))
			}()
		}
		wg.Wait()
	})

	It("normalizes double slashes in paths", func() {
		r := q.New()
		r.GET("/api/users", func(c *q.Context) { c.Text(http.StatusOK, "found") })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "//api//users", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("found"))
	})

	It("panics on conflicting param names at the same level", func() {
		r := q.New()
		r.GET("/users/:id", func(c *q.Context) { c.Status(http.StatusOK) })

		Expect(func() {
			r.GET("/users/:userId", func(c *q.Context) { c.Status(http.StatusOK) })
		}).To(PanicWith(ContainSubstring("conflicting param name")))
	})

	It("panics on nil handler at registration time", func() {
		r := q.New()
		Expect(func() {
			r.GET("/bad", nil)
		}).To(PanicWith("quokka: nil handler"))
	})

	It("serves static files via HEAD method", func() {
		r := q.New()
		r.ServeFiles("/pub", http.FS(memFS{"/a.txt": "hello"}))

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodHead, "/pub/a.txt", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		// HEAD should have no body
		Expect(rr.Body.Len()).To(Equal(0))
	})

	It("returns 404 for missing static files", func() {
		r := q.New()
		r.ServeFiles("/pub", http.FS(memFS{"/a.txt": "hello"}))

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/pub/missing.txt", nil))
		Expect(rr.Code).To(Equal(http.StatusNotFound))
	})

	It("allows custom NotFound and MethodNotAllowed handlers", func() {
		r := q.New()
		r.NotFound(func(c *q.Context) { c.Text(http.StatusNotFound, "custom 404") })
		r.MethodNotAllowed(func(c *q.Context) { c.Text(http.StatusMethodNotAllowed, "custom 405") })
		r.POST("/things", func(c *q.Context) { c.Status(http.StatusCreated) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nope", nil))
		Expect(rr.Code).To(Equal(http.StatusNotFound))
		Expect(rr.Body.String()).To(Equal("custom 404"))

		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/things", nil))
		Expect(rr.Code).To(Equal(http.StatusMethodNotAllowed))
		Expect(rr.Body.String()).To(Equal("custom 405"))
	})

	It("supports all HTTP method helpers", func() {
		r := q.New()
		methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions, http.MethodHead}
		h := func(c *q.Context) { c.Text(http.StatusOK, c.R.Method) }

		r.GET("/m", h)
		r.POST("/m", h)
		r.PUT("/m", h)
		r.DELETE("/m", h)
		r.PATCH("/m", h)
		r.OPTIONS("/m", h)
		r.HEAD("/m", h)

		for _, m := range methods {
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(m, "/m", nil))
			Expect(rr.Code).To(Equal(http.StatusOK))
		}
	})

	It("supports group method helpers", func() {
		r := q.New()
		g := r.Group("/api")
		h := func(c *q.Context) { c.Text(http.StatusOK, "ok") }

		g.POST("/r", h)
		g.PUT("/r", h)
		g.DELETE("/r", h)
		g.PATCH("/r", h)
		g.OPTIONS("/r", h)
		g.HEAD("/r", h)

		for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions, http.MethodHead} {
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(m, "/api/r", nil))
			Expect(rr.Code).To(Equal(http.StatusOK))
		}
	})
})

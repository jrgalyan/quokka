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
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

func decompressGzip(data []byte) (string, error) {
	r, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

var _ = Describe("Gzip Middleware", func() {
	longText := strings.Repeat("Hello, World! This is a test of gzip compression. ", 20)

	It("compresses JSON response when client accepts gzip", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))
		r.GET("/api", func(c *q.Context) {
			c.JSON(http.StatusOK, map[string]string{"data": longText})
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Encoding")).To(Equal("gzip"))
		Expect(rr.Header().Get("Vary")).To(ContainSubstring("Accept-Encoding"))

		body, err := decompressGzip(rr.Body.Bytes())
		Expect(err).To(BeNil())
		Expect(body).To(ContainSubstring("Hello, World!"))
	})

	It("does not compress when client does not accept gzip", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))
		r.GET("/api", func(c *q.Context) {
			c.JSON(http.StatusOK, map[string]string{"data": longText})
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		// No Accept-Encoding
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Encoding")).To(BeEmpty())
		Expect(rr.Body.String()).To(ContainSubstring("Hello, World!"))
	})

	It("does not compress responses below minimum length", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{MinLength: 1024}))
		r.GET("/api", func(c *q.Context) {
			c.Text(http.StatusOK, "short")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Encoding")).NotTo(Equal("gzip"))
		Expect(rr.Body.String()).To(Equal("short"))
	})

	It("compresses responses at or above minimum length", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{MinLength: 10}))
		r.GET("/api", func(c *q.Context) {
			c.Text(http.StatusOK, "this is more than ten bytes of data")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Encoding")).To(Equal("gzip"))

		body, err := decompressGzip(rr.Body.Bytes())
		Expect(err).To(BeNil())
		Expect(body).To(Equal("this is more than ten bytes of data"))
	})

	It("skips compression for image/jpeg content type", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{MinLength: 1}))
		fakeJPEG := strings.Repeat("\xFF\xD8\xFF", 100)
		r.GET("/img", func(c *q.Context) {
			c.Bytes(http.StatusOK, []byte(fakeJPEG), "image/jpeg")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/img", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Content-Encoding")).NotTo(Equal("gzip"))
	})

	It("skips compression for image/png content type", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{MinLength: 1}))
		fakePNG := strings.Repeat("\x89PNG", 100)
		r.GET("/img", func(c *q.Context) {
			c.Bytes(http.StatusOK, []byte(fakePNG), "image/png")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/img", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Content-Encoding")).NotTo(Equal("gzip"))
	})

	It("skips compression for application/gzip content type", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{MinLength: 1}))
		fakeGzip := strings.Repeat("\x1f\x8b", 100)
		r.GET("/dl", func(c *q.Context) {
			c.Bytes(http.StatusOK, []byte(fakeGzip), "application/gzip")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/dl", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Content-Encoding")).NotTo(Equal("gzip"))
	})

	It("works with Text responses", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))
		r.GET("/t", func(c *q.Context) {
			c.Text(http.StatusOK, longText)
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Encoding")).To(Equal("gzip"))

		body, err := decompressGzip(rr.Body.Bytes())
		Expect(err).To(BeNil())
		Expect(body).To(Equal(longText))
	})

	It("handles NoContent (204) without error", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))
		r.GET("/nc", func(c *q.Context) {
			c.NoContent()
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/nc", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusNoContent))
		Expect(rr.Header().Get("Content-Encoding")).NotTo(Equal("gzip"))
		Expect(rr.Body.Len()).To(Equal(0))
	})

	It("handles Redirect without error", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))
		r.GET("/redir", func(c *q.Context) {
			c.Redirect(http.StatusFound, "/other")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/redir", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusFound))
		Expect(rr.Header().Get("Location")).To(Equal("/other"))
	})

	It("preserves double-write protection", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))
		r.GET("/dw", func(c *q.Context) {
			c.JSON(http.StatusOK, map[string]string{"data": longText})
			c.Text(http.StatusConflict, "should not appear")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/dw", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Encoding")).To(Equal("gzip"))
		body, err := decompressGzip(rr.Body.Bytes())
		Expect(err).To(BeNil())
		Expect(body).NotTo(ContainSubstring("should not appear"))
	})

	It("sets Vary: Accept-Encoding even when not compressing due to small size", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{MinLength: 10000}))
		r.GET("/small", func(c *q.Context) {
			c.Text(http.StatusOK, "tiny")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/small", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Vary")).To(ContainSubstring("Accept-Encoding"))
		Expect(rr.Header().Get("Content-Encoding")).NotTo(Equal("gzip"))
	})

	It("works with Bytes response", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))
		data := []byte(longText)
		r.GET("/b", func(c *q.Context) {
			c.Bytes(http.StatusOK, data, "application/octet-stream")
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/b", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Encoding")).To(Equal("gzip"))

		body, err := decompressGzip(rr.Body.Bytes())
		Expect(err).To(BeNil())
		Expect(body).To(Equal(longText))
	})

	It("default config uses 256-byte minimum threshold", func() {
		r := q.New()
		r.Use(q.Gzip(q.GzipConfig{}))

		// Under 256 bytes — should not compress
		r.GET("/small", func(c *q.Context) {
			c.Text(http.StatusOK, strings.Repeat("a", 200))
		})
		// Over 256 bytes — should compress
		r.GET("/large", func(c *q.Context) {
			c.Text(http.StatusOK, strings.Repeat("a", 300))
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/small", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)
		Expect(rr.Header().Get("Content-Encoding")).NotTo(Equal("gzip"))

		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/large", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(rr, req)
		Expect(rr.Header().Get("Content-Encoding")).To(Equal("gzip"))
	})
})

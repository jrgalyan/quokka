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
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("CORS Middleware", func() {
	It("sets CORS headers on simple request with default config", func() {
		r := q.New()
		r.Use(q.CORS(q.DefaultCORSConfig()))
		r.GET("/api", func(c *q.Context) { c.Text(http.StatusOK, "ok") })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(Equal("*"))
		Expect(rr.Header().Get("Vary")).To(ContainSubstring("Origin"))
		Expect(rr.Body.String()).To(Equal("ok"))
	})

	It("handles preflight OPTIONS request and returns 204", func() {
		r := q.New()
		r.Use(q.CORS(q.DefaultCORSConfig()))
		handlerCalled := false
		r.GET("/api", func(c *q.Context) { handlerCalled = true; c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusNoContent))
		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(Equal("*"))
		Expect(rr.Header().Get("Access-Control-Allow-Methods")).To(ContainSubstring("POST"))
		Expect(rr.Header().Get("Access-Control-Allow-Headers")).To(ContainSubstring("Content-Type"))
		Expect(handlerCalled).To(BeFalse())
	})

	It("passes through non-CORS requests without headers", func() {
		r := q.New()
		r.Use(q.CORS(q.DefaultCORSConfig()))
		r.GET("/api", func(c *q.Context) { c.Text(http.StatusOK, "ok") })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		// No Origin header
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(BeEmpty())
	})

	It("does not add CORS headers for disallowed origin", func() {
		cfg := q.DefaultCORSConfig()
		cfg.AllowOrigins = []string{"http://allowed.com"}
		r := q.New()
		r.Use(q.CORS(cfg))
		handlerCalled := false
		r.OPTIONS("/api", func(c *q.Context) { handlerCalled = true; c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api", nil)
		req.Header.Set("Origin", "http://evil.com")
		req.Header.Set("Access-Control-Request-Method", "POST")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(BeEmpty())
		Expect(handlerCalled).To(BeTrue())
	})

	It("allows specific configured origin", func() {
		cfg := q.DefaultCORSConfig()
		cfg.AllowOrigins = []string{"http://allowed.com"}
		r := q.New()
		r.Use(q.CORS(cfg))
		r.GET("/api", func(c *q.Context) { c.Text(http.StatusOK, "ok") })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "http://allowed.com")
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(Equal("http://allowed.com"))
	})

	It("reflects origin when credentials enabled with wildcard", func() {
		cfg := q.DefaultCORSConfig()
		cfg.AllowCredentials = true
		r := q.New()
		r.Use(q.CORS(cfg))
		r.GET("/api", func(c *q.Context) { c.Text(http.StatusOK, "ok") })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(Equal("http://example.com"))
		Expect(rr.Header().Get("Access-Control-Allow-Credentials")).To(Equal("true"))
	})

	It("sets Access-Control-Max-Age on preflight", func() {
		cfg := q.DefaultCORSConfig()
		cfg.MaxAge = 3600
		r := q.New()
		r.Use(q.CORS(cfg))
		r.GET("/api", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Access-Control-Max-Age")).To(Equal("3600"))
	})

	It("sets Access-Control-Expose-Headers on actual request", func() {
		cfg := q.DefaultCORSConfig()
		cfg.ExposeHeaders = []string{"X-Custom-Header", "X-Other"}
		r := q.New()
		r.Use(q.CORS(cfg))
		r.GET("/api", func(c *q.Context) { c.Text(http.StatusOK, "ok") })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Access-Control-Expose-Headers")).To(Equal("X-Custom-Header, X-Other"))
	})

	It("does not set expose headers when list is empty", func() {
		r := q.New()
		r.Use(q.CORS(q.DefaultCORSConfig()))
		r.GET("/api", func(c *q.Context) { c.Text(http.StatusOK, "ok") })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		r.ServeHTTP(rr, req)

		Expect(rr.Header().Get("Access-Control-Expose-Headers")).To(BeEmpty())
	})

	It("treats OPTIONS without Access-Control-Request-Method as a normal request", func() {
		r := q.New()
		r.Use(q.CORS(q.DefaultCORSConfig()))
		handlerCalled := false
		r.OPTIONS("/api", func(c *q.Context) { handlerCalled = true; c.Text(http.StatusOK, "options") })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		// No Access-Control-Request-Method header
		r.ServeHTTP(rr, req)

		Expect(handlerCalled).To(BeTrue())
		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(Equal("*"))
		Expect(rr.Header().Get("Vary")).To(ContainSubstring("Origin"))
	})

	It("works with multiple allowed origins", func() {
		cfg := q.DefaultCORSConfig()
		cfg.AllowOrigins = []string{"http://a.com", "http://b.com"}
		r := q.New()
		r.Use(q.CORS(cfg))
		r.GET("/api", func(c *q.Context) { c.Text(http.StatusOK, "ok") })

		// Allowed origin
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "http://b.com")
		r.ServeHTTP(rr, req)
		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(Equal("http://b.com"))

		// Disallowed origin
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/api", nil)
		req.Header.Set("Origin", "http://c.com")
		r.ServeHTTP(rr, req)
		Expect(rr.Header().Get("Access-Control-Allow-Origin")).To(BeEmpty())
	})

	It("sets correct Vary header on preflight", func() {
		r := q.New()
		r.Use(q.CORS(q.DefaultCORSConfig()))
		r.GET("/api", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api", nil)
		req.Header.Set("Origin", "http://example.com")
		req.Header.Set("Access-Control-Request-Method", "GET")
		r.ServeHTTP(rr, req)

		vary := rr.Header().Get("Vary")
		Expect(vary).To(ContainSubstring("Origin"))
		Expect(vary).To(ContainSubstring("Access-Control-Request-Method"))
		Expect(vary).To(ContainSubstring("Access-Control-Request-Headers"))
	})
})

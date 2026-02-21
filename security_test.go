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

var _ = Describe("SecurityHeaders", func() {
	handler := func(c *q.Context) { c.Text(http.StatusOK, "ok") }

	It("sets all default security headers", func() {
		r := q.New()
		r.Use(q.SecurityHeaders(q.DefaultSecurityHeadersConfig()))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Strict-Transport-Security")).To(Equal("max-age=63072000; includeSubDomains"))
		Expect(rr.Header().Get("X-Content-Type-Options")).To(Equal("nosniff"))
		Expect(rr.Header().Get("X-Frame-Options")).To(Equal("DENY"))
		Expect(rr.Header().Get("Referrer-Policy")).To(Equal("strict-origin-when-cross-origin"))
	})

	It("omits HSTS when HSTSMaxAge is 0", func() {
		cfg := q.DefaultSecurityHeadersConfig()
		cfg.HSTSMaxAge = 0
		r := q.New()
		r.Use(q.SecurityHeaders(cfg))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Header().Get("Strict-Transport-Security")).To(BeEmpty())
		Expect(rr.Header().Get("X-Content-Type-Options")).To(Equal("nosniff"))
	})

	It("includes preload directive when enabled", func() {
		cfg := q.DefaultSecurityHeadersConfig()
		cfg.HSTSPreload = true
		r := q.New()
		r.Use(q.SecurityHeaders(cfg))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Header().Get("Strict-Transport-Security")).To(Equal("max-age=63072000; includeSubDomains; preload"))
	})

	It("uses custom frame option", func() {
		cfg := q.DefaultSecurityHeadersConfig()
		cfg.FrameOption = "SAMEORIGIN"
		r := q.New()
		r.Use(q.SecurityHeaders(cfg))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Header().Get("X-Frame-Options")).To(Equal("SAMEORIGIN"))
	})

	It("omits nosniff when disabled", func() {
		cfg := q.DefaultSecurityHeadersConfig()
		cfg.ContentTypeNosniff = false
		r := q.New()
		r.Use(q.SecurityHeaders(cfg))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Header().Get("X-Content-Type-Options")).To(BeEmpty())
	})

	It("uses custom referrer policy", func() {
		cfg := q.DefaultSecurityHeadersConfig()
		cfg.ReferrerPolicy = "no-referrer"
		r := q.New()
		r.Use(q.SecurityHeaders(cfg))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Header().Get("Referrer-Policy")).To(Equal("no-referrer"))
	})
})

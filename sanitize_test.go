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
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("Sanitizer", func() {
	Describe("Path", func() {
		It("redacts a single param value", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Params: []string{"email"},
			})
			Expect(san).NotTo(BeNil())

			result := san.Path("/users/jeff@example.com", map[string]string{"email": "jeff@example.com"})
			Expect(result).To(Equal("/users/***"))
		})

		It("redacts multiple param values", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Params: []string{"org", "user"},
			})

			result := san.Path("/orgs/acme/users/bob", map[string]string{"org": "acme", "user": "bob"})
			Expect(result).To(Equal("/orgs/***/users/***"))
		})

		It("returns path unchanged when configured params do not exist in inputs", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Params: []string{"email"},
			})

			result := san.Path("/users/42", map[string]string{"id": "42"})
			Expect(result).To(Equal("/users/42"))
		})
	})

	Describe("Query", func() {
		It("redacts configured keys and preserves others", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				QueryParams: []string{"token", "secret"},
			})

			result := san.Query("token=abc123&page=2&secret=s3cret")
			Expect(result).To(ContainSubstring("token=***"))
			Expect(result).To(ContainSubstring("secret=***"))
			Expect(result).To(ContainSubstring("page=2"))
		})

		It("redacts multi-value query params", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				QueryParams: []string{"tag"},
			})

			result := san.Query("tag=a&tag=b&page=1")
			Expect(result).To(ContainSubstring("tag=***"))
			Expect(result).NotTo(ContainSubstring("tag=a"))
			Expect(result).NotTo(ContainSubstring("tag=b"))
			Expect(result).To(ContainSubstring("page=1"))
		})

		It("returns empty string unchanged", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				QueryParams: []string{"token"},
			})

			Expect(san.Query("")).To(Equal(""))
		})

		It("returns query unchanged when configured keys are absent", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				QueryParams: []string{"secret"},
			})

			Expect(san.Query("page=1&limit=10")).To(Equal("page=1&limit=10"))
		})
	})

	Describe("Headers", func() {
		It("redacts configured keys case-insensitively and preserves others", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Headers: []string{"x-api-key", "Authorization"},
			})

			h := http.Header{}
			h.Set("X-Api-Key", "key-12345")
			h.Set("Authorization", "Bearer tok")
			h.Set("Accept", "application/json")

			result := san.Headers(h)
			Expect(result).NotTo(BeNil())
			Expect(result.Get("X-Api-Key")).To(Equal("***"))
			Expect(result.Get("Authorization")).To(Equal("***"))
			Expect(result.Get("Accept")).To(Equal("application/json"))
		})

		It("returns a clone without modifying originals", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Headers: []string{"Authorization"},
			})

			h := http.Header{}
			h.Set("Authorization", "Bearer tok")

			result := san.Headers(h)
			Expect(result.Get("Authorization")).To(Equal("***"))
			Expect(h.Get("Authorization")).To(Equal("Bearer tok"))
		})

		It("returns nil when configured headers are not present", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Headers: []string{"X-Secret"},
			})

			h := http.Header{}
			h.Set("Accept", "text/html")

			result := san.Headers(h)
			// Still returns a clone since headerSet is non-empty
			Expect(result).NotTo(BeNil())
			Expect(result.Get("Accept")).To(Equal("text/html"))
		})
	})

	Describe("custom mask", func() {
		It("uses the configured mask string", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Params:      []string{"id"},
				QueryParams: []string{"token"},
				Headers:     []string{"Authorization"},
				Mask:        "[REDACTED]",
			})

			Expect(san.Path("/users/42", map[string]string{"id": "42"})).To(Equal("/users/[REDACTED]"))
			Expect(san.Query("token=abc")).To(ContainSubstring("token=[REDACTED]"))
			h := http.Header{}
			h.Set("Authorization", "Bearer tok")
			Expect(san.Headers(h).Get("Authorization")).To(Equal("[REDACTED]"))
		})
	})

	Describe("default mask", func() {
		It("defaults to *** when Mask is empty", func() {
			san := q.NewSanitizer(q.SanitizeConfig{
				Params: []string{"id"},
				Mask:   "",
			})

			Expect(san.Path("/users/42", map[string]string{"id": "42"})).To(Equal("/users/***"))
		})
	})

	Describe("nil Sanitizer", func() {
		It("returns inputs unchanged", func() {
			san := q.NewSanitizer(q.DefaultSanitizeConfig()) // all lists empty → nil
			Expect(san).To(BeNil())

			Expect(san.Path("/users/42", map[string]string{"id": "42"})).To(Equal("/users/42"))
			Expect(san.Query("token=abc")).To(Equal("token=abc"))
			Expect(san.Headers(http.Header{"Authorization": []string{"Bearer tok"}})).To(BeNil())
		})
	})

	Describe("Logger integration", func() {
		It("Logger with Sanitize config logs the sanitized path", func() {
			cfg := q.DefaultSanitizeConfig()
			cfg.Params = []string{"email"}

			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, nil))

			r := q.New()
			r.Use(q.Logger(q.LoggerConfig{Logger: logger, Sanitize: &cfg}))
			r.GET("/users/:email", func(c *q.Context) {
				c.Status(http.StatusOK)
			})

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/users/jeff@example.com", nil)
			r.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusOK))
			logOutput := logBuf.String()
			Expect(logOutput).To(ContainSubstring("/users/***"))
			Expect(logOutput).NotTo(ContainSubstring("jeff@example.com"))
		})
	})
})

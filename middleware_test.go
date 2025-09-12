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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("Middleware", func() {
	It("Logger injects request id and logs", func() {
		r := q.New()
		// use default logger
		r.Use(q.Logger(nil))
		var seen string
		r.GET("/id", func(c *q.Context) {
			if v, ok := q.RequestID(c.Context()); ok {
				seen = v
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/id", nil)
		req.Header.Set("X-Request-Id", "abc123")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(seen).To(Equal("abc123"))
	})

	It("Recover returns 500 on panic", func() {
		r := q.New()
		r.Use(q.Recover(slog.Default()))
		r.GET("/p", func(c *q.Context) { panic("boom") })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/p", nil))
		Expect(rr.Code).To(Equal(http.StatusInternalServerError))
		Expect(rr.Body.String()).To(ContainSubstring("internal server error"))
	})

	It("Timeout applies deadline to request context", func() {
		r := q.New()
		r.Use(q.Timeout(50 * time.Millisecond))
		var hadDeadline bool
		r.GET("/t", func(c *q.Context) { _, ok := c.Context().Deadline(); hadDeadline = ok; c.Status(http.StatusOK) })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/t", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(hadDeadline).To(BeTrue())
	})
})

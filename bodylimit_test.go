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
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("BodyLimit", func() {
	It("allows requests within the size limit", func() {
		r := q.New()
		r.Use(q.BodyLimit(1024))
		r.POST("/upload", func(c *q.Context) {
			body, err := io.ReadAll(c.R.Body)
			if err != nil {
				c.JSON(http.StatusRequestEntityTooLarge, q.ErrorResponse{Error: "too large"})
				return
			}
			c.Text(http.StatusOK, string(body))
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("hello")))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("hello"))
	})

	It("rejects requests exceeding the size limit", func() {
		r := q.New()
		r.Use(q.BodyLimit(10))
		r.POST("/upload", func(c *q.Context) {
			_, err := io.ReadAll(c.R.Body)
			if err != nil {
				c.JSON(http.StatusRequestEntityTooLarge, q.ErrorResponse{Error: "too large"})
				return
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		body := strings.Repeat("x", 100)
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(body)))
		Expect(rr.Code).To(Equal(http.StatusRequestEntityTooLarge))
	})

	It("does not enforce limit when maxBytes is 0", func() {
		r := q.New()
		r.Use(q.BodyLimit(0))
		r.POST("/upload", func(c *q.Context) {
			body, err := io.ReadAll(c.R.Body)
			if err != nil {
				c.JSON(http.StatusRequestEntityTooLarge, q.ErrorResponse{Error: "too large"})
				return
			}
			c.Text(http.StatusOK, string(body))
		})

		rr := httptest.NewRecorder()
		bigBody := strings.Repeat("x", 10000)
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(bigBody)))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal(bigBody))
	})

	It("passes through GET requests with no body", func() {
		r := q.New()
		r.Use(q.BodyLimit(10))
		r.GET("/ping", func(c *q.Context) { c.Text(http.StatusOK, "pong") })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/ping", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("pong"))
	})
})

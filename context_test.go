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
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("Context", func() {
	It("writes JSON with content type", func() {
		r := q.New()
		r.GET("/j", func(c *q.Context) { c.JSON(http.StatusCreated, map[string]any{"a": 1}) })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/j", nil))
		Expect(rr.Code).To(Equal(http.StatusCreated))
		Expect(rr.Header().Get("Content-Type")).To(ContainSubstring("application/json"))
		var m map[string]int
		Expect(json.Unmarshal(rr.Body.Bytes(), &m)).To(Succeed())
		Expect(m["a"]).To(Equal(1))
	})

	It("writes Text and Bytes with proper content type", func() {
		r := q.New()
		r.GET("/t", func(c *q.Context) { c.Text(http.StatusOK, "hello") })
		r.GET("/b", func(c *q.Context) { c.Bytes(http.StatusOK, []byte{1, 2, 3}, "application/octet-stream") })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/t", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Type")).To(ContainSubstring("text/plain"))
		Expect(rr.Body.String()).To(Equal("hello"))
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/b", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Type")).To(Equal("application/octet-stream"))
		Expect(rr.Body.Bytes()).To(Equal([]byte{1, 2, 3}))
	})

	It("supports Status and NoContent and Redirect", func() {
		r := q.New()
		r.GET("/n", func(c *q.Context) { c.NoContent() })
		r.GET("/r", func(c *q.Context) { c.Redirect(0, "/next") })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/n", nil))
		Expect(rr.Code).To(Equal(http.StatusNoContent))
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/r", nil))
		Expect(rr.Code).To(Equal(http.StatusFound))
		Expect(rr.Header().Get("Location")).To(Equal("/next"))
	})

	It("handles cookies set and get", func() {
		r := q.New()
		r.GET("/set", func(c *q.Context) { c.SetCookie("n", "v 1", &http.Cookie{Path: "/"}); c.Status(http.StatusOK) })
		r.GET("/get", func(c *q.Context) {
			v, ok := c.Cookie("n")
			if !ok {
				c.Status(http.StatusNotFound)
				return
			}
			c.Text(http.StatusOK, v)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/set", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		ck := rr.Header().Get("Set-Cookie")
		Expect(ck).To(ContainSubstring("n="))

		req := httptest.NewRequest(http.MethodGet, "/get", nil)
		req.Header.Set("Cookie", ck)
		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("v 1"))
	})

	It("binds JSON and rejects unknown fields", func() {
		r := q.New()
		type X struct {
			A int `json:"a"`
		}
		r.POST("/bind", func(c *q.Context) {
			var x X
			if err := c.BindJSON(&x); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: "bad json"})
				return
			}
			if x.A == 1 {
				c.Status(http.StatusOK)
			} else {
				c.Status(http.StatusTeapot)
			}
		})
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString(`{"a":1}`)))
		Expect(rr.Code).To(Equal(http.StatusOK))

		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString(`{"a":1,"b":2}`)))
		Expect(rr.Code).To(Equal(http.StatusBadRequest))
	})
})

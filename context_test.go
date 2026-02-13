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
	"strings"

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

	It("rejects malformed JSON in BindJSON", func() {
		r := q.New()
		type X struct {
			A int `json:"a"`
		}
		r.POST("/bind", func(c *q.Context) {
			var x X
			if err := c.BindJSON(&x); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: err.Error()})
				return
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString(`{invalid json`)))
		Expect(rr.Code).To(Equal(http.StatusBadRequest))
	})

	It("rejects oversized body in BindJSON with custom MaxBodySize", func() {
		r := q.New()
		r.MaxBodySize = 16 // very small limit
		type X struct {
			A string `json:"a"`
		}
		r.POST("/bind", func(c *q.Context) {
			var x X
			if err := c.BindJSON(&x); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: "too large"})
				return
			}
			c.Status(http.StatusOK)
		})

		// Body larger than 16 bytes
		bigBody := `{"a":"` + strings.Repeat("x", 100) + `"}`
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/bind", bytes.NewBufferString(bigBody)))
		Expect(rr.Code).To(Equal(http.StatusBadRequest))
	})

	It("prevents double-write: JSON then Text is silently ignored", func() {
		r := q.New()
		r.GET("/dw", func(c *q.Context) {
			c.JSON(http.StatusOK, map[string]string{"a": "b"})
			c.Text(http.StatusConflict, "should not appear")
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/dw", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Header().Get("Content-Type")).To(ContainSubstring("application/json"))
		Expect(rr.Body.String()).NotTo(ContainSubstring("should not appear"))
	})

	It("prevents double-write: Text then JSON is silently ignored", func() {
		r := q.New()
		r.GET("/dw2", func(c *q.Context) {
			c.Text(http.StatusOK, "first")
			c.JSON(http.StatusConflict, map[string]string{"a": "b"})
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/dw2", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("first"))
	})

	It("prevents double-write: Bytes then Text is silently ignored", func() {
		r := q.New()
		r.GET("/dw3", func(c *q.Context) {
			c.Bytes(http.StatusOK, []byte("first"), "application/octet-stream")
			c.Text(http.StatusConflict, "should not appear")
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/dw3", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("first"))
	})

	It("reads query parameters", func() {
		r := q.New()
		r.GET("/search", func(c *q.Context) {
			c.Text(http.StatusOK, c.Query("q"))
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/search?q=hello", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("hello"))
	})

	It("reads request headers", func() {
		r := q.New()
		r.GET("/h", func(c *q.Context) {
			c.Text(http.StatusOK, c.Header("X-Custom"))
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/h", nil)
		req.Header.Set("X-Custom", "myval")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("myval"))
	})

	It("sets response headers", func() {
		r := q.New()
		r.GET("/sh", func(c *q.Context) {
			c.SetHeader("X-Custom", "resp")
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/sh", nil))
		Expect(rr.Header().Get("X-Custom")).To(Equal("resp"))
	})

	It("returns false for missing cookie", func() {
		r := q.New()
		r.GET("/c", func(c *q.Context) {
			_, ok := c.Cookie("missing")
			if !ok {
				c.Status(http.StatusNotFound)
				return
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/c", nil))
		Expect(rr.Code).To(Equal(http.StatusNotFound))
	})
})

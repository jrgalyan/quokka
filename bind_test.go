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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("BindQuery and BindForm", func() {
	type Params struct {
		Name   string  `query:"name" form:"name"`
		Page   int     `query:"page" form:"page"`
		Limit  int64   `query:"limit" form:"limit"`
		Score  float64 `query:"score" form:"score"`
		Active bool    `query:"active" form:"active"`
	}

	It("binds all supported types from query params", func() {
		r := q.New()
		r.GET("/search", func(c *q.Context) {
			var p Params
			if err := c.BindQuery(&p); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: err.Error()})
				return
			}
			c.Text(http.StatusOK, fmt.Sprintf("%s,%d,%d,%.1f,%t", p.Name, p.Page, p.Limit, p.Score, p.Active))
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/search?name=foo&page=2&limit=50&score=9.5&active=true", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("foo,2,50,9.5,true"))
	})

	It("skips fields without tags", func() {
		type NoTag struct {
			Name    string `query:"name"`
			Ignored string // no tag
		}
		r := q.New()
		r.GET("/", func(c *q.Context) {
			var p NoTag
			if err := c.BindQuery(&p); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: err.Error()})
				return
			}
			c.Text(http.StatusOK, fmt.Sprintf("%s|%s", p.Name, p.Ignored))
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?name=hello&Ignored=world", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("hello|"))
	})

	It("leaves zero values for missing params", func() {
		r := q.New()
		r.GET("/", func(c *q.Context) {
			var p Params
			_ = c.BindQuery(&p)
			c.Text(http.StatusOK, fmt.Sprintf("%s,%d,%d,%.1f,%t", p.Name, p.Page, p.Limit, p.Score, p.Active))
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?name=only", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("only,0,0,0.0,false"))
	})

	It("returns error for invalid int", func() {
		r := q.New()
		r.GET("/", func(c *q.Context) {
			var p Params
			if err := c.BindQuery(&p); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: err.Error()})
				return
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?page=abc", nil))
		Expect(rr.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns error for invalid bool", func() {
		r := q.New()
		r.GET("/", func(c *q.Context) {
			var p Params
			if err := c.BindQuery(&p); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: err.Error()})
				return
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?active=notabool", nil))
		Expect(rr.Code).To(Equal(http.StatusBadRequest))
	})

	It("returns error for non-struct destination", func() {
		r := q.New()
		r.GET("/", func(c *q.Context) {
			var s string
			if err := c.BindQuery(&s); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: err.Error()})
				return
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?x=1", nil))
		Expect(rr.Code).To(Equal(http.StatusBadRequest))
		Expect(rr.Body.String()).To(ContainSubstring("struct"))
	})

	It("binds form values via BindForm", func() {
		r := q.New()
		r.POST("/form", func(c *q.Context) {
			var p Params
			if err := c.BindForm(&p); err != nil {
				c.JSON(http.StatusBadRequest, q.ErrorResponse{Error: err.Error()})
				return
			}
			c.Text(http.StatusOK, fmt.Sprintf("%s,%d", p.Name, p.Page))
		})

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader("name=bar&page=3"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("bar,3"))
	})

	It("skips fields with dash tag", func() {
		type DashTag struct {
			Name    string `query:"name"`
			Skipped string `query:"-"`
		}
		r := q.New()
		r.GET("/", func(c *q.Context) {
			var p DashTag
			_ = c.BindQuery(&p)
			c.Text(http.StatusOK, fmt.Sprintf("%s|%s", p.Name, p.Skipped))
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/?name=hi&Skipped=no", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(rr.Body.String()).To(Equal("hi|"))
	})
})

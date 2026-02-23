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
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("Middleware", func() {
	It("Logger injects request id and logs", func() {
		r := q.New()
		// use default logger
		r.Use(q.Logger(q.LoggerConfig{}))
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

	It("Timeout cancels context for slow handlers", func() {
		r := q.New()
		r.Use(q.Timeout(20 * time.Millisecond))
		var cancelled bool
		r.GET("/slow", func(c *q.Context) {
			select {
			case <-c.Context().Done():
				cancelled = true
			case <-time.After(200 * time.Millisecond):
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/slow", nil))
		Expect(cancelled).To(BeTrue())
	})

	It("executes middleware in registration order", func() {
		r := q.New()
		order := []string{}

		r.Use(func(next q.Handler) q.Handler {
			return func(c *q.Context) {
				order = append(order, "first-before")
				next(c)
				order = append(order, "first-after")
			}
		})
		r.Use(func(next q.Handler) q.Handler {
			return func(c *q.Context) {
				order = append(order, "second-before")
				next(c)
				order = append(order, "second-after")
			}
		})
		r.Use(func(next q.Handler) q.Handler {
			return func(c *q.Context) {
				order = append(order, "third-before")
				next(c)
				order = append(order, "third-after")
			}
		})
		r.GET("/order", func(c *q.Context) {
			order = append(order, "handler")
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/order", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(order).To(Equal([]string{
			"first-before", "second-before", "third-before",
			"handler",
			"third-after", "second-after", "first-after",
		}))
	})

	It("Recover handles string panic", func() {
		r := q.New()
		r.Use(q.Recover(slog.Default()))
		r.GET("/p", func(c *q.Context) { panic("string panic") })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/p", nil))
		Expect(rr.Code).To(Equal(http.StatusInternalServerError))
		Expect(rr.Body.String()).To(ContainSubstring("internal server error"))
	})

	It("Recover handles error-type panic", func() {
		r := q.New()
		r.Use(q.Recover(slog.Default()))
		r.GET("/p", func(c *q.Context) { panic(errors.New("error panic")) })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/p", nil))
		Expect(rr.Code).To(Equal(http.StatusInternalServerError))
		Expect(rr.Body.String()).To(ContainSubstring("internal server error"))
	})

	It("Recover handles integer panic", func() {
		r := q.New()
		r.Use(q.Recover(slog.Default()))
		r.GET("/p", func(c *q.Context) { panic(42) })
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/p", nil))
		Expect(rr.Code).To(Equal(http.StatusInternalServerError))
		Expect(rr.Body.String()).To(ContainSubstring("internal server error"))
	})

	It("Logger generates request ID when not provided", func() {
		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{}))
		var seen string
		r.GET("/id", func(c *q.Context) {
			if v, ok := q.RequestID(c.Context()); ok {
				seen = v
			}
			c.Status(http.StatusOK)
		})

		rr := httptest.NewRecorder()
		// No X-Request-Id header
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/id", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(seen).NotTo(BeEmpty())
		Expect(len(seen)).To(Equal(32)) // hex-encoded 16 bytes
	})

	It("Logger logs status 200 when handler writes no explicit status", func() {
		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{}))
		r.GET("/implicit", func(c *q.Context) {
			// handler doesn't call any response method
		})

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/implicit", nil))
		// Go's default behavior writes 200 implicitly; no error expected
	})

	It("Logger writes to Output writer when Logger is nil", func() {
		var buf bytes.Buffer
		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{Output: &buf}))
		r.GET("/out", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/out", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(buf.String()).To(ContainSubstring("/out"))
	})

	It("Logger writes to multiple outputs via io.MultiWriter", func() {
		var console, file bytes.Buffer
		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{Output: io.MultiWriter(&console, &file)}))
		r.GET("/multi", func(c *q.Context) { c.Status(http.StatusNoContent) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/multi", nil))
		Expect(console.String()).To(ContainSubstring("/multi"))
		Expect(file.String()).To(Equal(console.String()))
	})

	It("Logger explicit Logger field takes precedence over Output", func() {
		var output bytes.Buffer
		var explicit bytes.Buffer
		explicitLogger := slog.New(slog.NewTextHandler(&explicit, nil))

		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{Logger: explicitLogger, Output: &output}))
		r.GET("/prec", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/prec", nil))
		Expect(explicit.String()).To(ContainSubstring("/prec"))
		Expect(output.String()).To(BeEmpty())
	})

	It("OpenLogFile creates a writable log file", func() {
		path := filepath.Join(GinkgoT().TempDir(), "test.log")
		f, err := q.OpenLogFile(path)
		Expect(err).NotTo(HaveOccurred())
		defer f.Close()

		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{Output: f}))
		r.GET("/file", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/file", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))

		Expect(f.Sync()).To(Succeed())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("/file"))
	})

	It("OpenLogFile creates missing parent directories", func() {
		path := filepath.Join(GinkgoT().TempDir(), "a", "b", "c", "app.log")
		f, err := q.OpenLogFile(path)
		Expect(err).NotTo(HaveOccurred())
		f.Close()
		_, err = os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Logger Dir writes access.log in the given directory", func() {
		dir := GinkgoT().TempDir()
		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{Dir: dir}))
		r.GET("/dirlog", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/dirlog", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))

		data, err := os.ReadFile(filepath.Join(dir, "access.log"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring("/dirlog"))
	})

	It("Logger Dir creates the directory if it does not exist", func() {
		dir := filepath.Join(GinkgoT().TempDir(), "logs", "app")
		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{Dir: dir}))
		r.GET("/mkdir", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/mkdir", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))

		_, err := os.Stat(filepath.Join(dir, "access.log"))
		Expect(err).NotTo(HaveOccurred())
	})

	It("Logger Output takes precedence over Dir", func() {
		var buf bytes.Buffer
		dir := GinkgoT().TempDir()
		r := q.New()
		r.Use(q.Logger(q.LoggerConfig{Output: &buf, Dir: dir}))
		r.GET("/prec2", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/prec2", nil))
		Expect(buf.String()).To(ContainSubstring("/prec2"))
		_, err := os.Stat(filepath.Join(dir, "access.log"))
		Expect(err).To(HaveOccurred()) // file not created
	})
})

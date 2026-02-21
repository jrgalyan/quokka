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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("RateLimit", func() {
	handler := func(c *q.Context) { c.Text(http.StatusOK, "ok") }

	It("allows requests within the rate limit", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{Rate: 100, Burst: 10}))
		r.GET("/", handler)

		for i := 0; i < 10; i++ {
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
			Expect(rr.Code).To(Equal(http.StatusOK))
		}
	})

	It("returns 429 when burst is exceeded", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{Rate: 1, Burst: 2}))
		r.GET("/", handler)

		// First 2 should succeed (burst)
		for i := 0; i < 2; i++ {
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
			Expect(rr.Code).To(Equal(http.StatusOK))
		}

		// Third should be rate limited
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusTooManyRequests))
	})

	It("includes Retry-After header on 429", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{Rate: 1, Burst: 1}))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))

		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusTooManyRequests))
		ra := rr.Header().Get("Retry-After")
		Expect(ra).NotTo(BeEmpty())
		seconds, err := strconv.Atoi(ra)
		Expect(err).NotTo(HaveOccurred())
		Expect(seconds).To(BeNumerically(">=", 1))
	})

	It("returns JSON error body on 429", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{Rate: 1, Burst: 1}))
		r.GET("/", handler)

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusTooManyRequests))
		var errResp q.ErrorResponse
		Expect(json.Unmarshal(rr.Body.Bytes(), &errResp)).To(Succeed())
		Expect(errResp.Error).To(Equal("rate limit exceeded"))
	})

	It("tracks clients independently", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{Rate: 1, Burst: 1}))
		r.GET("/", handler)

		// Client A exhausts its limit
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))

		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusTooManyRequests))

		// Client B should still be allowed
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.6.7.8:5678"
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
	})

	It("refills tokens over time", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{Rate: 100, Burst: 1}))
		r.GET("/", handler)

		// Exhaust the single token
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))

		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusTooManyRequests))

		// Wait for tokens to refill (rate=100/s, so 20ms should give ~2 tokens)
		time.Sleep(50 * time.Millisecond)

		rr = httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
	})

	It("uses X-Forwarded-For for client identification", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{Rate: 1, Burst: 1}))
		r.GET("/", handler)

		// Request with X-Forwarded-For
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))

		// Same X-Forwarded-For should be rate limited
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusTooManyRequests))
	})

	It("supports custom KeyFunc", func() {
		r := q.New()
		r.Use(q.RateLimit(q.RateLimitConfig{
			Rate:  1,
			Burst: 1,
			KeyFunc: func(c *q.Context) string {
				return c.R.Header.Get("X-API-Key")
			},
		}))
		r.GET("/", handler)

		// Key "a" uses its token
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "a")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))

		// Key "a" is rate limited
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "a")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusTooManyRequests))

		// Key "b" is independent
		rr = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-API-Key", "b")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
	})
})

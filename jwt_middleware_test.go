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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	jwt "github.com/golang-jwt/jwt/v5"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("JWT Middleware", func() {
	secret := []byte("testsecret")
	keyfunc := func(token *jwt.Token) (interface{}, error) { return secret, nil }

	It("accepts valid HS256 token and exposes claims", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc, Issuer: "quokka"}))
		var sub string
		r.GET("/me", func(c *q.Context) {
			if claims, ok := q.JWTClaims(c.Context()); ok {
				if v, ok2 := claims["sub"].(string); ok2 {
					sub = v
				}
			}
			c.Status(http.StatusOK)
		})

		// create token
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": "quokka",
			"sub": "user1",
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(secret)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)

		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(sub).To(Equal("user1"))
	})

	It("rejects missing/invalid token with 401 and WWW-Authenticate", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc}))
		r.GET("/p", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/p", nil))
		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
		Expect(rr.Header().Get("WWW-Authenticate")).To(ContainSubstring("Bearer"))
		Expect(rr.Body.String()).To(ContainSubstring("unauthorized"))
	})

	It("allows optional mode to pass through without token", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc, Optional: true}))
		r.GET("/open", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/open", nil))
		Expect(rr.Code).To(Equal(http.StatusOK))
	})

	It("rejects expired token", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user1",
			"exp": time.Now().Add(-1 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(secret)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
	})
})

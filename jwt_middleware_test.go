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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
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

	It("accepts valid RSA-signed token", func() {
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).To(BeNil())

		rsaKeyfunc := func(token *jwt.Token) (interface{}, error) { return &rsaKey.PublicKey, nil }

		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: rsaKeyfunc}))
		var sub string
		r.GET("/me", func(c *q.Context) {
			if claims, ok := q.JWTClaims(c.Context()); ok {
				if v, ok2 := claims["sub"].(string); ok2 {
					sub = v
				}
			}
			c.Status(http.StatusOK)
		})

		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": "rsa-user",
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(rsaKey)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(sub).To(Equal("rsa-user"))
	})

	It("accepts valid ECDSA-signed token", func() {
		ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		Expect(err).To(BeNil())

		ecKeyfunc := func(token *jwt.Token) (interface{}, error) { return &ecKey.PublicKey, nil }

		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: ecKeyfunc}))
		var sub string
		r.GET("/me", func(c *q.Context) {
			if claims, ok := q.JWTClaims(c.Context()); ok {
				if v, ok2 := claims["sub"].(string); ok2 {
					sub = v
				}
			}
			c.Status(http.StatusOK)
		})

		tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
			"sub": "ec-user",
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(ecKey)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(sub).To(Equal("ec-user"))
	})

	It("accepts valid EdDSA-signed token", func() {
		_, edKey, err := ed25519.GenerateKey(rand.Reader)
		Expect(err).To(BeNil())

		edKeyfunc := func(token *jwt.Token) (interface{}, error) { return edKey.Public(), nil }

		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: edKeyfunc}))
		var sub string
		r.GET("/me", func(c *q.Context) {
			if claims, ok := q.JWTClaims(c.Context()); ok {
				if v, ok2 := claims["sub"].(string); ok2 {
					sub = v
				}
			}
			c.Status(http.StatusOK)
		})

		tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
			"sub": "ed-user",
			"iat": time.Now().Unix(),
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(edKey)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
		Expect(sub).To(Equal("ed-user"))
	})

	It("rejects tampered token", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user1",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(secret)
		Expect(err).To(BeNil())

		// Tamper with the payload by flipping a character
		parts := strings.SplitN(s, ".", 3)
		tampered := parts[0] + "." + parts[1] + "X" + "." + parts[2]

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+tampered)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects token signed with wrong key", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		wrongSecret := []byte("wrong-secret")
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user1",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(wrongSecret)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
	})

	It("rejects non-Bearer authorization scheme", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
	})

	It("validates issuer when configured", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc, Issuer: "trusted-issuer"}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		// Token with wrong issuer
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": "wrong-issuer",
			"sub": "user1",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(secret)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
	})

	It("validates audience when configured", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc, Audience: "my-api"}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		// Token with wrong audience
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"aud": "other-api",
			"sub": "user1",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(secret)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusUnauthorized))
	})

	It("respects clock skew tolerance", func() {
		r := q.New()
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc, Skew: 2 * time.Minute}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		// Token expired 1 minute ago, but skew is 2 minutes — should pass
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
		Expect(rr.Code).To(Equal(http.StatusOK))
	})

	It("skips issuer/audience validation when not configured", func() {
		r := q.New()
		// No Issuer/Audience set — should not validate them
		r.Use(q.JWTAuth(q.JWTConfig{Keyfunc: keyfunc}))
		r.GET("/t", func(c *q.Context) { c.Status(http.StatusOK) })

		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user1",
			"iss": "any-issuer",
			"aud": "any-audience",
			"exp": time.Now().Add(5 * time.Minute).Unix(),
		})
		s, err := tok.SignedString(secret)
		Expect(err).To(BeNil())

		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+s)
		r.ServeHTTP(rr, req)
		Expect(rr.Code).To(Equal(http.StatusOK))
	})
})

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

package quokka

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

// jwtClaimsKey is the context key for JWT claims storage.
type jwtClaimsKey struct{}

var jwtContextKey = jwtClaimsKey{}

// WithJWTClaims stores JWT claims into a context.
func WithJWTClaims(ctx context.Context, claims jwt.MapClaims) context.Context {
	return context.WithValue(ctx, jwtContextKey, claims)
}

// JWTClaims retrieves JWT claims from context if present.
func JWTClaims(ctx context.Context) (jwt.MapClaims, bool) {
	v := ctx.Value(jwtContextKey)
	if v == nil {
		return nil, false
	}
	mc, ok := v.(jwt.MapClaims)
	return mc, ok
}

// JWTConfig configures the JWT middleware.
// Provide at least a Keyfunc to resolve the verification key.
// Optional fields can enforce issuer/audience and clock skew.
// If Optional is true, requests without Authorization header pass through unmodified.
// Only Bearer tokens are considered.
// Errors result in 401 with WWW-Authenticate and JSON error payload.
// Note: This middleware does not perform authorization beyond claim validation.
type JWTConfig struct {
	Keyfunc  jwt.Keyfunc
	Issuer   string
	Audience string
	Skew     time.Duration
	Optional bool
}

// JWTAuth creates a middleware that validates Bearer JWTs and injects claims into the request context.
func JWTAuth(cfg JWTConfig) Middleware {
	if cfg.Skew == 0 {
		cfg.Skew = 30 * time.Second
	}
	return func(next Handler) Handler {
		return func(c *Context) {
			authz := c.R.Header.Get("Authorization")
			if authz == "" {
				if cfg.Optional {
					next(c)
					return
				}
				unauthorized(c, "missing Authorization header")
				return
			}
			parts := strings.SplitN(authz, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				unauthorized(c, "invalid Authorization scheme")
				return
			}
			tokStr := parts[1]

			opts := []jwt.ParserOption{
				jwt.WithValidMethods([]string{"HS256", "HS384", "HS512", "RS256", "RS384", "RS512", "ES256", "EdDSA"}),
				jwt.WithLeeway(cfg.Skew),
			}
			if cfg.Issuer != "" {
				opts = append(opts, jwt.WithIssuer(cfg.Issuer))
			}
			if cfg.Audience != "" {
				opts = append(opts, jwt.WithAudience(cfg.Audience))
			}
			parser := jwt.NewParser(opts...)

			var claims jwt.MapClaims
			tok, err := parser.ParseWithClaims(tokStr, jwt.MapClaims{}, cfg.Keyfunc)
			if err != nil {
				unauthorized(c, fmt.Sprintf("token parse/verify failed: %v", err))
				return
			}
			var ok bool
			claims, ok = tok.Claims.(jwt.MapClaims)
			if !ok || !tok.Valid {
				unauthorized(c, "invalid token claims")
				return
			}

			// store claims in context and proceed
			c.R = c.R.WithContext(WithJWTClaims(c.R.Context(), claims))
			next(c)
		}
	}
}

func unauthorized(c *Context, desc string) {
	c.W.Header().Set("WWW-Authenticate", "Bearer error=\"invalid_token\", error_description=\""+escapeAuthParam(desc)+"\"")
	c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "unauthorized", Message: desc})
}

// escapeAuthParam per RFC 6750 to safely include in WWW-Authenticate param
func escapeAuthParam(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

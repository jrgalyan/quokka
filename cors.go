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
	"net/http"
	"strconv"
	"strings"
)

// CORSConfig configures the CORS middleware.
type CORSConfig struct {
	// AllowOrigins is the list of origins permitted to make cross-origin requests.
	// Use ["*"] to allow all origins. Default: ["*"].
	AllowOrigins []string

	// AllowMethods is the list of HTTP methods allowed for cross-origin requests.
	// Default: GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS.
	AllowMethods []string

	// AllowHeaders is the list of request headers allowed in cross-origin requests.
	// Default: Origin, Content-Type, Accept, Authorization, X-Request-Id.
	AllowHeaders []string

	// ExposeHeaders is the list of response headers that browsers are allowed to access.
	// Default: empty (browser defaults apply).
	ExposeHeaders []string

	// MaxAge is the duration in seconds that preflight responses can be cached.
	// Default: 86400 (24 hours).
	MaxAge int

	// AllowCredentials indicates whether the request can include credentials
	// (cookies, HTTP authentication, client-side SSL certificates).
	// When true and AllowOrigins contains "*", the middleware reflects the
	// actual request Origin instead of emitting "*" (per the CORS spec).
	AllowCredentials bool
}

// DefaultCORSConfig returns a CORSConfig with sensible defaults.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
			http.MethodHead,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			"X-Request-Id",
		},
		ExposeHeaders:    []string{},
		MaxAge:           86400,
		AllowCredentials: false,
	}
}

// CORS creates a middleware that handles Cross-Origin Resource Sharing.
// It supports preflight requests, configurable origins, credentials, and header exposure.
func CORS(cfg CORSConfig) Middleware {
	allowMethodsStr := strings.Join(cfg.AllowMethods, ", ")
	allowHeadersStr := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeadersStr := strings.Join(cfg.ExposeHeaders, ", ")
	maxAgeStr := strconv.Itoa(cfg.MaxAge)
	allowAll := len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*"

	return func(next Handler) Handler {
		return func(c *Context) {
			origin := c.R.Header.Get("Origin")
			if origin == "" {
				next(c)
				return
			}

			if !allowAll && !originAllowed(origin, cfg.AllowOrigins) {
				next(c)
				return
			}

			allowOriginValue := "*"
			if cfg.AllowCredentials || !allowAll {
				allowOriginValue = origin
			}

			// Preflight request
			if c.R.Method == http.MethodOptions &&
				c.R.Header.Get("Access-Control-Request-Method") != "" {
				h := c.W.Header()
				h.Set("Access-Control-Allow-Origin", allowOriginValue)
				h.Set("Access-Control-Allow-Methods", allowMethodsStr)
				h.Set("Access-Control-Allow-Headers", allowHeadersStr)
				if cfg.MaxAge > 0 {
					h.Set("Access-Control-Max-Age", maxAgeStr)
				}
				if cfg.AllowCredentials {
					h.Set("Access-Control-Allow-Credentials", "true")
				}
				h.Set("Vary", "Origin, Access-Control-Request-Method, Access-Control-Request-Headers")
				c.Status(http.StatusNoContent)
				return
			}

			// Actual request
			h := c.W.Header()
			h.Set("Access-Control-Allow-Origin", allowOriginValue)
			if cfg.AllowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
			if exposeHeadersStr != "" {
				h.Set("Access-Control-Expose-Headers", exposeHeadersStr)
			}
			h.Add("Vary", "Origin")
			next(c)
		}
	}
}

func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
	}
	return false
}

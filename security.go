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
	"fmt"
)

// SecurityHeadersConfig configures the SecurityHeaders middleware.
type SecurityHeadersConfig struct {
	// HSTSMaxAge is the max-age value for Strict-Transport-Security in seconds.
	// Set to 0 to omit the HSTS header entirely. Default: 63072000 (2 years).
	HSTSMaxAge int

	// HSTSIncludeSubdomains adds includeSubDomains to the HSTS header.
	// Default: true.
	HSTSIncludeSubdomains bool

	// HSTSPreload adds the preload directive to the HSTS header.
	// Default: false.
	HSTSPreload bool

	// ContentTypeNosniff sets X-Content-Type-Options: nosniff.
	// Default: true.
	ContentTypeNosniff bool

	// FrameOption sets the X-Frame-Options header value (e.g. "DENY",
	// "SAMEORIGIN"). Empty string omits the header. Default: "DENY".
	FrameOption string

	// ReferrerPolicy sets the Referrer-Policy header value.
	// Empty string omits the header. Default: "strict-origin-when-cross-origin".
	ReferrerPolicy string
}

// DefaultSecurityHeadersConfig returns a SecurityHeadersConfig with sensible
// production defaults.
func DefaultSecurityHeadersConfig() SecurityHeadersConfig {
	return SecurityHeadersConfig{
		HSTSMaxAge:            63072000, // 2 years
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
		ContentTypeNosniff:    true,
		FrameOption:           "DENY",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
	}
}

// SecurityHeaders creates a middleware that sets common security-related HTTP
// response headers such as HSTS, X-Content-Type-Options, X-Frame-Options, and
// Referrer-Policy.
func SecurityHeaders(cfg SecurityHeadersConfig) Middleware {
	// Pre-compute the HSTS header value so we don't build it per-request.
	var hstsValue string
	if cfg.HSTSMaxAge > 0 {
		hstsValue = fmt.Sprintf("max-age=%d", cfg.HSTSMaxAge)
		if cfg.HSTSIncludeSubdomains {
			hstsValue += "; includeSubDomains"
		}
		if cfg.HSTSPreload {
			hstsValue += "; preload"
		}
	}

	return func(next Handler) Handler {
		return func(c *Context) {
			h := c.W.Header()
			if hstsValue != "" {
				h.Set("Strict-Transport-Security", hstsValue)
			}
			if cfg.ContentTypeNosniff {
				h.Set("X-Content-Type-Options", "nosniff")
			}
			if cfg.FrameOption != "" {
				h.Set("X-Frame-Options", cfg.FrameOption)
			}
			if cfg.ReferrerPolicy != "" {
				h.Set("Referrer-Policy", cfg.ReferrerPolicy)
			}
			next(c)
		}
	}
}

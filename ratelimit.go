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
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimitConfig configures the RateLimit middleware.
type RateLimitConfig struct {
	// Rate is the sustained requests per second allowed per client key.
	// Default: 10.
	Rate float64

	// Burst is the maximum number of requests allowed in a single burst.
	// Must be >= 1. Default: 20.
	Burst int

	// CleanupInterval is how often stale entries are removed from the map.
	// Default: 1 minute.
	CleanupInterval time.Duration

	// StaleAfter is the duration after which an idle client entry is removed.
	// Default: 5 minutes.
	StaleAfter time.Duration

	// KeyFunc extracts a client key from the request. When nil, the default
	// uses the first IP in X-Forwarded-For, falling back to RemoteAddr.
	KeyFunc func(*Context) string
}

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimit creates a middleware that enforces per-client rate limiting using a
// token bucket algorithm. When the limit is exceeded a 429 Too Many Requests
// response is returned with a Retry-After header.
func RateLimit(cfg RateLimitConfig) Middleware {
	if cfg.Rate <= 0 {
		cfg.Rate = 10
	}
	if cfg.Burst < 1 {
		cfg.Burst = 20
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = time.Minute
	}
	if cfg.StaleAfter <= 0 {
		cfg.StaleAfter = 5 * time.Minute
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = defaultKeyFunc
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*bucket)
	)

	// Background goroutine to remove stale entries.
	go func() {
		ticker := time.NewTicker(cfg.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for k, b := range clients {
				if now.Sub(b.lastSeen) > cfg.StaleAfter {
					delete(clients, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next Handler) Handler {
		return func(c *Context) {
			key := cfg.KeyFunc(c)
			now := time.Now()

			mu.Lock()
			b, ok := clients[key]
			if !ok {
				b = &bucket{tokens: float64(cfg.Burst), lastSeen: now}
				clients[key] = b
			}

			// Refill tokens based on elapsed time.
			elapsed := now.Sub(b.lastSeen).Seconds()
			b.tokens += elapsed * cfg.Rate
			if b.tokens > float64(cfg.Burst) {
				b.tokens = float64(cfg.Burst)
			}
			b.lastSeen = now

			if b.tokens < 1 {
				retryAfter := int(math.Ceil((1 - b.tokens) / cfg.Rate))
				mu.Unlock()
				c.SetHeader("Retry-After", strconv.Itoa(retryAfter))
				c.JSON(http.StatusTooManyRequests, ErrorResponse{Error: "rate limit exceeded"})
				return
			}

			b.tokens--
			mu.Unlock()
			next(c)
		}
	}
}

func defaultKeyFunc(c *Context) string {
	if xff := c.R.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the first (client) IP from the chain.
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(c.R.RemoteAddr)
	if err != nil {
		return c.R.RemoteAddr
	}
	return host
}

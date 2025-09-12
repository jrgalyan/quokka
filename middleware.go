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
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b)
}

// chain composes middlewares around a final handler
func chain(mw []Middleware, h Handler) Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// Logger provides structured access logging with request id
func Logger(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next Handler) Handler {
		return func(c *Context) {
			id := c.R.Header.Get("X-Request-Id")
			if id == "" {
				id = randomID()
			}
			c.R = c.R.WithContext(WithRequestID(c.R.Context(), id))
			start := time.Now()
			next(c)
			dur := time.Since(start)
			logger.Info("request",
				slog.String("id", id),
				slog.String("method", c.R.Method),
				slog.String("path", c.R.URL.Path),
				slog.Int("status", c.status),
				slog.String("duration", dur.String()),
			)
		}
	}
}

// Recover gracefully handles panics and returns 500
func Recover(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next Handler) Handler {
		return func(c *Context) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("panic recovered", slog.Any("err", r), slog.String("stack", string(debug.Stack())))
					c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "internal server error"})
				}
			}()
			next(c)
		}
	}
}

// Timeout aborts long-running requests
func Timeout(d time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(c *Context) {
			if d > 0 {
				ctx, cancel := context.WithTimeout(c.R.Context(), d)
				defer cancel()
				c.R = c.R.WithContext(ctx)
			}
			next(c)
		}
	}
}

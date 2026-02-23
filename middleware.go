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
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
	"time"
)

var idCounter uint64

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", time.Now().UTC().Format("20060102150405.000000000"), atomic.AddUint64(&idCounter, 1))
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

// LoggerConfig configures the Logger middleware.
type LoggerConfig struct {
	// Logger is the slog.Logger used for output. When set, Output is ignored.
	// nil falls through to Output, then slog.Default().
	Logger *slog.Logger

	// Output directs log lines to this writer when Logger is nil.
	// Use os.Stderr for console, an *os.File for a log file, or
	// io.MultiWriter(os.Stderr, f) to write to both simultaneously.
	Output io.Writer

	// Dir is the directory in which to write "access.log" when Logger and
	// Output are both nil. The directory (and any parents) is created
	// automatically with mode 0750 if it does not exist.
	Dir string

	// Sanitize enables redaction of sensitive path parameters, query parameters,
	// and headers in log output. nil means no sanitization.
	Sanitize *SanitizeConfig
}

// Logger provides structured access logging with request id.
func Logger(cfg LoggerConfig) Middleware {
	logger := cfg.Logger
	if logger == nil {
		switch {
		case cfg.Output != nil:
			logger = slog.New(slog.NewTextHandler(cfg.Output, nil))
		case cfg.Dir != "":
			path := filepath.Join(cfg.Dir, "access.log")
			f, err := OpenLogFile(path)
			if err != nil {
				panic("quokka: cannot open log file " + path + ": " + err.Error())
			}
			logger = slog.New(slog.NewTextHandler(f, nil))
		default:
			logger = slog.Default()
		}
	}

	var san *Sanitizer
	if cfg.Sanitize != nil {
		san = NewSanitizer(*cfg.Sanitize)
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
			status := c.status
			if status == 0 {
				status = http.StatusOK
			}
			logPath := san.Path(c.R.URL.Path, c.params)
			logger.Info("request",
				slog.String("id", id),
				slog.String("method", c.R.Method),
				slog.String("path", logPath),
				slog.Int("status", status),
				slog.String("duration", dur.String()),
			)
		}
	}
}

// OpenLogFile opens or creates a file for appending structured log output.
// The caller is responsible for closing the file when the server shuts down.
//
// Example (file + console):
//
//	f, err := quokka.OpenLogFile("/var/log/app.log")
//	if err != nil { ... }
//	defer f.Close()
//	r.Use(quokka.Logger(quokka.LoggerConfig{
//	    Output: io.MultiWriter(os.Stderr, f),
//	}))
func OpenLogFile(path string) (*os.File, error) {
	safePath := filepath.Clean(path)
	if dir := filepath.Dir(safePath); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(safePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
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

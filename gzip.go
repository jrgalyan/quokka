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
	"compress/gzip"
	"net/http"
	"strings"
)

// GzipConfig configures the Gzip compression middleware.
type GzipConfig struct {
	// Level is the gzip compression level (1-9, or gzip.DefaultCompression,
	// gzip.BestSpeed, gzip.BestCompression). Default: gzip.DefaultCompression.
	Level int

	// MinLength is the minimum response body size in bytes before compression
	// is applied. Responses smaller than this are sent uncompressed.
	// Default: 256.
	MinLength int
}

// Content types that are already compressed and should not be gzip-compressed.
var skippedContentTypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
	"image/avif",
	"video/",
	"audio/",
	"application/zip",
	"application/gzip",
	"application/x-gzip",
	"application/x-compressed",
	"application/x-bzip2",
	"application/x-xz",
	"application/zstd",
	"application/wasm",
}

func shouldSkipContentType(ct string) bool {
	ct = strings.ToLower(ct)
	for _, skip := range skippedContentTypes {
		if strings.HasPrefix(ct, skip) {
			return true
		}
	}
	return false
}

// gzipResponseWriter wraps http.ResponseWriter to transparently compress responses.
// It buffers writes until MinLength is reached, then decides whether to compress.
type gzipResponseWriter struct {
	http.ResponseWriter
	gw            *gzip.Writer
	buf           []byte
	minLength     int
	level         int
	decided       bool
	compressing   bool
	statusCode    int
	headerWritten bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	// For status codes that indicate no body, forward immediately
	if code == http.StatusNoContent || code == http.StatusNotModified || (code >= 100 && code < 200) {
		w.decided = true
		w.compressing = false
		w.ResponseWriter.WriteHeader(code)
		w.headerWritten = true
	}
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.decided {
		w.buf = append(w.buf, b...)
		if len(w.buf) >= w.minLength {
			w.decide()
			return len(b), w.flush()
		}
		return len(b), nil
	}
	if w.compressing {
		return w.gw.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gzipResponseWriter) decide() {
	w.decided = true
	ct := w.ResponseWriter.Header().Get("Content-Type")
	if shouldSkipContentType(ct) {
		w.compressing = false
		return
	}
	w.compressing = true
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.Header().Set("Content-Encoding", "gzip")
	var err error
	w.gw, err = gzip.NewWriterLevel(w.ResponseWriter, w.level)
	if err != nil {
		// Fallback to default compression on invalid level
		w.gw = gzip.NewWriter(w.ResponseWriter)
	}
}

func (w *gzipResponseWriter) flush() error {
	if !w.headerWritten && w.statusCode != 0 {
		w.ResponseWriter.WriteHeader(w.statusCode)
		w.headerWritten = true
	}
	if len(w.buf) == 0 {
		return nil
	}
	if w.compressing && w.gw != nil {
		_, err := w.gw.Write(w.buf)
		w.buf = nil
		return err
	}
	_, err := w.ResponseWriter.Write(w.buf)
	w.buf = nil
	return err
}

func (w *gzipResponseWriter) close() error {
	if !w.decided {
		// Response was smaller than minLength — send uncompressed
		w.decided = true
		w.compressing = false
	}
	if !w.headerWritten && w.statusCode != 0 {
		w.ResponseWriter.WriteHeader(w.statusCode)
		w.headerWritten = true
	}
	if len(w.buf) > 0 {
		_, _ = w.ResponseWriter.Write(w.buf)
		w.buf = nil
	}
	if w.compressing && w.gw != nil {
		return w.gw.Close()
	}
	return nil
}

// Flush implements http.Flusher for streaming compatibility.
func (w *gzipResponseWriter) Flush() {
	if w.compressing && w.gw != nil {
		_ = w.gw.Flush()
	}
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Gzip creates a middleware that compresses responses using gzip encoding.
// Responses smaller than MinLength bytes are sent uncompressed.
// Already-compressed content types (images, archives) are skipped.
func Gzip(cfg GzipConfig) Middleware {
	if cfg.Level == 0 {
		cfg.Level = gzip.DefaultCompression
	}
	if cfg.MinLength <= 0 {
		cfg.MinLength = 256
	}

	return func(next Handler) Handler {
		return func(c *Context) {
			if !strings.Contains(c.R.Header.Get("Accept-Encoding"), "gzip") {
				next(c)
				return
			}

			c.W.Header().Add("Vary", "Accept-Encoding")

			grw := &gzipResponseWriter{
				ResponseWriter: c.W,
				minLength:      cfg.MinLength,
				level:          cfg.Level,
			}

			original := c.W
			c.W = grw
			defer func() {
				_ = grw.close()
				c.W = original
			}()

			next(c)
		}
	}
}

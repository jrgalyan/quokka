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
	"strings"
)

// SanitizeConfig configures the Sanitizer.
type SanitizeConfig struct {
	// Params is the list of path parameter names to redact (without ":" prefix).
	Params []string

	// QueryParams is the list of query parameter names to redact.
	QueryParams []string

	// Headers is the list of header names to redact (case-insensitive).
	Headers []string

	// Mask is the replacement string for redacted values. Default: "***".
	Mask string
}

// DefaultSanitizeConfig returns a SanitizeConfig with sensible defaults.
// All redaction lists are empty (no-op) and the mask is "***".
func DefaultSanitizeConfig() SanitizeConfig {
	return SanitizeConfig{
		Params:      []string{},
		QueryParams: []string{},
		Headers:     []string{},
		Mask:        "***",
	}
}

// Sanitizer provides reusable sanitization of request fields for logging.
// Create once via NewSanitizer and reuse across requests. Methods on a nil
// *Sanitizer return inputs unchanged, so callers can skip a nil check.
type Sanitizer struct {
	mask      string
	paramSet  map[string]struct{}
	querySet  map[string]struct{}
	headerSet map[string]struct{} // canonicalized keys
}

// NewSanitizer creates a Sanitizer from the given config. It returns nil if
// all redaction lists are empty (no work to do).
func NewSanitizer(cfg SanitizeConfig) *Sanitizer {
	paramSet := toSet(cfg.Params)
	querySet := toSet(cfg.QueryParams)
	headerSet := make(map[string]struct{}, len(cfg.Headers))
	for _, h := range cfg.Headers {
		headerSet[http.CanonicalHeaderKey(h)] = struct{}{}
	}

	if len(paramSet) == 0 && len(querySet) == 0 && len(headerSet) == 0 {
		return nil
	}

	mask := cfg.Mask
	if mask == "" {
		mask = "***"
	}

	return &Sanitizer{
		mask:      mask,
		paramSet:  paramSet,
		querySet:  querySet,
		headerSet: headerSet,
	}
}

// Path returns the request path with redacted parameter values. Segments whose
// values match a configured param name are replaced with the mask. If s is nil,
// the original path is returned unchanged.
func (s *Sanitizer) Path(path string, params map[string]string) string {
	if s == nil || len(s.paramSet) == 0 {
		return path
	}

	redactValues := make(map[string]struct{})
	for name := range s.paramSet {
		if v, ok := params[name]; ok && v != "" {
			redactValues[v] = struct{}{}
		}
	}
	if len(redactValues) == 0 {
		return path
	}

	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if _, found := redactValues[seg]; found {
			segments[i] = s.mask
		}
	}
	return strings.Join(segments, "/")
}

// Query returns the raw query string with redacted values for configured query
// parameter names. If s is nil or there are no query params to redact, the
// original query string is returned unchanged.
func (s *Sanitizer) Query(rawQuery string) string {
	if s == nil || len(s.querySet) == 0 || rawQuery == "" {
		return rawQuery
	}

	q := parseQuery(rawQuery)
	changed := false
	for key := range s.querySet {
		if vals, ok := q[key]; ok {
			for i := range vals {
				vals[i] = s.mask
			}
			changed = true
		}
	}
	if !changed {
		return rawQuery
	}
	return encodeQuery(q)
}

// Headers returns a clone of the provided headers with redacted values for
// configured header names. If s is nil or there are no headers to redact,
// nil is returned.
func (s *Sanitizer) Headers(h http.Header) http.Header {
	if s == nil || len(s.headerSet) == 0 {
		return nil
	}

	clone := h.Clone()
	for key := range s.headerSet {
		if vals := clone[key]; len(vals) > 0 {
			for i := range vals {
				vals[i] = s.mask
			}
		}
	}
	return clone
}

func toSet(items []string) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}

// parseQuery is a minimal query parser that avoids net/url import overhead.
func parseQuery(rawQuery string) map[string][]string {
	m := make(map[string][]string)
	for rawQuery != "" {
		var key string
		key, rawQuery, _ = strings.Cut(rawQuery, "&")
		if key == "" {
			continue
		}
		k, v, _ := strings.Cut(key, "=")
		m[k] = append(m[k], v)
	}
	return m
}

// encodeQuery re-encodes parsed query params back to a raw query string.
func encodeQuery(m map[string][]string) string {
	var buf strings.Builder
	for k, vals := range m {
		for _, v := range vals {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(k)
			buf.WriteByte('=')
			buf.WriteString(v)
		}
	}
	return buf.String()
}

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

import "net/http"

// BodyLimit creates a middleware that restricts the maximum size of the request
// body. If the client sends more than maxBytes, subsequent reads from the body
// will return an error and the handler is responsible for returning an
// appropriate status (typically 413 Request Entity Too Large).
//
// A maxBytes of 0 or negative means no limit is enforced.
func BodyLimit(maxBytes int64) Middleware {
	return func(next Handler) Handler {
		return func(c *Context) {
			if maxBytes > 0 {
				c.R.Body = http.MaxBytesReader(c.W, c.R.Body, maxBytes)
			}
			next(c)
		}
	}
}

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
	"crypto/tls"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	q "github.com/jrgalyan/quokka"
)

var _ = Describe("Server", func() {
	It("applies defaults when zero values provided", func() {
		r := http.NewServeMux()
		s := q.NewServer(q.ServerConfig{}, r, nil)
		Expect(s.HTTP.Addr).To(Equal(":8080"))
		Expect(s.HTTP.ReadTimeout).To(Equal(15 * time.Second))
		Expect(s.HTTP.WriteTimeout).To(Equal(30 * time.Second))
		Expect(s.HTTP.IdleTimeout).To(Equal(120 * time.Second))
		Expect(s.HTTP.TLSConfig).To(BeNil())
	})

	It("uses provided TLS config when set", func() {
		r := http.NewServeMux()
		cfg := &tls.Config{MinVersion: tls.VersionTLS12}
		s := q.NewServer(q.ServerConfig{Addr: ":0", TLSConfig: cfg}, r, nil)
		Expect(s.HTTP.TLSConfig).To(Equal(cfg))
	})
})

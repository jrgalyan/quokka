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

	It("applies ReadHeaderTimeout default of 5 seconds", func() {
		r := http.NewServeMux()
		s := q.NewServer(q.ServerConfig{}, r, nil)
		Expect(s.HTTP.ReadHeaderTimeout).To(Equal(5 * time.Second))
	})

	It("uses custom ReadHeaderTimeout when provided", func() {
		r := http.NewServeMux()
		s := q.NewServer(q.ServerConfig{ReadHeaderTimeout: 10 * time.Second}, r, nil)
		Expect(s.HTTP.ReadHeaderTimeout).To(Equal(10 * time.Second))
	})

	It("uses custom timeouts when provided", func() {
		r := http.NewServeMux()
		s := q.NewServer(q.ServerConfig{
			Addr:         ":9090",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		}, r, nil)
		Expect(s.HTTP.Addr).To(Equal(":9090"))
		Expect(s.HTTP.ReadTimeout).To(Equal(5 * time.Second))
		Expect(s.HTTP.WriteTimeout).To(Equal(10 * time.Second))
		Expect(s.HTTP.IdleTimeout).To(Equal(60 * time.Second))
	})

	It("returns error for TLS config without certificates", func() {
		r := http.NewServeMux()
		cfg := &tls.Config{MinVersion: tls.VersionTLS12}
		s := q.NewServer(q.ServerConfig{Addr: ":0", TLSConfig: cfg}, r, nil)
		err := s.Start()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("no certificates"))
	})

	It("creates logger when nil is provided", func() {
		r := http.NewServeMux()
		s := q.NewServer(q.ServerConfig{}, r, nil)
		Expect(s.Logger).NotTo(BeNil())
	})
})

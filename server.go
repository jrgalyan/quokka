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
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server wraps http.Server with graceful shutdown and health endpoints.
type Server struct {
	HTTP   *http.Server
	Logger *slog.Logger
}

type ServerConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	TLSConfig         *tls.Config
}

func NewServer(cfg ServerConfig, handler http.Handler, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Addr == "" {
		cfg.Addr = ":8080"
	}
	hs := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadTimeout:       defaultDur(cfg.ReadTimeout, 15*time.Second),
		WriteTimeout:      defaultDur(cfg.WriteTimeout, 30*time.Second),
		IdleTimeout:       defaultDur(cfg.IdleTimeout, 120*time.Second),
		ReadHeaderTimeout: defaultDur(cfg.ReadHeaderTimeout, 5*time.Second),
		TLSConfig:         cfg.TLSConfig,
	}
	return &Server{HTTP: hs, Logger: logger}
}

func defaultDur(v, def time.Duration) time.Duration {
	if v == 0 {
		return def
	}
	return v
}

// Start runs the server and listens for shutdown signals.
func (s *Server) Start() error {
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		sig := <-ch
		s.Logger.Info("shutdown signal received", slog.String("signal", sig.String()))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.HTTP.Shutdown(ctx); err != nil {
			s.Logger.Error("shutdown error", slog.Any("err", err))
		}
	}()
	s.Logger.Info("server starting", slog.String("addr", s.HTTP.Addr))
	if s.HTTP.TLSConfig != nil {
		if len(s.HTTP.TLSConfig.Certificates) == 0 && s.HTTP.TLSConfig.GetCertificate == nil {
			return errors.New("quokka: TLSConfig has no certificates and no GetCertificate function")
		}
		return s.HTTP.ListenAndServeTLS("", "")
	}
	return s.HTTP.ListenAndServe()
}

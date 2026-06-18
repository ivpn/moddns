package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

// readinessTimeout bounds the datastore checks performed by /health/ready.
const readinessTimeout = 2 * time.Second

// ReadyFunc reports whether the service's dependencies are reachable. It is
// invoked by the /health/ready handler; a non-nil error yields HTTP 503.
type ReadyFunc func(ctx context.Context) error

// Server serves Prometheus metrics and health endpoints over HTTP.
type Server struct {
	srv *http.Server
}

// New creates a metrics/health server on the given port. The ready callback
// backs /health/ready (e.g. ping Mongo and Redis); it may be nil, in which case
// readiness always reports healthy.
func New(port int, ready ReadyFunc) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              fmt.Sprintf(":%d", port),
			Handler:           handler(ready),
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

// handler builds the mux serving /metrics, /health/live and /health/ready.
func handler(ready ReadyFunc) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Liveness: process is up and serving HTTP.
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Readiness: dependencies (Mongo, Redis) are reachable.
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if ready == nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()
		if err := ready(ctx); err != nil {
			log.Warn().Err(err).Msg("readiness check failed")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

// Start begins serving. Blocks until the server stops.
func (s *Server) Start() error {
	log.Info().Str("addr", s.srv.Addr).Msg("metrics server starting")
	err := s.srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

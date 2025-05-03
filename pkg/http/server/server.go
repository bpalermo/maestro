package server

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/bpalermo/maestro/pkg/http/handlers"
	"go.uber.org/atomic"
	"k8s.io/klog/v2"
)

type HTTPServer struct {
	server  *http.Server
	healthy *atomic.Bool
}

func NewServer(addr string, logger klog.Logger) *HTTPServer {
	mux := http.NewServeMux()

	s := &HTTPServer{
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
		healthy: atomic.NewBool(true),
	}

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.Handle("/validate", handlers.NewAdmissionValidationHandler(logger))

	mux.HandleFunc("GET /-/-/liveness", s.livenessHandler)
	mux.HandleFunc("GET /-/-/readiness", s.readinessHandler)

	return s
}

func (s *HTTPServer) Start(logger klog.Logger, errChan chan error) {
	logger.Info("Server listening", "addr", s.server.Addr)
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		s.healthy.Store(false)
		logger.Error(err, "Server failed to start")
		errChan <- err
		return
	}

	logger.Info("Server stopped")
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.server.SetKeepAlivesEnabled(false)
	s.healthy.Store(false)
	return s.server.Shutdown(ctx)
}

func (s *HTTPServer) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *HTTPServer) readinessHandler(w http.ResponseWriter, _ *http.Request) {
	if s.healthy.Load() {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

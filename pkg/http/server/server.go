package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/bpalermo/maestro/pkg/http/handlers"
	"go.uber.org/atomic"
	"k8s.io/klog/v2"
)

type Server struct {
	port    int
	router  *http.ServeMux
	server  *http.Server
	healthy *atomic.Bool
}

type HttpServerOption func(*Server)

func NewServer(port int, logger klog.Logger) *Server {
	s := &Server{
		port: port,
		server: &http.Server{
			Addr: fmt.Sprintf(":%d", port),
		},
		router:  http.NewServeMux(),
		healthy: atomic.NewBool(true),
	}

	s.router.HandleFunc("/debug/pprof/", pprof.Index)
	s.router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.router.Handle("/validate", handlers.NewAdmissionValidationHandler(logger))

	s.router.HandleFunc("GET /-/-/liveness", s.livenessHandler)
	s.router.HandleFunc("GET /-/-/readiness", s.readinessHandler)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) Start(logger klog.Logger, errChan chan error) {
	logger.Info("Server listening on address %s", s.server.Addr)
	err := s.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		s.healthy.Store(false)
		logger.Error(err, "Server failed to start")
		errChan <- err
		return
	}

	logger.Info("Server stopped")
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.server.SetKeepAlivesEnabled(false)
	s.healthy.Store(false)
	return s.server.Shutdown(ctx)
}

func (s *Server) livenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) readinessHandler(w http.ResponseWriter, _ *http.Request) {
	if s.healthy.Load() {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

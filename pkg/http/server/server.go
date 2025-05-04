package server

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/bpalermo/maestro/pkg/http/handlers"
	"go.uber.org/atomic"
	"k8s.io/klog/v2"
)

type HTTPServerArgs struct {
	Addr     string
	CertFile string
	KeyFile  string
}

type HTTPServer struct {
	certFile string
	keyFile  string
	server   *http.Server
	healthy  *atomic.Bool
}

func NewHTTPServerArgs() *HTTPServerArgs {
	return &HTTPServerArgs{
		Addr:     ":8443",
		CertFile: "/var/maestro/certs/tls.crt",
		KeyFile:  "/var/maestro/certs/tls.key",
	}
}

func NewServer(args *HTTPServerArgs, logger klog.Logger) *HTTPServer {
	mux := http.NewServeMux()

	s := &HTTPServer{
		server: &http.Server{
			Addr:    args.Addr,
			Handler: mux,
		},
		healthy:  atomic.NewBool(true),
		certFile: args.CertFile,
		keyFile:  args.KeyFile,
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
	logger.V(1).Info("Using TLS certificate", "cert", s.certFile, "key", s.keyFile)

	err := s.server.ListenAndServeTLS(s.certFile, s.keyFile)
	if err != nil && err != http.ErrServerClosed {
		s.healthy.Store(false)
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

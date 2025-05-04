package server

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/bpalermo/maestro/pkg/http/handlers"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"go.uber.org/atomic"
	"k8s.io/klog/v2"
)

type HTTPServerArgs struct {
	Addr            string
	SpireSocketPath string
}

type HTTPServer struct {
	server  *http.Server
	healthy *atomic.Bool
}

func NewHTTPServerArgs() *HTTPServerArgs {
	return &HTTPServerArgs{
		Addr:            ":443",
		SpireSocketPath: "unix:///spiffe-workload-api/spire-agent.sock",
	}
}

func NewServer(args *HTTPServerArgs, source *workloadapi.X509Source, logger klog.Logger) (*HTTPServer, error) {
	mux := http.NewServeMux()

	tlsConfig := tlsconfig.TLSServerConfig(source)

	s := &HTTPServer{
		server: &http.Server{
			Addr:      args.Addr,
			Handler:   mux,
			TLSConfig: tlsConfig,
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

	return s, nil
}

func (s *HTTPServer) Start(logger klog.Logger, errChan chan error) {
	logger.Info("Server listening", "addr", s.server.Addr)

	err := s.server.ListenAndServeTLS("", "")
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

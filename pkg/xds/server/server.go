package server

import (
	"context"
	"fmt"
	"net"
	"time"

	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"
	cachev3 "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	serverv3 "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
)

const (
	loggerName = "xds-server"

	defaultServerVersion = 0
	defaultServerNetwork = "tcp"
	defaultServerAddress = ":50051"
)

type XdsServer struct {
	log logr.Logger

	network string
	address string

	version       int
	snapshotCache cachev3.SnapshotCache

	grpcServer *grpc.Server

	shutdownTimeout time.Duration
}

type XdsServerOption func(*XdsServer)

func NewXdsServer(log logr.Logger, opts ...XdsServerOption) *XdsServer {
	srv := &XdsServer{
		log:     log.WithName(loggerName),
		version: defaultServerVersion,
		network: defaultServerNetwork,
		address: defaultServerAddress,
	}

	for _, option := range opts {
		option(srv)
	}

	srv.snapshotCache = cachev3.NewSnapshotCache(true, cachev3.IDHash{}, nil)
	server := serverv3.NewServer(context.Background(), srv.snapshotCache, nil)
	srv.grpcServer = grpc.NewServer()

	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(srv.grpcServer, server)
	endpointservice.RegisterEndpointDiscoveryServiceServer(srv.grpcServer, server)
	clusterservice.RegisterClusterDiscoveryServiceServer(srv.grpcServer, server)
	routeservice.RegisterRouteDiscoveryServiceServer(srv.grpcServer, server)
	listenerservice.RegisterListenerDiscoveryServiceServer(srv.grpcServer, server)

	return srv
}

func WithShutdownTimeout(timeout time.Duration) XdsServerOption {
	return func(s *XdsServer) {
		s.shutdownTimeout = timeout
	}
}

func (s *XdsServer) Start(ctx context.Context) error {
	s.log.Info("XDS server listening", "network", s.network, "address", s.address)

	// Create listener
	listener, err := net.Listen(s.network, s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s://%s: %w", s.network, s.address, err)
	}

	s.log.Info("XDS server listening", "network", s.network, "address", s.address)

	// Monitor context cancellation
	go func() {
		<-ctx.Done()
		s.log.Info("context cancelled, initiating graceful shutdown")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()

		if err := s.Shutdown(shutdownCtx); err != nil {
			s.log.Error(err, "failed to shutdown XDS server")
		}
	}()

	return s.grpcServer.Serve(listener)

}

// Shutdown gracefully stops the XDS server with timeout
func (s *XdsServer) Shutdown(ctx context.Context) error {
	s.log.Info("attempt to graceful shutdown XDS server")

	// Channel to signal shutdown completion
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Perform a graceful shutdown
		s.grpcServer.GracefulStop()
	}()

	// Wait for shutdown or timeout
	select {
	case <-done:
		s.log.Info("XDS server graceful shutdown complete")
		return nil
	case <-ctx.Done():
		s.log.Info("shutdown timeout exceeded, forcing stop")
		// Force stop if timeout reached
		s.grpcServer.Stop()

		// Wait briefly for the forced stop to complete
		select {
		case <-done:
			s.log.Info("XDS server stopped after forced shutdown")
		case <-time.After(1 * time.Second):
			s.log.Error(nil, "XDS server failed to stop after forced shutdown")
		}

		return fmt.Errorf("shutdown timeout exceeded: %w", ctx.Err())
	}
}

package sidecar

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/bpalermo/maestro/internal/log"
	discoveryV3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	xds "github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultAddress = "0.0.0.0:13000"
)

type XdsSidecar struct {
	l *log.Logger
	c cache.SnapshotCache

	// pushVersion stores the numeric push version. This should be accessed via NextVersion()
	pushVersion atomic.Uint64

	healthSrv  *health.Server
	grpcServer *grpc.Server
}

func NewXdsSidecar(ctx context.Context, l *log.Logger) *XdsSidecar {
	c := cache.NewSnapshotCache(true, cache.IDHash{}, l)

	grpcServer := grpc.NewServer()
	healthSrv := health.NewServer()

	xdsSidecar := &XdsSidecar{
		l,
		c,
		atomic.Uint64{},
		healthSrv,
		grpcServer,
	}

	xdsSrv := xds.NewServer(ctx, c, xdsSidecar)

	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)
	discoveryV3.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsSrv)

	return xdsSidecar
}

func (s *XdsSidecar) Start() {
	s.l.Info().Msg("starting xds server")

	lis, err := net.Listen("tcp", defaultAddress)
	if err != nil {
		s.l.Fatal().Err(err).Msg("failed to start grpc server")
	}

	if err := s.grpcServer.Serve(lis); err != nil {
		s.l.Err(err).Msg("error serving grpc")
	}
}

func (s *XdsSidecar) Close() error {
	s.healthSrv.Shutdown()
	s.grpcServer.GracefulStop()
	return nil
}

func (s *XdsSidecar) NextVersion() string {
	return time.Now().Format(time.RFC3339) + "/" + strconv.FormatUint(s.pushVersion.Inc(), 10)
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// StatusConnectionFailure indicates the connection failed.
	StatusConnectionFailure = 1
	// StatusRPCFailure indicates rpc failed.
	StatusRPCFailure = 2
	// StatusUnhealthy indicates rpc succeeded but indicates unhealthy service.
	StatusUnhealthy = 3
)

type rpcHeaders struct{ metadata.MD }

func (s *rpcHeaders) String() string { return fmt.Sprintf("%v", s.MD) }

func (s *rpcHeaders) Set(value string) error {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid RPC header, expected 'key: value', got %q", value)
	}
	trimmed := strings.TrimLeftFunc(parts[1], unicode.IsSpace)
	s.Append(parts[0], trimmed)
	return nil
}

func (s *rpcHeaders) Type() string {
	return "rpcHeaders"
}

var (
	flAddr       string
	flService    string
	flRPCTimeout int64
	flRPCHeaders = rpcHeaders{MD: make(metadata.MD)}

	// grpcProbeCmd represents the grpcprobe command
	grpcProbeCmd = &cobra.Command{
		Use:   "grpcprobe",
		Short: "Runs a gRPC health check probe",
		Run:   runGrpcProbeCmd,
	}
)

func init() {
	rootCmd.AddCommand(grpcProbeCmd)

	grpcProbeCmd.Flags().StringVar(&flAddr, "addr", "", "tcp host:port to connect.")
	_ = grpcProbeCmd.MarkFlagRequired("addr")
	grpcProbeCmd.Flags().StringVar(&flService, "service", "", "service name to check.")
	grpcProbeCmd.Flags().Int64Var(&flRPCTimeout, "rpcTimeout", 1, "timeout for health check rpc, in seconds.")
	grpcProbeCmd.Flags().Var(&flRPCHeaders, "rpcHeader", "additional RPC headers in 'name: value' format. May specify more than one via multiple flags.")

}

func runGrpcProbeCmd(_ *cobra.Command, _ []string) {
	runHealthProbe()
}

func runHealthProbe() {
	retCode := 0
	defer func() { os.Exit(retCode) }()

	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		sig := <-c
		if sig == os.Interrupt {
			log.Printf("cancellation received")
			cancel()
			return
		}
	}()

	conn, err := grpc.NewClient(
		flAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		if err == context.DeadlineExceeded {
			log.Printf("timeout: failed to connect service %q", flAddr)
		} else {
			log.Err(err).Msgf("error: failed to connect service at %q", flAddr)
		}
		retCode = StatusConnectionFailure
		return
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	rpcCtx, rpcCancel := context.WithTimeout(ctx, time.Duration(flRPCTimeout)*time.Second)
	defer rpcCancel()
	rpcCtx = metadata.NewOutgoingContext(rpcCtx, flRPCHeaders.MD)

	resp, err := healthpb.NewHealthClient(conn).Check(rpcCtx,
		&healthpb.HealthCheckRequest{
			Service: flService})
	if err != nil {
		if stat, ok := status.FromError(err); ok && stat.Code() == codes.Unimplemented {
			log.Printf("error: this server does not implement the grpc health protocol (grpc.health.v1.Health): %s", stat.Message())
		} else if stat, ok := status.FromError(err); ok && stat.Code() == codes.DeadlineExceeded {
			log.Printf("timeout: health rpc did not complete within %v", flRPCTimeout)
		} else {
			log.Err(err).Msg("error: health rpc failed")
		}
		retCode = StatusRPCFailure
		return
	}

	if resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		log.Printf("service unhealthy (responded with %q)", resp.GetStatus().String())
		retCode = StatusUnhealthy
		return
	}

	log.Printf("status: %v", resp.GetStatus().String())
}

package envoy

import (
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	defaultHealthCheckTimeout            = time.Second * 1
	defaultHealthCheckInterval           = time.Second * 10
	defaultHealthCheckHealthyThreshold   = 1
	defaultHealthCheckUnhealthyThreshold = 1
)

func HttpHealthCheck(host string, path string) *corev3.HealthCheck {
	h := healthCheck()

	h.HealthChecker = &corev3.HealthCheck_HttpHealthCheck_{
		HttpHealthCheck: &corev3.HealthCheck_HttpHealthCheck{
			Host: host,
			Path: path,
		},
	}

	return h
}

func GrpcHealthCheck(serviceName string) *corev3.HealthCheck {
	h := healthCheck()

	h.HealthChecker = &corev3.HealthCheck_GrpcHealthCheck_{
		GrpcHealthCheck: &corev3.HealthCheck_GrpcHealthCheck{
			ServiceName: serviceName,
		},
	}

	return h
}

func healthCheck() *corev3.HealthCheck {
	return &corev3.HealthCheck{
		Interval: durationpb.New(defaultHealthCheckInterval),
		Timeout:  durationpb.New(defaultHealthCheckTimeout),
		HealthyThreshold: &wrapperspb.UInt32Value{
			Value: defaultHealthCheckHealthyThreshold,
		},
		UnhealthyThreshold: &wrapperspb.UInt32Value{
			Value: defaultHealthCheckUnhealthyThreshold,
		},
	}
}

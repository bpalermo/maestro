package envoy

import (
	"time"

	"github.com/bpalermo/maestro/internal/util"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	ext_authzv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	jwt_authnv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/jwt_authn/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	authzTimeout = time.Millisecond * 500
)

func authn() *http_connection_managerv3.HttpFilter {
	typedConfig := &jwt_authnv3.JwtAuthentication{
		BypassCorsPreflight: false,
	}
	return httpFilter("envoy.filters.http.jwt_authn", typedConfig)
}

func authz(clusterName string) *http_connection_managerv3.HttpFilter {
	typedConfig := &ext_authzv3.ExtAuthz{
		TransportApiVersion: corev3.ApiVersion_V3,
		Services: &ext_authzv3.ExtAuthz_GrpcService{
			GrpcService: &corev3.GrpcService{
				TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
						ClusterName: clusterName,
					},
				},
				Timeout: durationpb.New(authzTimeout),
			},
		},
		WithRequestBody: &ext_authzv3.BufferSettings{
			MaxRequestBytes:     8192,
			AllowPartialMessage: true,
		},
		FailureModeAllow: false,
	}
	return httpFilter("envoy.filters.http.ext_authz", typedConfig)
}

func router() *http_connection_managerv3.HttpFilter {
	typedConfig := &routerv3.Router{}
	return httpFilter("envoy.filters.http.router", typedConfig)
}

func httpFilter(name string, typedConfig proto.Message) *http_connection_managerv3.HttpFilter {
	return &http_connection_managerv3.HttpFilter{
		Name: name,
		ConfigType: &http_connection_managerv3.HttpFilter_TypedConfig{
			TypedConfig: util.MustAny(typedConfig),
		},
	}
}

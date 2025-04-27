package envoy

import (
	"fmt"

	"github.com/bpalermo/maestro/internal/config/constants"
	"github.com/bpalermo/maestro/internal/util"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	transport_sockets_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
)

func generateInboundHTTPFilterChain(enableAuthn bool, authzClusterName string, spireDomain string, vhosts []*routev3.VirtualHost) []*listenerv3.FilterChain {
	filterChains := make([]*listenerv3.FilterChain, 0)

	filterChains = append(filterChains, httpTLSFilterChain(enableAuthn, authzClusterName, spireDomain, vhosts))

	return filterChains
}

func httpTLSFilterChain(enableAuthn bool, authzClusterName string, spireDomain string, vhosts []*routev3.VirtualHost) *listenerv3.FilterChain {
	filters := make([]*listenerv3.Filter, 0)

	hcm := HttpConnectionManager("inbound_http", vhosts)
	hcm.HttpFilters = hcmHttpFilters(enableAuthn, authzClusterName)

	hcm.ForwardClientCertDetails = http_connection_managerv3.HttpConnectionManager_SANITIZE_SET
	hcm.SetCurrentClientCertDetails = &http_connection_managerv3.HttpConnectionManager_SetCurrentClientCertDetails{
		Uri: true,
	}

	hcmFilter := networkFilter("envoy.http_connection_manager", hcm)

	filters = append(filters, hcmFilter)

	filterChain := &listenerv3.FilterChain{
		Filters: filters,
	}

	if spireDomain != "" {
		filterChain.TransportSocket = &corev3.TransportSocket{
			Name: "envoy.transport_sockets.tls",
			ConfigType: &corev3.TransportSocket_TypedConfig{
				TypedConfig: util.MustAny(
					&transport_sockets_v3.DownstreamTlsContext{
						CommonTlsContext: &transport_sockets_v3.CommonTlsContext{
							ValidationContextType: &transport_sockets_v3.CommonTlsContext_ValidationContextSdsSecretConfig{
								ValidationContextSdsSecretConfig: &transport_sockets_v3.SdsSecretConfig{
									Name: fmt.Sprintf("spiffe://%s", spireDomain),
									SdsConfig: &corev3.ConfigSource{
										ResourceApiVersion: corev3.ApiVersion_V3,
										ConfigSourceSpecifier: &corev3.ConfigSource_ApiConfigSource{
											ApiConfigSource: &corev3.ApiConfigSource{
												ApiType: corev3.ApiConfigSource_GRPC,
												GrpcServices: []*corev3.GrpcService{
													{
														TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
															EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
																ClusterName: constants.ClusterNameLocalSpire.ToString(),
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				),
			},
		}
	}

	return filterChain
}

func hcmHttpFilters(enableAuthn bool, authzClusterName string) []*http_connection_managerv3.HttpFilter {
	filters := make([]*http_connection_managerv3.HttpFilter, 0)

	if enableAuthn {
		filters = append(filters, authn())
	}

	if authzClusterName != "" {
		filters = append(filters, authz(authzClusterName))
	}

	filters = append(filters, router())

	return filters
}

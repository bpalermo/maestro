package envoy

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

const (
	inboundHTTPListenerAddress = "0.0.0.0"
	inboundHTTPListenerPort    = 18080
)

func GenerateInboundHTTPListener(enableAuthn bool, authzClusterName string, spireDomain string, vhosts []*routev3.VirtualHost) *listenerv3.Listener {
	return &listenerv3.Listener{
		Name: "inbound_http",
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: inboundHTTPListenerAddress,
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: inboundHTTPListenerPort,
					},
				},
			},
		},
		FilterChains: generateInboundHTTPFilterChain(enableAuthn, authzClusterName, spireDomain, vhosts),
	}
}

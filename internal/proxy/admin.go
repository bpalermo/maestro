package proxy

import (
	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
)

const (
	adminAddress = "0.0.0.0"
	adminPort    = 9901
)

func generateAdminResource() *bootstrapv3.Admin {
	return &bootstrapv3.Admin{
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: adminAddress,
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: adminPort,
					},
				},
			},
		},
	}
}

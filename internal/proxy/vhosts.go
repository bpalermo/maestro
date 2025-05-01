package proxy

import (
	"fmt"

	configv1 "github.com/bpalermo/maestro/api/config/v1"
	"github.com/bpalermo/maestro/internal/proxy/envoy"
	"github.com/bpalermo/maestro/internal/util"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

func generateVHosts(serviceName string, servicePorts []*configv1.Service_ServicePort) []*routev3.VirtualHost {
	hostname := util.HostnameFromServiceName(serviceName)

	vhosts := make([]*routev3.VirtualHost, 0)
	for _, svcPort := range servicePorts {
		name := fmt.Sprintf("local_service_%d", svcPort.Port)
		sni := fmt.Sprintf("%s_%d", hostname, svcPort.Port)
		vhosts = append(vhosts, envoy.VirtualHost(name, sni))
	}

	// catch all
	vhosts = append(vhosts, catchAllVHost())

	return vhosts
}

func catchAllVHost() *routev3.VirtualHost {
	return &routev3.VirtualHost{
		Name:    "catch_all",
		Domains: []string{"*"},
		Routes: []*routev3.Route{
			{
				Match: &routev3.RouteMatch{
					PathSpecifier: &routev3.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				RequestHeadersToAdd: []*corev3.HeaderValueOption{
					{
						AppendAction: corev3.HeaderValueOption_OVERWRITE_IF_EXISTS_OR_ADD,
						Header: &corev3.HeaderValue{
							Key:   "x-maestro-catch-all",
							Value: "true",
						},
					},
				},
				Action: &routev3.Route_DirectResponse{
					DirectResponse: &routev3.DirectResponseAction{
						Status: 404,
					},
				},
			},
		},
	}
}

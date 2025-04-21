package envoy

import (
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

func VirtualHost(name string, sni string) *routev3.VirtualHost {
	return &routev3.VirtualHost{
		Name:    name,
		Domains: []string{sni},
		Routes: []*routev3.Route{
			{
				Match: &routev3.RouteMatch{
					PathSpecifier: &routev3.RouteMatch_Prefix{
						Prefix: "/",
					},
				},
				Action: &routev3.Route_Route{
					Route: &routev3.RouteAction{
						ClusterSpecifier: &routev3.RouteAction_Cluster{
							Cluster: name,
						},
					},
				},
			},
		},
	}
}

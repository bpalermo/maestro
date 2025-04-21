package envoy

import (
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
)

func LocalCluster(name string, address string, port uint32, hcPort *uint32) *clusterv3.Cluster {
	c := cluster(name, clusterv3.Cluster_STATIC)

	if hcPort == nil {
		hcPort = &port
	}

	c.LoadAssignment = &endpointv3.ClusterLoadAssignment{
		ClusterName: name,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpointv3.LbEndpoint{
					{
						HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
							Endpoint: &endpointv3.Endpoint{
								Address: &corev3.Address{
									Address: &corev3.Address_SocketAddress{
										SocketAddress: &corev3.SocketAddress{
											Address: address,
											PortSpecifier: &corev3.SocketAddress_PortValue{
												PortValue: port,
											},
										},
									},
								},
								HealthCheckConfig: &endpointv3.Endpoint_HealthCheckConfig{
									PortValue: *hcPort,
								},
							},
						},
					},
				},
			},
		},
	}

	return c
}

func LocalUDSCluster(name string, path string) *clusterv3.Cluster {
	c := cluster(name, clusterv3.Cluster_STATIC)

	c.LoadAssignment = &endpointv3.ClusterLoadAssignment{
		ClusterName: name,
		Endpoints: []*endpointv3.LocalityLbEndpoints{
			{
				LbEndpoints: []*endpointv3.LbEndpoint{
					{
						HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
							Endpoint: &endpointv3.Endpoint{
								Address: &corev3.Address{
									Address: &corev3.Address_Pipe{
										Pipe: &corev3.Pipe{
											Path: path,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return c
}

func cluster(name string, discoveryType clusterv3.Cluster_DiscoveryType) *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name: name,
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: discoveryType,
		},
	}
}

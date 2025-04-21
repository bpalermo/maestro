package proxy

import (
	"fmt"

	"github.com/bpalermo/maestro/internal/core/config/envoy"
	"github.com/bpalermo/maestro/internal/util"
	"github.com/bpalermo/maestro/pkg/config/constants"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	httpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"google.golang.org/protobuf/types/known/anypb"
)

func (g *BootstrapGenerator) svcLocalCluster(port uint32) *clusterv3.Cluster {
	name := fmt.Sprintf("local_service_%d", port)

	c := envoy.LocalCluster(name, "127.0.0.1", port, nil)

	c.TypedExtensionProtocolOptions = map[string]*anypb.Any{
		"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": util.MustAny(
			&httpv3.HttpProtocolOptions{
				UpstreamProtocolOptions: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_{
					ExplicitHttpConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig{
						ProtocolConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{},
					},
				},
			},
		),
	}

	return c
}

func (g *BootstrapGenerator) xdsLocalCluster() *clusterv3.Cluster {
	c := envoy.LocalCluster(constants.ClusterNameLocalXDS.ToString(), g.xdsConfig.Address, g.xdsConfig.Port, nil)

	c.ClusterDiscoveryType = &clusterv3.Cluster_Type{
		Type: g.xdsConfig.DiscoveryType,
	}

	c.HealthChecks = []*corev3.HealthCheck{
		envoy.GrpcHealthCheck(""),
	}

	c.TypedExtensionProtocolOptions = map[string]*anypb.Any{
		"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": util.MustAny(
			&httpv3.HttpProtocolOptions{
				UpstreamProtocolOptions: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_{
					ExplicitHttpConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig{
						ProtocolConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
					},
				},
			},
		),
	}

	return c
}

func (g *BootstrapGenerator) opaLocalCluster() *clusterv3.Cluster {
	c := envoy.LocalCluster(constants.ClusterNameLocalOPA.ToString(), g.opaConfig.Address, g.opaConfig.Port, &g.opaConfig.HcPort)

	c.HealthChecks = []*corev3.HealthCheck{
		envoy.HttpHealthCheck(g.opaConfig.Address, g.opaConfig.HcPath),
	}

	c.TypedExtensionProtocolOptions = map[string]*anypb.Any{
		"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": util.MustAny(
			&httpv3.HttpProtocolOptions{
				UpstreamProtocolOptions: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_{
					ExplicitHttpConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig{
						ProtocolConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
					},
				},
			},
		),
	}

	return c
}

func (g *BootstrapGenerator) spireLocalCluster() *clusterv3.Cluster {
	c := envoy.LocalUDSCluster(constants.ClusterNameLocalSDS.ToString(), "/tmp/sds/spire.socket", nil)

	c.TypedExtensionProtocolOptions = map[string]*anypb.Any{
		"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": util.MustAny(
			&httpv3.HttpProtocolOptions{
				UpstreamProtocolOptions: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_{
					ExplicitHttpConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig{
						ProtocolConfig: &httpv3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
					},
				},
			},
		),
	}

	return c
}

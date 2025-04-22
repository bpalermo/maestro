package proxy

import (
	"errors"
	"os"
	"path/filepath"

	configV1 "github.com/bpalermo/maestro/config/v1"
	"github.com/bpalermo/maestro/internal/core/config/envoy"
	"github.com/bpalermo/maestro/internal/util"
	"github.com/bpalermo/maestro/pkg/bootstrap"
	"github.com/bpalermo/maestro/pkg/config/constants"
	"github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"

	"github.com/rs/zerolog/log"
)

const (
	adminAddress = "0.0.0.0"
	adminPort    = 9901
)

type BootstrapGenerator struct {
	node        *corev3.Node
	sidecarCfg  *configV1.SidecarConfiguration
	xdsConfig   *bootstrap.XdsConfig
	opaConfig   *bootstrap.OpaConfig
	spireConfig *bootstrap.SpireConfig
}

func NewBootstrapGenerator(serviceCluster string, serviceNode string, sidecarCfg *configV1.SidecarConfiguration, xdsCfg *bootstrap.XdsConfig, opaCfg *bootstrap.OpaConfig, spireCfg *bootstrap.SpireConfig) *BootstrapGenerator {
	return &BootstrapGenerator{
		&corev3.Node{
			Id:      serviceNode,
			Cluster: serviceCluster,
		},
		sidecarCfg,
		xdsCfg,
		opaCfg,
		spireCfg,
	}
}

func (g *BootstrapGenerator) generateBootstrapConfiguration() *bootstrapv3.Bootstrap {
	svc := g.sidecarCfg.Service

	return &bootstrapv3.Bootstrap{
		Node:             g.node,
		Admin:            g.generateAdminResource(),
		StaticResources:  g.generateStaticResource(svc),
		DynamicResources: g.generateDynamicResource(),
	}
}

func (g *BootstrapGenerator) generateAdminResource() *bootstrapv3.Admin {
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

func (g *BootstrapGenerator) generateStaticResource(svc *configV1.Service) *bootstrapv3.Bootstrap_StaticResources {
	return &bootstrapv3.Bootstrap_StaticResources{
		Listeners: g.generateStaticListeners(svc),
		Clusters:  g.generateStaticClusters(svc),
	}
}

func (g *BootstrapGenerator) generateStaticListeners(svc *configV1.Service) []*listenerv3.Listener {
	servicePorts := svc.ServicePorts
	enableAuthn := svc.GetAuthn() != nil

	authzClusterName := ""
	if svc.GetAuthz() != nil {
		authzClusterName = string(constants.ClusterNameLocalOPA)
	}

	listeners := make([]*listenerv3.Listener, 0)

	vhosts := generateVHosts(svc.Name, servicePorts)

	listeners = append(listeners, envoy.GenerateInboundHTTPListener(enableAuthn, authzClusterName, g.spireConfig.Domain, vhosts))

	return listeners
}

func (g *BootstrapGenerator) generateDynamicResource() *bootstrapv3.Bootstrap_DynamicResources {
	return &bootstrapv3.Bootstrap_DynamicResources{
		AdsConfig: &corev3.ApiConfigSource{
			ApiType: corev3.ApiConfigSource_DELTA_GRPC,
			GrpcServices: []*corev3.GrpcService{
				{
					TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
						EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
							ClusterName: constants.ClusterNameLocalXDS.ToString(),
						},
					},
				},
			},
		},
		CdsConfig: &corev3.ConfigSource{
			ResourceApiVersion:    corev3.ApiVersion_V3,
			ConfigSourceSpecifier: &corev3.ConfigSource_Ads{},
		},
	}
}

func (g *BootstrapGenerator) generateStaticClusters(svc *configV1.Service) []*clusterv3.Cluster {
	clusters := make([]*clusterv3.Cluster, 0)

	clusters = append(clusters, g.xdsLocalCluster())
	clusters = append(clusters, g.opaLocalCluster())

	if g.spireConfig.Enabled {
		clusters = append(clusters, g.spireLocalCluster())
	}

	for _, svcPort := range svc.ServicePorts {
		clusters = append(clusters, g.svcLocalCluster(svcPort.Port))
	}

	return clusters
}

func (g *BootstrapGenerator) WriteToFile(filename string) error {
	b := g.generateBootstrapConfiguration()

	y := util.MustMarshalProtoToYaml(b)

	dir := filepath.Dir(filename)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		log.Debug().Msgf("folder doesn't exist: %s", dir)
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		log.Debug().Msgf("failed to create file %s", filename)
		return err
	}
	defer util.MustCloseFile(f)

	return util.WriteData(f, y)
}

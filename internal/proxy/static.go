package proxy

import (
	configv1 "github.com/bpalermo/maestro/api/config/v1"
	"github.com/bpalermo/maestro/internal/config/constants"
	"github.com/bpalermo/maestro/internal/proxy/envoy"
	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
)

func generateStaticResources(svc *configv1.Service, spiffeDomain string) *bootstrapv3.Bootstrap_StaticResources {
	return &bootstrapv3.Bootstrap_StaticResources{
		Listeners: generateStaticListeners(svc, spiffeDomain),
		Clusters:  generateStaticClusters(svc),
	}
}

func generateStaticListeners(svc *configv1.Service, spiffeDomain string) []*listenerv3.Listener {
	servicePorts := svc.ServicePorts
	enableAuthn := svc.GetAuthn() != nil

	authzClusterName := ""
	if svc.GetAuthz() != nil {
		authzClusterName = string(constants.ClusterNameLocalOPA)
	}

	listeners := make([]*listenerv3.Listener, 0)

	vhosts := generateVHosts(svc.Name, servicePorts)

	listeners = append(listeners, envoy.GenerateInboundHTTPListener(enableAuthn, authzClusterName, spiffeDomain, vhosts))

	return listeners
}

func generateStaticClusters(_ *configv1.Service) []*clusterv3.Cluster {
	clusters := make([]*clusterv3.Cluster, 0)

	return clusters
}

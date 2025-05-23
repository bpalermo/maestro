package proxy

import (
	"github.com/bpalermo/maestro/internal/util"
	configv1 "github.com/bpalermo/maestro/pkg/apis/config/v1"
	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
)

func GenerateBootstrap(proxyConfig *configv1.ProxyConfig, spiffeDomain string) string {
	bootstrap := generateBootstrap(proxyConfig, spiffeDomain)
	return string(util.MustMarshalProtoToYaml(bootstrap))
}

func generateBootstrap(proxyConfig *configv1.ProxyConfig, spiffeDomain string) *bootstrapv3.Bootstrap {
	svc := proxyConfig.Spec.Service
	return &bootstrapv3.Bootstrap{
		Admin:           generateAdminResource(),
		StaticResources: generateStaticResources(svc, spiffeDomain),
	}
}

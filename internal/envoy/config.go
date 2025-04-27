package envoy

import (
	"github.com/bpalermo/maestro/internal/util"
	maestrov1 "github.com/bpalermo/maestro/pkg/apis/maestrocontroller/v1"
	bootstrapv3 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
)

func GenerateBootstrap(_ *maestrov1.ProxyConfig) string {
	bootstrap := generateBootstrap()
	return string(util.MustMarshalProtoToYaml(bootstrap))
}

func generateBootstrap() *bootstrapv3.Bootstrap {
	return &bootstrapv3.Bootstrap{
		Admin: generateAdminResource(),
	}
}

package proxy

import (
	"path/filepath"
	"testing"

	configV1 "github.com/bpalermo/maestro/config/v1"
	"github.com/bpalermo/maestro/pkg/bootstrap"
	"github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestGenerateBootstrapConfiguration(t *testing.T) {
	sidecarCfg := &configV1.SidecarConfiguration{
		Service: &configV1.Service{
			Name: "fake.test.service",
			ServicePorts: []*configV1.Service_ServicePort{{
				Port: 8080,
			}},
		},
	}

	bootstrapGen := NewBootstrapGenerator(
		sidecarCfg,
		bootstrap.NewXdsConfig(),
		bootstrap.NewOpaConfig(),
		bootstrap.NewSpireConfig(),
	)

	b := bootstrapGen.generateBootstrapConfiguration()
	assert.NotNil(t, b)

	err := b.Validate()
	assert.Nil(t, err)

	assertAdmin(t, b.Admin)
	//assertStaticResources(t, bootstrap.StaticResources)
}

func assertAdmin(t *testing.T, actual *bootstrapv3.Admin) {
	expected := &bootstrapv3.Admin{
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: 9901,
					},
				},
			},
		},
	}

	if res := proto.Equal(expected, actual); res != true {
		t.Errorf("expected %v, actual %v", expected, actual)
	}
}

func TestBootstrapGenerator_WriteToFile(t *testing.T) {
	dir := t.TempDir()
	tmpFile := filepath.Join(dir, "config.yaml")

	sidecarCfg := &configV1.SidecarConfiguration{
		Service: &configV1.Service{
			Name: "fake.test.service",
			ServicePorts: []*configV1.Service_ServicePort{{
				Port: 8080,
			}},
		},
	}

	bootstrapGen := NewBootstrapGenerator(
		sidecarCfg,
		bootstrap.NewXdsConfig(),
		bootstrap.NewOpaConfig(),
		bootstrap.NewSpireConfig(),
	)

	b := bootstrapGen.generateBootstrapConfiguration()
	assert.NotNil(t, b)

	err := b.Validate()
	assert.Nil(t, err)

	err = bootstrapGen.WriteToFile(tmpFile)
	assert.NoError(t, err)
}

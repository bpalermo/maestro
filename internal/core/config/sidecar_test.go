package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	configV1 "github.com/bpalermo/maestro/config/v1"
	"github.com/bufbuild/protovalidate-go"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestSidecarConfig_Set(t *testing.T) {

	var tests = []struct {
		name      string
		inputFile string
		want      *configV1.SidecarConfiguration
		err       error
	}{
		{
			name:      "test good sidecar config",
			inputFile: "default.yaml",
			want: &configV1.SidecarConfiguration{
				Service: &configV1.Service{
					Name: "fake.test.service",
					ServicePorts: []*configV1.Service_ServicePort{
						{
							Port: 8080,
							HealthCheckSpecifier: &configV1.Service_ServicePort_HttpHealthCheck_{
								HttpHealthCheck: &configV1.Service_ServicePort_HttpHealthCheck{
									Path: "/",
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name:      "test service with multiple ports",
			inputFile: "service_with_multiple_ports.yaml",
			want: &configV1.SidecarConfiguration{
				Service: &configV1.Service{
					Name: "fake.test.service",
					ServicePorts: []*configV1.Service_ServicePort{
						{
							Port: 8080,
							HealthCheckSpecifier: &configV1.Service_ServicePort_HttpHealthCheck_{
								HttpHealthCheck: &configV1.Service_ServicePort_HttpHealthCheck{
									Path: "/",
								},
							},
						},
						{
							Port: 8081,
							HealthCheckSpecifier: &configV1.Service_ServicePort_HttpHealthCheck_{
								HttpHealthCheck: &configV1.Service_ServicePort_HttpHealthCheck{
									Path: "/health/",
								},
							},
						},
					},
				},
			},
			err: nil,
		},
		{
			name:      "invalid service",
			inputFile: "service_invalid.yaml",
			want:      nil,
			err:       &protovalidate.ValidationError{},
		},
		{
			name:      "invalid service port",
			inputFile: "service_invalid_port.yaml",
			want:      nil,
			err:       &protovalidate.ValidationError{},
		},
		{
			name:      "invalid service missing health check",
			inputFile: "service_missing_health_check.yaml",
			want:      nil,
			err:       &protovalidate.ValidationError{},
		},
	}

	for _, tc := range tests {
		y := readTestFile(t, tc.inputFile)
		assert.NotNil(t, y)

		actual := &SidecarConfig{}
		err := actual.Set(y)

		if tc.err != nil {
			ok := errors.As(err, &tc.err)
			assert.True(t, ok)
		} else {
			assert.Nil(t, err)
			if res := proto.Equal(tc.want, actual.cfg); res != true {
				t.Errorf("%v: Equal(%v, %v) = %v, want %v", tc.name, tc.want, actual, res, true)
			}
		}
	}
}

func readTestFile(t *testing.T, file string) string {
	f := filepath.Join("testdata", file)
	b, err := os.ReadFile(f)
	if err != nil {
		t.Fatal("error reading file:", err)
	}
	return string(b)
}

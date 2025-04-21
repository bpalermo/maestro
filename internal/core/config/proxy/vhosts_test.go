package proxy

import (
	"fmt"
	"testing"

	configV1 "github.com/bpalermo/maestro/config/v1"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestGenerateVHosts(t *testing.T) {
	serviceName := "fake.test.service"
	servicePort := 8080
	svcPorts := []*configV1.Service_ServicePort{
		{
			Port: 8080,
		},
	}
	actual := generateVHosts(serviceName, svcPorts)

	expected := []*routev3.VirtualHost{
		{
			Name:    fmt.Sprintf("local_service_%d", servicePort),
			Domains: []string{"service.test.fake_8080"},
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
								Cluster: fmt.Sprintf("local_service_%d", servicePort),
							},
						},
					},
				},
			},
		},
		{
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
		},
	}

	assert.Equal(t, len(expected), len(actual))

	for i, vhost := range expected {
		if res := proto.Equal(vhost, actual[i]); res != true {
			t.Errorf("expected %v, actual %v", expected, actual)
		}
	}
}

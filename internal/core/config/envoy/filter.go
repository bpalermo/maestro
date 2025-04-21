package envoy

import (
	"github.com/bpalermo/maestro/internal/util"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	localRouteName = "local_route"
)

var (
	rfc1918CidrRanges = []*corev3.CidrRange{
		{
			AddressPrefix: "10.0.0.0",
			PrefixLen:     wrapperspb.UInt32(8),
		},
		{
			AddressPrefix: "172.16.0.0",
			PrefixLen:     wrapperspb.UInt32(12),
		},
		{
			AddressPrefix: "192.168.0.0",
			PrefixLen:     wrapperspb.UInt32(16),
		},
	}
)

func HttpConnectionManager(statPrefix string, vhosts []*routev3.VirtualHost) *http_connection_managerv3.HttpConnectionManager {
	return &http_connection_managerv3.HttpConnectionManager{
		CodecType:  http_connection_managerv3.HttpConnectionManager_AUTO,
		StatPrefix: statPrefix,
		RouteSpecifier: &http_connection_managerv3.HttpConnectionManager_RouteConfig{
			RouteConfig: &routev3.RouteConfiguration{
				Name:         localRouteName,
				VirtualHosts: vhosts,
			},
		},
		InternalAddressConfig: &http_connection_managerv3.HttpConnectionManager_InternalAddressConfig{
			CidrRanges: rfc1918CidrRanges,
		},
	}
}

func networkFilter(name string, message proto.Message) *listenerv3.Filter {
	return &listenerv3.Filter{
		Name: name,
		ConfigType: &listenerv3.Filter_TypedConfig{
			TypedConfig: util.MustAny(message),
		},
	}
}

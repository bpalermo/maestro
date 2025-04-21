package envoy

import (
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	http_connection_managerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
)

func generateInboundHTTPFilterChain(enableAuthn bool, authzClusterName string, vhosts []*routev3.VirtualHost) []*listenerv3.FilterChain {
	filterChains := make([]*listenerv3.FilterChain, 0)

	filterChains = append(filterChains, httpTLSFilterChain(enableAuthn, authzClusterName, vhosts))

	return filterChains
}

func httpTLSFilterChain(enableAuthn bool, authzClusterName string, vhosts []*routev3.VirtualHost) *listenerv3.FilterChain {
	filters := make([]*listenerv3.Filter, 0)

	hcm := HttpConnectionManager("inbound_http", vhosts)
	hcm.HttpFilters = hcmHttpFilters(enableAuthn, authzClusterName)

	hcmFilter := networkFilter("envoy.http_connection_manager", hcm)

	filters = append(filters, hcmFilter)

	return &listenerv3.FilterChain{
		Filters: filters,
	}
}

func hcmHttpFilters(enableAuthn bool, authzClusterName string) []*http_connection_managerv3.HttpFilter {
	filters := make([]*http_connection_managerv3.HttpFilter, 0)

	if enableAuthn {
		filters = append(filters, authn())
	}

	if authzClusterName != "" {
		filters = append(filters, authz(authzClusterName))
	}

	filters = append(filters, router())

	return filters
}

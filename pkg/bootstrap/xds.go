package bootstrap

import (
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
)

const (
	defaultXdsAddress = "127.0.0.1"
	defaultXdsPort    = 13000
)

type XdsConfigOption func(*XdsConfig)

type XdsConfig struct {
	Address       string
	Port          uint32
	DiscoveryType clusterv3.Cluster_DiscoveryType
}

func NewXdsConfig(opts ...XdsConfigOption) *XdsConfig {
	c := XdsConfig{
		defaultXdsAddress,
		defaultXdsPort,
		clusterv3.Cluster_STATIC,
	}

	for _, o := range opts {
		o(&c)
	}

	return &c
}

func WithAddress(address string) XdsConfigOption {
	return func(c *XdsConfig) {
		c.Address = address
	}
}

func WithPort(port uint32) XdsConfigOption {
	return func(c *XdsConfig) {
		c.Port = port
	}
}

func (c *XdsConfig) SetStrictDNSDiscovery() {
	c.DiscoveryType = clusterv3.Cluster_STRICT_DNS
}

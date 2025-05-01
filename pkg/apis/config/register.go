package config

import "k8s.io/apimachinery/pkg/runtime/schema"

// GroupName is the group name used in this package
const (
	GroupName = "config.maestro.io"
)

var (
	ProxyConfigGroupVersionResource = schema.GroupVersionResource{
		Group:    GroupName,
		Version:  "v1",
		Resource: "proxyconfigs",
	}
)

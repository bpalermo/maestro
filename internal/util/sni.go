package util

import (
	"slices"
	"strings"
)

func HostnameFromServiceName(serviceName string) string {
	parts := strings.Split(serviceName, ".")
	slices.Reverse(parts)
	return strings.Join(parts, ".")
}

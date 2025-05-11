package annotation

const (
	maestroNamespace = "maestro.io"
	sidecarNamespace = "sidecar." + maestroNamespace

	ServiceName = maestroNamespace + "/serviceName"

	SidecarInject = sidecarNamespace + "/inject"
	SidecarStatus = sidecarNamespace + "/status"
)

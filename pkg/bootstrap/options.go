package bootstrap

// SidecarArgs provides all the configuration parameters for the Sidecar mode.
type SidecarArgs struct {
	CfgFilename         string
	EnvoyConfigFilename string

	XdsCfg *XdsConfig
}

func NewSidecarArgs() *SidecarArgs {
	return &SidecarArgs{}
}

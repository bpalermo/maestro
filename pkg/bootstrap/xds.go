package bootstrap

const (
	defaultXdsAddress = "127.0.0.1"
	defaultXdsPort    = 13000
)

type XdsConfigOption func(*XdsConfig)

type XdsConfig struct {
	Address string
	Port    uint32
}

func NewXdsConfig(opts ...XdsConfigOption) *XdsConfig {
	c := XdsConfig{
		defaultXdsAddress,
		defaultXdsPort,
	}

	for _, o := range opts {
		o(&c)
	}

	return &c
}

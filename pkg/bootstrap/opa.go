package bootstrap

const (
	defaultOpaAddress = "127.0.0.1"
	defaultOpaPort    = 9191
	defaultOpaHcPath  = "/health?plugins"
	defaultOpaHcPort  = 8282
)

type OpaConfigOption func(*OpaConfig)

type OpaConfig struct {
	Address string
	Port    uint32
	HcPath  string
	HcPort  uint32
}

func NewOpaConfig(opts ...OpaConfigOption) *OpaConfig {
	c := OpaConfig{
		defaultOpaAddress,
		defaultOpaPort,
		defaultOpaHcPath,
		defaultOpaHcPort,
	}

	for _, o := range opts {
		o(&c)
	}

	return &c
}

package bootstrap

type SpireConfigOption func(*SpireConfig)

type SpireConfig struct {
}

func NewSpireConfig(opts ...SpireConfigOption) *SpireConfig {
	c := SpireConfig{}

	for _, o := range opts {
		o(&c)
	}

	return &c
}

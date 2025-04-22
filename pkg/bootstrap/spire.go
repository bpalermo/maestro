package bootstrap

const (
	defaultSpirePath   = "/run/spire/sockets/agent.sock"
	defaultSpireDomain = "cluster.local"
)

type SpireConfigOption func(*SpireConfig)

type SpireConfig struct {
	Enabled bool
	Path    string
	Domain  string
}

func NewSpireConfig(opts ...SpireConfigOption) *SpireConfig {
	c := SpireConfig{
		true,
		defaultSpirePath,
		defaultSpireDomain,
	}

	for _, o := range opts {
		o(&c)
	}

	return &c
}

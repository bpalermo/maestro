package manager

import (
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	k8sManager "sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	defaultManagerName        = "unknown"
	defaultMetricsBindAddress = ":8080"
)

type MaestroManager struct {
	k8sManager.Manager

	name string
}

var _ k8sManager.Manager = &MaestroManager{}

type MaestroManagerOptions func(*MaestroManager)

func NewMaestroManager(options ...MaestroManagerOptions) (m *MaestroManager, err error) {
	mgr, err := k8sManager.New(config.GetConfigOrDie(), k8sManager.Options{
		Metrics: metricsserver.Options{
			BindAddress: defaultMetricsBindAddress,
		},
	})
	if err != nil {
		return nil, err
	}

	m = &MaestroManager{
		name:    defaultManagerName,
		Manager: mgr,
	}
	for _, option := range options {
		option(m)
	}

	// Add a liveness check
	if err = m.Manager.AddHealthzCheck(m.name, healthz.Ping); err != nil {
		return nil, err
	}

	return m, nil
}

// WithName sets the name of the manager
func WithName(name string) MaestroManagerOptions {
	return func(m *MaestroManager) {
		m.name = name
	}
}

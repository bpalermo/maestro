package manager

import (
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func NewManager(name string) (mgr manager.Manager, err error) {
	mgr, err = manager.New(config.GetConfigOrDie(), manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
	})
	if err != nil {
		return nil, err
	}

	// Add a liveness check
	if err = mgr.AddHealthzCheck(name, healthz.Ping); err != nil {
		return nil, err
	}

	return mgr, nil
}

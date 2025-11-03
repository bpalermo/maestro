package manager

import (
	"context"

	"github.com/bpalermo/maestro/pkg/reconciler"
	"github.com/go-logr/logr"
	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type RegistrarManager struct {
	*MaestroManager
}

var _ manager.Manager = &RegistrarManager{}

type RegistrarManagerOptions func(*RegistrarManager)

func NewRegistrarManager(name string, clusterName string, log logr.Logger, _ ...RegistrarManagerOptions) (m *RegistrarManager, err error) {
	mMgr, err := NewMaestroManager(WithName(name))
	if err != nil {
		return nil, err
	}

	m = &RegistrarManager{
		MaestroManager: mMgr,
	}

	err = builder.
		ControllerManagedBy(m).
		For(&discoveryv1.EndpointSlice{}).
		Complete(reconciler.NewRegistrarReconciler(m.GetClient(), clusterName, log))
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (m *RegistrarManager) Start(ctx context.Context) error {
	return m.Manager.Start(ctx)
}

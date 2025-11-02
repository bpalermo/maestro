package reconciler

import (
	"context"

	"github.com/bpalermo/maestro/internal/types"
	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type RegistrarReconciler struct {
	BaseReconciler

	clusterName string

	registry map[types.ServiceID]map[string]map[*types.Endpoint]struct{}
}

func NewRegistrarReconciler(c client.Client, clusterName string) *RegistrarReconciler {
	return &RegistrarReconciler{
		BaseReconciler: BaseReconciler{
			Client: c,
		},
		clusterName: clusterName,
		registry:    map[types.ServiceID]map[string]map[*types.Endpoint]struct{}{},
	}
}

func (a *RegistrarReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	slice := &discoveryv1.EndpointSlice{}
	err := a.Get(ctx, req.NamespacedName, slice)
	if err != nil {
		return reconcile.Result{}, err
	}

	endpointSliceName := slice.Name
	svcID := types.NewServiceID(slice.Labels[serviceNameLabel], slice.Namespace)

	if len(slice.Endpoints) == 0 {
		// remove all associated endpoints
		delete(a.registry[svcID], endpointSliceName)
	} else {
		// Create map for unique endpoints
		endpoints := make(map[*types.Endpoint]struct{})
		for _, e := range slice.Endpoints {
			for _, addr := range e.Addresses {
				for _, port := range slice.Ports {
					if port.Port != nil && port.AppProtocol != nil {
						endpoints[types.NewEndpoint(addr, port.Port, port.AppProtocol)] = struct{}{}
					}
				}
			}
		}

		a.registry[svcID][endpointSliceName] = endpoints
	}

	// push updated registry

	return reconcile.Result{}, nil
}

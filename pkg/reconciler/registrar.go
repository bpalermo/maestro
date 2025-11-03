package reconciler

import (
	"context"

	"github.com/bpalermo/maestro/internal/types"
	"github.com/go-logr/logr"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	registrarReconcilerLoggerName = "reconciler"
)

var (
	defaultAppProtocol = pointer.String("tcp")
)

type RegistrarReconciler struct {
	MaestroReconciler

	clusterName string

	registry map[types.ServiceID]map[string]map[string]*types.Endpoint
}

func NewRegistrarReconciler(c client.Client, clusterName string, log logr.Logger) *RegistrarReconciler {
	return &RegistrarReconciler{
		MaestroReconciler: MaestroReconciler{
			log:    log.WithName(registrarReconcilerLoggerName),
			Client: c,
		},
		clusterName: clusterName,
		registry:    map[types.ServiceID]map[string]map[string]*types.Endpoint{},
	}
}

func (r *RegistrarReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	es := &discoveryv1.EndpointSlice{}
	err := r.Get(ctx, req.NamespacedName, es)
	if err != nil {
		return reconcile.Result{}, err
	}

	endpointSliceName := es.Name
	svcID := types.NewServiceID(es.Labels[serviceNameLabel], es.Namespace)

	r.log.Info("reconciling endpoint slice", "name", endpointSliceName, "serviceID", svcID)

	err = r.Get(ctx, client.ObjectKeyFromObject(es), es)
	if err != nil {
		r.log.Error(err, "error getting endpoint slice", "name", endpointSliceName)
		return reconcile.Result{}, err
	}

	if len(es.Endpoints) == 0 {
		if _, exists := r.registry[svcID][endpointSliceName]; exists {
			r.log.Info("removing endpoint slice", "name", endpointSliceName)
			// remove all associated endpoints
			delete(r.registry[svcID], endpointSliceName)
		}
	} else {
		// Create map for unique endpoints
		if _, exists := r.registry[svcID]; !exists {
			r.registry[svcID] = map[string]map[string]*types.Endpoint{}
		}

		if _, exists := r.registry[svcID][endpointSliceName]; !exists {
			r.registry[svcID][endpointSliceName] = map[string]*types.Endpoint{}
		}

		for _, e := range es.Endpoints {
			for _, addr := range e.Addresses {
				for _, port := range es.Ports {
					if port.Port != nil {
						appProtocol := port.AppProtocol
						if appProtocol == nil {
							appProtocol = defaultAppProtocol
						}
						endpoint := types.NewEndpoint(addr, port.Port, appProtocol)
						r.registry[svcID][endpointSliceName][endpoint.String()] = endpoint
					}
				}
			}
		}
	}

	// push updated registry
	uniqueEndpoints := r.getUniqueEndpointsForService(svcID)
	count := len(uniqueEndpoints)
	if count > 0 {
		r.log.Info("sending unique endpoints from slice to xDS server", "name", endpointSliceName, "count", count)
	}

	return reconcile.Result{}, nil
}

func (r *RegistrarReconciler) getUniqueEndpointsForService(svcID types.ServiceID) []*types.Endpoint {
	uniqueEndpoints := make(map[string]*types.Endpoint)

	if endpointSlices, exists := r.registry[svcID]; exists {
		for _, endpoints := range endpointSlices {
			for key, endpoint := range endpoints {
				uniqueEndpoints[key] = endpoint
			}
		}
	}

	result := make([]*types.Endpoint, 0, len(uniqueEndpoints))
	for _, endpoint := range uniqueEndpoints {
		result = append(result, endpoint)
	}

	return result
}

package reconciler

import (
	"context"
	"testing"

	"github.com/bpalermo/maestro/internal/types"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestNewRegistrarReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = discoveryv1.AddToScheme(scheme)

	fClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	log := testr.New(t)
	reconciler := NewRegistrarReconciler(fClient, "test-cluster", log)

	assert.NotNil(t, reconciler)
	assert.Equal(t, "test-cluster", reconciler.clusterName)
	assert.NotNil(t, reconciler.registry)
	assert.Equal(t, fClient, reconciler.Client)
}

func TestRegistrarReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = discoveryv1.AddToScheme(scheme)

	tests := []struct {
		name              string
		endpointSlice     *discoveryv1.EndpointSlice
		existingRegistry  map[types.ServiceID]map[string]map[string]*types.Endpoint
		expectedEndpoints int
		expectError       bool
	}{
		{
			name: "new endpoint slice with endpoints",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "default",
					Labels: map[string]string{
						serviceNameLabel: "test-service",
					},
				},
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1", "10.0.0.2"},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Port:        pointer.Int32(8080),
						AppProtocol: pointer.String("http"),
					},
					{
						Port:        pointer.Int32(9090),
						AppProtocol: pointer.String("grpc"),
					},
				},
			},
			expectedEndpoints: 4, // 2 addresses × 2 ports
		},
		{
			name: "empty endpoint slice removes endpoints",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "default",
					Labels: map[string]string{
						serviceNameLabel: "test-service",
					},
				},
				Endpoints: []discoveryv1.Endpoint{},
			},
			expectedEndpoints: 0,
		},
		{
			name: "endpoint slice with no ports",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "default",
					Labels: map[string]string{
						serviceNameLabel: "test-service",
					},
				},
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
					},
				},
				Ports: []discoveryv1.EndpointPort{},
			},
			expectedEndpoints: 0,
		},
		{
			name: "endpoint slice with null port values",
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-slice",
					Namespace: "default",
					Labels: map[string]string{
						serviceNameLabel: "test-service",
					},
				},
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.0.0.1"},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Port:        nil,
						AppProtocol: pointer.String("http"),
					},
					{
						Port:        pointer.Int32(8080),
						AppProtocol: nil,
					},
				},
			},
			expectedEndpoints: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.endpointSlice).
				Build()

			reconciler := NewRegistrarReconciler(fClient, "test-cluster", testr.New(t))
			if tt.existingRegistry != nil {
				reconciler.registry = tt.existingRegistry
			}

			req := reconcile.Request{
				NamespacedName: client.ObjectKeyFromObject(tt.endpointSlice),
			}

			result, err := reconciler.Reconcile(context.Background(), req)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, reconcile.Result{}, result)

				svcID := types.NewServiceID(tt.endpointSlice.Labels[serviceNameLabel], tt.endpointSlice.Namespace)
				endpoints := reconciler.getUniqueEndpointsForService(svcID)
				assert.Len(t, endpoints, tt.expectedEndpoints)
			}
		})
	}
}

func TestRegistrarReconciler_Reconcile_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = discoveryv1.AddToScheme(scheme)

	fClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewRegistrarReconciler(fClient, "test-cluster", testr.New(t))

	req := reconcile.Request{
		NamespacedName: client.ObjectKey{
			Name:      "non-existent",
			Namespace: "default",
		},
	}

	_, err := reconciler.Reconcile(context.Background(), req)
	assert.Error(t, err)
}

func TestRegistrarReconciler_getUniqueEndpointsForService(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = discoveryv1.AddToScheme(scheme)

	fClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := NewRegistrarReconciler(fClient, "test-cluster", testr.New(t))

	svcID := types.NewServiceID("test-service", "default")

	// Create duplicate endpoints across different slices
	endpoint1 := types.NewEndpoint("10.0.0.1", pointer.Int32(8080), pointer.String("http"))
	endpoint2 := types.NewEndpoint("10.0.0.2", pointer.Int32(8080), pointer.String("http"))
	endpoint3 := types.NewEndpoint("10.0.0.1", pointer.Int32(8080), pointer.String("http")) // duplicate of endpoint1

	reconciler.registry[svcID] = map[string]map[string]*types.Endpoint{
		"slice1": {
			endpoint1.String(): endpoint1,
			endpoint2.String(): endpoint2,
		},
		"slice2": {
			endpoint3.String(): endpoint3, // duplicate endpoint
			endpoint2.String(): endpoint2, // same endpoint in different slice
		},
	}

	uniqueEndpoints := reconciler.getUniqueEndpointsForService(svcID)

	// Should deduplicate based on endpoint.String()
	assert.Len(t, uniqueEndpoints, 2)

	// Test non-existent service
	nonExistentSvcID := types.NewServiceID("non-existent", "default")
	emptyEndpoints := reconciler.getUniqueEndpointsForService(nonExistentSvcID)
	assert.Empty(t, emptyEndpoints)
}

func TestRegistrarReconciler_MultipleEndpointSlices(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = discoveryv1.AddToScheme(scheme)

	slice1 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "slice1",
			Namespace: "default",
			Labels: map[string]string{
				serviceNameLabel: "multi-service",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.1"}},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: pointer.Int32(8080), AppProtocol: pointer.String("http")},
		},
	}

	slice2 := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "slice2",
			Namespace: "default",
			Labels: map[string]string{
				serviceNameLabel: "multi-service",
			},
		},
		Endpoints: []discoveryv1.Endpoint{
			{Addresses: []string{"10.0.0.2"}},
		},
		Ports: []discoveryv1.EndpointPort{
			{Port: pointer.Int32(8080), AppProtocol: pointer.String("http")},
		},
	}

	fClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(slice1, slice2).
		Build()

	reconciler := NewRegistrarReconciler(fClient, "test-cluster", testr.New(t))

	// Reconcile first slice
	_, err := reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(slice1),
	})
	require.NoError(t, err)

	// Reconcile second slice
	_, err = reconciler.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(slice2),
	})
	require.NoError(t, err)

	svcID := types.NewServiceID("multi-service", "default")
	endpoints := reconciler.getUniqueEndpointsForService(svcID)

	assert.Len(t, endpoints, 2)
	assert.Len(t, reconciler.registry[svcID], 2) // Should have 2 slices
}

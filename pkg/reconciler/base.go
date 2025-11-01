package reconciler

import "sigs.k8s.io/controller-runtime/pkg/client"

const (
	serviceNameLabel = "kubernetes.io/service-name"
)

type BaseReconciler struct {
	client.Client
}

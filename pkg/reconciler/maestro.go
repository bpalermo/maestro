package reconciler

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	serviceNameLabel = "kubernetes.io/service-name"
)

type MaestroReconciler struct {
	client.Client

	log logr.Logger
}

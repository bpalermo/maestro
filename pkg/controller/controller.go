package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appsv1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	ctx context.Context

	fooClient  foo_clientset.Interface
	kubeClient kubernetes.Interface

	queue workqueue.TypedRateLimitingInterface[any]

	barInformer cache.SharedIndexInformer
	barLister   foo_listers.BarLister

	deployInformer cache.SharedIndexInformer
	deployLister   appsv1.DeploymentLister

	recorder record.EventRecorder
}

func NewController(ctx context.Context, fooClient foo_clientset.Interface, kubeClient kubernetes.Interface, recorder record.EventRecorder) *Controller {
	return nil
}

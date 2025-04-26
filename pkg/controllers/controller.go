package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/bpalermo/maestro/internal/envoy"
	maestrov1 "github.com/bpalermo/maestro/pkg/apis/maestro/v1"
	clientset "github.com/bpalermo/maestro/pkg/generated/clientset/versioned"
	maestroscheme "github.com/bpalermo/maestro/pkg/generated/clientset/versioned/scheme"
	informers "github.com/bpalermo/maestro/pkg/generated/informers/externalversions/maestro/v1"
	listers "github.com/bpalermo/maestro/pkg/generated/listers/maestro/v1"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const controllerAgentName = "maestro"

const (
	// SuccessSynced is used as part of the Event 'reason' when a ProxyConfig is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a ProxyConfig fails
	// to sync due to a ConfigMap of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a ConfigMap already existing
	MessageResourceExists = "Resource %q already exists and is not managed by ProxyConfig"
	// MessageResourceSynced is the message used for an Event fired when a ProxyConfig
	// is synced successfully
	MessageResourceSynced = "ProxyConfig synced successfully"
	// FieldManager distinguishes this controller from other things writing to API objects
	FieldManager = controllerAgentName
)

type MaestroController struct {
	// kubeClientSet is a standard kubernetes client set
	kubeClientSet kubernetes.Interface
	// maestroClientSet is a client set for our own API group
	maestroClientSet clientset.Interface

	configMapsLister   corelisters.ConfigMapLister
	configMapsSynced   cache.InformerSynced
	proxyConfigLister  listers.ProxyConfigLister
	proxyConfigsSynced cache.InformerSynced

	// workqueue is a rate-limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed number of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.TypedRateLimitingInterface[cache.ObjectName]
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewMaestroController returns a new sample controller
func NewMaestroController(
	ctx context.Context,
	kubeClientSet kubernetes.Interface,
	maestroClientSet clientset.Interface,
	configMapInformer coreinformers.ConfigMapInformer,
	proxyConfigInformer informers.ProxyConfigInformer) *MaestroController {
	logger := klog.FromContext(ctx)

	// Create an event broadcaster
	// Add maestro types to the default Kubernetes Scheme so Events can be
	// logged for maestro types.
	runtime.Must(maestroscheme.AddToScheme(scheme.Scheme))
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientSet.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[cache.ObjectName](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[cache.ObjectName]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &MaestroController{
		kubeClientSet:      kubeClientSet,
		maestroClientSet:   maestroClientSet,
		configMapsLister:   configMapInformer.Lister(),
		configMapsSynced:   configMapInformer.Informer().HasSynced,
		proxyConfigLister:  proxyConfigInformer.Lister(),
		proxyConfigsSynced: proxyConfigInformer.Informer().HasSynced,
		workqueue:          workqueue.NewTypedRateLimitingQueue(ratelimiter),
		recorder:           recorder,
	}

	logger.Info("Setting up event handlers")
	// Set up an event handler for when ProxyConfig resources change
	_, _ = proxyConfigInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueProxyConfig,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueProxyConfig(new)
		},
	})
	// Set up an event handler for when ConfigMap resources change. This
	// handler will look up the owner of the given ConfigMap, and if it is
	// owned by a ProxyConfig resource, then the handler will enqueue that ProxyConfig resource for
	// processing. This way, we don't need to implement custom logic for
	// handling ConfigMap resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	_, _ = configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.ConfigMap)
			oldDepl := old.(*corev1.ConfigMap)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known ConfigMap.
				// Two different versions of the same ConfigMap will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shut down the workqueue and wait for
// workers to finish processing their current work items.
func (c *MaestroController) Run(ctx context.Context, workers int) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	// Start the informer factories to begin populating the informer caches
	logger.Info("Starting Maestro controller")

	// Wait for the caches to be synced before starting workers
	logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.configMapsSynced, c.proxyConfigsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	// Launch two workers to process ProxyConfig resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *MaestroController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *MaestroController) processNextWorkItem(ctx context.Context) bool {
	objRef, shutdown := c.workqueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We call Done at the end of this func so the workqueue knows we have
	// finished processing this item. We also must remember to call Forget
	// if we do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer c.workqueue.Done(objRef)

	// Run the syncHandler, passing it the structured reference to the object to be synced.
	err := c.syncHandler(ctx, objRef)
	if err == nil {
		// If no error occurs, then we Forget this item, so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(objRef)
		logger.Info("Successfully synced", "objectName", objRef)
		return true
	}
	// There was a failure, so be sure to report it. This method allows for
	// pluggable error handling which can be used for things like
	// cluster-monitoring.
	runtime.HandleErrorWithContext(ctx, err, "Error syncing; requeuing for later retry", "objectReference", objRef)
	// Since we failed, we should requeue the item to work on later.  This
	// method will add a backoff to avoid hotlooping on particular items
	// (they're probably still not going to work right away) and overall
	// controller protection (everything I've done is broken, this controller
	// needs to calm down, or it can starve other useful work) cases.
	c.workqueue.AddRateLimited(objRef)
	return true
}

// syncHandler compares the actual state with the desired and attempts to
// converge the two. It then updates the Status block of the ProxyConfig resource
// with the current status of the resource.
func (c *MaestroController) syncHandler(ctx context.Context, objectRef cache.ObjectName) error {
	// Get the ProxyConfig resource with this namespace/name
	proxyConfig, err := c.proxyConfigLister.ProxyConfigs(objectRef.Namespace).Get(objectRef.Name)
	if err != nil {
		// The ProxyConfig resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleErrorWithContext(ctx, err, "ProxyConfig referenced by item in work queue no longer exists", "objectReference", objectRef)
			return nil
		}

		return err
	}

	proxyConfigName := proxyConfig.Name
	if proxyConfigName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated,
		// the resource will be queued again.
		runtime.HandleErrorWithContext(ctx, nil, "ConfigMap name missing from object reference", "objectReference", objectRef)
		return nil
	}

	// Get the config map with the name specified in ProxyConfig.spec
	configMap, err := c.configMapsLister.ConfigMaps(proxyConfig.Namespace).Get(proxyConfigName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		configMap, err = c.kubeClientSet.CoreV1().ConfigMaps(proxyConfig.Namespace).Create(ctx, newConfigMap(proxyConfig), metav1.CreateOptions{FieldManager: FieldManager})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or, any other transient reason.
	if err != nil {
		return err
	}

	// If the ConfigMap is not controlled by this ProxyConfig resource, we should log
	// a warning to the event recorder and return an error msg.
	if !metav1.IsControlledBy(configMap, proxyConfig) {
		msg := fmt.Sprintf(MessageResourceExists, configMap.Name)
		c.recorder.Event(proxyConfig, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	// Finally, we update the status block of the ProxyConfig resource to reflect the
	// current state of the world
	err = c.updateProxyConfigStatus(ctx, proxyConfig, configMap)
	if err != nil {
		return err
	}

	c.recorder.Event(proxyConfig, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *MaestroController) updateProxyConfigStatus(ctx context.Context, proxyConfig *maestrov1.ProxyConfig, configMap *corev1.ConfigMap) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of the original object and modify this copy
	// Or create a copy manually for better performance
	proxyConfigCopy := proxyConfig.DeepCopy()
	proxyConfigCopy.Status.ResourceVersion = configMap.ResourceVersion
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the ProxyConfig resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.maestroClientSet.MaestroV1().ProxyConfigs(proxyConfig.Namespace).UpdateStatus(ctx, proxyConfigCopy, metav1.UpdateOptions{FieldManager: FieldManager})
	return err
}

// enqueueProxyConfig takes a ProxyConfig resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ProxyConfig.
func (c *MaestroController) enqueueProxyConfig(obj interface{}) {
	if objectRef, err := cache.ObjectToName(obj); err != nil {
		runtime.HandleError(err)
		return
	} else {
		c.workqueue.Add(objectRef)
	}
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the ProxyConfig resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that ProxyConfig resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *MaestroController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	logger := klog.FromContext(context.Background())
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			// If the object value is not too big and does not contain sensitive information, then
			// it may be useful to include it.
			runtime.HandleErrorWithContext(context.Background(), nil, "Error decoding object, invalid type", "type", fmt.Sprintf("%T", obj))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			// If the object value is not too big and does not contain sensitive information, then
			// it may be useful to include it.
			runtime.HandleErrorWithContext(context.Background(), nil, "Error decoding object tombstone, invalid type", "type", fmt.Sprintf("%T", tombstone.Obj))
			return
		}
		logger.V(4).Info("Recovered deleted object", "resourceName", object.GetName())
	}
	logger.V(4).Info("Processing object", "object", klog.KObj(object))
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a ProxyConfig, we should not do anything more
		// with it.
		if ownerRef.Kind != "ProxyConfig" {
			return
		}

		proxyConfig, err := c.proxyConfigLister.ProxyConfigs(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			logger.V(4).Info("Ignore orphaned object", "object", klog.KObj(object), "ProxyConfig", ownerRef.Name)
			return
		}

		c.enqueueProxyConfig(proxyConfig)
		return
	}
}

// newConfigMap creates a new ConfigMap for a ProxyConfig resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ProxyConfig resource that 'owns' it.
func newConfigMap(proxyConfig *maestrov1.ProxyConfig) *corev1.ConfigMap {
	labels := map[string]string{
		"controller": proxyConfig.Name,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      proxyConfig.Name,
			Namespace: proxyConfig.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(proxyConfig, maestrov1.SchemeGroupVersion.WithKind("ProxyConfig")),
			},
			Labels: labels,
		},
		Data: map[string]string{
			"envoy.yaml": envoy.GenerateBootstrap(proxyConfig),
		},
	}
}

package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/bpalermo/maestro/internal/proxy"
	configv1 "github.com/bpalermo/maestro/pkg/apis/config/v1"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
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

	defaultConfigMapPrefix = "proxy-config-"
)

type MaestroControllerArgs struct {
	MasterURL       string
	KubeConfig      string
	ConfigMapPrefix string
	Spire           *SpireConfig
}

type SpireConfig struct {
	TrustDomain string
}

func NewControllerArgs() *MaestroControllerArgs {
	return &MaestroControllerArgs{
		Spire: &SpireConfig{},
	}
}

// MaestroControllerOption is a functional option type that allows us to configure the Controller.
type MaestroControllerOption func(*MaestroController)

type MaestroController struct {
	// ctx is the context for the controller.
	ctx context.Context

	// kubeClientSet is a standard kubernetes client set
	kubeClientSet kubernetes.Interface
	// dynamicClientSet is a client set for our own API group
	dynamicClientSet *dynamic.DynamicClient

	// kubeInformerFactory is a shared informer factory for the kubernetes API
	kubeInformerFactory informers.SharedInformerFactory

	// dynamicInformerFactory is a shared informer factory for our own API group
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory

	configMapsLister   corelisters.ConfigMapLister
	configMapsSynced   cache.InformerSynced
	proxyConfigLister  cache.GenericLister
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

	// configMapPrefix is the prefix used for the ConfigMap resources created by this controller.
	configMapPrefix string

	// spiffeTrustDomain SPIFFE trust domain
	spiffeTrustDomain string
}

// NewMaestroController returns a new sample controller
func NewMaestroController(
	ctx context.Context,
	args *MaestroControllerArgs,
	options ...MaestroControllerOption) *MaestroController {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Creating event broadcaster")

	cfg, err := clientcmd.BuildConfigFromFlags(args.MasterURL, args.KubeConfig)
	if err != nil {
		logger.Error(err, "Error building kubeconfig")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	maestroClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second*30)
	dynamicInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(maestroClient, time.Second*30, corev1.NamespaceAll, nil)

	configMapInformer := kubeInformerFactory.Core().V1().ConfigMaps()
	proxyConfigsInformer := dynamicInformerFactory.ForResource(configv1.ProxyConfigSchemaGVR)

	// Create an event broadcaster
	// Add maestro types to the default Kubernetes Scheme so Events can be
	// logged for maestro types.
	eventBroadcaster := record.NewBroadcaster(record.WithContext(ctx))
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
	ratelimiter := workqueue.NewTypedMaxOfRateLimiter(
		workqueue.NewTypedItemExponentialFailureRateLimiter[cache.ObjectName](5*time.Millisecond, 1000*time.Second),
		&workqueue.TypedBucketRateLimiter[cache.ObjectName]{Limiter: rate.NewLimiter(rate.Limit(50), 300)},
	)

	controller := &MaestroController{
		ctx:                    ctx,
		kubeInformerFactory:    kubeInformerFactory,
		dynamicInformerFactory: dynamicInformerFactory,
		kubeClientSet:          kubeClient,
		dynamicClientSet:       maestroClient,
		configMapsLister:       configMapInformer.Lister(),
		configMapsSynced:       configMapInformer.Informer().HasSynced,
		proxyConfigLister:      proxyConfigsInformer.Lister(),
		proxyConfigsSynced:     proxyConfigsInformer.Informer().HasSynced,
		workqueue:              workqueue.NewTypedRateLimitingQueue(ratelimiter),
		recorder:               recorder,
		spiffeTrustDomain:      args.Spire.TrustDomain,
		configMapPrefix:        defaultConfigMapPrefix,
	}

	// Apply all the functional options to configure the controller.
	for _, opt := range options {
		opt(controller)
	}

	logger.Info("Setting up event handlers")
	// Set up an event handler for when ProxyConfig resources change
	_, _ = proxyConfigsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueProxyConfig,
		UpdateFunc: func(_, n interface{}) {
			controller.enqueueProxyConfig(n)
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
		UpdateFunc: func(o, n interface{}) {
			newDepl := n.(*corev1.ConfigMap)
			oldDepl := o.(*corev1.ConfigMap)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known ConfigMap.
				// Two different versions of the same ConfigMap will always have different RVs.
				return
			}
			controller.handleObject(n)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// WithConfigMapPrefix is a functional option to set the config map prefix.
func WithConfigMapPrefix(prefix string) MaestroControllerOption {
	return func(c *MaestroController) {
		c.configMapPrefix = prefix
	}
}

func (c *MaestroController) Start(logger klog.Logger, errChan chan error) {
	// notice that there is no need to run Start methods in a separate goroutine. (i.e., go kubeInformerFactory.Start(ctx.done())
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	c.kubeInformerFactory.Start(c.ctx.Done())
	c.dynamicInformerFactory.Start(c.ctx.Done())

	if err := c.Run(c.ctx, 2); err != nil {
		errChan <- err
	}
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shut down the workqueue and wait for
// workers to finish processing their current work items.
func (c *MaestroController) Run(ctx context.Context, workers int) error {
	defer utilRuntime.HandleCrash()
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
// processNextWorkItem function to read and process a message on the
// workqueue.
func (c *MaestroController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it by calling the syncHandler.
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
	// put back in the workqueue and attempted again after a back-off
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
	utilRuntime.HandleErrorWithContext(ctx, err, "Error syncing; re-queuing for later retry", "objectReference", objRef)
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
	obj, err := c.proxyConfigLister.ByNamespace(objectRef.Namespace).Get(objectRef.Name)
	if err != nil {
		// The ProxyConfig resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilRuntime.HandleErrorWithContext(ctx, err, "ProxyConfig referenced by item in work queue no longer exists", "objectReference", objectRef)
			return nil
		}

		return err
	}

	u := obj.(*unstructured.Unstructured)

	proxyConfigName := u.GetName()
	if proxyConfigName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated,
		// the resource will be queued again.
		utilRuntime.HandleErrorWithContext(ctx, nil, "ConfigMap name missing from object reference", "objectReference", objectRef)
		return nil
	}

	// Get the config map with the expected name
	proxyConfigConfigMapName := c.configMapName(proxyConfigName)
	configMap, err := c.configMapsLister.ConfigMaps(u.GetNamespace()).Get(proxyConfigConfigMapName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		var proxyConfigMap *corev1.ConfigMap
		proxyConfigMap, err = c.newProxyConfigConfigMap(u)
		if err != nil {
			return err
		}
		configMap, err = c.kubeClientSet.CoreV1().ConfigMaps(u.GetNamespace()).Create(ctx, proxyConfigMap, metav1.CreateOptions{FieldManager: FieldManager})
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or, any other transient reason.
	if err != nil {
		return err
	}

	// If the ConfigMap is not controlled by this ProxyConfig resource, we should log
	// a warning to the event recorder and return an error msg.
	if !metav1.IsControlledBy(configMap, u) {
		msg := fmt.Sprintf(MessageResourceExists, configMap.Name)
		c.recorder.Event(obj, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	data, err := c.generateProxyConfigConfigMapData(u)
	if err != nil {
		return err
	}

	if !reflect.DeepEqual(configMap.Data, data) {
		configMap, err = c.updateProxyConfigMapData(ctx, u, configMap)
	}

	// If an error occurs during updating, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or, any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the ProxyConfig resource to reflect the
	// current state of the world
	err = c.updateProxyConfigStatus(ctx, u, configMap)
	if err != nil {
		return err
	}

	c.recorder.Event(obj, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *MaestroController) updateProxyConfigMapData(ctx context.Context, proxyConfigU *unstructured.Unstructured, configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of the original object and modify this copy
	// Or create a copy manually for better performance
	configMapCopy := configMap.DeepCopy()
	data, err := c.generateProxyConfigConfigMapData(proxyConfigU)
	if err != nil {
		return nil, err
	}

	configMapCopy.Data = data
	return c.kubeClientSet.CoreV1().ConfigMaps(configMap.Namespace).Update(ctx, configMapCopy, metav1.UpdateOptions{FieldManager: FieldManager})
}

func (c *MaestroController) updateProxyConfigStatus(ctx context.Context, proxyConfigU *unstructured.Unstructured, configMap *corev1.ConfigMap) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of the original object and modify this copy
	// Or create a copy manually for better performance
	proxyConfigCopy := proxyConfigU.DeepCopy()
	err := unstructured.SetNestedStringMap(proxyConfigCopy.Object, map[string]string{"ResourceVersion": configMap.ResourceVersion}, "status")
	if err != nil {
		return err
	}

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the ProxyConfig resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err = c.dynamicClientSet.Resource(configv1.ProxyConfigSchemaGVR).Namespace(proxyConfigU.GetNamespace()).UpdateStatus(ctx, proxyConfigU, metav1.UpdateOptions{FieldManager: FieldManager})
	return err
}

// enqueueProxyConfig takes a ProxyConfig resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed to resources of any type other than ProxyConfig.
func (c *MaestroController) enqueueProxyConfig(obj interface{}) {
	objectRef, err := cache.ObjectToName(obj)
	if err != nil {
		utilRuntime.HandleError(err)
		return
	}
	c.workqueue.Add(objectRef)
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
			utilRuntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object, invalid type", "type", fmt.Sprintf("%T", obj))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			// If the object value is not too big and does not contain sensitive information, then
			// it may be useful to include it.
			utilRuntime.HandleErrorWithContext(context.Background(), nil, "Error decoding object tombstone, invalid type", "type", fmt.Sprintf("%T", tombstone.Obj))
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

		proxyConfig, err := c.proxyConfigLister.ByNamespace(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			logger.V(4).Info("Ignore orphaned object", "object", klog.KObj(object), "ProxyConfig", ownerRef.Name)
			return
		}

		c.enqueueProxyConfig(proxyConfig)
		return
	}
}

// newProxyConfigConfigMap creates a new ConfigMap for a ProxyConfig resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ProxyConfig resource that 'owns' it.
func (c *MaestroController) newProxyConfigConfigMap(proxyConfigU *unstructured.Unstructured) (*corev1.ConfigMap, error) {
	data, err := c.generateProxyConfigConfigMapData(proxyConfigU)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"controller": controllerAgentName,
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.configMapName(proxyConfigU.GetName()),
			Namespace: proxyConfigU.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(proxyConfigU, configv1.SchemeGroupVersion.WithKind("ProxyConfig")),
			},
			Labels: labels,
		},
		Data: data,
	}, nil
}

func (c *MaestroController) generateProxyConfigConfigMapData(proxyConfigU *unstructured.Unstructured) (map[string]string, error) {
	var proxyConfig *configv1.ProxyConfig
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(proxyConfigU.Object, &proxyConfig)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"envoy.yaml": proxy.GenerateBootstrap(proxyConfig, c.spiffeTrustDomain),
	}, nil
}

func (c *MaestroController) configMapName(proxyConfigName string) string {
	return fmt.Sprintf("%s%s", c.configMapPrefix, proxyConfigName)
}

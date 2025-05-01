package cmd

import (
	"time"

	"github.com/bpalermo/maestro/pkg/apis/config"
	"github.com/bpalermo/maestro/pkg/controller"
	"github.com/bpalermo/maestro/pkg/signals"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	masterURL  string
	kubeconfig string

	controllerArgs = controller.NewControllerArgs()

	// controllerCmd represents the controller command
	controllerCmd = &cobra.Command{
		Use:   "controller",
		Short: "Starts the controller",
		RunE:  runController,
	}
)

func init() {
	rootCmd.AddCommand(controllerCmd)

	controllerCmd.Flags().StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	controllerCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")

	controllerCmd.Flags().StringVar(&controllerArgs.ConfigMapPrefix, "configMapPrefix", "proxy-config-", "Prefix for proxy config config maps")
	controllerCmd.Flags().StringVar(&controllerArgs.Spire.TrustDomain, "spireTrustDomain", "cluster.local", "Spire SPIFFE trust domain")
}

func runController(_ *cobra.Command, _ []string) error {
	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()
	logger := klog.FromContext(ctx)

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
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

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	dynamicInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(maestroClient, time.Second*30, corev1.NamespaceAll, nil)

	var opts []controller.MaestroControllerOption
	if controllerArgs.ConfigMapPrefix != "" {
		opts = append(opts, controller.WithConfigMapPrefix(controllerArgs.ConfigMapPrefix))
	}

	c := controller.NewMaestroController(
		ctx,
		kubeClient,
		maestroClient,
		kubeInformerFactory.Core().V1().ConfigMaps(),
		dynamicInformerFactory.ForResource(config.ProxyConfigGroupVersionResource),
		controllerArgs.Spire.TrustDomain,
		opts...,
	)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e., go kubeInformerFactory.Start(ctx.done())
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(ctx.Done())
	dynamicInformerFactory.Start(ctx.Done())

	if err = c.Run(ctx, 2); err != nil {
		logger.Error(err, "Error running controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	return nil
}

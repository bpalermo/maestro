package cmd

import (
	"context"
	"time"

	"github.com/bpalermo/maestro/internal/core/shutdown"
	"github.com/bpalermo/maestro/pkg/controller"
	"github.com/bpalermo/maestro/pkg/http/server"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
)

var (
	httpListenAddr          string
	gracefulShutdownTimeout time.Duration

	controllerArgs = controller.NewControllerArgs()

	// controllerCmd represents the controller command
	controllerCmd = &cobra.Command{
		Use:   "controller",
		Short: "Starts the controller",
		Run:   runController,
	}
)

func init() {
	rootCmd.AddCommand(controllerCmd)

	controllerCmd.Flags().StringVar(&controllerArgs.MasterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	controllerCmd.Flags().StringVar(&controllerArgs.KubeConfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	controllerCmd.Flags().StringVar(&httpListenAddr, "httpListenAddr", "0.0.0.0:8080", "HTTP server listen address.")
	controllerCmd.Flags().DurationVar(&gracefulShutdownTimeout, "gracefulShutdownTimeout", 30, "Graceful shutdown timeout in seconds")

	controllerCmd.Flags().StringVar(&controllerArgs.ConfigMapPrefix, "configMapPrefix", "proxy-config-", "Prefix for proxy config config maps")
	controllerCmd.Flags().StringVar(&controllerArgs.Spire.TrustDomain, "spireTrustDomain", "cluster.local", "Spire SPIFFE trust domain")
}

func runController(_ *cobra.Command, _ []string) {
	klog.InitFlags(nil)

	ctx := context.Background()
	logger := klog.FromContext(ctx)

	var opts []controller.MaestroControllerOption
	if controllerArgs.ConfigMapPrefix != "" {
		opts = append(opts, controller.WithConfigMapPrefix(controllerArgs.ConfigMapPrefix))
	}

	c := controller.NewMaestroController(
		ctx,
		controllerArgs,
		opts...,
	)

	s := server.NewServer(httpListenAddr, logger)

	errChan := make(chan error, 1)

	go c.Start(logger, errChan)
	go s.Start(logger, errChan)

	go func() {
		err := <-errChan
		logger.Error(err, "Error running controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}()

	shutdown.AddShutdownHook(ctx, logger, 30*time.Second)
}

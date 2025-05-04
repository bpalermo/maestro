package cmd

import (
	"context"
	"flag"
	"time"

	"github.com/bpalermo/maestro/internal/core/shutdown"
	"github.com/bpalermo/maestro/pkg/controller"
	"github.com/bpalermo/maestro/pkg/http/server"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

var (
	gracefulShutdownTimeout time.Duration

	controllerArgs = controller.NewControllerArgs()
	httpServerArgs = server.NewHTTPServerArgs()

	// controllerCmd represents the controller command
	controllerCmd = &cobra.Command{
		Use:   "controller",
		Short: "Starts the controller",
		Run:   runController,
	}
)

func init() {
	klog.InitFlags(nil)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	rootCmd.AddCommand(controllerCmd)

	controllerCmd.Flags().StringVar(&controllerArgs.MasterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	controllerCmd.Flags().StringVar(&controllerArgs.KubeConfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")

	controllerCmd.Flags().StringVar(&httpServerArgs.Addr, "httpListenAddr", ":8443", "HTTP server listen address.")
	controllerCmd.Flags().StringVar(&httpServerArgs.CertFile, "httpCertFile", "/var/maestro/certs/tls.crt", "HTTP server TLS certificate file path.")
	controllerCmd.Flags().StringVar(&httpServerArgs.KeyFile, "httpKeyFile", "/var/maestro/certs/tls.key", "HTTP server TLS key file path.")

	controllerCmd.Flags().DurationVar(&gracefulShutdownTimeout, "gracefulShutdownTimeout", 30, "Graceful shutdown timeout in seconds")

	controllerCmd.Flags().StringVar(&controllerArgs.ConfigMapPrefix, "configMapPrefix", "proxy-config-", "Prefix for proxy config config maps")
	controllerCmd.Flags().StringVar(&controllerArgs.Spire.TrustDomain, "spireTrustDomain", "cluster.local", "Spire SPIFFE trust domain")
}

func runController(_ *cobra.Command, _ []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	s := server.NewServer(httpServerArgs, logger)

	errChan := make(chan error, 1)

	go c.Start(logger, errChan)
	go s.Start(logger, errChan)

	go func() {
		err := <-errChan
		logger.Error(err, "Error running controller")
		cancel()
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}()

	shutdown.AddShutdownHook(ctx, logger, 30*time.Second)
}

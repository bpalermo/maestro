package cmd

import (
	"context"
	"time"

	"github.com/bpalermo/maestro/internal/core/shutdown"
	"github.com/bpalermo/maestro/internal/util"
	"github.com/bpalermo/maestro/pkg/controller"
	"github.com/bpalermo/maestro/pkg/http/server"
	"github.com/spf13/cobra"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
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
	rootCmd.AddCommand(controllerCmd)

	controllerCmd.Flags().StringVar(&controllerArgs.MasterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	controllerCmd.Flags().StringVar(&controllerArgs.KubeConfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")

	controllerCmd.Flags().StringVar(&httpServerArgs.Addr, "httpListenAddr", ":443", "HTTP server listen address.")
	controllerCmd.Flags().StringVar(&httpServerArgs.SpireSocketPath, "spireSocketPath", "unix:///spiffe-workload-api/spire-agent.sock", "Provides an address for the Workload API. The value of the SPIFFE_ENDPOINT_SOCKET environment variable will be used if the option is unused.")

	controllerCmd.Flags().DurationVar(&gracefulShutdownTimeout, "gracefulShutdownTimeout", 30*time.Second, "Graceful shutdown timeout in seconds")

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

	// Create a `workloadapi.X509Source`, it will connect to Workload API using the provided socket.
	// If the socket path is not defined using `workloadapi.SourceOption`, the value from environment variable `SPIFFE_ENDPOINT_SOCKET` is used.
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(httpServerArgs.SpireSocketPath)))
	if err != nil {
		logger.Error(err, "Unable to create X509Source")
		cancel()
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
	defer util.MustClose(source)

	s, err := server.NewServer(httpServerArgs, source, logger)
	if err != nil {
		logger.Error(err, "Could not create a HTTP server")
		cancel()
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

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

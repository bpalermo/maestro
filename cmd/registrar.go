package cmd

import (
	"os"
	"time"

	"github.com/bpalermo/maestro/pkg/manager"
	"github.com/bpalermo/maestro/pkg/xds/server"
	"github.com/spf13/cobra"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

const (
	defaultClusterName     = "unknown"
	defaultShutdownTimeout = 30 * time.Second
)

var (
	clusterName           string
	serverShutdownTimeout time.Duration

	// registrarCmd represents the controller command
	registrarCmd = &cobra.Command{
		Use:   "registrar",
		Short: "Starts the registrar controller",
		Run:   runRegistrar,
	}
)

func init() {
	rootCmd.AddCommand(registrarCmd)

	registrarCmd.Flags().StringVar(&clusterName, "cluster", defaultClusterName, "Cluster name. It will be used to push registration info to the control plane.")
	registrarCmd.Flags().DurationVar(&serverShutdownTimeout, "serverShutdownTimeout", defaultShutdownTimeout, "Timeout for graceful shutdown.")
}

func runRegistrar(cmd *cobra.Command, _ []string) {
	logf.SetLogger(zap.New())
	log := logf.Log.WithName(cmd.Name())

	log.Info("starting registrar", "cluster", clusterName)

	// Create XDS server
	srv := server.NewXdsServer(log, server.WithShutdownTimeout(serverShutdownTimeout))

	// Create manager
	mgr, err := manager.NewRegistrarManager(cmd.Name(), clusterName, log)
	if err != nil {
		log.Error(err, "failed to create registrar manager")
		os.Exit(1)
	}

	// Add xDS server to the manager as runnable
	err = mgr.Add(srv)
	if err != nil {
		log.Error(err, "failed to add xDS server to manager")
		os.Exit(1)
	}

	log.Info("starting controller manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "failed to start manager")
		os.Exit(1)
	}

	log.Info("all services stopped gracefully")
}

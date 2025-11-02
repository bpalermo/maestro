package cmd

import (
	"os"

	"github.com/bpalermo/maestro/pkg/manager"
	"github.com/bpalermo/maestro/pkg/reconciler"
	"github.com/spf13/cobra"
	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

const (
	defaultClusterName = "unknown"
)

var (
	clusterName string

	// registrarCmd represents the controller command
	registrarCmd = &cobra.Command{
		Use:   "registrar",
		Short: "Starts the registrar controller",
		Run:   runEnroller,
	}
)

func init() {
	rootCmd.AddCommand(registrarCmd)

	registrarCmd.Flags().StringVar(&clusterName, "cluster", defaultClusterName, "Cluster name. It will be used to push registration info to the control plane.")
}

func runEnroller(cmd *cobra.Command, _ []string) {
	logf.SetLogger(zap.New())

	log := logf.Log.WithName(cmd.Name())

	mgr, err := manager.NewManager(cmd.Name())
	if err != nil {
		log.Error(err, "could not create manager")
		os.Exit(1)
	}

	err = builder.
		ControllerManagedBy(mgr).          // Create the ControllerManagedBy
		For(&discoveryv1.EndpointSlice{}). // EndpointSlice is the Endpoints API
		Complete(reconciler.NewRegistrarReconciler(mgr.GetClient(), clusterName))
	if err != nil {
		log.Error(err, "could not create controller")
		os.Exit(1)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		log.Error(err, "could not start manager")
		os.Exit(1)
	}
}

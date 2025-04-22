package cmd

import (
	"context"

	"github.com/bpalermo/maestro/internal/core/config"
	"github.com/bpalermo/maestro/internal/core/config/proxy"
	"github.com/bpalermo/maestro/internal/core/server"
	"github.com/bpalermo/maestro/internal/core/sidecar"
	"github.com/bpalermo/maestro/internal/log"
	"github.com/bpalermo/maestro/pkg/bootstrap"
	"github.com/spf13/cobra"
)

var (
	logger *log.Logger

	serviceCluster string
	serviceNode    string

	sidecarArgs *bootstrap.SidecarArgs
	sidecarCfg  = config.NewSidecarConfig()

	bootstrapGenerator *proxy.BootstrapGenerator

	xdsCfg                = bootstrap.NewXdsConfig()
	xdsStrictDNSDiscovery bool

	opaCfg   = bootstrap.NewOpaConfig()
	spireCfg = bootstrap.NewSpireConfig()

	// sidecarCmd represents the sidecar command
	sidecarCmd = &cobra.Command{
		Use:   "sidecar",
		Short: "Start maestro in sidecar mode",
		PreRunE: func(c *cobra.Command, args []string) error {
			// TODO: setup logging
			return nil
		},
		Run: runSidecarCmd,
	}
)

func init() {
	rootCmd.AddCommand(sidecarCmd)

	sidecarCmd.Flags().StringVar(&serviceCluster, "serviceCluster", "", "envoy service cluster name")
	_ = sidecarCmd.MarkFlagRequired("serviceCluster")
	sidecarCmd.Flags().StringVar(&serviceNode, "serviceNode", "", "envoy service node id")
	_ = sidecarCmd.MarkFlagRequired("serviceNode")

	sidecarArgs = bootstrap.NewSidecarArgs()

	// Sidecar
	sidecarCmd.Flags().VarP(sidecarCfg, "config", "c", "sidecar configuration")
	sidecarCmd.Flags().StringVarP(&sidecarArgs.CfgFilename, "configFile", "f", "", "sidecar configuration file")
	sidecarCmd.Flags().StringVarP(&sidecarArgs.EnvoyConfigFilename, "outputConfigFile", "o", "", "envoy configuration file to write to")
	sidecarCmd.MarkFlagsOneRequired("config", "configFile")
	sidecarCmd.MarkFlagsMutuallyExclusive("config", "configFile")

	// xDS
	sidecarCmd.Flags().StringVar(&xdsCfg.Address, "xdsClusterAddress", "127.0.0.1", "xDS cluster address")
	sidecarCmd.Flags().Uint32Var(&xdsCfg.Port, "xdsClusterPort", 13000, "xDS cluster port")

	// Spire
	sidecarCmd.Flags().StringVar(&spireCfg.Domain, "spireDomain", "cluster.local", "SPIRE domain")
}

func runSidecarCmd(c *cobra.Command, _ []string) {
	var err error

	debug, _ := c.Flags().GetBool(debugModeFlagName)
	logger = log.NewLogger(debug)

	logger.Info().Msg("starting in sidecar mode")

	if sidecarArgs.CfgFilename != "" {
		sidecarCfg, err = config.NewSidecarConfigFromFile(sidecarArgs.CfgFilename)
		if err != nil {
			logger.Fatal().Err(err).Msg("could not read sidecar configuration file")
		}
	}

	bootstrapGenerator = proxy.NewBootstrapGenerator(
		serviceCluster,
		serviceNode,
		sidecarCfg.GetConfig(),
		xdsCfg,
		opaCfg,
		spireCfg,
	)

	writeSidecarConfiguration(sidecarArgs)
	runSidecar()
}

func writeSidecarConfiguration(sidecarArgs *bootstrap.SidecarArgs) {
	err := bootstrapGenerator.WriteToFile(sidecarArgs.EnvoyConfigFilename)
	if err != nil {
		logger.Fatal().Err(err).Msgf("could not write envoy configuration to file '%s'", sidecarArgs.EnvoyConfigFilename)
	}
}

func runSidecar() {
	ctx := context.Background()

	xdsSrv := sidecar.NewXdsSidecar(ctx, logger)
	go xdsSrv.Start()

	server.AddShutdownHook(ctx, xdsSrv)
}

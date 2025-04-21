package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	debugModeFlagName = "debug"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "maestro",
	Short: "Maestro is a xDS implementation for Envoy on Kubernetes",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().Bool(debugModeFlagName, false, "enable debug mode")
}

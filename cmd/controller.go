package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// controllerCmd represents the controller command
var controllerCmd = &cobra.Command{
	Use:   "controller",
	Short: "Start the kubernetes controller",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("controller called")
	},
}

func init() {
	rootCmd.AddCommand(controllerCmd)
}

package cmd

import (
	"fmt"

	"remote-studio/pkg/config"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Remote Studio",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

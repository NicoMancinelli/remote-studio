package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of Remote Studio",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("9.0")
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}

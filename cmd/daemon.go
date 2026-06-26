package cmd

import (
	"fmt"

	"remote-studio/pkg/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the Remote Studio background control plane daemon",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Starting Remote Studio background control plane daemon...")
		if err := daemon.StartDaemon(); err != nil {
			return fmt.Errorf("daemon failure: %w", err)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(daemonCmd)
}

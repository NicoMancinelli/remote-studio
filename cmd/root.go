package cmd

import (
	"fmt"
	"os"

	"remote-studio/pkg/session"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "res",
	Short: "Remote Studio CLI",
	Long:  `Remote Studio CLI (remote-studio) is a control plane for remote session management.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func RegisterDynamicProfileCommands() {
	reg, err := loadAllProfilesCombined()
	if err != nil {
		return
	}
	for k := range reg.Profiles {
		pkey := k
		cmd := &cobra.Command{
			Use:   pkey,
			Short: fmt.Sprintf("Apply display profile: %s", pkey),
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				return session.ApplyProfile(pkey)
			},
		}
		RootCmd.AddCommand(cmd)
	}
}

func Execute() {
	RegisterDynamicProfileCommands()
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}


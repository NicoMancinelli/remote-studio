package cmd

import (
	"fmt"
	"os"
	"strings"

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
	// Cobra prints "Error: unknown command \"<name>\" for \"res\"" to stderr
	// by default. We rewrite that message to use a capital "Unknown"
	// (matching the historical bash-side wording) before printing it.
	if err := RootCmd.Execute(); err != nil {
		msg := err.Error()
		if strings.HasPrefix(msg, "unknown command ") {
			msg = "Unknown command " + strings.TrimPrefix(msg, "unknown command ")
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}


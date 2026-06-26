package cmd

import (
	"fmt"

	"remote-studio/pkg/session"
	"github.com/spf13/cobra"
)

func createActionCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := session.DoAction(use); err != nil {
				return fmt.Errorf("action %s failed: %w", use, err)
			}
			return nil
		},
	}
}

func init() {
	RootCmd.AddCommand(createActionCmd("speed", "Toggle performance speed mode (disable effects/animations/wallpaper)"))
	RootCmd.AddCommand(createActionCmd("theme", "Toggle GTK light/dark theme"))
	RootCmd.AddCommand(createActionCmd("night", "Toggle red night shift gamma color temperature"))
	RootCmd.AddCommand(createActionCmd("caf", "Toggle caffeine screensaver inhibitor lock"))
	RootCmd.AddCommand(createActionCmd("privacy", "Lock screensaver and force display power off (DPMS)"))
	RootCmd.AddCommand(createActionCmd("clip", "Clear primary and clipboard copy-paste selections"))
	RootCmd.AddCommand(createActionCmd("service", "Restart RustDesk background service"))
	RootCmd.AddCommand(createActionCmd("audio", "Restart PulseAudio server daemon"))
	RootCmd.AddCommand(createActionCmd("keys", "Reset keyboard layout mapping to US standard English"))
	RootCmd.AddCommand(createActionCmd("fix", "Apply common clipboard, audio, and keyboard layout fixes"))
	RootCmd.AddCommand(createActionCmd("reset", "Reset display resolution and Cinnamon UI scaling back to defaults"))
}

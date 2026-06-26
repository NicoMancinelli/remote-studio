package cmd

import (
	"fmt"

	"remote-studio/pkg/display"
	"remote-studio/pkg/session"

	"github.com/spf13/cobra"
)

var rotateCmd = &cobra.Command{
	Use:   "rotate [normal | left | right | inverted]",
	Short: "Rotate the active display output screen orientation",
	Long: `Rotate the active display output. Auto-detects whether the session
is running X11 or Wayland and uses the appropriate backend.

This command is an alias for 'res display rotate'.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "normal"
		if len(args) > 0 {
			dir = args[0]
		}

		output, _ := display.GetConnectedOutput()

		if err := display.Rotate(dir); err != nil {
			return fmt.Errorf("failed to rotate display: %w", err)
		}

		session.LogEvent(fmt.Sprintf("Rotate: %s [%s]", dir, display.DetectBackend()))
		if output != "" {
			fmt.Printf("Rotated %s to %s (%s)\n", output, dir, display.DetectBackend())
		} else {
			fmt.Printf("Rotated to %s (%s)\n", dir, display.DetectBackend())
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(rotateCmd)
}

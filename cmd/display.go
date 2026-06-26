package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"remote-studio/pkg/display"
	"remote-studio/pkg/session"

	"github.com/spf13/cobra"
)

var displayCmd = &cobra.Command{
	Use:   "display",
	Short: "Manage display resolution, scale, and rotation (auto-detects X11/Wayland)",
	Long: `Display management commands that auto-detect the active session type
(X11 or Wayland) and use the appropriate backend (xrandr, wlr-randr,
or gnome-randr).

Subcommands:
  backend      Show the detected display backend
  outputs      List connected display outputs
  resolution   Set display resolution (WIDTHxHEIGHT)
  scale        Set display scale factor
  rotate       Rotate the display orientation`,
}

var displayBackendCmd = &cobra.Command{
	Use:   "backend",
	Short: "Show the detected display backend (x11, wayland, unknown)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := display.DetectBackend()
		fmt.Println(backend)
		return nil
	},
}

var displayOutputsCmd = &cobra.Command{
	Use:   "outputs",
	Short: "List connected display outputs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputs, err := display.ListOutputs()
		if err != nil {
			return err
		}

		fmt.Printf("Backend: %s\n\n", display.DetectBackend())
		fmt.Printf("%-20s %-12s %s\n", "OUTPUT", "STATUS", "RESOLUTION")
		fmt.Printf("%-20s %-12s %s\n", "------", "------", "----------")
		for _, o := range outputs {
			status := "disconnected"
			if o.Connected {
				status = "connected"
			}
			res := o.CurrentResolution
			if res == "" {
				res = "-"
			}
			fmt.Printf("%-20s %-12s %s\n", o.Name, status, res)
		}
		return nil
	},
}

var displayResolutionCmd = &cobra.Command{
	Use:   "resolution <WIDTHxHEIGHT>",
	Short: "Set display resolution (e.g. 1920x1080)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		parts := strings.SplitN(args[0], "x", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid resolution format %q — expected WIDTHxHEIGHT (e.g. 1920x1080)", args[0])
		}

		width, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid width %q: %w", parts[0], err)
		}
		height, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid height %q: %w", parts[1], err)
		}

		if err := display.SetResolution(width, height); err != nil {
			return err
		}

		session.LogEvent(fmt.Sprintf("Display resolution: %dx%d [%s]", width, height, display.DetectBackend()))
		fmt.Printf("Resolution set to %dx%d (%s)\n", width, height, display.DetectBackend())
		return nil
	},
}

var displayScaleCmd = &cobra.Command{
	Use:   "scale <FACTOR>",
	Short: "Set display scale factor (e.g. 1.5)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		factor, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return fmt.Errorf("invalid scale factor %q: %w", args[0], err)
		}
		if factor <= 0 {
			return fmt.Errorf("scale factor must be positive, got %.2f", factor)
		}

		if err := display.SetScale(factor); err != nil {
			return err
		}

		session.LogEvent(fmt.Sprintf("Display scale: %.2f [%s]", factor, display.DetectBackend()))
		fmt.Printf("Scale set to %.2f (%s)\n", factor, display.DetectBackend())
		return nil
	},
}

var displayRotateCmd = &cobra.Command{
	Use:   "rotate [normal|left|right|inverted]",
	Short: "Rotate the display orientation",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "normal"
		if len(args) > 0 {
			dir = args[0]
		}

		if err := display.Rotate(dir); err != nil {
			return err
		}

		session.LogEvent(fmt.Sprintf("Display rotate: %s [%s]", dir, display.DetectBackend()))
		fmt.Printf("Rotated to %s (%s)\n", dir, display.DetectBackend())
		return nil
	},
}

func init() {
	displayCmd.AddCommand(displayBackendCmd)
	displayCmd.AddCommand(displayOutputsCmd)
	displayCmd.AddCommand(displayResolutionCmd)
	displayCmd.AddCommand(displayScaleCmd)
	displayCmd.AddCommand(displayRotateCmd)
	RootCmd.AddCommand(displayCmd)
}

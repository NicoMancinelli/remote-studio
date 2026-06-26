//go:build linux

package cmd

import (
	"fmt"

	"remote-studio/pkg/audio"

	"github.com/spf13/cobra"
)

var audioCmd = &cobra.Command{
	Use:   "audio [create|destroy|status]",
	Short: "Manage the PipeWire virtual audio sink",
	Long: `Create, destroy, or query the status of the RemoteStudio virtual audio sink.

The virtual sink captures desktop audio for remote streaming and can
optionally mute local physical speakers.

Examples:
  res audio create    Create the virtual sink
  res audio destroy   Remove the virtual sink
  res audio status    Show virtual sink status
  res audio mute      Mute physical speakers
  res audio unmute    Unmute physical speakers`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "create":
			if err := audio.CreateVirtualSink(); err != nil {
				return fmt.Errorf("audio create: %w", err)
			}
			fmt.Printf("✓ Virtual sink '%s' created\n", audio.VirtualSinkName)

		case "destroy":
			if err := audio.RemoveVirtualSink(); err != nil {
				return fmt.Errorf("audio destroy: %w", err)
			}
			fmt.Printf("✓ Virtual sink '%s' removed\n", audio.VirtualSinkName)

		case "status":
			fmt.Println(audio.Status())

		case "mute":
			if err := audio.MutePhysicalOutputs(); err != nil {
				return fmt.Errorf("audio mute: %w", err)
			}
			fmt.Println("✓ Physical outputs muted")

		case "unmute":
			if err := audio.UnmutePhysicalOutputs(); err != nil {
				return fmt.Errorf("audio unmute: %w", err)
			}
			fmt.Println("✓ Physical outputs unmuted")

		default:
			return fmt.Errorf("unknown subcommand %q — use create, destroy, status, mute, or unmute", args[0])
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(audioCmd)
}

//go:build linux

package cmd

import (
	"fmt"
	"sync"

	"remote-studio/pkg/input"

	"github.com/spf13/cobra"
)

// activeDevices tracks the currently active virtual input devices so
// we can destroy them from the CLI.
var (
	activeKeyboard *input.VirtualDevice
	activeMouse    *input.VirtualDevice
	inputMu        sync.Mutex
)

var inputCmd = &cobra.Command{
	Use:   "input [create|destroy|status]",
	Short: "Manage virtual KVM input devices (keyboard + mouse)",
	Long: `Create, destroy, or query the status of the uinput virtual keyboard
and mouse devices used for Remote Studio's software KVM functionality.

These virtual devices allow Remote Studio to inject keyboard and mouse
events into the local display server without a physical input device.

Examples:
  res input create    Create virtual keyboard + mouse
  res input destroy   Destroy virtual input devices
  res input status    Show device status`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "create":
			return inputCreate()
		case "destroy":
			return inputDestroy()
		case "status":
			inputStatus()
			return nil
		default:
			return fmt.Errorf("unknown subcommand %q — use create, destroy, or status", args[0])
		}
	},
}

func inputCreate() error {
	inputMu.Lock()
	defer inputMu.Unlock()

	if activeKeyboard != nil || activeMouse != nil {
		return fmt.Errorf("virtual input devices are already active — destroy them first")
	}

	kb, err := input.CreateVirtualKeyboard("RemoteStudio Virtual Keyboard")
	if err != nil {
		return fmt.Errorf("create keyboard: %w", err)
	}

	ms, err := input.CreateVirtualMouse("RemoteStudio Virtual Mouse")
	if err != nil {
		kb.Close()
		return fmt.Errorf("create mouse: %w", err)
	}

	activeKeyboard = kb
	activeMouse = ms

	fmt.Println("✓ Virtual keyboard created:", kb.Name())
	fmt.Println("✓ Virtual mouse created:   ", ms.Name())
	return nil
}

func inputDestroy() error {
	inputMu.Lock()
	defer inputMu.Unlock()

	if activeKeyboard == nil && activeMouse == nil {
		fmt.Println("No virtual input devices are active.")
		return nil
	}

	var firstErr error
	if activeKeyboard != nil {
		if err := activeKeyboard.Close(); err != nil {
			firstErr = fmt.Errorf("destroy keyboard: %w", err)
		} else {
			fmt.Println("✓ Virtual keyboard destroyed")
		}
		activeKeyboard = nil
	}

	if activeMouse != nil {
		if err := activeMouse.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("destroy mouse: %w", err)
		} else if err == nil {
			fmt.Println("✓ Virtual mouse destroyed")
		}
		activeMouse = nil
	}

	return firstErr
}

func inputStatus() {
	inputMu.Lock()
	defer inputMu.Unlock()

	if activeKeyboard != nil {
		fmt.Printf("✓ Virtual keyboard active: %s\n", activeKeyboard.Name())
	} else {
		fmt.Println("✗ No virtual keyboard active")
	}

	if activeMouse != nil {
		fmt.Printf("✓ Virtual mouse active:    %s\n", activeMouse.Name())
	} else {
		fmt.Println("✗ No virtual mouse active")
	}
}

func init() {
	RootCmd.AddCommand(inputCmd)
}

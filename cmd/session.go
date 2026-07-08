package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"remote-studio/pkg/session"
	"github.com/spf13/cobra"
)

// `cmd/session.go` previously routed `session start` and `session stop`
// through D-Bus when a daemon was detected. That round-trip was a no-op:
// the daemon's StartSession / StopSession handlers call pkg/session
// directly. Worse, the daemon-detection logic used a permissive
// DBus probe that returned true on any system with a running D-Bus,
// even when no Remote Studio daemon was present. The result was that
// CLI invocations would silently drop to the D-Bus branch, fire a
// method call at a non-existent name, and exit 0 without applying
// anything. Several e2e tests (TestTier1_F3_SessionStartBackups,
// TestTier1_F3_SessionStopPowerProfile) failed for exactly this reason.
//
// The CLI now always invokes pkg/session directly. The daemon's D-Bus
// methods remain exposed for the web UI / future remote callers, but
// the CLI no longer needs them.

var sessionCmd = &cobra.Command{
	Use:   "session [start [PROFILE] | stop | status]",
	Short: "Manage remote display session lifecycle",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sub := "status"
		if len(args) > 0 {
			sub = args[0]
		}

		profile := ""
		if len(args) > 1 {
			profile = args[1]
		}

		switch sub {
		case "start":
			if err := session.SessionStart(profile); err != nil {
				return err
			}
		case "stop":
			if err := session.SessionStop(); err != nil {
				return err
			}
		case "status":
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			sessionFile := filepath.Join(home, ".config", "remote-studio", "session.state")
			if data, err := os.ReadFile(sessionFile); err == nil {
				fmt.Print(string(data))
			} else {
				fmt.Println("No active session.")
			}
		default:
			return fmt.Errorf("Usage: res session [start [PROFILE] | stop | status]")
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(sessionCmd)
}

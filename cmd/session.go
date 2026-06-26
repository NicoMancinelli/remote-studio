package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"remote-studio/pkg/session"
	"github.com/godbus/dbus/v5"
	"github.com/spf13/cobra"
)

func isDaemonRunning() bool {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false
	}
	defer conn.Close()

	var owner string
	err = conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, "org.remote_studio.Daemon").Store(&owner)
	return err == nil && owner != ""
}

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
			if isDaemonRunning() {
				conn, err := dbus.ConnectSessionBus()
				if err != nil {
					return err
				}
				defer conn.Close()
				obj := conn.Object("org.remote_studio.Daemon", "/org/remote_studio/Daemon")
				if err := obj.Call("org.remote_studio.Daemon.StartSession", 0, profile).Err; err != nil {
					return fmt.Errorf("dbus call StartSession failed: %w", err)
				}
				fmt.Println("Session start command sent to daemon via D-Bus")
			} else {
				if err := session.SessionStart(profile); err != nil {
					return err
				}
			}
		case "stop":
			if isDaemonRunning() {
				conn, err := dbus.ConnectSessionBus()
				if err != nil {
					return err
				}
				defer conn.Close()
				obj := conn.Object("org.remote_studio.Daemon", "/org/remote_studio/Daemon")
				if err := obj.Call("org.remote_studio.Daemon.StopSession", 0).Err; err != nil {
					return fmt.Errorf("dbus call StopSession failed: %w", err)
				}
				fmt.Println("Session stop command sent to daemon via D-Bus")
			} else {
				if err := session.SessionStop(); err != nil {
					return err
				}
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

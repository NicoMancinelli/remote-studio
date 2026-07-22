package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"remote-studio/pkg/config"

	"github.com/spf13/cobra"
)

// tailscaleNotFoundMessage is printed when the tailscale binary is
// missing AND LAN mode is off. Mirrors the bash version in
// lib/services.sh::show_tailnet.
const tailscaleNotFoundMessage = `tailscale: command not found
Install with: curl -fsSL https://tailscale.com/install.sh | sh
Or enable LAN mode: res config set-toml lan_enabled true`

// lanModeMessage is printed when the user explicitly opted in to
// LAN-only operation and runs a tailnet command.
const lanModeMessage = `LAN mode active — tailnet disabled.
LAN IP: %s
RustDesk direct: %s:21118
Use 'arp -a' or your router's admin UI to list LAN hosts.`

var tailnetCmd = &cobra.Command{
	Use:   "tailnet [peer [NODE] | doctor | hosts | exit-node]",
	Short: "Show tailnet status and details",
	RunE: func(cmd *cobra.Command, args []string) error {
		sub := ""
		if len(args) > 0 {
			sub = args[0]
		}

		// LAN mode: the tailnet is intentionally absent. Print a clear
		// diagnostic and the LAN IP so the user knows where to point
		// RustDesk. Mirrors the bash helper in lib/services.sh.
		if config.LANModeActive() {
			fmt.Printf(lanModeMessage+"\n", getLanIP(), getLanIP())
			return nil
		}

		// No tailscale binary AND not in LAN mode. Tell the user how
		// to fix it. We check this BEFORE running any subcommand so
		// the user gets a clean error instead of empty output.
		if _, err := exec.LookPath("tailscale"); err != nil {
			fmt.Println(tailscaleNotFoundMessage)
			return fmt.Errorf("tailscale binary not found")
		}

		switch sub {
		case "peer":
			peer := ""
			if len(args) > 1 {
				peer = args[1]
			}
			if peer == "" {
				out, _ := exec.Command("tailscale", "status", "--peers=true").Output()
				// output first 20 lines
				lines := strings.Split(string(out), "\n")
				for i := 0; i < len(lines) && i < 20; i++ {
					fmt.Println(lines[i])
				}
				return nil
			}
			fmt.Printf("Checking %s...\n", peer)
			// tailscale ping
			pingOut, _ := exec.Command("tailscale", "ping", peer).Output()
			fmt.Print(string(pingOut))
			fmt.Println()
			// tailscale status | grep -i peer
			statusOut, _ := exec.Command("tailscale", "status").Output()
			lines := strings.Split(string(statusOut), "\n")
			for _, line := range lines {
				if strings.Contains(strings.ToLower(line), strings.ToLower(peer)) {
					fmt.Println(line)
				}
			}
		case "doctor":
			fmt.Println("Tailnet Doctor")
			cmd := exec.Command("tailscale", "netcheck")
			cmd.Stdout = os.Stdout
			out, _ := cmd.CombinedOutput()
			fmt.Print(string(out))
		case "hosts":
			fmt.Println("Tailnet peers:")
			out, err := exec.Command("tailscale", "status", "--peers=true").Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				if len(lines) > 1 {
					for _, line := range lines[1:] {
						fields := strings.Fields(line)
						if len(fields) >= 2 {
							fmt.Printf("  %-20s %s\n", fields[1], fields[0])
						}
					}
				}
			}
		case "exit-node":
			out, _ := exec.Command("tailscale", "exit-node", "list").Output()
			lines := strings.Split(string(out), "\n")
			exitNode := "none"
			for _, line := range lines {
				if strings.Contains(line, "selected") {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						exitNode = fields[0]
						break
					}
				}
			}
			fmt.Printf("Exit node: %s\n", exitNode)
		case "":
			ip := getTailscaleIP()
			if ip == "" {
				return fmt.Errorf("Tailscale IPv4 unavailable")
			}
			fmt.Printf("Tailscale IP: %s\n", ip)
			fmt.Printf("RustDesk direct: %s:21118\n", ip)

			// exit-node
			out, _ := exec.Command("tailscale", "exit-node", "list").Output()
			lines := strings.Split(string(out), "\n")
			exitNode := "none"
			for _, line := range lines {
				if strings.Contains(line, "selected") {
					fields := strings.Fields(line)
					if len(fields) > 0 {
						exitNode = fields[0]
						break
					}
				}
			}
			fmt.Printf("Exit node: %s\n", exitNode)
		default:
			return fmt.Errorf("Usage: res tailnet [peer [NODE] | doctor | hosts | exit-node]")
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(tailnetCmd)
}

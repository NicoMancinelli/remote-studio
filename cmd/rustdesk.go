package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"remote-studio/pkg/config"
	"remote-studio/pkg/status"
	"github.com/spf13/cobra"
)

func getRustdeskFiles(preset string) (string, string, string, string) {
	home, _ := os.UserHomeDir()
	configFile := filepath.Join(home, ".config", "rustdesk", "RustDesk_default.toml")
	optionsFile := filepath.Join(home, ".config", "rustdesk", "RustDesk2.options.toml")

	configDir := config.FindConfigDir()

	optionsSource := filepath.Join(configDir, "RustDesk2.options.toml")
	if _, err := os.Stat(optionsSource); os.IsNotExist(err) {
		optionsSource = "/usr/share/remote-studio/RustDesk2.options.toml"
	}

	sourceFile := filepath.Join(configDir, fmt.Sprintf("RustDesk_%s.toml", preset))
	if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
		sourceFile = fmt.Sprintf("/usr/share/remote-studio/RustDesk_%s.toml", preset)
	}

	return configFile, optionsFile, optionsSource, sourceFile
}

func mergeRustdeskConfig(sourcePath, targetPath string) error {
	preserve := []string{"id", "key", "password", "salt", "relay-server", "api-server"}
	preservedValues := make(map[string]string)

	// Read target values to preserve
	if data, err := os.ReadFile(targetPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			for _, field := range preserve {
				if strings.HasPrefix(strings.TrimSpace(line), field+" =") {
					preservedValues[field] = line
				}
			}
		}
	}

	// Read source config
	srcData, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}

	var mergedLines []string
	srcLines := strings.Split(string(srcData), "\n")
	appliedFields := make(map[string]bool)

	for _, line := range srcLines {
		trimmed := strings.TrimSpace(line)
		replaced := false
		for _, field := range preserve {
			if strings.HasPrefix(trimmed, field+" =") {
				if val, ok := preservedValues[field]; ok {
					mergedLines = append(mergedLines, val)
					appliedFields[field] = true
					replaced = true
					break
				}
			}
		}
		if !replaced {
			mergedLines = append(mergedLines, line)
		}
	}

	// Append any preserved fields that were not in source
	for _, field := range preserve {
		if val, ok := preservedValues[field]; ok && !appliedFields[field] {
			mergedLines = append(mergedLines, val)
		}
	}

	return os.WriteFile(targetPath, []byte(strings.Join(mergedLines, "\n")), 0644)
}

var rustdeskCmd = &cobra.Command{
	Use:   "rustdesk [backup | diff [PRESET] | apply [PRESET] | status | log [LINES]]",
	Short: "Manage RustDesk config templates and telemetry status",
	RunE: func(cmd *cobra.Command, args []string) error {
		sub := "status"
		if len(args) > 0 {
			sub = args[0]
		}

		preset := "default"
		if len(args) > 1 {
			preset = args[1]
		}

		configFile, optionsFile, optionsSource, sourceFile := getRustdeskFiles(preset)

		switch sub {
		case "backup":
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				fmt.Println("No config.")
				return nil
			}
			backupPath := fmt.Sprintf("%s.bak.%s", configFile, time.Now().Format("2006-01-02_15-04-05"))
			data, err := os.ReadFile(configFile)
			if err != nil {
				return err
			}
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				return err
			}
			fmt.Println("Backed up.")

		case "diff":
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				return fmt.Errorf("Missing files (preset: %s)", preset)
			}
			if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
				return fmt.Errorf("Missing files (preset: %s)", preset)
			}
			diffCmd := exec.Command("diff", "--color=always", "-u", configFile, sourceFile)
			out, _ := diffCmd.CombinedOutput()
			fmt.Print(string(out))

		case "apply":
			if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
				return fmt.Errorf("No template %s.", sourceFile)
			}
			_ = os.MkdirAll(filepath.Dir(configFile), 0755)

			preApplyFile := configFile + ".pre-apply"
			_ = os.Remove(preApplyFile)
			if _, err := os.Stat(configFile); err == nil {
				// backup
				data, _ := os.ReadFile(configFile)
				_ = os.WriteFile(preApplyFile, data, 0644)
			}

			if err := mergeRustdeskConfig(sourceFile, configFile); err != nil {
				return err
			}
			fmt.Printf("Merged %s (Identity preserved).\n", preset)

			if _, err := os.Stat(optionsSource); err == nil {
				optData, _ := os.ReadFile(optionsSource)
				_ = os.WriteFile(optionsFile, optData, 0644)
				fmt.Println("Merged RustDesk2.options (options only, no identity).")
			}

			// check if config changed
			changed := true
			if _, err := os.Stat(preApplyFile); err == nil {
				c1, _ := os.ReadFile(configFile)
				c2, _ := os.ReadFile(preApplyFile)
				if string(c1) == string(c2) {
					changed = false
				}
			}

			if !changed {
				fmt.Println("Configuration unchanged. Skipping restart.")
			} else {
				fmt.Println("Configuration changed. Restarting rustdesk...")
				_ = exec.Command("sudo", "systemctl", "restart", "rustdesk").Run()
			}

		case "status":
			users, connType, _ := status.GetActiveUsersAndConnection()
			fmt.Printf("Active sessions : %d\n", users)

			if users > 0 {
				fmt.Printf("Connection type : %s\n", connType)

				// Get remote IP and local port from ss
				ssOut, err := exec.Command("ss", "-tnp").Output()
				if err == nil {
					lines := strings.Split(string(ssOut), "\n")
					remoteIP := ""
					localPort := ""
					for _, line := range lines {
						if strings.Contains(line, "ESTAB") && strings.Contains(line, "rustdesk") {
							fields := strings.Fields(line)
							if len(fields) >= 5 {
								addr := fields[4]
								ip := addr
								if idx := strings.LastIndex(addr, ":"); idx != -1 {
									ip = addr[:idx]
								}
								remoteIP = strings.TrimSuffix(strings.TrimPrefix(ip, "["), "]")

								localAddr := fields[3]
								if idx := strings.LastIndex(localAddr, ":"); idx != -1 {
									localPort = localAddr[idx+1:]
								}
								break
							}
						}
					}
					if remoteIP != "" {
						fmt.Printf("Remote IP       : %s\n", remoteIP)
					}
					if localPort != "" {
						fmt.Printf("Local port      : %s\n", localPort)
					}
				}
			}

			// recent codec/perf events
			home, _ := os.UserHomeDir()
			logFile := filepath.Join(home, ".local", "share", "rustdesk", "log", "rustdesk.log")
			if _, err := os.Stat(logFile); os.IsNotExist(err) {
				logFile = filepath.Join(home, ".rustdesk", "log", "rustdesk.log")
			}

			if _, err := os.Stat(logFile); err == nil {
				fmt.Println("\n-- Recent codec/perf events (last 50 log lines) --")
				data, _ := os.ReadFile(logFile)
				lines := strings.Split(string(data), "\n")
				start := len(lines) - 50
				if start < 0 {
					start = 0
				}
				codec := ""
				fps := ""
				bitrate := ""
				for i := start; i < len(lines); i++ {
					lineLower := strings.ToLower(lines[i])
					if strings.Contains(lineLower, "codec") {
						codec = lines[i]
					}
					if strings.Contains(lineLower, "fps") {
						fps = lines[i]
					}
					if strings.Contains(lineLower, "bitrate") {
						bitrate = lines[i]
					}
				}
				if codec != "" {
					fmt.Printf("  Codec   : %s\n", codec)
				}
				if fps != "" {
					fmt.Printf("  FPS     : %s\n", fps)
				}
				if bitrate != "" {
					fmt.Printf("  Bitrate : %s\n", bitrate)
				}
				if codec == "" && fps == "" && bitrate == "" {
					fmt.Println("  (no codec/fps/bitrate found in last 50 lines)")
				}
			} else {
				fmt.Println("(RustDesk log not found — check ~/.local/share/rustdesk/log/)")
			}

		case "log":
			nlines := 50
			if len(args) > 1 {
				if n, err := strconv.Atoi(args[1]); err == nil {
					nlines = n
				}
			}
			out, err := exec.Command("journalctl", "-u", "rustdesk", "-n", strconv.Itoa(nlines), "--no-pager").Output()
			if err != nil {
				fmt.Println("journalctl unavailable.")
			} else {
				fmt.Print(string(out))
			}

		default:
			return fmt.Errorf("Usage: res rustdesk [backup | diff [PRESET] | apply [PRESET] | status | log [LINES]]")
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(rustdeskCmd)
}

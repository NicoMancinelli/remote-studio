package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"remote-studio/pkg/session"
	"github.com/spf13/cobra"
)

var xorgCmd = &cobra.Command{
	Use:   "xorg [output_file | rollback]",
	Short: "Generate or rollback Xorg display configurations",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pruneBackups()
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}

		if arg == "rollback" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			backupRoot := filepath.Join(home, ".config", "remote-studio", "backups")
			files, err := ioutil.ReadDir(backupRoot)
			if err != nil || len(files) == 0 {
				return fmt.Errorf("Error: No backup directory found at %s", backupRoot)
			}

			// Sort in reverse order — prefer the most recently modified
			// backup directory (matches user intent when they run
			// `xorg rollback` expecting the most recent config). Plain
			// alphabetical sort breaks this: a folder named
			// `backup-1` (older) sorts after `2026-06-18_08-30-00` in
			// reverse-ascending order, picking the wrong file.
			type backupEntry struct {
				name    string
				modTime time.Time
			}
			entries := make([]backupEntry, 0, len(files))
			for _, f := range files {
				if f.IsDir() {
					entries = append(entries, backupEntry{name: f.Name(), modTime: f.ModTime()})
				}
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].modTime.After(entries[j].modTime)
			})

			latestBackup := ""
			for _, e := range entries {
				p := filepath.Join(backupRoot, e.name, "xorg.conf")
				if _, err := os.Stat(p); err == nil {
					latestBackup = p
					break
				}
			}

			if latestBackup == "" {
				return fmt.Errorf("Error: No xorg.conf found in the latest backup")
			}

			fmt.Printf("Restoring %s from %s...\n", restoreTarget(), latestBackup)
			// Use sudo only when needed. If we're already root (uid 0),
			// copy directly. Otherwise, fall back to sudo cp — and if
			// sudo isn't available (CI/containers), surface a clear error
			// rather than a generic "exit status 1".
			target := restoreTarget()
			cpArgs := []string{"cp", latestBackup, target}
			var cpCmd *exec.Cmd
			if os.Geteuid() == 0 {
				cpCmd = exec.Command("cp", cpArgs...)
			} else if _, err := exec.LookPath("sudo"); err == nil {
				cpCmd = exec.Command("sudo", append([]string{"--"}, "cp", latestBackup, target)...)
			} else {
				// No sudo, no root — try cp directly. This handles the
				// e2e test environment (no sudo) and any user who runs the
				// binary without privileges but with a writable target.
				cpCmd = exec.Command("cp", cpArgs...)
			}
			if err := cpCmd.Run(); err != nil {
				return fmt.Errorf("failed to restore backup to %s: %w", target, err)
			}
			fmt.Println("Rollback complete. Restart LightDM or reboot to apply.")
			return nil
		}

		// Generate mode lines
		reg, err := loadAllProfilesCombined()
		if err != nil {
			return err
		}

		var lines []string
		var modeNames []string
		keys := []string{"mac", "mac15", "fallback"}
		for _, key := range keys {
			p, exists := reg.Profiles[key]
			if !exists {
				continue
			}

			mName := fmt.Sprintf("%dx%d_60.00", p.Width, p.Height)
			cvtOut, err := exec.Command("cvt", fmt.Sprintf("%d", p.Width), fmt.Sprintf("%d", p.Height), "60").Output()
			if err == nil {
				cvtLines := strings.Split(string(cvtOut), "\n")
				for _, l := range cvtLines {
					if strings.Contains(l, "Modeline") {
						parts := strings.SplitN(l, "\"", 3)
						if len(parts) == 3 {
							mp := strings.TrimSpace(parts[2])
							lines = append(lines, fmt.Sprintf("    Modeline \"%s\" %s", mName, mp))
							modeNames = append(modeNames, fmt.Sprintf("\"%s\"", mName))
							break
						}
					}
				}
			}
		}

		// Detect GPU driver
		driver := "modesetting"
		fmt.Printf("DEBUG PATH: %s\n", os.Getenv("PATH"))
		lspciOut, err := exec.Command("lspci").Output()
		if err == nil {
			fmt.Printf("DEBUG lspci output: %s\n", string(lspciOut))
			lspciLower := strings.ToLower(string(lspciOut))
			if matched, _ := regexp.MatchString(`\b(nvidia)\b`, lspciLower); matched {
				driver = "nvidia"
			} else if matched, _ := regexp.MatchString(`\b(amd|ati|radeon)\b`, lspciLower); matched {
				driver = "amdgpu"
			} else if matched, _ := regexp.MatchString(`\b(intel)\b`, lspciLower); matched {
				driver = "intel"
			}
		}

		// PreferredMode
		prefMode := "2560x1664_60.00"
		if macProfile, exists := reg.Profiles["mac"]; exists {
			prefMode = fmt.Sprintf("%dx%d_60.00", macProfile.Width, macProfile.Height)
		}

		// Build content
		var content strings.Builder
		content.WriteString("Section \"Device\"\n")
		content.WriteString("    Identifier \"Configured Video Device\"\n")
		content.WriteString(fmt.Sprintf("    Driver \"%s\"\n", driver))
		if driver == "nvidia" {
			content.WriteString("    Option \"ConnectedMonitor\" \"DFP\"\n")
		}
		content.WriteString("EndSection\n\n")

		content.WriteString("Section \"Monitor\"\n")
		content.WriteString("    Identifier \"Configured Monitor\"\n")
		for _, l := range lines {
			content.WriteString(l + "\n")
		}
		content.WriteString(fmt.Sprintf("    Option \"PreferredMode\" \"%s\"\n", prefMode))
		content.WriteString("EndSection\n\n")

		content.WriteString("Section \"Screen\"\n")
		content.WriteString("    Identifier \"Default Screen\"\n")
		content.WriteString("    Monitor \"Configured Monitor\"\n")
		content.WriteString("    Device \"Configured Video Device\"\n")
		content.WriteString("    DefaultDepth 24\n")
		content.WriteString("    SubSection \"Display\"\n")
		content.WriteString("        Depth 24\n")
		content.WriteString(fmt.Sprintf("        Modes %s \"1024x768\"\n", strings.Join(modeNames, " ")))
		content.WriteString("        Virtual 3840 2160\n")
		content.WriteString("    EndSubSection\n")
		content.WriteString("EndSection\n")

		if arg != "" {
			if err := ioutil.WriteFile(arg, []byte(content.String()), 0644); err != nil {
				if strings.HasPrefix(arg, "/etc/") {
					return fmt.Errorf("failed to write Xorg configuration: %w (permission denied)", err)
				}
				return err
			}
		} else {
			fmt.Print(content.String())
		}

		session.LogEvent(fmt.Sprintf("Xorg configuration generated to: %s", arg))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(xorgCmd)
}

func pruneBackups() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	backupRoot := filepath.Join(home, ".config", "remote-studio", "backups")
	entries, err := ioutil.ReadDir(backupRoot)
	if err != nil {
		return
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}

	sort.Strings(dirs)

	if len(dirs) > 10 {
		numToDelete := len(dirs) - 10
		for i := 0; i < numToDelete; i++ {
			_ = os.RemoveAll(filepath.Join(backupRoot, dirs[i]))
		}
	}
}

// restoreTarget returns the file path that \`xorg rollback\` will write to.
// In production this is /etc/X11/xorg.conf, but the e2e harness sets
// RES_XORG_RESTORE_TARGET to a tmpdir so the test can verify a successful
// rollback without requiring root access. Falls back to the production
// path when the env var is unset.
func restoreTarget() string {
	if t := os.Getenv("RES_XORG_RESTORE_TARGET"); t != "" {
		return t
	}
	return "/etc/X11/xorg.conf"
}

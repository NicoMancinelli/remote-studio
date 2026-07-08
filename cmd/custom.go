package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"remote-studio/pkg/session"
	"github.com/spf13/cobra"
)

func isTerminal() bool {
	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

var customCmd = &cobra.Command{
	Use:   "custom <width> <height> [scale]",
	Short: "Apply a custom resolution and scaling setting",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
			return cmd.Help()
		}
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("accepts between 2 and 3 arg(s), received %d", len(args))
		}

		w, err := strconv.Atoi(args[0])
		if err != nil || w <= 0 {
			return fmt.Errorf("invalid width: %s", args[0])
		}

		h, err := strconv.Atoi(args[1])
		if err != nil || h <= 0 {
			return fmt.Errorf("invalid height: %s", args[1])
		}

		if w > 5000 || h > 5000 {
			return fmt.Errorf("dimensions exceed limit (5000x5000): %dx%d", w, h)
		}

		scale := 1.0
		if len(args) > 2 {
			s, err := strconv.ParseFloat(args[2], 64)
			if err != nil || s <= 0 {
				return fmt.Errorf("invalid scaling setting: %s", args[2])
			}
			scale = s
		}

		// Headless check (after arg validation, so negative/invalid args still
		// get their dedicated error messages). We check env vars directly so
		// a fake xrandr cannot trick this gate in test environments.
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return fmt.Errorf("no display server detected (DISPLAY and WAYLAND_DISPLAY are both unset)")
		}

		textScale := 1.0
		if scale > 1.0 {
			textScale = 1.5
		}
		cursor := int(24 * scale)

		label := fmt.Sprintf("Custom %dx%d", w, h)
		if err := session.ApplyAll(w, h, scale, textScale, cursor, label); err != nil {
			return err
		}

		if isTerminal() {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Save as profile? [y/N] ")
			ans, _ := reader.ReadString('\n')
			ans = strings.TrimSpace(strings.ToLower(ans))
			if ans == "y" || ans == "yes" {
				fmt.Print("Profile key (e.g. 'work'): ")
				pkey, _ := reader.ReadString('\n')
				pkey = strings.TrimSpace(pkey)

				matched, _ := regexp.MatchString("^[a-z][a-z0-9_-]*$", pkey)
				if !matched {
					fmt.Printf("Invalid key '%s': use lowercase letters, digits, hyphens, underscores (e.g. 'work')\n", pkey)
				} else {
					home, err := os.UserHomeDir()
					if err == nil {
						userProfilesDir := filepath.Join(home, ".config", "remote-studio")
						userProfilesPath := filepath.Join(userProfilesDir, "profiles.conf")
						_ = os.MkdirAll(userProfilesDir, 0755)

						f, err := os.OpenFile(userProfilesPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
						if err == nil {
							defer f.Close()
							line := fmt.Sprintf("%s=Custom %dx%d|%d|%d|%g|%g|%d\n", pkey, w, h, w, h, scale, textScale, cursor)
							_, _ = f.WriteString(line)
							fmt.Printf("Saved to %s\n", userProfilesPath)
							session.LogEvent(fmt.Sprintf("Profile saved: %s %dx%d", pkey, w, h))
						}
					}
				}
			}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(customCmd)
}

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"remote-studio/pkg/config"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List available device profiles",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := loadAllProfilesCombined()
		if err != nil {
			return err
		}

		curMode := getCurrentMode()
		sortedKeys := config.SortProfileKeys(reg)

		fmt.Printf("%-12s %-26s %-14s %s\n", "KEY", "LABEL", "RESOLUTION", "SOURCE")
		fmt.Printf("%-12s %-26s %-14s %s\n", "---", "-----", "----------", "------")

		for _, k := range sortedKeys {
			profile := reg.Profiles[k]
			src := "default"
			if isUserKey(k) {
				src = "user"
			}
			activeMarker := ""
			if profile.Label == curMode {
				activeMarker = " *"
			}
			resStr := fmt.Sprintf("%dx%d@%s", profile.Width, profile.Height, strconv.FormatFloat(profile.Scaling, 'f', -1, 64))
			fmt.Printf("%-12s %-26s %-14s %s%s\n", k, profile.Label, resStr, src, activeMarker)
		}
		return nil
	},
}

func loadAllProfilesCombined() (*config.ProfileRegistry, error) {
	reg := config.NewProfileRegistry()

	defaultPath, _ := config.ResolveProfilesPath()
	if _, err := os.Stat(defaultPath); err == nil {
		_ = reg.LoadProfiles(defaultPath)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".config", "remote-studio", "profiles.conf")
		if _, err := os.Stat(userPath); err == nil {
			_ = reg.LoadProfiles(userPath)
		}
	}

	return reg, nil
}

func isUserKey(key string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	userPath := filepath.Join(home, ".config", "remote-studio", "profiles.conf")
	file, err := os.Open(userPath)
	if err != nil {
		return false
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) > 0 && strings.TrimSpace(parts[0]) == key {
			return true
		}
	}
	return false
}

func getCurrentMode() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "None"
	}
	statePath := filepath.Join(home, ".res_state")
	file, err := os.Open(statePath)
	if err != nil {
		return "None"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "state=") {
			parts := strings.Split(line, "'")
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	return "None"
}

func init() {
	RootCmd.AddCommand(profilesCmd)
}

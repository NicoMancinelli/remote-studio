package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"remote-studio/pkg/config"
	"github.com/spf13/cobra"
)

var keyRegex = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show all configuration settings",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _, err := config.FindAndLoadConfig()
		if err != nil {
			return err
		}
		var keys []string
		for k := range cfg.Values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("%s=%s\n", k, cfg.Values[k])
		}
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Get a configuration setting",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		if !keyRegex.MatchString(key) {
			return fmt.Errorf("invalid key format: %s", key)
		}
		cfg, _, err := config.FindAndLoadConfig()
		if err != nil {
			return err
		}
		val, exists := cfg.Values[key]
		if exists {
			fmt.Println(val)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set KEY VALUE",
	Short: "Set a configuration setting",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]
		if !keyRegex.MatchString(key) {
			return fmt.Errorf("invalid key format: %s", key)
		}

		return writeConfigValue(key, value)
	},
}

func writeConfigValue(key, value string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".config", "remote-studio")
	configPath := filepath.Join(configDir, "remote-studio.conf")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	var lines []string
	found := false

	file, err := os.Open(configPath)
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") || trimmed == "" {
				lines = append(lines, line)
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				if k == key {
					lines = append(lines, fmt.Sprintf("%s=%s", key, value))
					found = true
					continue
				}
			}
			lines = append(lines, line)
		}
		file.Close()
	}

	if !found {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	tmpFile, err := os.CreateTemp(configDir, "config-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	writer := bufio.NewWriter(tmpFile)
	for _, l := range lines {
		if _, err := writer.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	return os.Rename(tmpFile.Name(), configPath)
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	RootCmd.AddCommand(configCmd)
}

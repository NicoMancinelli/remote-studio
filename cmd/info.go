package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"remote-studio/pkg/config"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show environment variables and active configuration settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Remote Studio Info")
		fmt.Println("\nActive Configuration Settings:")
		cfg, path, _ := config.FindAndLoadConfig()
		fmt.Printf("Source File: %s\n", path)

		// Print sorted config values
		var keys []string
		for k := range cfg.Values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s=%s\n", k, cfg.Values[k])
		}

		fmt.Println("\nRelevant Environment Variables:")
		// Print relevant env vars
		envKeys := []string{"HOME", "DISPLAY", "XDG_RUNTIME_DIR", "AUTO_SESSION"}
		for _, env := range os.Environ() {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				if strings.HasPrefix(key, "REMOTE_STUDIO_") || strings.HasPrefix(key, "RES_") {
					envKeys = append(envKeys, key)
				}
			}
		}
		// Dedup and sort envKeys
		uniqueEnvKeys := make(map[string]bool)
		var sortedEnvKeys []string
		for _, k := range envKeys {
			if !uniqueEnvKeys[k] {
				uniqueEnvKeys[k] = true
				sortedEnvKeys = append(sortedEnvKeys, k)
			}
		}
		sort.Strings(sortedEnvKeys)

		for _, k := range sortedEnvKeys {
			val := os.Getenv(k)
			if val == "" {
				fmt.Printf("  %s is not set\n", k)
			} else {
				fmt.Printf("  %s=%s\n", k, val)
			}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(infoCmd)
}

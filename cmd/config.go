package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

// ---------- res config get-toml ----------

// configGetTOMLCmd looks up a single field in remote-studio.toml and writes
// just its string value to stdout. Returns exit code 0 + empty stdout when
// the key is unset or the TOML file doesn't exist (so the caller can do
// `val=$(res config get-toml xorg_driver)` without checking errors).
//
// Recognised keys:
//   xorg_driver       – display.xorg_driver
//   default_backend   – display.default_backend
//   default_profile   – display.default_profile
//   auto_session      – general.auto_session
//   log_level         – general.log_level
//   poll_interval     – daemon.poll_interval
//   websocket_port    – daemon.websocket_port
//   http_port         – daemon.http_port
var configGetTOMLCmd = &cobra.Command{
	Use:   "get-toml KEY",
	Short: "Get a TOML configuration value",
	Long: `Read a single field from remote-studio.toml. Prints just the value
to stdout (no key, no quotes, no decoration). Exits 0 with empty stdout
when the file doesn't exist or the key is unset — so shell callers can do
'val=$(res config get-toml xorg_driver)' and get a clean empty string
when there's no value.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.ResolveTOMLConfigPath()
		cfg, err := loadTOMLOrDefault(path)
		if err != nil {
			return err
		}
		val, err := getTOMLField(cfg, args[0])
		if err != nil {
			return err
		}
		fmt.Println(val)
		return nil
	},
}

// getTOMLField maps a flat KEY to its TOML struct location and returns the
// string representation. Empty string is returned for unset fields (which
// is what callers want — bash uses XORG_DRIVER=${XORG_DRIVER:-fallback}).
func getTOMLField(cfg *config.TOMLConfig, key string) (string, error) {
	switch key {
	case "xorg_driver":
		return cfg.Display.XorgDriver, nil
	case "default_backend":
		return cfg.Display.DefaultBackend, nil
	case "default_profile":
		return cfg.Display.DefaultProfile, nil
	case "auto_session":
		if cfg.General.AutoSession {
			return "true", nil
		}
		return "false", nil
	case "log_level":
		return cfg.General.LogLevel, nil
	case "poll_interval":
		return strconv.Itoa(cfg.Daemon.PollInterval), nil
	case "websocket_port":
		return strconv.Itoa(cfg.Daemon.WebsocketPort), nil
	case "http_port":
		return strconv.Itoa(cfg.Daemon.HTTPPort), nil
	default:
		return "", fmt.Errorf("unknown TOML key: %q (valid keys: xorg_driver, default_backend, default_profile, auto_session, log_level, poll_interval, websocket_port, http_port)", key)
	}
}

// ---------- res config set-toml ----------

var configSetTOMLCmd = &cobra.Command{
	Use:   "set-toml KEY VALUE",
	Short: "Set a TOML configuration value",
	Long: `Update a single field in remote-studio.toml. Loads the existing
file (or defaults), mutates the field, and writes the result back. Creates
~/.config/remote-studio/ if it doesn't exist.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.ResolveUserTOMLConfigPath()

		// Ensure the parent directory exists before we try to read/write.
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		cfg, err := loadTOMLOrDefault(path)
		if err != nil {
			return err
		}

		if err := setTOMLField(cfg, args[0], args[1]); err != nil {
			return err
		}

		// Validate before writing so a bad value doesn't silently corrupt
		// the config.
		warns := config.ValidateConfig(cfg)
		for _, w := range warns {
			if strings.HasPrefix(w, "display.xorg_driver") ||
				strings.HasPrefix(w, "display.default_backend") ||
				strings.HasPrefix(w, "display.default_profile") {
				fmt.Fprintf(os.Stderr, "warning: %s\n", w)
			}
		}

		if err := config.SaveTOMLConfig(path, cfg); err != nil {
			return err
		}
		fmt.Printf("✓ %s.%s = %s (saved to %s)\n",
			tomlSectionForKey(args[0]), args[0], args[1], path)
		return nil
	},
}

// tomlSectionForKey returns the [section] header a key lives under. Used
// only for the success message.
func tomlSectionForKey(key string) string {
	switch key {
	case "xorg_driver", "default_backend", "default_profile":
		return "display"
	case "auto_session", "log_level", "log_path":
		return "general"
	case "poll_interval", "websocket_port", "http_port", "socket_activated":
		return "daemon"
	case "virtual_sink_name", "auto_mute_physical":
		return "audio"
	case "trust_tailscale", "allowed_ips":
		return "security"
	default:
		return "?"
	}
}

func setTOMLField(cfg *config.TOMLConfig, key, value string) error {
	switch key {
	case "xorg_driver":
		cfg.Display.XorgDriver = value
	case "default_backend":
		cfg.Display.DefaultBackend = value
	case "default_profile":
		cfg.Display.DefaultProfile = value
	case "auto_session":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("auto_session: %w", err)
		}
		cfg.General.AutoSession = b
	case "log_level":
		cfg.General.LogLevel = value
	case "poll_interval":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("poll_interval: %w", err)
		}
		cfg.Daemon.PollInterval = n
	case "websocket_port":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("websocket_port: %w", err)
		}
		cfg.Daemon.WebsocketPort = n
	case "http_port":
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("http_port: %w", err)
		}
		cfg.Daemon.HTTPPort = n
	default:
		return fmt.Errorf("unknown TOML key: %q (valid keys: xorg_driver, default_backend, default_profile, auto_session, log_level, poll_interval, websocket_port, http_port)", key)
	}
	return nil
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowTOMLCmd)
	configCmd.AddCommand(configValidateTOMLCmd)
	configCmd.AddCommand(configInitTOMLCmd)
	configCmd.AddCommand(configGetTOMLCmd)
	configCmd.AddCommand(configSetTOMLCmd)
	RootCmd.AddCommand(configCmd)
}

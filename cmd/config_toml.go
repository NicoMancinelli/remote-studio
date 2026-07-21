package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"remote-studio/pkg/config"

	"github.com/spf13/cobra"
)

// ---------- res config show-toml ----------

var configShowTOMLCmd = &cobra.Command{
	Use:   "show-toml",
	Short: "Show the active TOML configuration",
	Long:  `Load and display the active remote-studio.toml configuration, merging defaults with user overrides.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		path := config.ResolveTOMLConfigPath()

		cfg, err := loadTOMLOrDefault(path)
		if err != nil {
			return err
		}

		if jsonFlag {
			b, err := json.MarshalIndent(cfg, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}

		printTOMLConfig(cfg, path)
		return nil
	},
}

// ---------- res config validate-toml ----------

var configValidateTOMLCmd = &cobra.Command{
	Use:   "validate-toml",
	Short: "Validate the TOML configuration",
	Long:  `Load remote-studio.toml and check for common misconfigurations or invalid values.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.ResolveTOMLConfigPath()

		cfg, err := loadTOMLOrDefault(path)
		if err != nil {
			return err
		}

		warns := config.ValidateConfig(cfg)
		if len(warns) == 0 {
			fmt.Printf("✓ %s is valid\n", path)
			return nil
		}

		fmt.Printf("⚠ %d issue(s) found in %s:\n\n", len(warns), path)
		for i, w := range warns {
			fmt.Printf("  %d. %s\n", i+1, w)
		}
		fmt.Println()
		return fmt.Errorf("config validation failed with %d warning(s)", len(warns))
	},
}

// ---------- res config init-toml ----------

var configInitTOMLCmd = &cobra.Command{
	Use:   "init-toml",
	Short: "Create a default remote-studio.toml",
	Long:  `Write a default remote-studio.toml to ~/.config/remote-studio/ if one does not exist.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := config.ResolveUserTOMLConfigPath()

		forceFlag, _ := cmd.Flags().GetBool("force")

		if _, err := os.Stat(path); err == nil && !forceFlag {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		cfg := config.DefaultTOMLConfig()
		if err := config.SaveTOMLConfig(path, cfg); err != nil {
			return err
		}
		fmt.Printf("✓ Wrote default config to %s\n", path)
		return nil
	},
}

// ---------- helpers ----------

func loadTOMLOrDefault(path string) (*config.TOMLConfig, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "note: %s not found, showing defaults\n", path)
		return config.DefaultTOMLConfig(), nil
	}
	return config.LoadTOMLConfig(path)
}

func printTOMLConfig(cfg *config.TOMLConfig, path string) {
	sep := strings.Repeat("─", 56)

	fmt.Printf("TOML config: %s\n", path)
	fmt.Println(sep)

	// General
	fmt.Println()
	fmt.Println("  [general]")
	fmt.Printf("    version          = %s\n", cfg.General.Version)
	fmt.Printf("    auto_session     = %s\n", fmtBoolDisplay(cfg.General.AutoSession))
	fmt.Printf("    log_level        = %s\n", cfg.General.LogLevel)
	fmt.Printf("    log_path         = %s\n", cfg.General.LogPath)

	// Display
	fmt.Println()
	fmt.Println("  [display]")
	fmt.Printf("    default_backend  = %s\n", cfg.Display.DefaultBackend)
	fmt.Printf("    default_profile  = %s\n", cfg.Display.DefaultProfile)
	if cfg.Display.XorgDriver != "" {
		fmt.Printf("    xorg_driver      = %s\n", cfg.Display.XorgDriver)
	} else {
		fmt.Println("    xorg_driver      = (auto-detect)")
	}

	// Daemon
	fmt.Println()
	fmt.Println("  [daemon]")
	fmt.Printf("    poll_interval    = %d\n", cfg.Daemon.PollInterval)
	fmt.Printf("    websocket_port   = %d\n", cfg.Daemon.WebsocketPort)
	fmt.Printf("    http_port        = %d\n", cfg.Daemon.HTTPPort)
	fmt.Printf("    socket_activated = %s\n", fmtBoolDisplay(cfg.Daemon.SocketActivated))

	// LAN
	if cfg.LAN.Enabled || cfg.LAN.BindAddress != "" {
		fmt.Println()
		fmt.Println("  [lan]")
		fmt.Printf("    enabled       = %s\n", fmtBoolDisplay(cfg.LAN.Enabled))
		if cfg.LAN.BindAddress != "" {
			fmt.Printf("    bind_address  = %s\n", cfg.LAN.BindAddress)
		} else {
			fmt.Println("    bind_address  = (0.0.0.0 default)")
		}
	}

	// Audio
	fmt.Println()
	fmt.Println("  [audio]")
	fmt.Printf("    virtual_sink_name   = %s\n", cfg.Audio.VirtualSinkName)
	fmt.Printf("    auto_mute_physical  = %s\n", fmtBoolDisplay(cfg.Audio.AutoMutePhysical))

	// Security
	fmt.Println()
	fmt.Println("  [security]")
	fmt.Printf("    trust_tailscale  = %s\n", fmtBoolDisplay(cfg.Security.TrustTailscale))
	if len(cfg.Security.AllowedIPs) == 0 {
		fmt.Printf("    allowed_ips      = []\n")
	} else {
		fmt.Printf("    allowed_ips      = [%s]\n", strings.Join(cfg.Security.AllowedIPs, ", "))
	}

	// Profiles
	fmt.Println()
	fmt.Println("  [[profiles]]")
	if len(cfg.Profiles) == 0 {
		fmt.Println("    (none)")
	} else {
		fmt.Printf("    %-12s %-24s %-14s %s\n", "KEY", "LABEL", "RESOLUTION", "SCALE")
		for _, p := range cfg.Profiles {
			res := fmt.Sprintf("%dx%d", p.Width, p.Height)
			scale := strconv.FormatFloat(p.Scale, 'f', -1, 64)
			fmt.Printf("    %-12s %-24s %-14s %s\n", p.Key, p.Label, res, scale)
		}
	}
	fmt.Println()
}

func fmtBoolDisplay(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func init() {
	configShowTOMLCmd.Flags().Bool("json", false, "Emit JSON format")
	configInitTOMLCmd.Flags().Bool("force", false, "Overwrite existing config file")

	configCmd.AddCommand(configShowTOMLCmd)
	configCmd.AddCommand(configValidateTOMLCmd)
	configCmd.AddCommand(configInitTOMLCmd)
}

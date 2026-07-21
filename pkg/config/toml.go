package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ---------- TOML config schema ----------

// TOMLConfig is the top-level declarative configuration for Remote Studio.
type TOMLConfig struct {
	General  GeneralConfig  `toml:"general"`
	Display  DisplayConfig  `toml:"display"`
	Daemon   DaemonConfig   `toml:"daemon"`
	Audio    AudioConfig    `toml:"audio"`
	Security SecurityConfig `toml:"security"`
	Profiles []ProfileTOML  `toml:"profiles"`
}

// GeneralConfig holds top-level options.
type GeneralConfig struct {
	Version     string `toml:"version"`
	AutoSession bool   `toml:"auto_session"`
	LogLevel    string `toml:"log_level"`
	LogPath     string `toml:"log_path"`
}

// DisplayConfig holds display-related settings.
type DisplayConfig struct {
	DefaultBackend string `toml:"default_backend"`
	DefaultProfile string `toml:"default_profile"`
	// XorgDriver overrides the auto-detected Xorg driver when generating
	// /etc/X11/xorg.conf (via `res xorg`). Common values:
	//   "nvidia"       – proprietary NVIDIA driver
	//   "amdgpu"       – AMD open driver (also matches ATI/Radeon)
	//   "intel"        – Intel integrated graphics
	//   "modesetting"  – generic kernel modesetting (default if unset)
	//   "dummy"        – headless / CI use case
	//   "" (empty)     – auto-detect from lspci (default)
	XorgDriver string `toml:"xorg_driver"`
}

// DaemonConfig holds daemon/polling settings.
type DaemonConfig struct {
	PollInterval    int  `toml:"poll_interval"`
	WebsocketPort   int  `toml:"websocket_port"`
	HTTPPort        int  `toml:"http_port"`
	SocketActivated bool `toml:"socket_activated"`
}

// AudioConfig holds audio subsystem settings.
type AudioConfig struct {
	VirtualSinkName  string `toml:"virtual_sink_name"`
	AutoMutePhysical bool   `toml:"auto_mute_physical"`
}

// SecurityConfig holds access-control settings.
type SecurityConfig struct {
	TrustTailscale bool     `toml:"trust_tailscale"`
	AllowedIPs     []string `toml:"allowed_ips"`
}

// ProfileTOML is a display profile defined in the TOML config.
type ProfileTOML struct {
	Key        string  `toml:"key"`
	Label      string  `toml:"label"`
	Width      int     `toml:"width"`
	Height     int     `toml:"height"`
	Scale      float64 `toml:"scale"`
	TextScale  float64 `toml:"text_scale"`
	CursorSize int     `toml:"cursor_size"`
}

// ---------- Defaults ----------

// DefaultTOMLConfig returns a sensible default configuration.
func DefaultTOMLConfig() *TOMLConfig {
	return &TOMLConfig{
		General: GeneralConfig{
			Version:     "1",
			AutoSession: false,
			LogLevel:    "info",
			LogPath:     "~/.remote_studio.log",
		},
		Display: DisplayConfig{
			DefaultBackend: "auto",
			DefaultProfile: "mac",
			// XorgDriver intentionally empty — the bash engine falls back
			// to lspci auto-detection when this is unset. Empty preserves
			// existing install behavior.
			XorgDriver: "",
		},
		Daemon: DaemonConfig{
			PollInterval:    5,
			WebsocketPort:   9600,
			HTTPPort:        9601,
			SocketActivated: false,
		},
		Audio: AudioConfig{
			VirtualSinkName:  "remote_studio_sink",
			AutoMutePhysical: true,
		},
		Security: SecurityConfig{
			TrustTailscale: true,
			AllowedIPs:     []string{},
		},
		Profiles: []ProfileTOML{
			{Key: "mac", Label: "MacBook Air 13", Width: 2560, Height: 1664, Scale: 1.0, TextScale: 1.5, CursorSize: 48},
			{Key: "ipad", Label: "iPad Pro 11\"", Width: 2424, Height: 1664, Scale: 2.0, TextScale: 1.1, CursorSize: 48},
			{Key: "fallback", Label: "Fallback 1920x1200", Width: 1920, Height: 1200, Scale: 1.0, TextScale: 1.1, CursorSize: 32},
		},
	}
}

// ---------- TOML path resolution ----------

// ResolveTOMLConfigPath returns the path to the first TOML config found,
// checking ~/.config/remote-studio/remote-studio.toml first.
func ResolveTOMLConfigPath() string {
	// Explicit user path first (per-user overrides win).
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".config", "remote-studio", "remote-studio.toml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p := filepath.Join(FindConfigDir(), "remote-studio.toml")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return "/etc/remote-studio/remote-studio.toml"
}

// ResolveUserTOMLConfigPath returns the per-user TOML config path,
// regardless of whether the file exists yet. Used by `res config set-toml`
// and `res config init-toml` so writes always go to the user's
// ~/.config/remote-studio/remote-studio.toml (the per-user layer) — never
// the system-wide /etc/remote-studio/ one, which requires root and is
// typically managed by the distribution package.
//
// Falls back to /etc/remote-studio/remote-studio.toml only if the user's
// home directory can't be determined (a misconfigured environment, not
// normal operation).
func ResolveUserTOMLConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/etc/remote-studio/remote-studio.toml"
	}
	return filepath.Join(home, ".config", "remote-studio", "remote-studio.toml")
}

// ---------- Load ----------

// LoadTOMLConfig reads a TOML file and populates a TOMLConfig struct.
func LoadTOMLConfig(path string) (*TOMLConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	cfg := DefaultTOMLConfig()
	section := ""          // current [section]
	inArray := ""          // current [[array]] name
	var currentProfile *ProfileTOML

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// [[array]] header
		if strings.HasPrefix(line, "[[") && strings.HasSuffix(line, "]]") {
			inArray = strings.TrimSpace(line[2 : len(line)-2])
			section = ""
			if inArray == "profiles" {
				// Start a new profile entry
				p := ProfileTOML{}
				cfg.Profiles = append(cfg.Profiles, p)
				currentProfile = &cfg.Profiles[len(cfg.Profiles)-1]
			}
			continue
		}

		// [section] header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			inArray = ""
			currentProfile = nil
			continue
		}

		// key = value
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue // skip malformed lines
		}
		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])

		// Strip inline comment (only if preceded by whitespace)
		if ci := findInlineComment(val); ci >= 0 {
			val = strings.TrimSpace(val[:ci])
		}

		// Route to correct section
		if inArray == "profiles" && currentProfile != nil {
			applyProfileField(currentProfile, key, val)
			continue
		}

		switch section {
		case "general":
			applyGeneralField(&cfg.General, key, val)
		case "display":
			applyDisplayField(&cfg.Display, key, val)
		case "daemon":
			applyDaemonField(&cfg.Daemon, key, val)
		case "audio":
			applyAudioField(&cfg.Audio, key, val)
		case "security":
			applySecurityField(&cfg.Security, key, val)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// If profiles were loaded from file, remove the defaults that were seeded
	// by DefaultTOMLConfig. We detect this by checking if more than the 3
	// defaults exist — the parser appended to the slice.  A cleaner approach:
	// start with an empty slice and only use defaults when the file has none.
	// We handle that here.
	if countFileProfiles(cfg) > 0 {
		// Trim the leading default profiles (first 3) that were pre-seeded.
		cfg.Profiles = cfg.Profiles[3:]
	}

	return cfg, nil
}

// countFileProfiles returns how many profiles were added beyond the 3 defaults.
func countFileProfiles(cfg *TOMLConfig) int {
	if len(cfg.Profiles) <= 3 {
		return 0
	}
	return len(cfg.Profiles) - 3
}

// ---------- Save ----------

// SaveTOMLConfig writes the config back to a TOML file.
func SaveTOMLConfig(path string, cfg *TOMLConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var b strings.Builder

	// [general]
	b.WriteString("[general]\n")
	b.WriteString(fmt.Sprintf("version = %q\n", cfg.General.Version))
	b.WriteString(fmt.Sprintf("auto_session = %s\n", fmtBool(cfg.General.AutoSession)))
	b.WriteString(fmt.Sprintf("log_level = %q\n", cfg.General.LogLevel))
	b.WriteString(fmt.Sprintf("log_path = %q\n", cfg.General.LogPath))
	b.WriteString("\n")

	// Display
	b.WriteString("[display]\n")
	b.WriteString(fmt.Sprintf("default_backend = %q\n", cfg.Display.DefaultBackend))
	b.WriteString(fmt.Sprintf("default_profile = %q\n", cfg.Display.DefaultProfile))
	// Only emit xorg_driver when explicitly set; empty = auto-detect, matches
	// the bash engine's fallback path and avoids cluttering configs with
	// the empty default.
	if cfg.Display.XorgDriver != "" {
		b.WriteString(fmt.Sprintf("xorg_driver = %q\n", cfg.Display.XorgDriver))
	}
	b.WriteString("\n")

	// [daemon]
	b.WriteString("[daemon]\n")
	b.WriteString(fmt.Sprintf("poll_interval = %d\n", cfg.Daemon.PollInterval))
	b.WriteString(fmt.Sprintf("websocket_port = %d\n", cfg.Daemon.WebsocketPort))
	b.WriteString(fmt.Sprintf("http_port = %d\n", cfg.Daemon.HTTPPort))
	b.WriteString(fmt.Sprintf("socket_activated = %s\n", fmtBool(cfg.Daemon.SocketActivated)))
	b.WriteString("\n")

	// [audio]
	b.WriteString("[audio]\n")
	b.WriteString(fmt.Sprintf("virtual_sink_name = %q\n", cfg.Audio.VirtualSinkName))
	b.WriteString(fmt.Sprintf("auto_mute_physical = %s\n", fmtBool(cfg.Audio.AutoMutePhysical)))
	b.WriteString("\n")

	// [security]
	b.WriteString("[security]\n")
	b.WriteString(fmt.Sprintf("trust_tailscale = %s\n", fmtBool(cfg.Security.TrustTailscale)))
	b.WriteString(fmt.Sprintf("allowed_ips = [%s]\n", fmtStringSlice(cfg.Security.AllowedIPs)))
	b.WriteString("\n")

	// [[profiles]]
	for _, p := range cfg.Profiles {
		b.WriteString("[[profiles]]\n")
		b.WriteString(fmt.Sprintf("key = %q\n", p.Key))
		b.WriteString(fmt.Sprintf("label = %q\n", p.Label))
		b.WriteString(fmt.Sprintf("width = %d\n", p.Width))
		b.WriteString(fmt.Sprintf("height = %d\n", p.Height))
		b.WriteString(fmt.Sprintf("scale = %s\n", fmtFloat(p.Scale)))
		b.WriteString(fmt.Sprintf("text_scale = %s\n", fmtFloat(p.TextScale)))
		b.WriteString(fmt.Sprintf("cursor_size = %d\n", p.CursorSize))
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}

// ---------- Validate ----------

// ValidateConfig checks a config for common mistakes and returns a list of
// human-readable warnings (empty slice = all good).
func ValidateConfig(cfg *TOMLConfig) []string {
	var warns []string

	// General
	switch cfg.General.LogLevel {
	case "debug", "info", "warn", "error":
		// ok
	default:
		warns = append(warns, fmt.Sprintf("general.log_level: unknown level %q (expected debug|info|warn|error)", cfg.General.LogLevel))
	}

	// Display
	switch cfg.Display.DefaultBackend {
	case "auto", "x11", "wayland":
		// ok
	default:
		warns = append(warns, fmt.Sprintf("display.default_backend: unknown backend %q (expected auto|x11|wayland)", cfg.Display.DefaultBackend))
	}
	if cfg.Display.DefaultProfile == "" {
		warns = append(warns, "display.default_profile: empty (should reference a profile key)")
	}
	if cfg.Display.XorgDriver != "" {
		// Validated against the bash engine's accepted set in
		// lib/engine.sh::generate_xorg. Keep this list in sync if the
		// engine's auto-detect regex gains more drivers.
		switch cfg.Display.XorgDriver {
		case "nvidia", "amdgpu", "intel", "modesetting", "dummy":
			// ok
		default:
			warns = append(warns, fmt.Sprintf("display.xorg_driver: unknown driver %q (expected nvidia|amdgpu|intel|modesetting|dummy or empty for auto-detect)", cfg.Display.XorgDriver))
		}
	}

	// Daemon
	if cfg.Daemon.PollInterval < 1 {
		warns = append(warns, "daemon.poll_interval: should be >= 1 second")
	}
	if cfg.Daemon.WebsocketPort < 1 || cfg.Daemon.WebsocketPort > 65535 {
		warns = append(warns, fmt.Sprintf("daemon.websocket_port: %d out of range 1-65535", cfg.Daemon.WebsocketPort))
	}
	if cfg.Daemon.HTTPPort < 1 || cfg.Daemon.HTTPPort > 65535 {
		warns = append(warns, fmt.Sprintf("daemon.http_port: %d out of range 1-65535", cfg.Daemon.HTTPPort))
	}
	if cfg.Daemon.WebsocketPort == cfg.Daemon.HTTPPort {
		warns = append(warns, "daemon: websocket_port and http_port must be different")
	}

	// Profiles
	if len(cfg.Profiles) == 0 {
		warns = append(warns, "profiles: no profiles defined")
	}
	seen := make(map[string]bool)
	for i, p := range cfg.Profiles {
		prefix := fmt.Sprintf("profiles[%d]", i)
		if p.Key == "" {
			warns = append(warns, fmt.Sprintf("%s: key is empty", prefix))
		}
		if seen[p.Key] {
			warns = append(warns, fmt.Sprintf("%s: duplicate key %q", prefix, p.Key))
		}
		seen[p.Key] = true
		if p.Width <= 0 || p.Height <= 0 {
			warns = append(warns, fmt.Sprintf("%s (%s): invalid resolution %dx%d", prefix, p.Key, p.Width, p.Height))
		}
		if p.Scale <= 0 {
			warns = append(warns, fmt.Sprintf("%s (%s): scale must be > 0", prefix, p.Key))
		}
		if p.TextScale <= 0 {
			warns = append(warns, fmt.Sprintf("%s (%s): text_scale must be > 0", prefix, p.Key))
		}
		if p.CursorSize < 0 {
			warns = append(warns, fmt.Sprintf("%s (%s): cursor_size must be >= 0", prefix, p.Key))
		}
	}

	// Check that default_profile references a defined profile key
	if cfg.Display.DefaultProfile != "" {
		found := false
		for _, p := range cfg.Profiles {
			if p.Key == cfg.Display.DefaultProfile {
				found = true
				break
			}
		}
		if !found {
			warns = append(warns, fmt.Sprintf("display.default_profile: %q does not match any defined profile key", cfg.Display.DefaultProfile))
		}
	}

	return warns
}

// ---------- field setters (hand-rolled TOML dispatch) ----------

func applyGeneralField(g *GeneralConfig, key, val string) {
	switch key {
	case "version":
		g.Version = unquote(val)
	case "auto_session":
		g.AutoSession = parseBool(val)
	case "log_level":
		g.LogLevel = unquote(val)
	case "log_path":
		g.LogPath = unquote(val)
	}
}

func applyDisplayField(d *DisplayConfig, key, val string) {
	switch key {
	case "default_backend":
		d.DefaultBackend = unquote(val)
	case "default_profile":
		d.DefaultProfile = unquote(val)
	case "xorg_driver":
		d.XorgDriver = unquote(val)
	}
}

func applyDaemonField(d *DaemonConfig, key, val string) {
	switch key {
	case "poll_interval":
		d.PollInterval = parseInt(val)
	case "websocket_port":
		d.WebsocketPort = parseInt(val)
	case "http_port":
		d.HTTPPort = parseInt(val)
	case "socket_activated":
		d.SocketActivated = parseBool(val)
	}
}

func applyAudioField(a *AudioConfig, key, val string) {
	switch key {
	case "virtual_sink_name":
		a.VirtualSinkName = unquote(val)
	case "auto_mute_physical":
		a.AutoMutePhysical = parseBool(val)
	}
}

func applySecurityField(s *SecurityConfig, key, val string) {
	switch key {
	case "trust_tailscale":
		s.TrustTailscale = parseBool(val)
	case "allowed_ips":
		s.AllowedIPs = parseStringArray(val)
	}
}

func applyProfileField(p *ProfileTOML, key, val string) {
	switch key {
	case "key":
		p.Key = unquote(val)
	case "label":
		p.Label = unquote(val)
	case "width":
		p.Width = parseInt(val)
	case "height":
		p.Height = parseInt(val)
	case "scale":
		p.Scale = parseFloat(val)
	case "text_scale":
		p.TextScale = parseFloat(val)
	case "cursor_size":
		p.CursorSize = parseInt(val)
	}
}

// ---------- low-level parse helpers ----------

// unquote strips surrounding double or single quotes.
func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// parseBool handles true/false (case-insensitive).
func parseBool(s string) bool {
	s = strings.TrimSpace(unquote(s))
	return strings.EqualFold(s, "true")
}

// parseInt parses a decimal integer, returning 0 on error.
func parseInt(s string) int {
	s = strings.TrimSpace(unquote(s))
	v, _ := strconv.Atoi(s)
	return v
}

// parseFloat parses a float64, returning 0 on error.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(unquote(s))
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseStringArray parses a TOML inline array like ["a", "b", "c"].
func parseStringArray(s string) []string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = s[1 : len(s)-1]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = unquote(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// findInlineComment returns the index of a # that starts an inline comment.
// A # inside quotes is not a comment. Returns -1 if none found.
func findInlineComment(s string) int {
	inQuote := false
	quoteChar := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
			continue
		}
		if c == '#' {
			// Must be preceded by whitespace (or at start of val)
			if i == 0 || s[i-1] == ' ' || s[i-1] == '\t' {
				return i
			}
		}
	}
	return -1
}

// ---------- format helpers ----------

func fmtBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func fmtFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func fmtStringSlice(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	quoted := make([]string, len(ss))
	for i, s := range ss {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

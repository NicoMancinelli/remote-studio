package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLANModeActiveDefault verifies that LAN mode is off when no env var
// or TOML config is present. This is the conservative default — existing
// installs must keep behaving the same.
func TestLANModeActiveDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("RES_LAN_MODE", "")
	t.Setenv("HOME", tmp)

	if LANModeActive() {
		t.Fatal("LANModeActive() should return false when no env or TOML is set")
	}
}

// TestLANModeActiveEnv verifies the RES_LAN_MODE env var wins over TOML.
func TestLANModeActiveEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Write a TOML that disables LAN mode.
	confDir := filepath.Join(tmp, ".config", "remote-studio")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "remote-studio.toml"),
		[]byte("[lan]\nenabled = false\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Env var says enabled — must win.
	t.Setenv("RES_LAN_MODE", "true")
	if !LANModeActive() {
		t.Fatal("RES_LAN_MODE=true should override TOML [lan] enabled=false")
	}

	// Flip the env var to false — must also win.
	t.Setenv("RES_LAN_MODE", "false")
	if LANModeActive() {
		t.Fatal("RES_LAN_MODE=false should override TOML [lan] enabled=true")
	}
}

// TestLANModeActiveTOML verifies the TOML config is consulted when the
// env var is unset.
func TestLANModeActiveTOML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("RES_LAN_MODE", "")

	confDir := filepath.Join(tmp, ".config", "remote-studio")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}

	// TOML says enabled.
	if err := os.WriteFile(filepath.Join(confDir, "remote-studio.toml"),
		[]byte("[lan]\nenabled = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if !LANModeActive() {
		t.Fatal("expected LANModeActive()=true when TOML [lan] enabled = true")
	}

	// Flip to false.
	if err := os.WriteFile(filepath.Join(confDir, "remote-studio.toml"),
		[]byte("[lan]\nenabled = false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if LANModeActive() {
		t.Fatal("expected LANModeActive()=false when TOML [lan] enabled = false")
	}
}

// TestLANModeActiveNoTOMLMissing verifies that the absence of a TOML file
// is treated as "not enabled" (not an error). This keeps existing tailnet
// installs from breaking when LAN mode is added.
func TestLANModeActiveNoTOMLMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("RES_LAN_MODE", "")

	// No TOML file at all.
	if LANModeActive() {
		t.Fatal("LANModeActive() should be false when no TOML exists")
	}
}

// TestLANBindAddressDefault verifies the default is 0.0.0.0 (all
// interfaces) — appropriate for a server-style daemon on a LAN.
func TestLANBindAddressDefault(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("RES_LAN_BIND", "")

	if got := LANBindAddress(); got != "0.0.0.0" {
		t.Fatalf("default LANBindAddress = %q, want 0.0.0.0", got)
	}
}

// TestLANBindAddressEnv verifies RES_LAN_BIND overrides TOML.
func TestLANBindAddressEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	confDir := filepath.Join(tmp, ".config", "remote-studio")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "remote-studio.toml"),
		[]byte("[lan]\nbind_address = \"192.168.1.50\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("RES_LAN_BIND", "10.0.0.1")
	if got := LANBindAddress(); got != "10.0.0.1" {
		t.Fatalf("env should win over TOML, got %q", got)
	}
}

// TestLANBindAddressTOML verifies the TOML value is used when env is unset.
func TestLANBindAddressTOML(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("RES_LAN_BIND", "")
	confDir := filepath.Join(tmp, ".config", "remote-studio")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "remote-studio.toml"),
		[]byte("[lan]\nbind_address = \"192.168.1.99\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if got := LANBindAddress(); got != "192.168.1.99" {
		t.Fatalf("LANBindAddress = %q, want 192.168.1.99", got)
	}
}

// TestLANConfigRoundTrip exercises the [lan] block through the full
// TOML parse → save → re-parse cycle. Catches serializer regressions.
func TestLANConfigRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "remote-studio.toml")

	original := DefaultTOMLConfig()
	original.LAN.Enabled = true
	original.LAN.BindAddress = "192.168.1.42"

	if err := SaveTOMLConfig(path, original); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadTOMLConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.LAN.Enabled {
		t.Error("LAN.Enabled lost across round-trip")
	}
	if loaded.LAN.BindAddress != "192.168.1.42" {
		t.Errorf("LAN.BindAddress = %q, want 192.168.1.42", loaded.LAN.BindAddress)
	}

	// Read back the raw text — ensure the section header was emitted.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "[lan]") {
		t.Error("Saved TOML is missing the [lan] section header")
	}
}

// TestLANConfigDefaults verifies the default config has LAN mode off and
// no bind address (clean defaults for tailnet-only installs).
func TestLANConfigDefaults(t *testing.T) {
	cfg := DefaultTOMLConfig()
	if cfg.LAN.Enabled {
		t.Error("DefaultTOMLConfig().LAN.Enabled should be false")
	}
	if cfg.LAN.BindAddress != "" {
		t.Errorf("DefaultTOMLConfig().LAN.BindAddress = %q, want empty", cfg.LAN.BindAddress)
	}
}

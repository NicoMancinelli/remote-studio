package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "remote-studio.conf")
	content := "DEFAULT_PROFILE=mac\nAUTO_SESSION=true\n#comment\nINVALID-KEY=val\n"
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadConfig(confPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.GetConfigValue("DEFAULT_PROFILE") != "mac" {
		t.Errorf("expected mac, got %s", cfg.GetConfigValue("DEFAULT_PROFILE"))
	}
}

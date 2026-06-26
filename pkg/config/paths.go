package config

import (
	"os"
	"path/filepath"
)

func FindConfigDir() string {
	dir, err := os.Getwd()
	if err == nil {
		for {
			p := filepath.Join(dir, "config")
			if info, err := os.Stat(p); err == nil && info.IsDir() {
				return p
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	execPath, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(execPath), "config")
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return "/usr/share/remote-studio"
}

func ResolveConfigPath() string {
	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".config", "remote-studio", "remote-studio.conf")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p := filepath.Join(FindConfigDir(), "remote-studio.conf")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return "/etc/remote-studio/remote-studio.conf"
}

func ResolveProfilesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".config", "remote-studio", "profiles.conf")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	p := filepath.Join(FindConfigDir(), "profiles.conf")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	return "/usr/share/remote-studio/profiles.conf", nil
}

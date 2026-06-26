package session

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func LogEvent(msg string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(home, ".remote_studio.log")

	if info, err := os.Stat(logPath); err == nil {
		if info.Size() > 1048576 {
			_ = os.Rename(logPath, logPath+".1")
		}
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, _ = f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
}

func DoAction(action string) error {
	switch action {
	case "speed":
		out, err := exec.Command("gsettings", "get", "org.cinnamon", "desktop-effects").Output()
		if err != nil {
			return err
		}
		status := strings.TrimSpace(strings.ReplaceAll(string(out), "'", ""))
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		wallpaperBackup := filepath.Join(home, ".wallpaper_backup")

		if status == "true" {
			wpOut, wpErr := exec.Command("gsettings", "get", "org.cinnamon.desktop.background", "picture-uri").Output()
			if wpErr == nil {
				_ = os.WriteFile(wallpaperBackup, []byte(strings.TrimSpace(string(wpOut))), 0644)
			}
			_ = exec.Command("gsettings", "set", "org.cinnamon", "desktop-effects", "false").Run()
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "enable-animations", "false").Run()
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.background", "picture-options", "none").Run()
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.background", "primary-color", "#000000").Run()
			LogEvent("Speed mode: ON")
		} else {
			_ = exec.Command("gsettings", "set", "org.cinnamon", "desktop-effects", "true").Run()
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "enable-animations", "true").Run()
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.background", "picture-options", "zoom").Run()
			if _, err := os.Stat(wallpaperBackup); err == nil {
				wpData, wpErr := os.ReadFile(wallpaperBackup)
				if wpErr == nil {
					_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.background", "picture-uri", string(wpData)).Run()
				}
				_ = os.Remove(wallpaperBackup)
			}
			LogEvent("Speed mode: OFF")
		}

	case "theme":
		out, err := exec.Command("gsettings", "get", "org.cinnamon.desktop.interface", "gtk-theme").Output()
		if err != nil {
			return err
		}
		cur := strings.TrimSpace(strings.ReplaceAll(string(out), "'", ""))
		if strings.Contains(cur, "Dark") {
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "gtk-theme", "Mint-Y").Run()
			LogEvent("Theme: Light")
		} else {
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "gtk-theme", "Mint-Y-Dark").Run()
			LogEvent("Theme: Dark")
		}

	case "night":
		out, err := exec.Command("xgamma").CombinedOutput()
		if err != nil {
			return err
		}
		fields := strings.Fields(string(out))
		gamma := "1.000"
		if len(fields) >= 4 {
			gamma = fields[3]
		}
		if gamma == "1.000" {
			_ = exec.Command("xgamma", "-rgamma", "1.0", "-ggamma", "0.8", "-bgamma", "0.6").Run()
			LogEvent("Night shift: ON")
		} else {
			_ = exec.Command("xgamma", "-gamma", "1.0").Run()
			LogEvent("Night shift: OFF")
		}

	case "caf":
		out, err := exec.Command("gsettings", "get", "org.cinnamon.desktop.screensaver", "lock-enabled").Output()
		if err != nil {
			return err
		}
		cur := strings.TrimSpace(string(out))
		if cur == "true" {
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.screensaver", "lock-enabled", "false").Run()
			LogEvent("Caffeine: ON")
		} else {
			_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.screensaver", "lock-enabled", "true").Run()
			LogEvent("Caffeine: OFF")
		}

	case "privacy":
		_ = exec.Command("cinnamon-screensaver-command", "-l").Run()
		_ = exec.Command("xset", "dpms", "force", "off").Run()
		LogEvent("Privacy shield activated")

	case "clip":
		cmd1 := exec.Command("xclip", "-selection", "primary")
		cmd1.Stdin = strings.NewReader("")
		_ = cmd1.Run()

		cmd2 := exec.Command("xclip", "-selection", "clipboard")
		cmd2.Stdin = strings.NewReader("")
		_ = cmd2.Run()

	case "service":
		_ = exec.Command("sudo", "systemctl", "restart", "rustdesk").Run()
		LogEvent("RustDesk service restarted")

	case "audio":
		_ = exec.Command("pulseaudio", "-k").Run()
		time.Sleep(1 * time.Second)
		_ = exec.Command("pulseaudio", "--start").Run()

	case "keys":
		_ = exec.Command("setxkbmap", "us").Run()

	case "fix":
		_ = DoAction("clip")
		_ = DoAction("audio")
		_ = DoAction("keys")
		LogEvent("Fix all: clip+audio+keys")

	case "reset":
		return ApplyAll(1024, 768, 1.0, 1.0, 24, "Reset")

	default:
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}

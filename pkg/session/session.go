package session

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"remote-studio/pkg/config"
)

func getConnectedDisplay() (string, error) {
	cmd := exec.Command("xrandr")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("no connected display found (xrandr failed: %w, stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		if strings.Contains(line, " connected") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				return parts[0], nil
			}
		}
	}
	return "", fmt.Errorf("no connected display found")
}

func ApplyAll(width, height int, scaling, textScale float64, cursor int, label string) error {
	dpi := int(96 * scaling)

	output, err := getConnectedDisplay()
	if err != nil {
		return err
	}

	modeName := fmt.Sprintf("remote-studio-%dx%d-60", width, height)

	// Remove stale modes
	xrandrOut, err := exec.Command("xrandr").Output()
	if err == nil {
		lines := strings.Split(string(xrandrOut), "\n")
		var currentMode string
		for _, line := range lines {
			if strings.Contains(line, "*") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					currentMode = fields[0]
				}
			}
		}

		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				m := fields[0]
				if m == modeName || strings.HasPrefix(m, fmt.Sprintf("%dx%d", width, height)) {
					if m != currentMode {
						_ = exec.Command("xrandr", "--delmode", output, m).Run()
						_ = exec.Command("xrandr", "--rmmode", m).Run()
					}
				}
			}
		}
	}

	// Generate modeline using cvt
	cvtOut, err := exec.Command("cvt", fmt.Sprintf("%d", width), fmt.Sprintf("%d", height), "60").Output()
	if err != nil {
		return fmt.Errorf("cvt command failed: %w", err)
	}

	var modelineParams string
	lines := strings.Split(string(cvtOut), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Modeline") {
			parts := strings.SplitN(line, "\"", 3)
			if len(parts) == 3 {
				modelineParams = strings.TrimSpace(parts[2])
				break
			}
		}
	}

	if modelineParams == "" {
		return fmt.Errorf("failed to generate modeline parameters from cvt")
	}

	args := append([]string{"--newmode", modeName}, strings.Fields(modelineParams)...)
	_ = exec.Command("xrandr", args...).Run()

	addErrOut, addErr := exec.Command("xrandr", "--addmode", output, modeName).CombinedOutput()
	if addErr != nil && !strings.Contains(string(addErrOut), "already exists") {
		return fmt.Errorf("xrandr --addmode failed: %s", string(addErrOut))
	}

	outputErrOut, outputErr := exec.Command("xrandr", "--output", output, "--mode", modeName).CombinedOutput()
	if outputErr != nil {
		return fmt.Errorf("xrandr --output --mode failed: %s", string(outputErrOut))
	}

	// Apply Cinnamon settings
	_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "scaling-factor", fmt.Sprintf("%d", int(scaling))).Run()
	_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "text-scaling-factor", fmt.Sprintf("%.2f", textScale)).Run()
	_ = exec.Command("gsettings", "set", "org.cinnamon.desktop.interface", "cursor-size", fmt.Sprintf("%d", cursor)).Run()

	// Set X11 DPI via xrdb
	xrdbCmd := exec.Command("xrdb", "-merge")
	xrdbCmd.Stdin = strings.NewReader(fmt.Sprintf("Xft.dpi: %d\n", dpi))
	_ = xrdbCmd.Run()

	// Save state file
	home, err := os.UserHomeDir()
	if err == nil {
		statePath := filepath.Join(home, ".res_state")
		stateContent := fmt.Sprintf("%d %d %g %g %d '%s'\n", width, height, scaling, textScale, cursor, label)
		_ = os.WriteFile(statePath, []byte(stateContent), 0644)
	}

	LogEvent("Mode: " + label)
	return nil
}

func LoadMergedProfiles() (*config.ProfileRegistry, error) {
	reg := config.NewProfileRegistry()

	defaultPath, _ := config.ResolveProfilesPath()
	_ = reg.LoadProfiles(defaultPath)

	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, ".config", "remote-studio", "profiles.conf")
		_ = reg.LoadProfiles(p)
	}
	return reg, nil
}

func ApplyProfile(name string) error {
	reg, err := LoadMergedProfiles()
	if err != nil {
		return err
	}
	p, exists := reg.Profiles[name]
	if !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}
	return ApplyAll(p.Width, p.Height, p.Scaling, p.TextScale, p.Cursor, p.Label)
}

func SpeedState() string {
	out, err := exec.Command("gsettings", "get", "org.cinnamon", "desktop-effects").Output()
	if err != nil {
		return "OFF"
	}
	status := strings.TrimSpace(strings.ReplaceAll(string(out), "'", ""))
	if status == "false" {
		return "ON"
	}
	return "OFF"
}

func CaffeineState() string {
	out, err := exec.Command("gsettings", "get", "org.cinnamon.desktop.screensaver", "lock-enabled").Output()
	if err != nil {
		return "OFF"
	}
	status := strings.TrimSpace(strings.ReplaceAll(string(out), "'", ""))
	if status == "false" {
		return "ON"
	}
	return "OFF"
}

func SessionStart(profileName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sessionFile := filepath.Join(home, ".config", "remote-studio", "session.state")
	stateFile := filepath.Join(home, ".res_state")

	if profileName == "" {
		// Read DEFAULT_SESSION_PROFILE from config
		cfg, _, _ := config.FindAndLoadConfig()
		profileName = cfg.GetConfigValue("DEFAULT_SESSION_PROFILE")
		if profileName == "" {
			profileName = cfg.GetConfigValue("DEFAULT_PROFILE")
		}
		if profileName == "" {
			profileName = "mac"
		}
	}

	stateContent := ""
	if data, err := os.ReadFile(stateFile); err == nil {
		stateContent = strings.TrimSpace(string(data))
	}

	startedAt := time.Now().Format("2006-01-02 15:04:05")
	_ = os.MkdirAll(filepath.Dir(sessionFile), 0755)

	sessionContent := fmt.Sprintf(
		"started_at=%s\nprofile=%s\nspeed=%s\ncaffeine=%s\nstate=%s\n",
		startedAt, profileName, SpeedState(), CaffeineState(), stateContent,
	)

	if err := ApplyProfile(profileName); err != nil {
		return fmt.Errorf("failed to apply profile '%s': %w", profileName, err)
	}

	_ = os.WriteFile(sessionFile, []byte(sessionContent), 0644)

	if SpeedState() != "ON" {
		_ = DoAction("speed")
	}
	if CaffeineState() != "ON" {
		_ = DoAction("caf")
	}

	if _, err := exec.LookPath("powerprofilesctl"); err == nil {
		_ = exec.Command("powerprofilesctl", "set", "performance").Run()
	}

	LogEvent("Session start: " + profileName)
	return nil
}

func SessionStop() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sessionFile := filepath.Join(home, ".config", "remote-studio", "session.state")

	if _, err := os.Stat(sessionFile); err == nil {
		// Read session file
		file, err := os.Open(sessionFile)
		if err == nil {
			var stateStr string
			var speedStr string
			var caffeineStr string
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "state=") {
					stateStr = strings.TrimPrefix(line, "state=")
				} else if strings.HasPrefix(line, "speed=") {
					speedStr = strings.TrimPrefix(line, "speed=")
				} else if strings.HasPrefix(line, "caffeine=") {
					caffeineStr = strings.TrimPrefix(line, "caffeine=")
				}
			}
			file.Close()

			if stateStr != "" {
				stateFile := filepath.Join(home, ".res_state")
				_ = os.WriteFile(stateFile, []byte(stateStr+"\n"), 0644)

				// Parse state fields
				// width height scaling text_scale cursor 'label'
				re := regexp.MustCompile(`^(\d+)\s+(\d+)\s+([\d.]+)\s+([\d.]+)\s+(\d+)\s+'([^']+)'`)
				matches := re.FindStringSubmatch(stateStr)
				if len(matches) == 7 {
					w, _ := strconv.Atoi(matches[1])
					h, _ := strconv.Atoi(matches[2])
					s, _ := strconv.ParseFloat(matches[3], 64)
					ts, _ := strconv.ParseFloat(matches[4], 64)
					c, _ := strconv.Atoi(matches[5])
					label := matches[6]
					_ = ApplyAll(w, h, s, ts, c, label)
				}
			}

			if speedStr == "OFF" && SpeedState() == "ON" {
				_ = DoAction("speed")
			}
			if caffeineStr == "OFF" && CaffeineState() == "ON" {
				_ = DoAction("caf")
			}
		}
		_ = os.Remove(sessionFile)
	}

	if _, err := exec.LookPath("powerprofilesctl"); err == nil {
		_ = exec.Command("powerprofilesctl", "set", "balanced").Run()
	}

	LogEvent("Session stop")
	return nil
}

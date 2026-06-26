package status

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SessionStatus struct {
	SessionActive bool    `json:"session_active"`
	SessionPID    int     `json:"session_pid"`
	Display       string  `json:"display"`
	Profile       string  `json:"profile"`
	NetworkStatus string  `json:"network_status"`
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	LastUpdated   string  `json:"last_updated"`
}

func ResolveStatusPath() string {
	primaryDir := "/var/run/remote-studio"
	if _, err := os.Stat(primaryDir); err == nil {
		return filepath.Join(primaryDir, "status.json")
	}
	fallbackDir := "/tmp/remote-studio"
	return filepath.Join(fallbackDir, "status.json")
}

// Stats gathering helpers used by both CLI and Daemon

func ResolveStatusDir() string {
	xdg := os.Getenv("XDG_RUNTIME_DIR")
	if xdg != "" {
		dir := filepath.Join(xdg, "remote-studio")
		if err := os.MkdirAll(dir, 0755); err == nil {
			return dir
		}
	}
	uid := os.Getuid()
	dir := fmt.Sprintf("/tmp/remote-studio-%d", uid)
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func GetCurrentMode() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "None"
	}
	statePath := filepath.Join(home, ".res_state")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return "None"
	}
	parts := strings.Split(string(data), "'")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "None"
}

func GetCurrentResolution() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "N/A"
	}
	statePath := filepath.Join(home, ".res_state")
	data, err := os.ReadFile(statePath)
	if err != nil {
		return "N/A"
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 2 {
		return fmt.Sprintf("%sx%s", fields[0], fields[1])
	}
	return "N/A"
}

func GetCpuTemp() string {
	sensorsOut, err := exec.Command("sensors").Output()
	if err == nil {
		lines := strings.Split(string(sensorsOut), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Package id 0") {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					return strings.ReplaceAll(fields[3], "+", "")
				}
			}
		}
	}
	return ""
}

func GetRamUsage() string {
	freeOut, err := exec.Command("free", "-m").Output()
	if err == nil {
		lines := strings.Split(string(freeOut), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "Mem:") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					total, err1 := strconv.ParseFloat(fields[1], 64)
					used, err2 := strconv.ParseFloat(fields[2], 64)
					if err1 == nil && err2 == nil && total > 0 {
						return fmt.Sprintf("%.1f%%", used*100.0/total)
					}
				}
			}
		}
	}
	return "0.0%"
}

func GetPingCached(statusDir string) string {
	cacheFile := filepath.Join(statusDir, ".ping_cache")
	now := time.Now().Unix()

	if data, err := os.ReadFile(cacheFile); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) >= 2 {
			ts, err1 := strconv.ParseInt(strings.TrimSpace(lines[0]), 10, 64)
			val := strings.TrimSpace(lines[1])
			if err1 == nil && (now-ts) <= 30 {
				return val
			}
		}
	}

	go func() {
		cmd := exec.Command("ping", "-c", "1", "-W", "1", "8.8.8.8")
		out, err := cmd.Output()
		latency := ""
		if err == nil {
			re := regexp.MustCompile(`time=([\d.]+)`)
			matches := re.FindStringSubmatch(string(out))
			if len(matches) >= 2 {
				tStr := matches[1]
				if dotIdx := strings.Index(tStr, "."); dotIdx != -1 {
					latency = tStr[:dotIdx]
				} else {
					latency = tStr
				}
			}
		}

		_ = os.MkdirAll(statusDir, 0755)
		content := fmt.Sprintf("%d\n%s\n", time.Now().Unix(), latency)
		_ = os.WriteFile(cacheFile, []byte(content), 0644)
	}()

	if data, err := os.ReadFile(cacheFile); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) >= 2 {
			return strings.TrimSpace(lines[1])
		}
	}
	return ""
}

func GetWarningSummary() (int, string) {
	var warnings []string

	// 1. Renderer check
	renderer := ""
	glxOut, err := exec.Command("glxinfo", "-B").Output()
	if err == nil {
		lines := strings.Split(string(glxOut), "\n")
		for _, line := range lines {
			if strings.Contains(line, "OpenGL renderer string:") {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) == 2 {
					renderer = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}
	if strings.Contains(renderer, "llvmpipe") {
		warnings = append(warnings, "software-rendering")
	}

	// 2. Rustdesk check
	rustdeskState := "unknown"
	rdActiveOut, err := exec.Command("systemctl", "is-active", "rustdesk").Output()
	if err == nil {
		rustdeskState = strings.TrimSpace(string(rdActiveOut))
	}
	if rustdeskState != "active" {
		warnings = append(warnings, fmt.Sprintf("rustdesk-%s", rustdeskState))
	}

	// 3. Tailscale check
	tailscaleState := "unknown"
	tsActiveOut, err := exec.Command("systemctl", "is-active", "tailscaled").Output()
	if err == nil {
		tailscaleState = strings.TrimSpace(string(tsActiveOut))
	}

	tsIP := ""
	tsBackendState := "unknown"
	tsStatusOut, err := exec.Command("tailscale", "status", "--json").Output()
	if err == nil {
		var tsStatus struct {
			Self struct {
				TailscaleIPs []string `json:"TailscaleIPs"`
			} `json:"Self"`
			BackendState string `json:"BackendState"`
		}
		if err := json.Unmarshal(tsStatusOut, &tsStatus); err == nil {
			if len(tsStatus.Self.TailscaleIPs) > 0 {
				tsIP = tsStatus.Self.TailscaleIPs[0]
			}
			tsBackendState = tsStatus.BackendState
		} else {
			reIP := regexp.MustCompile(`"TailscaleIPs":\s*\[\s*"([^"]+)"`)
			matchesIP := reIP.FindStringSubmatch(string(tsStatusOut))
			if len(matchesIP) >= 2 {
				tsIP = matchesIP[1]
			}
			reState := regexp.MustCompile(`"BackendState":\s*"([^"]+)"`)
			matchesState := reState.FindStringSubmatch(string(tsStatusOut))
			if len(matchesState) >= 2 {
				tsBackendState = matchesState[1]
			}
		}
	}

	if tailscaleState != "active" || tsIP == "" {
		warnings = append(warnings, "tailscale")
	}

	if tailscaleState == "active" {
		switch tsBackendState {
		case "NeedsLogin", "Stopped":
			warnings = append(warnings, fmt.Sprintf("tailscale-%s", strings.ToLower(tsBackendState)))
		case "NoState", "Starting", "NoNetwork":
			warnings = append(warnings, "tailscale-offline")
		}
	}

	// 4. Display check
	displayConnected := false
	xrandrOut, err := exec.Command("xrandr").Output()
	if err == nil {
		lines := strings.Split(string(xrandrOut), "\n")
		for _, line := range lines {
			if strings.Contains(line, " connected") {
				displayConnected = true
				break
			}
		}
	}
	if !displayConnected {
		warnings = append(warnings, "display")
	}

	// 5. Applet symlink check
	home, errHome := os.UserHomeDir()
	if errHome == nil {
		appletDir := filepath.Join(home, ".local", "share", "cinnamon", "applets", "remote-studio@neek")
		appletJS := filepath.Join(appletDir, "applet.js")
		info, errL := os.Lstat(appletJS)
		if errL != nil || (info.Mode()&os.ModeSymlink == 0) {
			warnings = append(warnings, "applet-symlink")
		}
	}

	if len(warnings) == 0 {
		return 0, "OK"
	}
	return len(warnings), strings.Join(warnings, ",")
}

func GetNetSpeed() string {
	routeOut, err := exec.Command("ip", "route", "get", "8.8.8.8").Output()
	if err != nil {
		return "n/a"
	}
	fields := strings.Fields(string(routeOut))
	iface := ""
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			iface = fields[i+1]
			break
		}
	}
	if iface == "" {
		return "n/a"
	}

	rxPath := fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", iface)
	txPath := fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", iface)

	r1Data, err1 := os.ReadFile(rxPath)
	t1Data, err2 := os.ReadFile(txPath)
	if err1 != nil || err2 != nil {
		return "n/a"
	}

	r1, _ := strconv.ParseInt(strings.TrimSpace(string(r1Data)), 10, 64)
	t1, _ := strconv.ParseInt(strings.TrimSpace(string(t1Data)), 10, 64)

	time.Sleep(500 * time.Millisecond)

	r2Data, err3 := os.ReadFile(rxPath)
	t2Data, err4 := os.ReadFile(txPath)
	if err3 != nil || err4 != nil {
		return "n/a"
	}

	r2, _ := strconv.ParseInt(strings.TrimSpace(string(r2Data)), 10, 64)
	t2, _ := strconv.ParseInt(strings.TrimSpace(string(t2Data)), 10, 64)

	rx := (r2 - r1) / 512
	tx := (t2 - t1) / 512

	return fmt.Sprintf("↓%dKB/s ↑%dKB/s", rx, tx)
}

func GetActiveUsersAndConnection() (int, string, []string) {
	ssOut, err := exec.Command("ss", "-tnp").Output()
	if err != nil {
		return 0, "None", nil
	}

	var ips []string
	hasDirect := false

	lines := strings.Split(string(ssOut), "\n")
	for _, line := range lines {
		if (strings.Contains(line, "ESTAB") || strings.Contains(line, "estab")) && strings.Contains(line, "rustdesk") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				addr := fields[4]
				ip := addr
				if idx := strings.LastIndex(addr, ":"); idx != -1 {
					ip = addr[:idx]
				}
				ip = strings.TrimPrefix(ip, "[")
				ip = strings.TrimSuffix(ip, "]")

				ips = append(ips, ip)

				if strings.Contains(line, ":21118") {
					hasDirect = true
				}
			}
		}
	}

	ipMap := make(map[string]bool)
	var uniqueIps []string
	for _, ip := range ips {
		if !ipMap[ip] {
			ipMap[ip] = true
			uniqueIps = append(uniqueIps, ip)
		}
	}

	users := len(uniqueIps)
	connType := "None"
	if users > 0 {
		if hasDirect {
			connType = "Direct"
		} else {
			connType = "Relayed"
		}
	}

	return users, connType, uniqueIps
}

func GetRustdeskCodec(users int) string {
	if users <= 0 {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	logFile := filepath.Join(home, ".local", "share", "rustdesk", "log", "rustdesk.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		logFile = filepath.Join(home, ".rustdesk", "log", "rustdesk.log")
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	start := len(lines) - 50
	if start < 0 {
		start = 0
	}

	re := regexp.MustCompile(`[A-Za-z0-9_-]+(264|265|VP[89]|AV1)[A-Za-z0-9_-]*`)
	var lastCodec string
	for i := start; i < len(lines); i++ {
		if strings.Contains(strings.ToLower(lines[i]), "codec") {
			matches := re.FindStringSubmatch(lines[i])
			if len(matches) > 0 {
				lastCodec = matches[0]
			}
		}
	}
	return lastCodec
}

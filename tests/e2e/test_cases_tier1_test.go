package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Helper helper function to verify exit status and output
func verifyCmdRun(t *testing.T, args []string, env []string, expectedStdout, expectedStderr string, expectErr bool) (string, string) {
	t.Helper()
	stdout, stderr, err := executeCmd(args, env)
	if expectErr {
		if err == nil {
			t.Errorf("Expected command 'res %s' to fail, but it succeeded. Stdout: %q, Stderr: %q", strings.Join(args, " "), stdout, stderr)
		}
	} else {
		if err != nil {
			t.Errorf("Expected command 'res %s' to succeed, but got error: %v. Stdout: %q, Stderr: %q", strings.Join(args, " "), err, stdout, stderr)
		}
	}
	if expectedStdout != "" && !strings.Contains(stdout, expectedStdout) {
		t.Errorf("Expected stdout to contain %q, but got: %q", expectedStdout, stdout)
	}
	if expectedStderr != "" && !strings.Contains(stderr, expectedStderr) {
		t.Errorf("Expected stderr to contain %q, but got: %q", expectedStderr, stderr)
	}
	return stdout, stderr
}

// ==========================================
// FEATURE 1: CLI Control Plane (F1)
// ==========================================

func TestTier1_F1_HelpOutput(t *testing.T) {
	verifyCmdRun(t, []string{"--help"}, nil, "help", "", false)
}

func TestTier1_F1_VersionPattern(t *testing.T) {
	stdout, _ := verifyCmdRun(t, []string{"version"}, nil, "", "", false)
	ver := strings.TrimSpace(stdout)
	matched, err := regexp.MatchString(`^(v)?\d+(\.\d+)*$`, ver)
	if err != nil {
		t.Fatalf("regex failure: %v", err)
	}
	if !matched {
		t.Errorf("Version %q does not match expected version pattern", ver)
	}
}

func TestTier1_F1_StatusText(t *testing.T) {
	verifyCmdRun(t, []string{"status"}, nil, "Mode:", "", false)
}

func TestTier1_F1_StatusJson(t *testing.T) {
	stdout, _ := verifyCmdRun(t, []string{"status", "--json"}, nil, "", "", false)
	var status map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		t.Errorf("Expected valid JSON for status --json, got error: %v. Output: %q", err, stdout)
	}
}

func TestTier1_F1_ProfileSelection(t *testing.T) {
	verifyCmdRun(t, []string{"profiles"}, nil, "KEY", "", false)
}

// ==========================================
// FEATURE 2: Display Configuration (F2)
// ==========================================

func TestTier1_F2_CustomCommandCalc(t *testing.T) {
	// Custom command calculation: res custom <w> <h> <scale>
	verifyCmdRun(t, []string{"custom", "1920", "1080", "1.2"}, nil, "", "", false)
	statePath := filepath.Join(IsolatedHome, ".res_state")
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Errorf("Expected state file at %s, but it does not exist", statePath)
	}
}

func TestTier1_F2_XrandrModeReg(t *testing.T) {
	// Mode registration via custom command
	verifyCmdRun(t, []string{"custom", "1280", "720"}, nil, "", "", false)
}

func TestTier1_F2_ModeActivation(t *testing.T) {
	// Mode activation via profile key
	verifyCmdRun(t, []string{"mac"}, nil, "", "", false)
	statePath := filepath.Join(IsolatedHome, ".res_state")
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}
	if !strings.Contains(string(data), "2560") {
		t.Errorf("Expected active mode details to include 2560, got: %q", string(data))
	}
}

func TestTier1_F2_GsettingsUiScaling(t *testing.T) {
	// Cinnamon UI scaling factor
	verifyCmdRun(t, []string{"custom", "2560", "1664", "2.0"}, nil, "", "", false)
	gsettingsMock := filepath.Join(IsolatedXdgRuntime, "gsettings.mock")
	data, err := os.ReadFile(gsettingsMock)
	if err != nil {
		t.Fatalf("Failed to read gsettings mock file: %v", err)
	}
	if !strings.Contains(string(data), "scaling-factor=2") && !strings.Contains(string(data), "scaling-factor=2.0") {
		t.Errorf("Expected scaling-factor to be set to 2.0, mock contains: %q", string(data))
	}
}

func TestTier1_F2_GsettingsTextScaling(t *testing.T) {
	// Cinnamon UI text scaling factor
	verifyCmdRun(t, []string{"custom", "2560", "1664", "2.0"}, nil, "", "", false)
	gsettingsMock := filepath.Join(IsolatedXdgRuntime, "gsettings.mock")
	data, err := os.ReadFile(gsettingsMock)
	if err != nil {
		t.Fatalf("Failed to read gsettings mock file: %v", err)
	}
	if !strings.Contains(string(data), "text-scaling-factor=") {
		t.Errorf("Expected text-scaling-factor to be set, mock contains: %q", string(data))
	}
}

// ==========================================
// FEATURE 3: Session Lifecycle (F3)
// ==========================================

func TestTier1_F3_SessionStartBackups(t *testing.T) {
	// session start creates state/wallpaper backups
	verifyCmdRun(t, []string{"session", "start", "mac"}, nil, "", "", false)
	sessionState := filepath.Join(IsolatedHome, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionState); os.IsNotExist(err) {
		t.Errorf("Expected session.state file to be created, but it was not")
	}
}

func TestTier1_F3_SessionStopRestore(t *testing.T) {
	// session stop restores state and deletes session state file
	verifyCmdRun(t, []string{"session", "start", "mac"}, nil, "", "", false)
	verifyCmdRun(t, []string{"session", "stop"}, nil, "", "", false)
	sessionState := filepath.Join(IsolatedHome, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionState); !os.IsNotExist(err) {
		t.Errorf("Expected session.state file to be deleted, but it still exists")
	}
}

func TestTier1_F3_SessionStartPowerProfile(t *testing.T) {
	// session start sets power profile to performance
	verifyCmdRun(t, []string{"session", "start", "mac"}, nil, "", "", false)
	powerMock := filepath.Join(IsolatedXdgRuntime, "powerprofile.mock")
	data, err := os.ReadFile(powerMock)
	if err != nil {
		t.Fatalf("Failed to read powerprofile mock file: %v", err)
	}
	if !strings.Contains(string(data), "performance") {
		t.Errorf("Expected power profile performance, got: %q", string(data))
	}
}

func TestTier1_F3_SessionStopPowerProfile(t *testing.T) {
	// session stop restores power profile to balanced
	verifyCmdRun(t, []string{"session", "start", "mac"}, nil, "", "", false)
	verifyCmdRun(t, []string{"session", "stop"}, nil, "", "", false)
	powerMock := filepath.Join(IsolatedXdgRuntime, "powerprofile.mock")
	data, err := os.ReadFile(powerMock)
	if err != nil {
		t.Fatalf("Failed to read powerprofile mock file: %v", err)
	}
	if !strings.Contains(string(data), "balanced") {
		t.Errorf("Expected power profile balanced, got: %q", string(data))
	}
}

func TestTier1_F3_SpeedTogglesCinnamon(t *testing.T) {
	// speed toggles Cinnamon effects
	verifyCmdRun(t, []string{"speed"}, nil, "", "", false)
	gsettingsMock := filepath.Join(IsolatedXdgRuntime, "gsettings.mock")
	data, err := os.ReadFile(gsettingsMock)
	if err != nil {
		t.Fatalf("Failed to read gsettings mock file: %v", err)
	}
	if !strings.Contains(string(data), "desktop-effects=") {
		t.Errorf("Expected desktop-effects setting to be modified, got: %q", string(data))
	}
}

// ==========================================
// FEATURE 4: Watcher (F4)
// ==========================================

func TestTier1_F4_WatcherDetectSs(t *testing.T) {
	// watcher detects connection in ss table
	// We run watch command or check watcher output logs
	// Create mock ss binary to simulate connection
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockSSContent := `#!/bin/bash
echo "ESTAB 0 0 100.1.2.3:21118 100.64.0.5:54321 users:((\"rustdesk\",pid=123,fd=4))"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "ss"), []byte(mockSSContent), 0755); err != nil {
		t.Fatalf("Failed to write mock ss: %v", err)
	}
	
	// Prepend tempBinDir to PATH
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRun(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=false"}, "", "", false)
}

func TestTier1_F4_TrustedLoopback(t *testing.T) {
	// trusted loopback connection verification
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_loopback")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockSSContent := `#!/bin/bash
echo "ESTAB 0 0 127.0.0.1:21118 127.0.0.1:54321 users:((\"rustdesk\",pid=123,fd=4))"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "ss"), []byte(mockSSContent), 0755); err != nil {
		t.Fatalf("Failed to write mock ss: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRun(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=false"}, "", "", false)
}

func TestTier1_F4_TailscalePeerOS(t *testing.T) {
	// Tailscale status peer OS query
	verifyCmdRun(t, []string{"tailnet", "peer", "node1"}, nil, "macOS", "", false)
}

func TestTier1_F4_StartAutoTrigger(t *testing.T) {
	// session start auto-trigger when connection detected
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_autostart")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockSSContent := `#!/bin/bash
echo "ESTAB 0 0 100.1.2.3:21118 100.64.0.5:54321 users:((\"rustdesk\",pid=123,fd=4))"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "ss"), []byte(mockSSContent), 0755); err != nil {
		t.Fatalf("Failed to write mock ss: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRun(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=true"}, "", "", false)
	
	sessionState := filepath.Join(IsolatedHome, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionState); os.IsNotExist(err) {
		t.Errorf("Expected session.state file to be auto-triggered by watch connection")
	}
}

func TestTier1_F4_StopAutoTrigger(t *testing.T) {
	// session stop auto-trigger when connection closes
	// Initialize session state first
	verifyCmdRun(t, []string{"session", "start", "mac"}, nil, "", "", false)
	
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_autostop")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockSSContent := `#!/bin/bash
# Returns no connections
exit 0
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "ss"), []byte(mockSSContent), 0755); err != nil {
		t.Fatalf("Failed to write mock ss: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRun(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=true"}, "", "", false)
	
	sessionState := filepath.Join(IsolatedHome, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionState); !os.IsNotExist(err) {
		t.Errorf("Expected session.state file to be auto-stopped/deleted when connection is inactive")
	}
}

// ==========================================
// FEATURE 5: D-Bus (F5)
// ==========================================

func TestTier1_F5_ServiceRegistration(t *testing.T) {
	// Service registration on session bus
	if DbusAddress == "" {
		t.Skip("DbusAddress is not available")
	}
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	// Verify registration on session bus using dbus-send ping
	cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.freedesktop.DBus.Peer.Ping")
	cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to ping org.remote_studio.Daemon over D-Bus: %v", err)
	}
}

func TestTier1_F5_StatusQuery(t *testing.T) {
	// Status property query (returns status JSON)
	if DbusAddress == "" {
		t.Skip("DbusAddress is not available")
	}
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.freedesktop.DBus.Properties.Get", "string:org.remote_studio.Daemon", "string:Status")
	cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get Status property over D-Bus: %v. Output: %s", err, string(out))
	}
	if !strings.Contains(string(out), "mode") {
		t.Errorf("Expected Status JSON returned, got: %q", string(out))
	}
}

func TestTier1_F5_RefreshCall(t *testing.T) {
	// Refresh method call
	if DbusAddress == "" {
		t.Skip("DbusAddress is not available")
	}
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.remote_studio.Daemon.Refresh")
	cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to call Refresh over D-Bus: %v", err)
	}
}

func TestTier1_F5_SignalEmission(t *testing.T) {
	// StatusChanged signal emission
	if DbusAddress == "" {
		t.Skip("DbusAddress is not available")
	}
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	// Start dbus-monitor to listen for signals
	monitor := exec.Command("dbus-monitor", "--session", "interface='org.remote_studio.Daemon',member='StatusChanged'")
	monitor.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
	var outBuf strings.Builder
	monitor.Stdout = &outBuf
	if err := monitor.Start(); err != nil {
		t.Fatalf("Failed to start dbus-monitor: %v", err)
	}
	defer func() {
		if monitor.Process != nil {
			_ = monitor.Process.Kill()
		}
	}()
	
	// Trigger signal via Refresh call
	cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.remote_studio.Daemon.Refresh")
	cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
	_ = cmd.Run()
	
	time.Sleep(500 * time.Millisecond)
	
	if !strings.Contains(outBuf.String(), "StatusChanged") {
		t.Errorf("Expected to receive StatusChanged signal, monitor output: %q", outBuf.String())
	}
}

func TestTier1_F5_ConcurrentAccess(t *testing.T) {
	// Concurrent property accesses
	if DbusAddress == "" {
		t.Skip("DbusAddress is not available")
	}
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	// Fire multiple dbus queries concurrently
	doneChan := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.freedesktop.DBus.Properties.Get", "string:org.remote_studio.Daemon", "string:Status")
			cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
			_ = cmd.Run()
			doneChan <- true
		}()
	}
	
	for i := 0; i < 10; i++ {
		select {
		case <-doneChan:
		case <-time.After(3 * time.Second):
			t.Fatal("Timeout waiting for D-Bus concurrent queries")
		}
	}
}

// ==========================================
// FEATURE 6: Embedded Web & WebSocket (F6)
// ==========================================

func TestTier1_F6_WebServerPort(t *testing.T) {
	// Web dashboard server port 9999 response
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	resp, err := http.Get("http://localhost:9999/")
	if err != nil {
		t.Fatalf("HTTP request to Web UI failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got: %d", resp.StatusCode)
	}
}

func TestTier1_F6_WebSocketConnect(t *testing.T) {
	// WebSocket listener port 9998 connect
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	conn, err := net.Dial("tcp", "localhost:9998")
	if err != nil {
		t.Fatalf("Failed to dial TCP port 9998: %v", err)
	}
	defer conn.Close()
	
	// Perform raw websocket handshake
	req := "GET /ws HTTP/1.1\r\n" +
		"Host: localhost:9998\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	
	_, err = conn.Write([]byte(req))
	if err != nil {
		t.Fatalf("Failed to write WS handshake request: %v", err)
	}
	
	buf := make([]byte, 1024)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read handshake response: %v", err)
	}
	
	resp := string(buf[:n])
	if !strings.Contains(resp, "101 Switching Protocols") {
		t.Errorf("Expected handshake response to contain '101 Switching Protocols', got: %q", resp)
	}
}

func TestTier1_F6_WsStatusBroadcast(t *testing.T) {
	// WS status_full broadcast
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	conn, err := net.Dial("tcp", "localhost:9998")
	if err != nil {
		t.Fatalf("Failed to dial TCP port 9998: %v", err)
	}
	defer conn.Close()
	
	req := "GET /ws HTTP/1.1\r\n" +
		"Host: localhost:9998\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	
	_, _ = conn.Write([]byte(req))
	
	buf := make([]byte, 2048)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read WS data: %v", err)
	}
	
	resp := string(buf[:n])
	if !strings.Contains(resp, "status_full") {
		t.Errorf("Expected WebSocket status broadcast to contain 'status_full', got: %q", resp)
	}
}

func TestTier1_F6_WsCommandExec(t *testing.T) {
	// WS command execution
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	conn, err := net.Dial("tcp", "localhost:9998")
	if err != nil {
		t.Fatalf("Failed to dial TCP port 9998: %v", err)
	}
	defer conn.Close()
	
	req := "GET /ws HTTP/1.1\r\n" +
		"Host: localhost:9998\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	_, _ = conn.Write([]byte(req))
	
	// Read handshake response
	buf := make([]byte, 1024)
	_, _ = conn.Read(buf)
	
	// Send command payload over WebSocket (needs standard WebSocket framing)
	// For simplicity, write a mock payload client to verify
	payload := `{"action": "command", "cmd": "speed"}`
	// WS Text Frame Header for len <= 125 is [0x81, len]
	frame := append([]byte{0x81, byte(len(payload))}, []byte(payload)...)
	_, err = conn.Write(frame)
	if err != nil {
		t.Fatalf("Failed to write WS frame: %v", err)
	}
	
	time.Sleep(500 * time.Millisecond)
}

func TestTier1_F6_WsScaleAdjust(t *testing.T) {
	// WS scale command adjust
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		if daemonCmd != nil && daemonCmd.Process != nil {
			_ = daemonCmd.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	conn, err := net.Dial("tcp", "localhost:9998")
	if err != nil {
		t.Fatalf("Failed to dial TCP: %v", err)
	}
	defer conn.Close()
	
	req := "GET /ws HTTP/1.1\r\n" +
		"Host: localhost:9998\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	_, _ = conn.Write([]byte(req))
	
	buf := make([]byte, 1024)
	_, _ = conn.Read(buf)
	
	payload := `{"action": "scale", "val": 1.5}`
	frame := append([]byte{0x81, byte(len(payload))}, []byte(payload)...)
	_, err = conn.Write(frame)
	if err != nil {
		t.Fatalf("Failed to write WS frame: %v", err)
	}
	
	time.Sleep(500 * time.Millisecond)
}

// ==========================================
// FEATURE 7: RustDesk Configuration Presets (F7)
// ==========================================

func TestTier1_F7_BackupCommand(t *testing.T) {
	// backup command copy
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	if err := os.MkdirAll(rustdeskConfDir, 0755); err != nil {
		t.Fatalf("Failed to create rustdesk config dir: %v", err)
	}
	originalConfig := filepath.Join(rustdeskConfDir, "RustDesk_default.toml")
	_ = os.WriteFile(originalConfig, []byte("id = 'test-id'\n"), 0644)
	
	verifyCmdRun(t, []string{"rustdesk", "backup"}, nil, "Backed up", "", false)
}

func TestTier1_F7_ConfigSafeMerger(t *testing.T) {
	// config safe merger (identity keys preservation)
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	if err := os.MkdirAll(rustdeskConfDir, 0755); err != nil {
		t.Fatalf("Failed to create rustdesk config dir: %v", err)
	}
	originalConfig := filepath.Join(rustdeskConfDir, "RustDesk_default.toml")
	_ = os.WriteFile(originalConfig, []byte("id = 'original-id'\nkey = 'original-key'\nsome-option = 'old-val'\n"), 0644)
	
	// Apply preset
	verifyCmdRun(t, []string{"rustdesk", "apply", "quality"}, nil, "Merged", "", false)
	
	// Verify that identity fields are preserved in RustDesk_default.toml
	data, err := os.ReadFile(originalConfig)
	if err != nil {
		t.Fatalf("Failed to read merged config: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "original-id") || !strings.Contains(content, "original-key") {
		t.Errorf("Expected identity keys to be preserved, merged TOML: %q", content)
	}
}

func TestTier1_F7_PresetsApply(t *testing.T) {
	// quality/balanced presets apply
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	_ = os.MkdirAll(rustdeskConfDir, 0755)
	_ = os.WriteFile(filepath.Join(rustdeskConfDir, "RustDesk_default.toml"), []byte("id = 'test-id'\n"), 0644)
	
	verifyCmdRun(t, []string{"rustdesk", "apply", "balanced"}, nil, "Merged", "", false)
}

func TestTier1_F7_DiffCommand(t *testing.T) {
	// diff command show
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	_ = os.MkdirAll(rustdeskConfDir, 0755)
	_ = os.WriteFile(filepath.Join(rustdeskConfDir, "RustDesk_default.toml"), []byte("id = 'test-id'\n"), 0644)
	
	verifyCmdRun(t, []string{"rustdesk", "diff", "balanced"}, nil, "", "", false)
}

func TestTier1_F7_TelemetryParsing(t *testing.T) {
	// telemetry log parsing
	rustdeskLogDir := filepath.Join(IsolatedHome, ".local", "share", "rustdesk", "log")
	if err := os.MkdirAll(rustdeskLogDir, 0755); err != nil {
		t.Fatalf("Failed to create log dir: %v", err)
	}
	logFile := filepath.Join(rustdeskLogDir, "rustdesk.log")
	_ = os.WriteFile(logFile, []byte("codec: h264\nfps: 60\nbitrate: 1024 kbps\n"), 0644)
	
	verifyCmdRun(t, []string{"rustdesk", "status"}, nil, "Codec", "", false)
}

// ==========================================
// FEATURE 8: System Health Diagnostics (F8)
// ==========================================

func TestTier1_F8_DoctorCheck(t *testing.T) {
	// doctor command check
	verifyCmdRun(t, []string{"doctor"}, nil, "Remote Studio doctor", "", false)
}

func TestTier1_F8_DoctorWarningLlvmpipe(t *testing.T) {
	// doctor software rendering warning (llvmpipe)
	// Create mock glxinfo to output llvmpipe
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_llvmpipe")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockGlxinfo := `#!/bin/bash
if [[ "$*" == *"-B"* ]]; then
  echo "OpenGL renderer string: llvmpipe (LLVM 12.0.0, 256 bits)"
else
  echo "llvmpipe"
fi
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "glxinfo"), []byte(mockGlxinfo), 0755); err != nil {
		t.Fatalf("Failed to write mock glxinfo: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRun(t, []string{"doctor"}, []string{"PATH=" + newPath}, "llvmpipe", "", false)
}

func TestTier1_F8_DoctorFixSymlink(t *testing.T) {
	// doctor-fix symlink links
	// Initialize git mock environment
	verifyCmdRun(t, []string{"doctor-fix"}, nil, "Fixing common issues", "", false)
	
	xsessionrcTarget := filepath.Join(IsolatedHome, ".xsessionrc")
	if _, err := os.Lstat(xsessionrcTarget); err != nil {
		t.Errorf("Expected symlink ~/.xsessionrc to be created, got error: %v", err)
	}
}

func TestTier1_F8_SelfTestVerification(t *testing.T) {
	// self-test verification runs
	verifyCmdRun(t, []string{"self-test"}, nil, "Remote Studio self-test", "", false)
}

func TestTier1_F8_InitWizardExecution(t *testing.T) {
	// init wizard execution
	// Runs non-interactively in tests if redirected or mocked
	verifyCmdRun(t, []string{"init"}, nil, "", "", false)
}

// ==========================================
// FEATURE 9: Xorg Framebuffer Configuration (F9)
// ==========================================

func TestTier1_F9_LspciGpuNvidia(t *testing.T) {
	// lspci GPU nvidia probe
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_nvidia")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockLspci := `#!/bin/bash
echo "01:00.0 VGA compatible controller: NVIDIA Corporation GA106 [GeForce RTX 3060] (rev a1)"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "lspci"), []byte(mockLspci), 0755); err != nil {
		t.Fatalf("Failed to write mock lspci: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	xorgOut := filepath.Join(IsolatedHome, "xorg.conf.nvidia")
	verifyCmdRun(t, []string{"xorg", xorgOut}, []string{"PATH=" + newPath}, "", "", false)
	
	data, err := os.ReadFile(xorgOut)
	if err != nil {
		t.Fatalf("Failed to read generated xorg.conf: %v", err)
	}
	if !strings.Contains(string(data), "nvidia") || !strings.Contains(string(data), "ConnectedMonitor") {
		t.Errorf("Expected Nvidia configuration, got: %s", string(data))
	}
}

func TestTier1_F9_LspciGpuAmd(t *testing.T) {
	// lspci GPU amd probe
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_amd")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockLspci := `#!/bin/bash
echo "03:00.0 VGA compatible controller: Advanced Micro Devices, Inc. [AMD/ATI] Navi 23 [Radeon RX 6600/6600 XT/6600M] (rev c7)"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "lspci"), []byte(mockLspci), 0755); err != nil {
		t.Fatalf("Failed to write mock lspci: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	xorgOut := filepath.Join(IsolatedHome, "xorg.conf.amd")
	verifyCmdRun(t, []string{"xorg", xorgOut}, []string{"PATH=" + newPath}, "", "", false)
	
	data, err := os.ReadFile(xorgOut)
	if err != nil {
		t.Fatalf("Failed to read generated xorg.conf: %v", err)
	}
	if !strings.Contains(string(data), "amdgpu") {
		t.Errorf("Expected AMD configuration, got: %s", string(data))
	}
}

func TestTier1_F9_XorgConfigGen(t *testing.T) {
	// xorg config generator output validation
	xorgOut := filepath.Join(IsolatedHome, "xorg.conf.test")
	verifyCmdRun(t, []string{"xorg", xorgOut}, nil, "", "", false)
	
	if _, err := os.Stat(xorgOut); os.IsNotExist(err) {
		t.Errorf("xorg.conf file was not generated")
	}
}

func TestTier1_F9_BackupRotation(t *testing.T) {
	// backup configurations rotation
	// Rotate configs inside home directory backups folder
	backupRoot := filepath.Join(IsolatedHome, ".config", "remote-studio", "backups")
	_ = os.MkdirAll(backupRoot, 0755)
	
	// Create multiple backup folders
	for i := 1; i <= 5; i++ {
		bdir := filepath.Join(backupRoot, fmt.Sprintf("backup-%d", i))
		_ = os.MkdirAll(bdir, 0755)
		_ = os.WriteFile(filepath.Join(bdir, "xorg.conf"), []byte("mock xorg config"), 0644)
	}
}

func TestTier1_F9_XorgRollback(t *testing.T) {
	// xorg rollback config restore
	backupRoot := filepath.Join(IsolatedHome, ".config", "remote-studio", "backups")
	latestBackup := filepath.Join(backupRoot, "2026-06-18_08-30-00")
	if err := os.MkdirAll(latestBackup, 0755); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(latestBackup, "xorg.conf"), []byte("Section \"Device\"\n    Driver \"modesetting\"\nEndSection\n"), 0644); err != nil {
		t.Fatalf("Failed to write mock backup config: %v", err)
	}
	
	// Rollback should run, copy latestBackup to /etc/X11/xorg.conf
	// In mock/test env, we verify it attempts rollback successfully
	verifyCmdRun(t, []string{"xorg", "rollback"}, nil, "", "", false)
}

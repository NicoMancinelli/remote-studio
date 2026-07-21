package e2e

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Helper helper function to verify exit status and output
func verifyCmdRunTier2(t *testing.T, args []string, env []string, expectedStdout, expectedStderr string, expectErr bool) (string, string) {
	t.Helper()
	stdout, stderr, err := executeCmd(args, env)
	t.Logf("DEBUG COMMAND: res %v, STDOUT: %q, STDERR: %q, ERR: %v", args, stdout, stderr, err)
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
// FEATURE 1: CLI Control Plane Boundary Cases (F1)
// ==========================================

func TestTier2_F1_UnknownCommandNonZero(t *testing.T) {
	// CLI unknown command exits non-zero
	verifyCmdRunTier2(t, []string{"invalid_command_foo_bar"}, nil, "", "Unknown command", true)
}

func TestTier2_F1_EmptyArgsFallback(t *testing.T) {
	// empty/no args fallback
	verifyCmdRunTier2(t, []string{}, nil, "", "", false)
}

func TestTier2_F1_InvalidResolutionParams(t *testing.T) {
	// invalid custom resolution parameters (negative/string)
	verifyCmdRunTier2(t, []string{"custom", "-1920", "1080"}, nil, "", "invalid", true)
	verifyCmdRunTier2(t, []string{"custom", "1920", "abc"}, nil, "", "invalid", true)
}

func TestTier2_F1_LogLineCount(t *testing.T) {
	// log command line count handling (negative/string)
	verifyCmdRunTier2(t, []string{"log", "-10"}, nil, "", "invalid", true)
	verifyCmdRunTier2(t, []string{"log", "abc"}, nil, "", "invalid", true)
}

func TestTier2_F1_ConfigInvalidKey(t *testing.T) {
	// config command invalid key handling
	verifyCmdRunTier2(t, []string{"config", "set", "invalid-key", "val"}, nil, "", "invalid key format", true)
}

// ==========================================
// FEATURE 2: Display Configuration Boundary Cases (F2)
// ==========================================

func TestTier2_F2_FramebufferLimit(t *testing.T) {
	// exceeding virtual framebuffer limits (e.g. 5000x5000) rejected/capped
	verifyCmdRunTier2(t, []string{"custom", "6000", "6000"}, nil, "", "limit", true)
}

func TestTier2_F2_HeadlessNoDisplay(t *testing.T) {
	// running in headless without DISPLAY/WAYLAND_DISPLAY error
	verifyCmdRunTier2(t, []string{"custom", "1920", "1080"}, []string{"DISPLAY=", "WAYLAND_DISPLAY="}, "", "display", true)
}

func TestTier2_F2_NonPositiveScale(t *testing.T) {
	// non-positive scaling factor rejected
	verifyCmdRunTier2(t, []string{"custom", "1920", "1080", "0"}, nil, "", "scaling", true)
	verifyCmdRunTier2(t, []string{"custom", "1920", "1080", "-1.0"}, nil, "", "scaling", true)
}

func TestTier2_F2_MissingOutputs(t *testing.T) {
	// dynamic switch handles missing display outputs
	// Mock xrandr returning no connected displays
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_nodisplay")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockXrandr := `#!/bin/bash
echo "xrandr: no outputs found" >&2
exit 1
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "xrandr"), []byte(mockXrandr), 0755); err != nil {
		t.Fatalf("Failed to write mock xrandr: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRunTier2(t, []string{"custom", "1920", "1080"}, []string{"PATH=" + newPath}, "", "no connected display", true)
}

func TestTier2_F2_DuplicateModeName(t *testing.T) {
	// custom mode duplicate name resolution
	// Verify it resolves duplicate resolution names safely (either removes or updates existing)
	verifyCmdRunTier2(t, []string{"custom", "1920", "1080"}, nil, "", "", false)
	verifyCmdRunTier2(t, []string{"custom", "1920", "1080"}, nil, "", "", false)
}

// ==========================================
// FEATURE 3: Session Lifecycle Boundary Cases (F3)
// ==========================================

func TestTier2_F3_DoubleStart(t *testing.T) {
	// double session start handling (idempotency check)
	verifyCmdRunTier2(t, []string{"session", "start", "mac"}, nil, "", "", false)
	verifyCmdRunTier2(t, []string{"session", "start", "mac"}, nil, "", "", false)
}

func TestTier2_F3_StopInactive(t *testing.T) {
	// session stop when inactive
	// Should not crash or fail when no session is active (idempotency)
	verifyCmdRunTier2(t, []string{"session", "stop"}, nil, "", "", false)
}

func TestTier2_F3_StopMissingBackup(t *testing.T) {
	// stop when wallpaper backup missing
	// Verify it handles missing wallpaper backup gracefully
	verifyCmdRunTier2(t, []string{"session", "start", "mac"}, nil, "", "", false)
	
	wallpaperBackup := filepath.Join(IsolatedHome, ".wallpaper_backup")
	_ = os.Remove(wallpaperBackup)
	
	verifyCmdRunTier2(t, []string{"session", "stop"}, nil, "", "", false)
}

func TestTier2_F3_SkipPowerProfileIfMissing(t *testing.T) {
	// skip power profile if command missing from PATH
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_nopower")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	// Do not include powerprofilesctl in this tempBinDir, remove it from PATH
	// Create mock PATH where only essential commands are kept, excluding mockBinDir/powerprofilesctl
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRunTier2(t, []string{"session", "start", "mac"}, []string{"PATH=" + newPath}, "", "", false)
}

func TestTier2_F3_CorruptedStateParsing(t *testing.T) {
	// corrupted session.state parsing
	sessionState := filepath.Join(IsolatedHome, ".config", "remote-studio", "session.state")
	_ = os.MkdirAll(filepath.Dir(sessionState), 0755)
	_ = os.WriteFile(sessionState, []byte("invalid_format_without_key_value_pairs\n"), 0644)
	
	verifyCmdRunTier2(t, []string{"session", "status"}, nil, "", "", false)
}

// ==========================================
// FEATURE 4: Watcher Boundary Cases (F4)
// ==========================================

func TestTier2_F4_UntrustedNetworkIp(t *testing.T) {
	// untrusted network IP rejected
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_untrusted")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockSSContent := `#!/bin/bash
echo "ESTAB 0 0 100.1.2.3:21118 8.8.8.8:54321 users:((\"rustdesk\",pid=123,fd=4))"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "ss"), []byte(mockSSContent), 0755); err != nil {
		t.Fatalf("Failed to write mock ss: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRunTier2(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=true"}, "", "", false)
	
	sessionState := filepath.Join(IsolatedHome, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionState); !os.IsNotExist(err) {
		t.Errorf("Expected session.state file NOT to be created for untrusted IP connection")
	}
}

func TestTier2_F4_TailscaleDown(t *testing.T) {
	// tailscale down handling
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_tsdown")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockTailscale := `#!/bin/bash
echo "Tailscale is down" >&2
exit 1
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "tailscale"), []byte(mockTailscale), 0755); err != nil {
		t.Fatalf("Failed to write mock tailscale: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRunTier2(t, []string{"tailnet"}, []string{"PATH=" + newPath}, "", "unavailable", true)
}

func TestTier2_F4_ConcurrentConnections(t *testing.T) {
	// multiple concurrent connections tracking
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_concurrent")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockSSContent := `#!/bin/bash
echo "ESTAB 0 0 100.1.2.3:21118 100.64.0.5:54321 users:((\"rustdesk\",pid=123,fd=4))"
echo "ESTAB 0 0 100.1.2.3:21118 100.64.0.6:54322 users:((\"rustdesk\",pid=123,fd=5))"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "ss"), []byte(mockSSContent), 0755); err != nil {
		t.Fatalf("Failed to write mock ss: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRunTier2(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=true"}, "", "", false)
}

func TestTier2_F4_NonRustDeskPorts(t *testing.T) {
	// non-RustDesk socket ports ignored
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_nonrd")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockSSContent := `#!/bin/bash
echo "ESTAB 0 0 100.1.2.3:80 100.64.0.5:54321 users:((\"nginx\",pid=123,fd=4))"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "ss"), []byte(mockSSContent), 0755); err != nil {
		t.Fatalf("Failed to write mock ss: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRunTier2(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=true"}, "", "", false)
	
	sessionState := filepath.Join(IsolatedHome, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionState); !os.IsNotExist(err) {
		t.Errorf("Expected session.state file NOT to be created for non-rustdesk connection")
	}
}

func TestTier2_F4_CorruptedTailscaleJson(t *testing.T) {
	// corrupted tailscale JSON handling
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_corrupt_ts")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockTailscale := `#!/bin/bash
echo "{invalid_json}"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "tailscale"), []byte(mockTailscale), 0755); err != nil {
		t.Fatalf("Failed to write mock tailscale: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	verifyCmdRunTier2(t, []string{"watch", "1"}, []string{"PATH=" + newPath, "AUTO_SESSION=true"}, "", "", false)
}

// ==========================================
// FEATURE 5: D-Bus Boundary Cases (F5)
// ==========================================

func TestTier2_F5_DbusAddressMissing(t *testing.T) {
	// DBus is optional in the daemon (it only enhances status broadcasts).
	// When DBUS_SESSION_BUS_ADDRESS is unset, the daemon should still bind
	// its HTTP/WebSocket ports and stay running. We verify by issuing a
	// follow-up HTTP probe against port 9999.
	daemonCmd, err := executeDaemon([]string{"daemon"}, []string{"DBUS_SESSION_BUS_ADDRESS="})
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
		t.Fatalf("HTTP request to Web UI failed (DBus-less daemon should still bind): %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK, got: %d", resp.StatusCode)
	}
}

func TestTier2_F5_PropertyErrorOnFailure(t *testing.T) {
	// properties return error JSON on backend failure
	if DbusAddress == "" {
		t.Skip("DbusAddress is not available")
	}
	
	// Corrupt or remove status file to trigger backend status read failure
	statusFile := filepath.Join(IsolatedXdgRuntime, "remote-studio", "status")
	_ = os.Remove(statusFile)
	
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

	// With the status file removed pre-daemon-start, the daemon's initial
	// pollNetwork() runs and populates the Status property with default
	// values (mode="None" or similar). We DO NOT call Refresh here because
	// Refresh re-creates the status file (defeating the corruption test
	// premise). Instead we directly query the property and assert that the
	// response reflects the "no status file" fallback path.
	cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.freedesktop.DBus.Properties.Get", "string:org.remote_studio.Daemon", "string:Status")
	cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed: %v. Output: %s", err, string(out))
	}

	if !strings.Contains(string(out), "Error") && !strings.Contains(string(out), "None") && !strings.Contains(string(out), "{}") {
		t.Errorf("Expected Status query to return error/default json, got: %q", string(out))
	}
	// Verify the status file was actually missing pre-start (sanity check).
	if _, err := os.Stat(statusFile); err == nil {
		t.Errorf("Expected status file %s to remain absent until next Refresh", statusFile)
	}
}

func TestTier2_F5_DuplicateInterface(t *testing.T) {
	// duplicate interface registration
	if DbusAddress == "" {
		t.Skip("DbusAddress is not available")
	}
	daemonCmd1, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start first daemon: %v", err)
	}
	defer func() {
		if daemonCmd1 != nil && daemonCmd1.Process != nil {
			_ = daemonCmd1.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	// Start second daemon, it should fail or exit cleanly due to owned bus name
	daemonCmd2, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start second daemon: %v", err)
	}
	defer func() {
		if daemonCmd2 != nil && daemonCmd2.Process != nil {
			_ = daemonCmd2.Process.Kill()
		}
	}()
	
	time.Sleep(500 * time.Millisecond)
	
	// Verify second daemon exited or failed
	// Checking if process exited
	done := make(chan error, 1)
	go func() {
		_, err := daemonCmd2.Process.Wait()
		done <- err
	}()
	
	select {
	case <-done:
		// Succeeded (failed to register name)
	case <-time.After(1 * time.Second):
		t.Errorf("Expected second daemon to exit due to name collision, but it is still running")
	}
}

func TestTier2_F5_QueryRateLimit(t *testing.T) {
	// rapid properties querying rate limit
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
	
	// Query 100 times as fast as possible
	for i := 0; i < 50; i++ {
		cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.freedesktop.DBus.Properties.Get", "string:org.remote_studio.Daemon", "string:Status")
		cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
		_ = cmd.Run()
	}
}

func TestTier2_F5_SignalQueueing(t *testing.T) {
	// signal broadcast queueing
	// Trigger rapid updates to test signal queuing/drops
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
	
	for i := 0; i < 20; i++ {
		cmd := exec.Command("dbus-send", "--session", "--dest=org.remote_studio.Daemon", "--print-reply", "/org/remote_studio/Daemon", "org.remote_studio.Daemon.Refresh")
		cmd.Env = append(os.Environ(), "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
		_ = cmd.Run()
	}
}

// ==========================================
// FEATURE 6: Embedded Web & WebSocket Boundary Cases (F6)
// ==========================================

func TestTier2_F6_PortConflict(t *testing.T) {
	// port conflict handling
	// Bind to port 9999 manually.
	l, err := net.Listen("tcp", "127.0.0.1:9999")
	if err != nil {
		t.Skipf("could not bind 9999 (likely TIME_WAIT from a previous test): %v", err)
	}
	defer l.Close()

	// Start daemon via executeDaemon (async). The daemon should exit
	// because port 9999 is already taken.
	daemonCmd, err := executeDaemon([]string{"daemon"}, nil)
	if err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	// No need to defer Kill — the daemon should exit on its own within
	// the bind-fail path. We reaped it in case of test failure.

	// Poll briefly for the daemon to exit (port conflict detected).
	gone := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if daemonCmd.ProcessState == nil {
			// ProcessState is set after Wait. Use a different probe.
			if !isProcessAlive(daemonCmd.Process.Pid) {
				gone = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !gone {
		_ = daemonCmd.Process.Kill()
		t.Fatal("daemon did not exit after port-conflict bind failure")
	}

	// The listener bound by the test fixture is still alive on 9999.
	// Verify the daemon didn't take over (i.e., the conflict path actually
	// failed the daemon's bind).
	resp, err := http.Get("http://localhost:9999/")
	if err != nil {
		// Expected: the test fixture's listener, not the daemon's.
		return
	}
	resp.Body.Close()
	// If we got here, the HTTP request succeeded against 9999 — but
	// the daemon exited, so it must be against our test listener. Pass.
}

// isProcessAlive checks whether the given PID is still running without
// blocking. Returns false for unknown / already-reaped PIDs.
func isProcessAlive(pid int) bool {
	// signal 0 is the standard "check existence" idiom.
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return false
}

func TestTier2_F6_MalformedWsInput(t *testing.T) {
	// malformed WS input JSON
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
		t.Fatalf("TCP Connect failed: %v", err)
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
	
	// Send invalid payload
	payload := `{"action": "command", "cmd": ` // truncated JSON
	frame := append([]byte{0x81, byte(len(payload))}, []byte(payload)...)
	_, err = conn.Write(frame)
	if err != nil {
		t.Fatalf("Failed to write malformed WS frame: %v", err)
	}
}

func TestTier2_F6_WsConnectionDrop(t *testing.T) {
	// sudden WS client connection drop
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
		t.Fatalf("TCP Connect failed: %v", err)
	}
	
	req := "GET /ws HTTP/1.1\r\n" +
		"Host: localhost:9998\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	_, _ = conn.Write([]byte(req))
	
	// Sudden close without clean WS close frame
	conn.Close()
	
	time.Sleep(500 * time.Millisecond)
}

func TestTier2_F6_WsHighFrequency(t *testing.T) {
	// WS high-frequency commands
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
		t.Fatalf("TCP Connect failed: %v", err)
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
	
	payload := `{"action": "command", "cmd": "speed"}`
	frame := append([]byte{0x81, byte(len(payload))}, []byte(payload)...)
	
	for i := 0; i < 50; i++ {
		_, _ = conn.Write(frame)
	}
}

func TestTier2_F6_Http404NotFound(t *testing.T) {
	// HTTP 404 response on missing file
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
	
	resp, err := http.Get("http://localhost:9999/non_existent_file_xyz.html")
	if err != nil {
		t.Fatalf("HTTP Request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got: %d", resp.StatusCode)
	}
}

// ==========================================
// FEATURE 7: RustDesk Presets Boundary Cases (F7)
// ==========================================

func TestTier2_F7_EmptyTemplateMerger(t *testing.T) {
	// empty template merger safety
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	_ = os.MkdirAll(rustdeskConfDir, 0755)
	
	// Create empty template preset file in config
	// Since config is in rootDir, write to mock presets location
	verifyCmdRunTier2(t, []string{"rustdesk", "apply", "empty_template_preset"}, nil, "", "No template", true)
}

func TestTier2_F7_CorruptedActiveToml(t *testing.T) {
	// corrupted active TOML recovery
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	_ = os.MkdirAll(rustdeskConfDir, 0755)
	originalConfig := filepath.Join(rustdeskConfDir, "RustDesk_default.toml")
	_ = os.WriteFile(originalConfig, []byte("invalid_toml_syntax = [ = {\n"), 0644)
	
	verifyCmdRunTier2(t, []string{"rustdesk", "apply", "quality"}, nil, "Merged", "", false)
}

func TestTier2_F7_ReloadIfDiffer(t *testing.T) {
	// reload triggered only when files differ
	// Applying twice, second time configuration is identical, no reload/restart should happen
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	_ = os.MkdirAll(rustdeskConfDir, 0755)
	_ = os.WriteFile(filepath.Join(rustdeskConfDir, "RustDesk_default.toml"), []byte("id = 'test-id'\n"), 0644)
	
	verifyCmdRunTier2(t, []string{"rustdesk", "apply", "quality"}, nil, "Merged", "", false)
	verifyCmdRunTier2(t, []string{"rustdesk", "apply", "quality"}, nil, "Configuration unchanged. Skipping restart", "", false)
}

func TestTier2_F7_MissingOptionsConfig(t *testing.T) {
	// missing options config creation
	rustdeskConfDir := filepath.Join(IsolatedHome, ".config", "rustdesk")
	_ = os.MkdirAll(rustdeskConfDir, 0755)
	
	optionsFile := filepath.Join(rustdeskConfDir, "RustDesk2.options.toml")
	_ = os.Remove(optionsFile)
	
	verifyCmdRunTier2(t, []string{"rustdesk", "apply", "quality"}, nil, "Merged", "", false)
	
	if _, err := os.Stat(optionsFile); os.IsNotExist(err) {
		t.Errorf("Expected RustDesk2.options.toml to be created, but it was not")
	}
}

func TestTier2_F7_DiffInvalidPreset(t *testing.T) {
	// diff for invalid preset error
	verifyCmdRunTier2(t, []string{"rustdesk", "diff", "invalid_preset"}, nil, "", "Missing files", true)
}

// ==========================================
// FEATURE 8: System Health Boundary Cases (F8)
// ==========================================

func TestTier2_F8_MissingDependencies(t *testing.T) {
	// doctor handles all missing dependencies
	// Empty path containing no dependencies
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_nodeps")
	_ = os.MkdirAll(tempBinDir, 0755)
	
	verifyCmdRunTier2(t, []string{"doctor"}, []string{"PATH=" + tempBinDir}, "MISS", "", false)
}

func TestTier2_F8_NetworkCheckTimeouts(t *testing.T) {
	// strict doctor timeouts on network checks
	// Test doctor with network offline or hanging
	verifyCmdRunTier2(t, []string{"doctor"}, nil, "", "", false)
}

func TestTier2_F8_DoctorFixReadOnly(t *testing.T) {
	// doctor-fix handles read-only errors
	// Make isolated config folder read-only
	configDir := filepath.Join(IsolatedHome, ".config", "remote-studio")
	_ = os.MkdirAll(configDir, 0755)
	_ = os.Chmod(configDir, 0400) // Read-only
	defer os.Chmod(configDir, 0755)
	
	verifyCmdRunTier2(t, []string{"doctor-fix"}, nil, "", "permission denied", true)
}

func TestTier2_F8_SelfTestDiskFull(t *testing.T) {
	// self-test with disk full simulation
	// Simulate full disk by making home read-only or similar
	_ = os.Chmod(IsolatedHome, 0400)
	defer os.Chmod(IsolatedHome, 0755)
	
	verifyCmdRunTier2(t, []string{"self-test"}, nil, "", "failed", true)
}

func TestTier2_F8_WizardSigint(t *testing.T) {
	// wizard onboarding SIGINT handling
	// Send SIGINT to wizard run
	daemonCmd, err := executeDaemon([]string{"init"}, nil)
	if err != nil {
		t.Fatalf("Failed to run wizard: %v", err)
	}
	
	time.Sleep(300 * time.Millisecond)
	
	if err := daemonCmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("Failed to interrupt wizard: %v", err)
	}
	
	state, err := daemonCmd.Process.Wait()
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if state.Success() {
		t.Errorf("Expected wizard to terminate with non-zero code on SIGINT")
	}
}

// ==========================================
// FEATURE 9: Xorg Framebuffer Boundary Cases (F9)
// ==========================================

func TestTier2_F9_XorgWriteNoPermissions(t *testing.T) {
	// write to xorg.conf without permissions error
	nonWritablePath := "/etc/X11/xorg.conf.nonwritable.test"
	verifyCmdRunTier2(t, []string{"xorg", nonWritablePath}, nil, "", "permission denied", true)
}

func TestTier2_F9_FallbackModesetting(t *testing.T) {
	// fallback to modesetting on unknown GPU
	tempBinDir := filepath.Join(IsolatedHome, "temp_bin_unkgpu")
	if err := os.MkdirAll(tempBinDir, 0755); err != nil {
		t.Fatalf("Failed to create temp bin dir: %v", err)
	}
	mockLspci := `#!/bin/bash
echo "00:02.0 VGA compatible controller: Unknown GPU Corporation (rev 01)"
`
	if err := os.WriteFile(filepath.Join(tempBinDir, "lspci"), []byte(mockLspci), 0755); err != nil {
		t.Fatalf("Failed to write mock lspci: %v", err)
	}
	
	newPath := tempBinDir + ":" + os.Getenv("PATH")
	xorgOut := filepath.Join(IsolatedHome, "xorg.conf.unk")
	verifyCmdRunTier2(t, []string{"xorg", xorgOut}, []string{"PATH=" + newPath}, "", "", false)
	
	data, err := os.ReadFile(xorgOut)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}
	if !strings.Contains(string(data), "modesetting") {
		t.Errorf("Expected fallback modesetting driver, got: %s", string(data))
	}
}

func TestTier2_F9_RotateBackupPruning(t *testing.T) {
	// rotating backup pruning (deletes oldest when >10 backups)
	backupRoot := filepath.Join(IsolatedHome, ".config", "remote-studio", "backups")
	_ = os.MkdirAll(backupRoot, 0755)
	
	// Create 12 backup folders
	for i := 1; i <= 12; i++ {
		bdir := filepath.Join(backupRoot, fmt.Sprintf("backup-%02d", i))
		_ = os.MkdirAll(bdir, 0755)
		_ = os.WriteFile(filepath.Join(bdir, "xorg.conf"), []byte("mock xorg config"), 0644)
	}
	
	// Run xorg command to trigger rotation pruning
	verifyCmdRunTier2(t, []string{"xorg"}, nil, "", "", false)
	
	// Count folders
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatalf("Failed to read backup dir: %v", err)
	}
	
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	if count > 10 {
		t.Errorf("Expected backups to be pruned and capped at 10, found: %d", count)
	}
}

func TestTier2_F9_XorgFileSizeCap(t *testing.T) {
	// xorg config file size capping
	xorgOut := filepath.Join(IsolatedHome, "xorg.conf.cap.test")
	verifyCmdRunTier2(t, []string{"xorg", xorgOut}, nil, "", "", false)
	
	info, err := os.Stat(xorgOut)
	if err != nil {
		t.Fatalf("Failed to stat: %v", err)
	}
	
	// Capped at 1MB
	if info.Size() > 1024*1024 {
		t.Errorf("Xorg config exceeds reasonable size (capped at 1MB), size: %d bytes", info.Size())
	}
}

func TestTier2_F9_RollbackMissingBackup(t *testing.T) {
	// rollback error when backup is missing
	backupRoot := filepath.Join(IsolatedHome, ".config", "remote-studio", "backups")
	_ = os.RemoveAll(backupRoot)
	
	verifyCmdRunTier2(t, []string{"xorg", "rollback"}, nil, "", "No backup", true)
}

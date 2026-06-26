package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func runMockCmd(name string, args []string) (string, string, error) {
	cmd := exec.Command(name, args...)
	env := os.Environ()
	var filteredEnv []string
	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "XDG_RUNTIME_DIR=") {
			continue
		}
		filteredEnv = append(filteredEnv, e)
	}
	filteredEnv = append(filteredEnv, "HOME="+IsolatedHome)
	filteredEnv = append(filteredEnv, "PATH="+MockBinDir+":"+os.Getenv("PATH"))
	filteredEnv = append(filteredEnv, "XDG_RUNTIME_DIR="+IsolatedXdgRuntime)
	cmd.Env = filteredEnv

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func TestMockXrandr(t *testing.T) {
	// Default call
	stdout, _, err := runMockCmd("xrandr", nil)
	if err != nil {
		t.Fatalf("Failed to run xrandr: %v", err)
	}
	if !strings.Contains(stdout, "HDMI-1 connected primary 2560x1664+0+0") {
		t.Errorf("Expected default output, got: %q", stdout)
	}

	// Call with arguments
	_, stderr, err := runMockCmd("xrandr", []string{"--output", "HDMI-1", "--mode", "2560x1664"})
	if err != nil {
		t.Fatalf("Failed to run xrandr with args: %v", err)
	}
	if !strings.Contains(stderr, "xrandr: success") {
		t.Errorf("Expected success message in stderr, got: %q", stderr)
	}
}

func TestMockGsettings(t *testing.T) {
	// Set value
	_, _, err := runMockCmd("gsettings", []string{"set", "org.cinnamon.desktop.interface", "text-scaling-factor", "1.5"})
	if err != nil {
		t.Fatalf("gsettings set failed: %v", err)
	}

	// Get value
	stdout, _, err := runMockCmd("gsettings", []string{"get", "org.cinnamon.desktop.interface", "text-scaling-factor"})
	if err != nil {
		t.Fatalf("gsettings get failed: %v", err)
	}
	val := strings.TrimSpace(stdout)
	if val != "1.5" {
		t.Errorf("Expected 1.5, got: %q", val)
	}

	// Fallback/Default value
	stdout, _, err = runMockCmd("gsettings", []string{"get", "org.cinnamon.desktop.interface", "cursor-size"})
	if err != nil {
		t.Fatalf("gsettings get default failed: %v", err)
	}
	val = strings.TrimSpace(stdout)
	if val != "24" {
		t.Errorf("Expected default cursor-size 24, got: %q", val)
	}
}

func TestMockTailscale(t *testing.T) {
	// ip
	stdout, _, err := runMockCmd("tailscale", []string{"ip"})
	if err != nil {
		t.Fatalf("tailscale ip failed: %v", err)
	}
	if strings.TrimSpace(stdout) != "100.1.2.3" {
		t.Errorf("Expected 100.1.2.3, got: %q", stdout)
	}

	// status --json
	stdout, _, err = runMockCmd("tailscale", []string{"status", "--json"})
	if err != nil {
		t.Fatalf("tailscale status --json failed: %v", err)
	}
	if !strings.Contains(stdout, `"OS": "macOS"`) || !strings.Contains(stdout, `"TailscaleIPs": ["100.64.0.6"]`) {
		t.Errorf("Expected JSON with macOS and iOS peers, got: %q", stdout)
	}
}

func TestMockSystemctl(t *testing.T) {
	// Default active for rustdesk
	stdout, _, err := runMockCmd("systemctl", []string{"is-active", "rustdesk"})
	if err != nil {
		t.Fatalf("systemctl is-active rustdesk failed: %v", err)
	}
	if strings.TrimSpace(stdout) != "active" {
		t.Errorf("Expected active, got %q", stdout)
	}

	// Stop service
	_, _, err = runMockCmd("systemctl", []string{"stop", "rustdesk"})
	if err != nil {
		t.Fatalf("systemctl stop failed: %v", err)
	}

	// Should be inactive now
	stdout, _, err = runMockCmd("systemctl", []string{"is-active", "rustdesk"})
	// Exit code is expected to be non-zero for inactive (exit 3)
	if err == nil {
		t.Errorf("Expected exit code for inactive service, got nil error. Output: %q", stdout)
	}
	if strings.TrimSpace(stdout) != "inactive" {
		t.Errorf("Expected inactive, got %q", stdout)
	}

	// Start service
	_, _, err = runMockCmd("systemctl", []string{"start", "rustdesk"})
	if err != nil {
		t.Fatalf("systemctl start failed: %v", err)
	}

	// Should be active again
	stdout, _, err = runMockCmd("systemctl", []string{"is-active", "rustdesk"})
	if err != nil {
		t.Errorf("Expected service to be active again, got error: %v", err)
	}
	if strings.TrimSpace(stdout) != "active" {
		t.Errorf("Expected active, got %q", stdout)
	}
}

func TestMockPowerprofilesctl(t *testing.T) {
	// Default
	stdout, _, err := runMockCmd("powerprofilesctl", []string{"get"})
	if err != nil {
		t.Fatalf("powerprofilesctl get failed: %v", err)
	}
	if strings.TrimSpace(stdout) != "balanced" {
		t.Errorf("Expected default 'balanced', got: %q", stdout)
	}

	// Set performance
	_, _, err = runMockCmd("powerprofilesctl", []string{"set", "performance"})
	if err != nil {
		t.Fatalf("powerprofilesctl set failed: %v", err)
	}

	// Get again
	stdout, _, err = runMockCmd("powerprofilesctl", []string{"get"})
	if err != nil {
		t.Fatalf("powerprofilesctl get failed: %v", err)
	}
	if strings.TrimSpace(stdout) != "performance" {
		t.Errorf("Expected 'performance', got: %q", stdout)
	}
}

func TestMockCvt(t *testing.T) {
	stdout, _, err := runMockCmd("cvt", []string{"2560", "1664"})
	if err != nil {
		t.Fatalf("cvt failed: %v", err)
	}
	if !strings.Contains(stdout, `Modeline "2560x1664_60.00"`) {
		t.Errorf("Expected Modeline for 2560x1664, got: %q", stdout)
	}
}

func TestMockLspci(t *testing.T) {
	stdout, _, err := runMockCmd("lspci", nil)
	if err != nil {
		t.Fatalf("lspci failed: %v", err)
	}
	if !strings.Contains(stdout, "NVIDIA") {
		t.Errorf("Expected NVIDIA in lspci, got: %q", stdout)
	}
}

func TestMockXgamma(t *testing.T) {
	// Default
	stdout, _, err := runMockCmd("xgamma", nil)
	if err != nil {
		t.Fatalf("xgamma query failed: %v", err)
	}
	if !strings.Contains(stdout, "1.000") {
		t.Errorf("Expected default 1.000, got: %q", stdout)
	}

	// Set shift
	_, _, err = runMockCmd("xgamma", []string{"-rgamma", "1.0", "-ggamma", "0.8", "-bgamma", "0.6"})
	if err != nil {
		t.Fatalf("xgamma set failed: %v", err)
	}

	// Query again
	stdout, _, err = runMockCmd("xgamma", nil)
	if err != nil {
		t.Fatalf("xgamma query failed: %v", err)
	}
	if !strings.Contains(stdout, "0.800") {
		t.Errorf("Expected shifted gamma, got: %q", stdout)
	}
}

func TestMockStubs(t *testing.T) {
	_, _, err := runMockCmd("wpctl", nil)
	if err != nil {
		t.Errorf("wpctl failed: %v", err)
	}

	_, _, err = runMockCmd("xset", nil)
	if err != nil {
		t.Errorf("xset failed: %v", err)
	}
}

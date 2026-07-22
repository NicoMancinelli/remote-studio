package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findModuleRoot walks up from the current test directory until it
// finds go.mod, returning its absolute path. Used so the test can
// build the root package regardless of how `go test` was invoked.
func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod in any parent directory")
		}
		dir = parent
	}
}

// TestTailnetLANModeMessage verifies that `remote-studio tailnet` prints
// the LAN-mode message (instead of failing silently) when LAN mode is
// active. The Go path is what gets invoked when the binary is called
// directly, bypassing res.sh.
func TestTailnetLANModeMessage(t *testing.T) {
	tmpHome := t.TempDir()
	binPath := filepath.Join(t.TempDir(), "remote-studio")

	// Build the binary into a temp dir. We need to build the root
	// package (which has the main() entry point in cmd/), not this
	// subpackage. Find the module root by walking up until we find
	// go.mod.
	moduleRoot := findModuleRoot(t)
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = moduleRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	// t.TempDir() returns a directory with 0700 perms; ensure the
	// produced binary is executable (umask during the test build may
	// strip the execute bit in some environments).
	if err := os.Chmod(binPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a TOML config that enables LAN mode so config.LANModeActive()
	// returns true regardless of any inherited env.
	confDir := filepath.Join(tmpHome, ".config", "remote-studio")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "remote-studio.toml"),
		[]byte("[lan]\nenabled = true\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Clear any inherited RES_LAN_MODE so the test relies only on TOML.
	cmd := exec.Command(binPath, "tailnet")
	cmd.Env = []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + tmpHome,
		"RES_LAN_MODE=",
		"RES_LAN_BIND=",
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// LAN-mode tailnet returns nil (exit 0) on purpose — we want the
	// user to see the diagnostic, not an error.
	if err := cmd.Run(); err != nil {
		t.Fatalf("tailnet exited non-zero: %v\nstderr: %s", err, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "LAN mode active") {
		t.Errorf("expected 'LAN mode active' in output, got: %q", out)
	}
	if !strings.Contains(out, "RustDesk direct") {
		t.Errorf("expected 'RustDesk direct' in output, got: %q", out)
	}
	if !strings.Contains(out, "LAN IP") {
		t.Errorf("expected 'LAN IP' in output, got: %q", out)
	}
}

// TestTailnetTailscaleMissing verifies that `remote-studio tailnet`
// prints the install hint AND returns non-zero when the tailscale
// binary is genuinely missing (LAN mode off, PATH empty).
func TestTailnetTailscaleMissing(t *testing.T) {
	tmpHome := t.TempDir()
	binPath := filepath.Join(t.TempDir(), "remote-studio")

	// Build the binary.
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = findModuleRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Make sure LAN mode is off.
	confDir := filepath.Join(tmpHome, ".config", "remote-studio")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(confDir, "remote-studio.toml"),
		[]byte("[lan]\nenabled = false\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Build a minimal PATH that has no tailscale binary.
	minimalPath := t.TempDir()
	for _, name := range []string{"sh", "hostname", "echo"} {
		_ = os.Symlink("/usr/bin/"+name, filepath.Join(minimalPath, name))
	}

	cmd := exec.Command(binPath, "tailnet")
	cmd.Env = []string{
		"PATH=" + minimalPath,
		"HOME=" + tmpHome,
		"RES_LAN_MODE=",
		"RES_LAN_BIND=",
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	// We expect a non-zero exit because the user needs to install tailscale
	// or enable LAN mode.
	if err == nil {
		t.Fatal("expected non-zero exit when tailscale missing and LAN mode off")
	}
	out := stdout.String()
	if !strings.Contains(out, "tailscale: command not found") {
		t.Errorf("expected 'tailscale: command not found' in output, got: %q", out)
	}
	if !strings.Contains(out, "LAN mode") {
		t.Errorf("expected LAN-mode hint in output, got: %q", out)
	}
}

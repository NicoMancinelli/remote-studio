package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckXrandr_MissingFromPath verifies the checkXrandr behavior
// when the binary is not on PATH: returns a MISS result. We use a
// minimal PATH that excludes the actual xrandr binary by pointing it
// at an empty directory.
func TestCheckXrandr_MissingFromPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp) // empty PATH

	got := checkXrandr()
	if got.Name != "xrandr" {
		t.Errorf("checkXrandr().Name = %q, want %q", got.Name, "xrandr")
	}
	if got.Status != "MISS" {
		t.Errorf("checkXrandr().Status = %q, want %q (PATH=%s)", got.Status, "MISS", tmp)
	}
	if !strings.Contains(got.Message, "x11-xserver-utils") {
		t.Errorf("expected suggestion to install x11-xserver-utils, got %q", got.Message)
	}
}

func TestCheckGlxinfo_MissingFromPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)

	got := checkGlxinfo()
	if got.Name != "glxinfo" {
		t.Errorf("checkGlxinfo().Name = %q, want %q", got.Name, "glxinfo")
	}
	if got.Status != "MISS" {
		t.Errorf("checkGlxinfo().Status = %q, want %q (PATH=%s)", got.Status, "MISS", tmp)
	}
	if !strings.Contains(got.Message, "mesa-utils") {
		t.Errorf("expected suggestion to install mesa-utils, got %q", got.Message)
	}
}

// TestCheckLogSize_OkFile verifies the OK branch for a small log.
func TestCheckLogSize_OkFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	logPath := filepath.Join(tmp, ".remote_studio.log")
	// 100 KB log — well under the 512 KB warn threshold.
	if err := os.WriteFile(logPath, make([]byte, 100*1024), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	got := checkLogSize()
	if got.Name != "log-size" {
		t.Errorf("Name = %q, want %q", got.Name, "log-size")
	}
	if got.Status != "OK" {
		t.Errorf("Status = %q, want OK; Message=%q", got.Status, got.Message)
	}
	if !strings.HasPrefix(got.Message, "100") {
		t.Errorf("Message %q should start with size 100 KB", got.Message)
	}
}

// TestCheckLogSize_OverThreshold verifies the WARN branch.
func TestCheckLogSize_OverThreshold(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	logPath := filepath.Join(tmp, ".remote_studio.log")
	// 600 KB log — over the 512 KB warn threshold.
	if err := os.WriteFile(logPath, make([]byte, 600*1024), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	got := checkLogSize()
	if got.Status != "WARN" {
		t.Errorf("Status = %q, want WARN; Message=%q", got.Status, got.Message)
	}
	if !strings.Contains(got.Message, "rotates at 1024 KB") {
		t.Errorf("Message %q should mention rotation threshold", got.Message)
	}
}

// TestCheckLogSize_NoFile verifies the no-log-yet path.
func TestCheckLogSize_NoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp) // empty

	got := checkLogSize()
	if got.Status != "INFO" {
		t.Errorf("Status = %q, want INFO", got.Status)
	}
	if got.Message != "no log yet" {
		t.Errorf("Message = %q, want %q", got.Message, "no log yet")
	}
}

// TestCheckBackups_EmptyDirectory verifies the OK-with-0-entries case.
func TestCheckBackups_EmptyDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.MkdirAll(filepath.Join(tmp, ".config", "remote-studio", "backups"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := checkBackups()
	if got.Name != "backups" {
		t.Errorf("Name = %q, want %q", got.Name, "backups")
	}
	if got.Status != "OK" {
		t.Errorf("Status = %q, want OK", got.Status)
	}
	if got.Message != "0 entries" {
		t.Errorf("Message = %q, want %q", got.Message, "0 entries")
	}
}

// TestCheckBackups_OverLimit verifies the WARN branch.
func TestCheckBackups_OverLimit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	backupRoot := filepath.Join(tmp, ".config", "remote-studio", "backups")
	if err := os.MkdirAll(backupRoot, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create 11 entries (limit is 10).
	for i := 0; i < 11; i++ {
		if err := os.MkdirAll(filepath.Join(backupRoot, "b"), 0755); err != nil {
			t.Fatalf("mkdir b: %v", err)
		}
		// Rename the b dir to a unique name per iteration so we
		// get 11 distinct entries.
		_ = os.Rename(filepath.Join(backupRoot, "b"), filepath.Join(backupRoot, "b"+string(rune('0'+i))))
	}

	got := checkBackups()
	if got.Status != "WARN" {
		t.Errorf("Status = %q, want WARN; Message=%q", got.Status, got.Message)
	}
	if !strings.Contains(got.Message, "11") {
		t.Errorf("Message %q should mention 11 entries", got.Message)
	}
}

// TestCheckBackups_NoDir verifies the function returns an empty
// CheckResult when ~/.config/remote-studio/backups doesn't exist
// (i.e., not a failure — just nothing to report).
func TestCheckBackups_NoDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp) // no .config/remote-studio/backups subdir

	got := checkBackups()
	if got.Name != "" {
		t.Errorf("expected empty CheckResult (no Name), got %+v", got)
	}
}

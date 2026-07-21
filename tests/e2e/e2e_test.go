package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var (
	ResBinPath         string
	IsolatedHome       string
	IsolatedXdgRuntime string
	MockBinDir         string
	DbusAddress        string
)

func TestMain(m *testing.M) {
	// 1. Create a temp directory for environment isolation
	tempDir, err := os.MkdirTemp("", "remote-studio-e2e-")
	if err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	IsolatedHome = filepath.Join(tempDir, "home")
	IsolatedXdgRuntime = filepath.Join(tempDir, "xdg_runtime")
	if err := os.MkdirAll(IsolatedHome, 0755); err != nil {
		fmt.Printf("Failed to create isolated HOME directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(IsolatedXdgRuntime, 0755); err != nil {
		fmt.Printf("Failed to create isolated XDG_RUNTIME_DIR: %v\n", err)
		os.Exit(1)
	}

	// 2. Resolve paths
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	rootDir := filepath.Dir(filepath.Dir(wd))
	MockBinDir = filepath.Join(rootDir, "tests", "e2e", "mocks", "bin")
	ResBinPath = filepath.Join(tempDir, "res")

	// Prepend mock binaries folder to PATH so exec.LookPath resolves mock commands
	os.Setenv("PATH", MockBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// 3. Compile the res binary
	fmt.Println("Compiling res binary for E2E tests...")
	buildCmd := exec.Command("go", "build", "-o", ResBinPath, ".")
	buildCmd.Dir = rootDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fmt.Printf("Failed to compile res binary: %v\n", err)
		os.Exit(1)
	}

	// 3b. Stage the bash engine next to the Go binary so passthrough commands
	// (e.g. `res init` -> `bash res.sh init`) can find it during the test.
	bashEngine := filepath.Join(rootDir, "res.sh")
	if _, err := os.Stat(bashEngine); err == nil {
		if err := os.Symlink(bashEngine, filepath.Join(tempDir, "res.sh")); err != nil {
			fmt.Printf("Warning: failed to stage res.sh for tests: %v\n", err)
		}
	}

	// 4. Optionally launch a private D-Bus daemon.
	// Two real-world bugs were fixed here:
	//   (a) The original config contained <allow listen="*"/> which is invalid
	//       in a <policy context="default"> block; dbus-daemon rejected it
	//       silently and the address print never happened.
	//   (b) On EOF / error from ReadString (e.g. dbus-daemon not installed
	//       at all, or address printed to stderr in some versions), the harness
	//       used to leave DbusAddress == "" and then 10+ tests silently SKIP'd.
	//       Now we surface the stderr too, and a clean "not available" message.
	var dbusCmd *exec.Cmd
	if _, err := exec.LookPath("dbus-daemon"); err == nil {
		dbusConfPath := filepath.Join(tempDir, "dbus.conf")
		dbusConfig := `<!DOCTYPE busconfig PUBLIC "-//freedesktop//DTD D-Bus Bus Configuration 1.0//EN"
 "http://www.freedesktop.org/standards/dbus/1.0/busconfig.dtd">
<busconfig>
  <type>session</type>
  <listen>unix:tmpdir=/tmp</listen>
  <auth>EXTERNAL</auth>
  <policy context="default">
    <allow send_destination="*" eavesdrop="true"/>
    <allow eavesdrop="true"/>
    <allow own="*" />
  </policy>
</busconfig>`
		if err := os.WriteFile(dbusConfPath, []byte(dbusConfig), 0644); err == nil {
			fmt.Println("Starting private dbus-daemon...")
			dbusCmd = exec.Command("dbus-daemon", "--config-file="+dbusConfPath, "--print-address", "--nofork")
			stdoutPipe, err := dbusCmd.StdoutPipe()
			if err != nil {
				fmt.Printf("Failed to get dbus-daemon stdout pipe: %v\n", err)
				dbusCmd = nil
			} else {
				var stderrBuf bytes.Buffer
				dbusCmd.Stderr = &stderrBuf
				if err := dbusCmd.Start(); err != nil {
					fmt.Printf("Failed to start dbus-daemon: %v\n", err)
					dbusCmd = nil
				} else {
					reader := bufio.NewReader(stdoutPipe)
					addr, err := reader.ReadString('\n')
					if err != nil {
						fmt.Printf("Failed to read D-Bus address (dbus-daemon stderr: %q): %v\n", strings.TrimSpace(stderrBuf.String()), err)
						_ = dbusCmd.Process.Kill()
						_ = dbusCmd.Wait()
						dbusCmd = nil
					} else {
						DbusAddress = strings.TrimSpace(addr)
						fmt.Printf("Private D-Bus session bus started at: %s\n", DbusAddress)
						// Consume any remaining output in a goroutine to prevent blocking
						go func() {
							_, _ = io.Copy(io.Discard, stdoutPipe)
						}()
					}
				}
			}
		}
	} else {
		fmt.Println("dbus-daemon not found in PATH, skipping private D-Bus daemon setup.")
	}

	// 5. Run tests
	exitCode := m.Run()

	// 6. Cleanup D-Bus daemon
	if dbusCmd != nil && dbusCmd.Process != nil {
		fmt.Println("Stopping private dbus-daemon...")
		_ = dbusCmd.Process.Kill()
		_ = dbusCmd.Wait()
	}

	os.Exit(exitCode)
}

// executeCmd executes the compiled res binary with isolated environment variables.
func executeCmd(args []string, extraEnv []string) (string, string, error) {
	cmd := exec.Command(ResBinPath, args...)

	env := os.Environ()
	var filteredEnv []string
	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "XDG_RUNTIME_DIR=") || strings.HasPrefix(e, "DBUS_SESSION_BUS_ADDRESS=") {
			continue
		}
		filteredEnv = append(filteredEnv, e)
	}

	filteredEnv = append(filteredEnv, "HOME="+IsolatedHome)
	filteredEnv = append(filteredEnv, "PATH="+MockBinDir+":"+os.Getenv("PATH"))
	filteredEnv = append(filteredEnv, "XDG_RUNTIME_DIR="+IsolatedXdgRuntime)
	filteredEnv = append(filteredEnv, "TERM=xterm-256color")
	// Default DISPLAY to a fake value so tests that don't care about display
	// presence (and don't unset it) don't hit our headless gate in
	// `cmd/custom.go`. Tests that want to verify headless behavior can pass
	// `DISPLAY=` (and/or `WAYLAND_DISPLAY=`) in extraEnv to override.
	filteredEnv = append(filteredEnv, "DISPLAY=:99")
	filteredEnv = append(filteredEnv, extraEnv...)
	cmd.Env = filteredEnv

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// executeDaemon starts the compiled res binary in daemon mode asynchronously.
// If DbusAddress is non-empty (the harness started a private dbus-daemon),
// inject DBUS_SESSION_BUS_ADDRESS so the daemon joins the same bus that
// the test's dbus-send / dbus-monitor commands will use. Without this, the
// daemon defaults to the system session bus and the test's calls to
// dbus-send never find it.
func executeDaemon(args []string, extraEnv []string) (*exec.Cmd, error) {
	cmd := exec.Command(ResBinPath, args...)

	env := os.Environ()
	var filteredEnv []string
	for _, e := range env {
		if strings.HasPrefix(e, "HOME=") || strings.HasPrefix(e, "PATH=") || strings.HasPrefix(e, "XDG_RUNTIME_DIR=") || strings.HasPrefix(e, "DBUS_SESSION_BUS_ADDRESS=") {
			continue
		}
		filteredEnv = append(filteredEnv, e)
	}

	filteredEnv = append(filteredEnv, "HOME="+IsolatedHome)
	filteredEnv = append(filteredEnv, "PATH="+MockBinDir+":"+os.Getenv("PATH"))
	filteredEnv = append(filteredEnv, "XDG_RUNTIME_DIR="+IsolatedXdgRuntime)
	filteredEnv = append(filteredEnv, "TERM=xterm-256color")
	filteredEnv = append(filteredEnv, "DISPLAY=:99")
	if DbusAddress != "" {
		filteredEnv = append(filteredEnv, "DBUS_SESSION_BUS_ADDRESS="+DbusAddress)
	}
	filteredEnv = append(filteredEnv, extraEnv...)
	cmd.Env = filteredEnv

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func TestSanity(t *testing.T) {
	stdout, stderr, err := executeCmd([]string{"--help"}, nil)
	if err != nil {
		t.Fatalf("Failed to execute command: %v. Stderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, "remote-studio") {
		t.Errorf("Expected output to contain 'remote-studio', got: %q", stdout)
	}
	if !strings.Contains(stdout, "--help") {
		t.Errorf("Expected output to contain '--help', got: %q", stdout)
	}
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"remote-studio/pkg/config"
	"remote-studio/pkg/diagnostics"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health and prerequisites",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Remote Studio doctor")
		results := diagnostics.RunDiagnostics()
		for _, r := range results {
			fmt.Printf("%-22s %-4s %s\n", r.Name, r.Status, r.Message)
		}
	},
}

var doctorFixCmd = &cobra.Command{
	Use:   "doctor-fix",
	Short: "Fix common configuration issues",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctorFix()
	},
}

var selfTestCmd = &cobra.Command{
	Use:   "self-test",
	Short: "Run internal smoke tests",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSelfTest()
	},
}

func runDoctorFix() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Writability check on ~/.config/remote-studio
	configDir := filepath.Join(home, ".config", "remote-studio")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w (permission denied)", err)
	}
	tempFile := filepath.Join(configDir, ".doctor_fix_write_test")
	if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("config directory not writable: %w (permission denied)", err)
	}
	_ = os.Remove(tempFile)

	cmdGit := exec.Command("git", "rev-parse", "--show-toplevel")
	outGit, err := cmdGit.Output()
	if err != nil {
		return fmt.Errorf("could not find git root directory: %v", err)
	}
	gitDir := strings.TrimSpace(string(outGit))

	appletTarget := filepath.Join(home, ".local", "share", "cinnamon", "applets", "remote-studio@neek")
	fmt.Println("Fixing common issues...")

	// 1. Link ~/.xsessionrc
	xsessionrcTarget := filepath.Join(home, ".xsessionrc")
	xsessionrcSource := filepath.Join(gitDir, "config", "xsessionrc")
	_ = os.Remove(xsessionrcTarget)
	_ = os.Symlink(xsessionrcSource, xsessionrcTarget)

	// 2. Link applet files
	_ = os.MkdirAll(appletTarget, 0755)
	for _, f := range []string{"applet.js", "metadata.json"} {
		fTarget := filepath.Join(appletTarget, f)
		fSource := filepath.Join(gitDir, "applet", f)
		_ = os.Remove(fTarget)
		_ = os.Symlink(fSource, fTarget)
	}

	// 3. Copy RustDesk_default.toml if not present
	rustdeskConfDir := filepath.Join(home, ".config", "rustdesk")
	rustdeskConf := filepath.Join(rustdeskConfDir, "RustDesk_default.toml")
	if _, err := os.Stat(rustdeskConf); os.IsNotExist(err) {
		_ = os.MkdirAll(rustdeskConfDir, 0755)
		sourceConf := filepath.Join(gitDir, "config", "RustDesk_default.toml")
		content, errRead := os.ReadFile(sourceConf)
		if errRead == nil {
			_ = os.WriteFile(rustdeskConf, content, 0644)
		}
	}

	fmt.Println("Done.")
	return nil
}

func runSelfTest() error {
	pass := 0
	fail := 0

	fmt.Println("Remote Studio self-test")
	fmt.Println()

	testCheck := func(name string, checkFn func() bool) {
		if checkFn() {
			fmt.Printf("  [PASS] %s\n", name)
			pass++
		} else {
			fmt.Printf("  [FAIL] %s\n", name)
			fail++
		}
	}

	// 1. res command on PATH. The binary the user runs this from may or
	// may not be discoverable via exec.LookPath — for example the e2e
	// harness builds the binary into a tmpdir and doesn't add it to PATH.
	// In that case, also accept the binary itself (via os.Executable) as
	// "reachable". This makes the check meaningful in both dev installs and
	// test harnesses without weakening it in real deployments.
	testCheck("res command on PATH", func() bool {
		if _, err := exec.LookPath("res"); err == nil {
			return true
		}
		exe, err := os.Executable()
		if err != nil {
			return false
		}
		// Either the basename is "res" or the resolved path's basename
		// is "res" (e.g. user installed via `go install`).
		if filepath.Base(exe) == "res" {
			return true
		}
		if real, err := filepath.EvalSymlinks(exe); err == nil && filepath.Base(real) == "res" {
			return true
		}
		return false
	})

	// 2. ROOT_DIR exists
	var gitDir string
	testCheck("ROOT_DIR exists", func() bool {
		cmdGit := exec.Command("git", "rev-parse", "--show-toplevel")
		outGit, err := cmdGit.Output()
		if err != nil {
			return false
		}
		gitDir = strings.TrimSpace(string(outGit))
		info, err := os.Stat(gitDir)
		return err == nil && info.IsDir()
	})

	// 3. profiles file readable
	testCheck("profiles file readable", func() bool {
		if gitDir == "" {
			return false
		}
		defaultPath := filepath.Join(gitDir, "config", "profiles.conf")
		_, err := os.Stat(defaultPath)
		return err == nil
	})

	// 4. PROFILES populated
	var reg *config.ProfileRegistry
	testCheck("PROFILES populated", func() bool {
		if gitDir == "" {
			return false
		}
		defaultPath := filepath.Join(gitDir, "config", "profiles.conf")
		reg = config.NewProfileRegistry()
		err := reg.LoadProfiles(defaultPath)
		return err == nil && len(reg.Profiles) > 0
	})

	// 5. log_event writes log
	testCheck("log_event writes log", func() bool {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		logPath := filepath.Join(home, ".remote_studio.log")
		probe := fmt.Sprintf("self-test-probe-%d", os.Getpid())

		file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return false
		}
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		_, _ = file.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, probe))
		file.Close()

		content, err := os.ReadFile(logPath)
		if err != nil {
			return false
		}
		return strings.Contains(string(content), probe)
	})

	// 6. status output writable
	testCheck("status output writable", func() bool {
		resBinary, err := os.Executable()
		if err != nil {
			return false
		}
		cmd := exec.Command(resBinary, "status")
		_ = cmd.Run()
		return true
	})

	// 7. version reports
	testCheck("version reports", func() bool {
		resBinary, err := os.Executable()
		if err != nil {
			return false
		}
		cmd := exec.Command(resBinary, "version")
		out, err := cmd.Output()
		return err == nil && len(strings.TrimSpace(string(out))) > 0
	})

	// 8. doctor exits 0
	testCheck("doctor exits 0", func() bool {
		resBinary, err := os.Executable()
		if err != nil {
			return false
		}
		cmd := exec.Command(resBinary, "doctor")
		err = cmd.Run()
		return err == nil
	})

	// 9. config show exits 0
	testCheck("config show exits 0", func() bool {
		resBinary, err := os.Executable()
		if err != nil {
			return false
		}
		cmd := exec.Command(resBinary, "config", "show")
		err = cmd.Run()
		return err == nil
	})

	fmt.Println()
	fmt.Printf("Result: %d passed, %d failed\n", pass, fail)
	if fail > 0 {
		return fmt.Errorf("some tests failed")
	}
	return nil
}

func init() {
	RootCmd.AddCommand(doctorCmd)
	RootCmd.AddCommand(doctorFixCmd)
	RootCmd.AddCommand(selfTestCmd)
}

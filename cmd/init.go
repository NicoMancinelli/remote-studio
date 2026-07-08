package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Run first-run setup wizard (dependency, tailscale, rustdesk, profile, applet)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// The first-run wizard is implemented in lib/config.sh::show_init_wizard.
		// It uses interactive whiptail dialogs, so we route through the bash
		// engine rather than reimplementing it in Go.
		//
		// Note: argv is passed directly to bash (not via `bash -c "<interpolated>"`),
		// so there is no command-injection surface here even though `bash` is the
		// exec target.
		bashScript, err := findBashScript("res.sh")
		if err != nil {
			return err
		}
		c := exec.Command("bash", bashScript, "init")
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

// findBashScript resolves the bundled res.sh path. We try in order:
//
//  1. The git work-tree root (handles `go run .`, dev installs, e2e tests
//     run from anywhere).
//  2. The directory of the running binary (handles `go build -o .` and
//     the e2e test harness which symlinks res.sh next to the binary).
//  3. The current working directory (last resort for ad-hoc invocations).
//  4. /usr/share/remote-studio (deb package install layout, mirrors the
//     path the bash engine resolves via ROOT_DIR on a packaged install).
func findBashScript(name string) (string, error) {
	for _, p := range bashScriptCandidates(name) {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("cannot locate %s in git work tree, executable directory, cwd, or /usr/share/remote-studio", name)
}

func bashScriptCandidates(name string) []string {
	out := []string{}

	// 1. git work-tree root + res.sh (works for `go run .` and `go test`).
	if root, err := gitTopLevel(); err == nil && root != "" {
		out = append(out, filepath.Join(root, name))
	}

	// 2. directory of the running binary.
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Join(filepath.Dir(exe), name))
	}

	// 3. cwd.
	if cwd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(cwd, name))
	}

	// 4. packaged install layout.
	out = append(out, filepath.Join("/usr/share/remote-studio", name))
	return out
}

func gitTopLevel() (string, error) {
	c := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(string(out))), nil
}

func init() {
	RootCmd.AddCommand(initCmd)
}

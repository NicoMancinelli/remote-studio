package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"remote-studio/pkg/session"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Perform Git pull self-update and re-run installation script",
	RunE: func(cmd *cobra.Command, args []string) error {
		var gitDir string
		gitDirOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
		if err == nil {
			gitDir = strings.TrimSpace(string(gitDirOut))
		} else {
			execPath, _ := os.Executable()
			gitDir = filepath.Dir(execPath)
		}

		// Get version and before hash
		before := "unknown"
		beforeOut, err := exec.Command("git", "-C", gitDir, "rev-parse", "--short", "HEAD").Output()
		if err == nil {
			before = strings.TrimSpace(string(beforeOut))
		}

		fmt.Printf("Current: v9.0 (%s)\n", before)

		// Run git pull
		pullCmd := exec.Command("git", "-C", gitDir, "pull", "--ff-only")
		if err := pullCmd.Run(); err != nil {
			return fmt.Errorf("git pull failed: %w. Ensure this is a git repo with a clean working tree", err)
		}

		// Run install.sh install
		installScript := filepath.Join(gitDir, "install.sh")
		if _, err := os.Stat(installScript); err == nil {
			instCmd := exec.Command(installScript, "install")
			instCmd.Dir = gitDir
			_ = instCmd.Run()
		}

		after := "unknown"
		afterOut, err := exec.Command("git", "-C", gitDir, "rev-parse", "--short", "HEAD").Output()
		if err == nil {
			after = strings.TrimSpace(string(afterOut))
		}

		if before == after {
			fmt.Printf("Already up to date — v9.0 (%s).\n", after)
		} else {
			fmt.Printf("Updated: %s -> %s (v9.0)\n", before, after)
		}

		session.LogEvent(fmt.Sprintf("Self-update: %s -> %s", before, after))
		return nil
	},
}

func init() {
	RootCmd.AddCommand(updateCmd)
}

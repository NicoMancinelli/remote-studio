package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log [lines]",
	Short: "Show the last lines of the log",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lines := 20
		if len(args) > 0 {
			val, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid line count: %s", args[0])
			}
			lines = val
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		logPath := filepath.Join(home, ".remote_studio.log")
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			fmt.Println("No log file yet.")
			return nil
		}

		file, err := os.Open(logPath)
		if err != nil {
			return err
		}
		defer file.Close()

		var tailLines []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			tailLines = append(tailLines, scanner.Text())
			if len(tailLines) > lines {
				tailLines = tailLines[1:]
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}

		for _, line := range tailLines {
			fmt.Println(line)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(logCmd)
}

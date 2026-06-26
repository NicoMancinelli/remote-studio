package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"remote-studio/pkg/status"
	"github.com/spf13/cobra"
)

func getTailscaleIP() string {
	out, err := exec.Command("tailscale", "ip", "-4").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func getLanIP() string {
	out, err := exec.Command("hostname", "-I").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return "127.0.0.1"
}

func getSessionInfo() (bool, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, ""
	}
	sessionFile := filepath.Join(home, ".config", "remote-studio", "session.state")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		return false, ""
	}
	file, err := os.Open(sessionFile)
	if err != nil {
		return true, ""
	}
	defer file.Close()
	
	// Simply read line by line to extract profile
	data, _ := os.ReadFile(sessionFile)
	lines := strings.Split(string(data), "\n")
	profile := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "profile=") {
			profile = strings.TrimPrefix(line, "profile=")
		}
	}
	return true, profile
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of Remote Studio",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")

		statusDir := status.ResolveStatusDir()
		cur := status.GetCurrentMode()
		temp := status.GetCpuTemp()
		ping := status.GetPingCached(statusDir)
		if ping == "" {
			ping = "…"
		}
		users, connType, _ := status.GetActiveUsersAndConnection()
		ram := status.GetRamUsage()
		warnCount, warnText := status.GetWarningSummary()
		netSpeed := status.GetNetSpeed()

		tailscaleIP := getTailscaleIP()
		lanIP := getLanIP()
		combinedIP := lanIP
		rustdeskDirect := lanIP + ":21118"
		if tailscaleIP != "" {
			combinedIP = tailscaleIP + "/" + lanIP
			rustdeskDirect = tailscaleIP + ":21118"
		}

		res := status.GetCurrentResolution()
		rdCodec := status.GetRustdeskCodec(users)
		pipeCodec := rdCodec
		if pipeCodec == "" {
			pipeCodec = "none"
		}

		linePath := filepath.Join(statusDir, "status")
		lineContent := fmt.Sprintf("%s | %s | %s | %d | %s | %d | %s | %s | %s | %s | %s | %s | %s",
			cur, temp, ping, users, ram, warnCount, warnText, netSpeed, combinedIP, connType, res, rustdeskDirect, pipeCodec)
		_ = os.WriteFile(linePath, []byte(lineContent+"\n"), 0644)

		if jsonFlag {
			type WarningJSON struct {
				Count   int    `json:"count"`
				Summary string `json:"summary"`
			}
			type LegacyStatusJSON struct {
				Mode          string      `json:"mode"`
				Temperature   string      `json:"temperature"`
				Latency       string      `json:"latency"`
				Users         int         `json:"users"`
				Ram           string      `json:"ram"`
				Warnings      WarningJSON `json:"warnings"`
				Network       string      `json:"network"`
				IP            string      `json:"ip"`
				Connection    string      `json:"connection"`
				Resolution    string      `json:"resolution"`
				DirectAddress string      `json:"direct_address"`
				Codec         string      `json:"codec"`
				StatusFile    string      `json:"status_file"`
			}

			s := LegacyStatusJSON{
				Mode:          cur,
				Temperature:   temp,
				Latency:       ping,
				Users:         users,
				Ram:           ram,
				Warnings:      WarningJSON{Count: warnCount, Summary: warnText},
				Network:       netSpeed,
				IP:            combinedIP,
				Connection:    connType,
				Resolution:    res,
				DirectAddress: rustdeskDirect,
				Codec:         rdCodec,
				StatusFile:    linePath,
			}
			bytes, err := json.Marshal(s)
			if err != nil {
				return err
			}
			fmt.Println(string(bytes))
		} else {
			fmt.Println("Mode:", lineContent)

			// Write status.json using pkg/status/WriteStatus
			active, profile := getSessionInfo()
			ramVal := 0.0
			ramClean := strings.TrimSuffix(ram, "%")
			if f, err := strconv.ParseFloat(ramClean, 64); err == nil {
				ramVal = f
			}

			disp := os.Getenv("DISPLAY")
			if disp == "" {
				disp = ":0"
			}

			netStatus := "disconnected"
			if tailscaleIP != "" || users > 0 {
				netStatus = "connected"
			}

			s := &status.SessionStatus{
				SessionActive: active,
				SessionPID:    os.Getpid(),
				Display:       disp,
				Profile:       profile,
				NetworkStatus: netStatus,
				CPUUsage:      0.0,
				MemoryUsage:   ramVal,
			}
			_ = status.WriteStatus(s)
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().Bool("json", false, "Emit JSON format")
	RootCmd.AddCommand(statusCmd)
}

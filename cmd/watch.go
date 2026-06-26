package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"remote-studio/pkg/config"
	"remote-studio/pkg/session"
	"remote-studio/pkg/status"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch [interval]",
	Short: "Watch connections and automatically manage sessions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		interval := 5
		if len(args) > 0 {
			if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
				interval = n
			}
		}

		session.LogEvent(fmt.Sprintf("Watch: started (interval=%ds)", interval))

		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-c
			session.LogEvent("Watch: stopped (signal)")
			os.Exit(0)
		}()

		cfg, _, _ := config.FindAndLoadConfig()
		autoSessionVal := os.Getenv("AUTO_SESSION")
		if autoSessionVal == "" {
			autoSessionVal = cfg.GetConfigValue("AUTO_SESSION")
		}
		autoSession := (autoSessionVal == "true")

		defaultProfile := cfg.GetConfigValue("DEFAULT_PROFILE")
		if defaultProfile == "" {
			defaultProfile = "mac"
		}

		isTest := strings.Contains(os.Getenv("HOME"), "remote-studio-e2e-") ||
			strings.Contains(os.Getenv("XDG_RUNTIME_DIR"), "remote-studio-e2e-")

		prevUsers := 0
		home, err := os.UserHomeDir()
		if err == nil {
			sessionFile := filepath.Join(home, ".config", "remote-studio", "session.state")
			if _, err := os.Stat(sessionFile); err == nil {
				prevUsers = 1
			}
		}

		for {
			users, _, ips := status.GetActiveUsersAndConnection()

			trusted := true
			peerOS := ""
			if users > 0 {
				trusted = false
				tsStatusOut, err := exec.Command("tailscale", "status", "--json").Output()
				if err == nil {
					var tsStatus struct {
						Peer map[string]struct {
							TailscaleIPs []string `json:"TailscaleIPs"`
							OS           string   `json:"OS"`
						} `json:"Peer"`
					}
					if err := json.Unmarshal(tsStatusOut, &tsStatus); err == nil {
						for _, ip := range ips {
							if ip == "127.0.0.1" || ip == "localhost" {
								trusted = true
								break
							}
							for _, peerInfo := range tsStatus.Peer {
								for _, tip := range peerInfo.TailscaleIPs {
									if ip == tip {
										trusted = true
										peerOS = peerInfo.OS
										break
									}
								}
								if trusted {
									break
								}
							}
						}
					}
				} else {
					if _, errTS := exec.LookPath("tailscale"); errTS != nil {
						trusted = true
					}
				}
			}

			if !trusted {
				users = 0
			}

			if users > 0 && prevUsers == 0 {
				session.LogEvent(fmt.Sprintf("Watch: session connected (%d user(s))", users))
				if autoSession {
					profile := defaultProfile
					if peerOS == "iOS" {
						profile = "ipad"
					} else if peerOS == "macOS" {
						profile = "mac"
					} else if peerOS == "windows" || peerOS == "linux" {
						profile = "fallback"
					}
					_ = session.SessionStart(profile)
				}
			} else if users == 0 && prevUsers > 0 {
				session.LogEvent("Watch: session disconnected")
				if autoSession {
					_ = session.SessionStop()
				}
			}

			prevUsers = users

			if isTest {
				break
			}
			time.Sleep(time.Duration(interval) * time.Second)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(watchCmd)
}

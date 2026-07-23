package diagnostics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"remote-studio/pkg/config"
)

type CheckResult struct {
	Name    string
	Status  string
	Message string
}

func RunDiagnostics() []CheckResult {
	var results []CheckResult

	// 1. xrandr
	results = append(results, checkXrandr())

	// 2. glxinfo
	results = append(results, checkGlxinfo())

	// 3. display
	results = append(results, checkDisplay())

	// 4. renderer
	results = append(results, checkRenderer())

	// 5. rustdesk
	results = append(results, checkRustdesk())

	// 6. tailscale
	results = append(results, checkTailscale())

	// exit-node (only if tailscale is installed)
	_, errTailscale := exec.LookPath("tailscale")
	if errTailscale == nil {
		results = append(results, checkExitNode())
	}

	// 7. update
	results = append(results, checkUpdate())

	// gh-release (only if git is available)
	_, errGit := exec.LookPath("git")
	if errGit == nil {
		results = append(results, checkGhRelease())
	}

	// 8. log-size
	results = append(results, checkLogSize())

	// backups
	backupsResult := checkBackups()
	if backupsResult.Name != "" {
		results = append(results, backupsResult)
	}

	// state
	stateResult := checkState()
	if stateResult.Name != "" {
		results = append(results, stateResult)
	}

	// symlink
	results = append(results, checkSymlink())

	// applet
	results = append(results, checkApplet())

	return results
}

func checkXrandr() CheckResult {
	path, err := exec.LookPath("xrandr")
	if err != nil {
		return CheckResult{Name: "xrandr", Status: "MISS", Message: "install x11-xserver-utils"}
	}
	return CheckResult{Name: "xrandr", Status: "OK", Message: path}
}

func checkGlxinfo() CheckResult {
	path, err := exec.LookPath("glxinfo")
	if err != nil {
		return CheckResult{Name: "glxinfo", Status: "MISS", Message: "install mesa-utils"}
	}
	return CheckResult{Name: "glxinfo", Status: "OK", Message: path}
}

func checkDisplay() CheckResult {
	_, err := exec.LookPath("xrandr")
	if err != nil {
		return CheckResult{Name: "display", Status: "WARN", Message: "no active X display"}
	}
	cmd := exec.Command("xrandr")
	out, err := cmd.Output()
	if err != nil {
		return CheckResult{Name: "display", Status: "WARN", Message: "no active X display"}
	}
	lines := strings.Split(string(out), "\n")
	var lastConnected string
	var foundActive string
	for _, line := range lines {
		if strings.Contains(line, " connected") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				lastConnected = parts[0]
			}
		}
		if strings.Contains(line, "*") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.Contains(p, "*") {
					foundActive = lastConnected + " " + p
					break
				}
			}
			if foundActive != "" {
				break
			}
		}
	}
	if foundActive != "" {
		return CheckResult{Name: "display", Status: "OK", Message: foundActive}
	}
	return CheckResult{Name: "display", Status: "WARN", Message: "no active X display"}
}

func checkRenderer() CheckResult {
	_, err := exec.LookPath("glxinfo")
	if err != nil {
		return CheckResult{Name: "renderer", Status: "WARN", Message: "unknown (SW)"}
	}
	cmd := exec.Command("glxinfo", "-B")
	out, err := cmd.Output()
	if err != nil {
		return CheckResult{Name: "renderer", Status: "WARN", Message: "unknown (SW)"}
	}
	renderer := "unknown"
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "OpenGL renderer string:") {
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				renderer = strings.TrimSpace(parts[1])
				break
			}
		}
	}
	if strings.Contains(renderer, "llvmpipe") {
		return CheckResult{Name: "renderer", Status: "WARN", Message: renderer + " (SW)"}
	}
	return CheckResult{Name: "renderer", Status: "OK", Message: renderer}
}

func checkRustdesk() CheckResult {
	_, err := exec.LookPath("rustdesk")
	hasRustdeskCmd := (err == nil)

	hasService := false
	cmdService := exec.Command("systemctl", "list-unit-files", "rustdesk.service")
	if err := cmdService.Run(); err == nil {
		hasService = true
	}

	if !hasRustdeskCmd && !hasService {
		return CheckResult{Name: "rustdesk", Status: "MISS", Message: "not installed (download from rustdesk.com)"}
	}

	cmdActive := exec.Command("systemctl", "is-active", "rustdesk")
	out, err := cmdActive.Output()
	status := "inactive"
	if err == nil {
		status = strings.TrimSpace(string(out))
	}
	if status == "active" {
		return CheckResult{Name: "rustdesk", Status: "OK", Message: "active"}
	}
	return CheckResult{Name: "rustdesk", Status: "WARN", Message: status}
}

func checkTailscale() CheckResult {
	_, err := exec.LookPath("tailscale")
	if err != nil {
		// LAN mode: tailscale is genuinely optional. Show INFO, not MISS,
		// so the user doesn't think the install is broken.
		if config.LANModeActive() {
			return CheckResult{Name: "tailscale", Status: "INFO", Message: "not installed (LAN mode active — skipping)"}
		}
		return CheckResult{Name: "tailscale", Status: "MISS", Message: "not installed (curl -fsSL https://tailscale.com/install.sh | sh)"}
	}
	cmdIP := exec.Command("tailscale", "ip", "-4")
	outIP, err := cmdIP.Output()
	tip := strings.TrimSpace(string(outIP))

	tsBackend := "unknown"
	cmdStatus := exec.Command("tailscale", "status", "--json")
	outStatus, err := cmdStatus.Output()
	if err == nil {
		// Match both `"BackendState":"Running"` (compact) and
		// `"BackendState": "Running"` (pretty-printed with space
		// after the colon — newer tailscale versions emit this).
		re := regexp.MustCompile(`"BackendState":\s*"([^"]*)"`)
		matches := re.FindStringSubmatch(string(outStatus))
		if len(matches) >= 2 {
			tsBackend = matches[1]
		}
	}

	if tip != "" {
		return CheckResult{Name: "tailscale", Status: "OK", Message: fmt.Sprintf("%s (%s)", tip, tsBackend)}
	}
	// LAN mode: a missing tailnet IP is expected, not a warning.
	if config.LANModeActive() {
		return CheckResult{Name: "tailscale", Status: "INFO", Message: "no tailnet IP (LAN mode active — skipping)"}
	}
	return CheckResult{Name: "tailscale", Status: "WARN", Message: fmt.Sprintf("no tailnet IP — state: %s (tailscale up?)", tsBackend)}
}

func checkExitNode() CheckResult {
	_, err := exec.LookPath("tailscale")
	if err != nil {
		return CheckResult{Name: "exit-node", Status: "INFO", Message: "none"}
	}
	cmdExit := exec.Command("tailscale", "exit-node", "list")
	outExit, err := cmdExit.Output()
	exitNode := "none"
	if err == nil {
		lines := strings.Split(string(outExit), "\n")
		for _, line := range lines {
			if strings.Contains(line, "selected") {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					exitNode = fields[0]
					break
				}
			}
		}
	}
	return CheckResult{Name: "exit-node", Status: "INFO", Message: exitNode}
}

func checkUpdate() CheckResult {
	cmdGit := exec.Command("git", "rev-parse", "--show-toplevel")
	outGit, err := cmdGit.Output()
	if err != nil {
		return CheckResult{Name: "update", Status: "INFO", Message: "cannot check (no remote)"}
	}
	gitDir := strings.TrimSpace(string(outGit))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmdFetch := exec.CommandContext(ctx, "git", "-C", gitDir,
		"-c", "http.lowSpeedLimit=1000",
		"-c", "http.lowSpeedTime=3",
		"-c", "http.connectTimeout=3",
		"fetch", "--quiet")
	_ = cmdFetch.Run()

	cmdHead := exec.Command("git", "-C", gitDir, "rev-parse", "HEAD")
	outHead, err := cmdHead.Output()
	head := strings.TrimSpace(string(outHead))

	cmdUpstream := exec.Command("git", "-C", gitDir, "rev-parse", "@{u}")
	outUpstream, err := cmdUpstream.Output()
	upstream := strings.TrimSpace(string(outUpstream))

	if head == "" || upstream == "" {
		return CheckResult{Name: "update", Status: "INFO", Message: "cannot check (no remote)"}
	} else if head == upstream {
		return CheckResult{Name: "update", Status: "OK", Message: "up to date"}
	}
	return CheckResult{Name: "update", Status: "WARN", Message: "update available (res update)"}
}

func checkGhRelease() CheckResult {
	cmdGit := exec.Command("git", "rev-parse", "--show-toplevel")
	outGit, err := cmdGit.Output()
	if err != nil {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	}
	gitDir := strings.TrimSpace(string(outGit))

	cmdURL := exec.Command("git", "-C", gitDir, "remote", "get-url", "origin")
	outURL, err := cmdURL.Output()
	if err != nil {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	}
	repoURL := strings.TrimSpace(string(outURL))
	if repoURL == "" {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	}

	repoPath := repoURL
	if strings.Contains(repoPath, "github.com") {
		parts := strings.SplitN(repoPath, "github.com", 2)
		if len(parts) == 2 {
			repoPath = parts[1]
			repoPath = strings.TrimPrefix(repoPath, ":")
			repoPath = strings.TrimPrefix(repoPath, "/")
			repoPath = strings.TrimSuffix(repoPath, ".git")
		}
	} else {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repoPath))
	if err != nil {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	}

	tagName := strings.TrimPrefix(release.TagName, "v")
	// currentVersion comes from a single source of truth in pkg/config.
	// The release Makefile target injects this at build time via
	// `-ldflags "-X remote-studio/pkg/config.Version=$(grep ...)"` so the
	// version number can't drift between the binary and the tag.
	currentVersion := config.Version

	if tagName == "" {
		return CheckResult{Name: "gh-release", Status: "INFO", Message: "could not fetch (offline or no releases)"}
	} else if tagName == currentVersion {
		return CheckResult{Name: "gh-release", Status: "OK", Message: fmt.Sprintf("v%s is the latest release", currentVersion)}
	}
	return CheckResult{Name: "gh-release", Status: "WARN", Message: fmt.Sprintf("v%s running, v%s released (res update)", currentVersion, tagName)}
}

func checkLogSize() CheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return CheckResult{Name: "log-size", Status: "INFO", Message: "no log yet"}
	}
	logPath := filepath.Join(home, ".remote_studio.log")
	info, err := os.Stat(logPath)
	if os.IsNotExist(err) {
		return CheckResult{Name: "log-size", Status: "INFO", Message: "no log yet"}
	}
	size := info.Size()
	sizeKB := size / 1024
	if size > 524288 {
		return CheckResult{Name: "log-size", Status: "WARN", Message: fmt.Sprintf("%d KB (rotates at 1024 KB)", sizeKB)}
	}
	return CheckResult{Name: "log-size", Status: "OK", Message: fmt.Sprintf("%d KB", sizeKB)}
}

func checkBackups() CheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return CheckResult{}
	}
	backupRoot := filepath.Join(home, ".config", "remote-studio", "backups")
	info, err := os.Stat(backupRoot)
	if os.IsNotExist(err) || !info.IsDir() {
		return CheckResult{}
	}

	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		return CheckResult{}
	}
	bcount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			bcount++
		}
	}

	if bcount > 10 {
		return CheckResult{Name: "backups", Status: "WARN", Message: fmt.Sprintf("%d entries (recommended: <= 10)", bcount)}
	}
	return CheckResult{Name: "backups", Status: "OK", Message: fmt.Sprintf("%d entries", bcount)}
}

func checkState() CheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return CheckResult{}
	}
	statePath := filepath.Join(home, ".res_state")
	file, err := os.Open(statePath)
	if os.IsNotExist(err) {
		return CheckResult{}
	}
	defer file.Close()

	stateLabel := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "state=") {
			parts := strings.Split(line, "'")
			if len(parts) >= 2 {
				stateLabel = parts[1]
				break
			}
		}
	}

	if stateLabel == "" {
		return CheckResult{}
	}

	reg := config.NewProfileRegistry()
	var defaultPath string
	execPath, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(execPath), "config", "profiles.conf")
		if _, err := os.Stat(p); err == nil {
			defaultPath = p
		}
	}
	if defaultPath == "" {
		defaultPath = "/usr/share/remote-studio/profiles.conf"
	}
	if _, err := os.Stat(defaultPath); err == nil {
		_ = reg.LoadProfiles(defaultPath)
	}
	userPath := filepath.Join(home, ".config", "remote-studio", "profiles.conf")
	if _, err := os.Stat(userPath); err == nil {
		_ = reg.LoadProfiles(userPath)
	}

	found := false
	for _, p := range reg.Profiles {
		if p.Label == stateLabel {
			found = true
			break
		}
	}

	if !found && !strings.HasPrefix(stateLabel, "Custom") {
		return CheckResult{Name: "state", Status: "WARN", Message: fmt.Sprintf("active mode '%s' no longer in profiles", stateLabel)}
	}
	return CheckResult{Name: "state", Status: "OK", Message: stateLabel}
}

func checkSymlink() CheckResult {
	path := "/usr/local/bin/res"
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return CheckResult{Name: "symlink", Status: "INFO", Message: "/usr/local/bin/res not installed"}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return CheckResult{Name: "symlink", Status: "WARN", Message: fmt.Sprintf("/usr/local/bin/res readlink error: %v", err)}
		}

		cmdGit := exec.Command("git", "rev-parse", "--show-toplevel")
		outGit, err := cmdGit.Output()
		if err == nil {
			gitDir := strings.TrimSpace(string(outGit))
			expectedTarget := filepath.Join(gitDir, "res.sh")
			targetAbs, errTarget := filepath.Abs(target)
			if errTarget == nil && targetAbs == expectedTarget {
				return CheckResult{Name: "symlink", Status: "OK", Message: fmt.Sprintf("/usr/local/bin/res -> %s", target)}
			}
		}

		return CheckResult{Name: "symlink", Status: "WARN", Message: fmt.Sprintf("/usr/local/bin/res -> %s", target)}
	}

	return CheckResult{Name: "symlink", Status: "WARN", Message: "/usr/local/bin/res exists but is not a symlink"}
}

func checkApplet() CheckResult {
	home, err := os.UserHomeDir()
	if err != nil {
		return CheckResult{}
	}

	cinnamonRunning := false
	cmdPgrep := exec.Command("pgrep", "-x", "cinnamon")
	if err := cmdPgrep.Run(); err == nil {
		cinnamonRunning = true
	}

	appletDir := filepath.Join(home, ".local", "share", "cinnamon", "applets", "remote-studio@neek")
	appletJS := filepath.Join(appletDir, "applet.js")
	_, errJS := os.Stat(appletJS)

	if cinnamonRunning {
		if errJS == nil {
			return CheckResult{Name: "applet", Status: "OK", Message: fmt.Sprintf("files present at %s", appletDir)}
		}
		return CheckResult{Name: "applet", Status: "WARN", Message: fmt.Sprintf("files missing at %s", appletDir)}
	}

	return CheckResult{Name: "applet", Status: "INFO", Message: "cinnamon not running"}
}

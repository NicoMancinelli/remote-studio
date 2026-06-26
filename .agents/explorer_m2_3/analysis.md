# Analysis and Design: Go Diagnostics Package and Doctor Subcommand

## 1. Executive Summary
This document defines the requirements, architecture, and Go package design for `pkg/diagnostics` and the associated `doctor` CLI command for the Remote Studio rewrite. The design ensures exact behavioral parity with the legacy Bash scripts (`lib/diagnostics.sh` and warning functions in `lib/core.sh`), guarantees complete testability via system call abstraction, and implements robust error handling to prevent runtime crashes.

The key features designed in this module include:
1. **Command existence checks** for `xrandr` and `glxinfo`.
2. **Systemd service state queries** for `rustdesk` and `tailscale`.
3. **Hardware capability and active display detection** (X11 connection and software vs hardware GLX renderer).
4. **Tailscale network inspections** (tailnet IP, BackendState, and exit nodes).
5. **Git and GitHub Release checkups** with network/timeout guards.
6. **File size, symlink, and Cinnamon applet validity checks**.
7. **`GetWarningSummary` generation** to feed the twin-write applet telemetry.
8. **An automated repair routine (`doctor-fix`)** to resolve configuration drift.

---

## 2. Requirements & Behavioral Parity

To replace the legacy Bash functions without breaking dependencies or desktop applet telemetry, the Go implementation must reproduce the checks, output strings, and warning tags exactly.

### A. Health Checks Specification

The following table maps the legacy checks from `lib/diagnostics.sh` to their expected Go behavior:

| Check Name | Target/Dependency | Legacy Shell Logic | Proposed Go Logic | Expected Outputs / Parity |
|---|---|---|---|---|
| **`xrandr`** | CLI Utility | `command -v xrandr` | `exec.LookPath("xrandr")` | `OK: <path>` or `MISS: install x11-xserver-utils` |
| **`glxinfo`** | CLI Utility | `command -v glxinfo` | `exec.LookPath("glxinfo")` | `OK: <path>` or `MISS: install mesa-utils` |
| **`display`** | X11 Display | `xrandr` output parsing for active mode | Run `xrandr`, match ` connected` and active resolution mode `*` | `OK: <display-mode>` or `WARN: no active X display` |
| **`renderer`** | GPU Driver | `glxinfo -B` output parsing for `OpenGL renderer string:` | Run `glxinfo -B`, parse renderer string. Warn if string contains `"llvmpipe"` | `OK: <renderer-name>` or `WARN: <renderer-name> (SW)` |
| **`rustdesk`** | Systemd service / binary | Check path/unit file. Query `systemctl is-active rustdesk` | Check `exec.LookPath("rustdesk")` and `/lib/systemd/system/rustdesk.service` existence. Execute `systemctl is-active rustdesk` | `MISS: not installed (download from rustdesk.com)`, `OK: active`, or `WARN: <state>` (e.g. `inactive`) |
| **`tailscale`** | Systemd service / CLI | Check path. Query `tailscale ip -4` and `tailscale status --json` | Check `exec.LookPath("tailscale")`. Run `tailscale ip -4` and `tailscale status --json` | `MISS: not installed (curl -fsSL https://tailscale.com/install.sh \| sh)`, `OK: <ip> (<state>)`, or `WARN: no tailnet IP — state: <state> (tailscale up?)` |
| **`exit-node`** | Tailscale Exit Node | `tailscale exit-node list` parsing for `"selected"` | Run `tailscale exit-node list` and parse output | `INFO: <exit-node-name>` or `INFO: none` |
| **`update`** | Local Git Repo | `git fetch --quiet` with timeout. Compare `HEAD` vs `@{u}` | Run `git fetch --quiet` (guarded by timeout/configs). Run `git rev-parse HEAD` and `git rev-parse @{u}` | `INFO: cannot check (no remote)`, `OK: up to date`, or `WARN: update available (res update)` |
| **`gh-release`** | GitHub API | Get origin URL. Query API `releases/latest` (3s timeout). Compare tag to `VERSION` | Read git remote URL, parse owner/repo. Perform `GET` request to GitHub API (3s timeout). Parse JSON `tag_name` and compare with version | `INFO: could not fetch (offline or no releases)`, `OK: v<version> is the latest release`, or `WARN: v<current> running, v<latest> released (res update)` |
| **`log-size`** | Log File | File size of `~/.remote_studio.log` vs `524288` bytes | Get `FileInfo` of log file. If size > 512 KB, raise warning | `INFO: no log yet`, `OK: <size> KB`, or `WARN: <size> KB (rotates at 1024 KB)` |
| **`backups`** | Backup Directory | Directory count at `~/.config/remote-studio/backups` | List directories in backups path. Warn if count > 10 | `OK: <count> entries` or `WARN: <count> entries (recommended: <= 10)` |
| **`state`** | Active Session State | File `~/.res_state` active mode vs loaded profile labels | Read state file, extract label, check if label is present in `ProfileRegistry` | `OK: <label>` or `WARN: active mode '<label>' no longer in profiles` (only if not `"Custom*"`) |
| **`symlink`** | Main CLI Symlink | Target check for `/usr/local/bin/res` | Verify if symlink exists at `/usr/local/bin/res` and targets the currently running `res` executable path | `OK: /usr/local/bin/res -> <target>`, `WARN: /usr/local/bin/res -> <target> (expected <exec>)`, `WARN: /usr/local/bin/res exists but is not a symlink`, or `INFO: /usr/local/bin/res not installed` |
| **`applet`** | Cinnamon Applet | `pgrep cinnamon` + file validity at `~/.local/share/cinnamon/applets/remote-studio@neek` | Check if `cinnamon` process is running. Verify `applet.js` and `metadata.json` link targets | `INFO: cinnamon not running`, `OK: files present at <applet_dir>`, or `WARN: files missing at <applet_dir>` |

---

## 3. Package Design: `pkg/diagnostics`

To achieve perfect testability, the system calls, file system checks, process listings, and HTTP calls are abstracted behind a `SystemContext` interface. This allows writing unit tests that simulate diverse environment states (offline, missing dependencies, custom display layouts, active/inactive systemd services) without interacting with the host system.

### A. Type Definitions (`pkg/diagnostics/types.go`)

```go
package diagnostics

import (
	"os"
	"time"
)

// Status represents the health status of a specific check.
type Status string

const (
	StatusOK   Status = "OK"
	StatusWarn Status = "WARN"
	StatusMiss Status = "MISS"
	StatusInfo Status = "INFO"
)

// CheckResult represents the outcome of a single diagnostics check.
type CheckResult struct {
	Name        string `json:"name"`
	Status      Status `json:"status"`
	Description string `json:"description"`
}

// SystemContext isolates external system dependencies for complete unit testability.
type SystemContext interface {
	// CommandExists returns true if the binary is on the system PATH.
	CommandExists(name string) bool
	
	// RunCommand executes a CLI command and returns stdout, or an error.
	RunCommand(cwd string, name string, args ...string) (string, error)
	
	// ReadFile reads the content of the file.
	ReadFile(path string) ([]byte, error)
	
	// Stat returns FileInfo for the path.
	Stat(path string) (os.FileInfo, error)
	
	// ReadLink returns the destination of the symlink.
	ReadLink(path string) (string, error)
	
	// Symlink creates a symbolic link pointing to target.
	Symlink(target, link string) error
	
	// Remove removes the named file or directory.
	Remove(path string) error
	
	// MkdirAll creates a directory and parent directories.
	MkdirAll(path string, perm os.FileMode) error
	
	// ProcessExists returns true if a process with the specified name is active.
	ProcessExists(name string) bool
	
	// HomeDir returns the user's home directory.
	HomeDir() string
	
	// ExecutablePath returns the path of the current running binary.
	ExecutablePath() string
	
	// HTTPGet executes a HTTP GET request with a timeout.
	HTTPGet(url string, timeout time.Duration) ([]byte, error)
}
```

### B. Diagnostics Rules Registry (`pkg/diagnostics/rules.go`)

We define the interface for individual checkups, and implement the rules using the `SystemContext`.

```go
package diagnostics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"remote-studio/pkg/config"
)

// Rule represents a single diagnostic check rule.
type Rule interface {
	Name() string
	Run(ctx SystemContext, registry *config.ProfileRegistry) CheckResult
}

// DiagnosticsEngine aggregates and executes rules.
type DiagnosticsEngine struct {
	Rules []Rule
}

func NewDiagnosticsEngine() *DiagnosticsEngine {
	return &DiagnosticsEngine{
		Rules: []Rule{
			&XrandrCheck{},
			&GlxinfoCheck{},
			&DisplayCheck{},
			&RendererCheck{},
			&RustdeskCheck{},
			&TailscaleCheck{},
			&ExitNodeCheck{},
			&UpdateCheck{},
			&GithubReleaseCheck{},
			&LogSizeCheck{},
			&BackupsCheck{},
			&StateCheck{},
			&SymlinkCheck{},
			&AppletCheck{},
		},
	}
}

func (e *DiagnosticsEngine) RunAll(ctx SystemContext, registry *config.ProfileRegistry) []CheckResult {
	results := make([]CheckResult, len(e.Rules))
	for i, rule := range e.Rules {
		results[i] = rule.Run(ctx, registry)
	}
	return results
}
```

### C. Individual Rule Implementations (Example Schemas)

#### 1. Display Connection Check (`pkg/diagnostics/rule_display.go`)
```go
package diagnostics

import (
	"regexp"
	"strings"
	"remote-studio/pkg/config"
)

type DisplayCheck struct{}

func (c *DisplayCheck) Name() string { return "display" }

func (c *DisplayCheck) Run(ctx SystemContext, reg *config.ProfileRegistry) CheckResult {
	if !ctx.CommandExists("xrandr") {
		return CheckResult{Name: c.Name(), Status: StatusWarn, Description: "no active X display"}
	}
	output, err := ctx.RunCommand("", "xrandr")
	if err != nil {
		return CheckResult{Name: c.Name(), Status: StatusWarn, Description: "no active X display"}
	}

	// Legacy regex behavior: find connected output and active resolution mode '*'
	// awk '/ connected/ {out=$1} /\*/ {print out " " $1; exit}'
	var connectedDisplay string
	lines := strings.Split(output, "\n")
	connectedRe := regexp.MustCompile(`^(\S+)\s+connected`)
	activeModeRe := regexp.MustCompile(`\*`)

	for _, line := range lines {
		if m := connectedRe.FindStringSubmatch(line); len(m) > 1 {
			connectedDisplay = m[1]
		}
		if activeModeRe.MatchString(line) && connectedDisplay != "" {
			// Extract mode token
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return CheckResult{
					Name:        c.Name(),
					Status:      StatusOK,
					Description: connectedDisplay + " " + fields[0],
				}
			}
		}
	}

	return CheckResult{Name: c.Name(), Status: StatusWarn, Description: "no active X display"}
}
```

#### 2. Renderer GPU Check (`pkg/diagnostics/rule_renderer.go`)
```go
type RendererCheck struct{}

func (c *RendererCheck) Name() string { return "renderer" }

func (c *RendererCheck) Run(ctx SystemContext, reg *config.ProfileRegistry) CheckResult {
	if !ctx.CommandExists("glxinfo") {
		return CheckResult{Name: c.Name(), Status: StatusWarn, Description: "unknown (SW)"}
	}
	output, err := ctx.RunCommand("", "glxinfo", "-B")
	if err != nil {
		return CheckResult{Name: c.Name(), Status: StatusWarn, Description: "unknown (SW)"}
	}

	// Parse 'OpenGL renderer string: <value>'
	var renderer string
	re := regexp.MustCompile(`OpenGL renderer string:\s*(.*)`)
	for _, line := range strings.Split(output, "\n") {
		if m := re.FindStringSubmatch(line); len(m) > 1 {
			renderer = strings.TrimSpace(m[1])
			break
		}
	}

	if renderer == "" {
		return CheckResult{Name: c.Name(), Status: StatusWarn, Description: "unknown (SW)"}
	}

	if strings.Contains(renderer, "llvmpipe") {
		return CheckResult{Name: c.Name(), Status: StatusWarn, Description: renderer + " (SW)"}
	}

	return CheckResult{Name: c.Name(), Status: StatusOK, Description: renderer}
}
```

#### 3. Tailscale Status Check (`pkg/diagnostics/rule_tailscale.go`)
```go
type TailscaleCheck struct{}

func (c *TailscaleCheck) Name() string { return "tailscale" }

func (c *TailscaleCheck) Run(ctx SystemContext, reg *config.ProfileRegistry) CheckResult {
	if !ctx.CommandExists("tailscale") {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusMiss,
			Description: "not installed (curl -fsSL https://tailscale.com/install.sh | sh)",
		}
	}

	// 1. Get IP
	ipOutput, _ := ctx.RunCommand("", "tailscale", "ip", "-4")
	ip := strings.TrimSpace(ipOutput)

	// 2. Get BackendState
	statusJson, err := ctx.RunCommand("", "tailscale", "status", "--json")
	var state string
	if err == nil {
		var statusStruct struct {
			BackendState string `json:"BackendState"`
		}
		if json.Unmarshal([]byte(statusJson), &statusStruct) == nil {
			state = statusStruct.BackendState
		}
	}

	if state == "" {
		state = "unknown"
	}

	if ip != "" {
		return CheckResult{
			Name:        c.Name(),
			Status:      StatusOK,
			Description: fmt.Sprintf("%s (%s)", ip, state),
		}
	}

	return CheckResult{
		Name:        c.Name(),
		Status:      StatusWarn,
		Description: fmt.Sprintf("no tailnet IP — state: %s (tailscale up?)", state),
	}
}
```

---

## 4. Warning Summary Logic (`GetWarningSummary`)

The Cinnamon Applet panel retrieves the warning status using a pipe-delimited summary: `count|warning1,warning2`.
The Go foundation must recreate this exact layout to support backwards compatibility.

### Warning Summary Rules Parity
1. **`software-rendering`**: Triggers if the GPU renderer is `"llvmpipe"`.
2. **`rustdesk-<state>`**: Triggers if rustdesk is not `"active"`. If `systemctl is-active rustdesk` returns a status like `"inactive"` or `"failed"`, it maps to `rustdesk-inactive` or `rustdesk-failed`.
3. **`tailscale`**: Triggers if tailscale is not `"active"` or the tailnet IP is empty.
4. **`display`**: Triggers if there is no active display connection.
5. **`applet-symlink`**: Triggers if Cinnamon is running and any of the files in `~/.local/share/cinnamon/applets/remote-studio@neek/` do not resolve as symlinks pointing to the repository's files.
6. **Tailscale Connection states**: When tailscale is active but not fully functional:
   - `NeedsLogin` or `Stopped` maps to `tailscale-needslogin` or `tailscale-stopped`.
   - `NoState`, `Starting`, or `NoNetwork` maps to `tailscale-offline`.

### Go Implementation Scheme (`pkg/diagnostics/warnings.go`)

```go
package diagnostics

import (
	"fmt"
	"strings"
	"remote-studio/pkg/config"
)

// WarningSummary generates the legacy format 'count|msg1,msg2' or '0|OK'
func WarningSummary(ctx SystemContext, reg *config.ProfileRegistry) string {
	warnings := 0
	messages := make([]string, 0)

	// 1. Check Renderer
	rc := (&RendererCheck{}).Run(ctx, reg)
	if rc.Status == StatusWarn {
		warnings++
		messages = append(messages, "software-rendering")
	}

	// 2. Check RustDesk Service
	var rustdeskState string
	if ctx.CommandExists("systemctl") {
		out, err := ctx.RunCommand("", "systemctl", "is-active", "rustdesk")
		if err == nil {
			rustdeskState = strings.TrimSpace(out)
		} else {
			rustdeskState = "inactive"
		}
	} else {
		rustdeskState = "unknown"
	}
	if rustdeskState != "active" {
		warnings++
		messages = append(messages, "rustdesk-"+rustdeskState)
	}

	// 3. Check Tailscale Service
	var tailscaleState string
	if ctx.CommandExists("systemctl") {
		out, err := ctx.RunCommand("", "systemctl", "is-active", "tailscaled")
		if err == nil {
			tailscaleState = strings.TrimSpace(out)
		} else {
			tailscaleState = "inactive"
		}
	} else {
		tailscaleState = "unknown"
	}

	// 4. Check Tailscale Status
	tsCheck := (&TailscaleCheck{}).Run(ctx, reg)
	if tailscaleState != "active" || tsCheck.Status == StatusWarn || tsCheck.Status == StatusMiss {
		warnings++
		messages = append(messages, "tailscale")
	}

	// 5. Check Display
	dispCheck := (&DisplayCheck{}).Run(ctx, reg)
	if dispCheck.Status == StatusWarn {
		warnings++
		messages = append(messages, "display")
	}

	// 6. Check Applet Symlinks
	appletCheck := (&AppletCheck{}).Run(ctx, reg)
	if appletCheck.Status == StatusWarn {
		warnings++
		messages = append(messages, "applet-symlink")
	}

	// 7. Check Tailscale Connection States
	if tailscaleState == "active" {
		statusJson, err := ctx.RunCommand("", "tailscale", "status", "--json")
		if err == nil {
			var ts struct {
				BackendState string `json:"BackendState"`
			}
			if json.Unmarshal([]byte(statusJson), &ts) == nil {
				switch ts.BackendState {
				case "NeedsLogin", "Stopped":
					warnings++
					messages = append(messages, "tailscale-"+strings.ToLower(ts.BackendState))
				case "NoState", "Starting", "NoNetwork":
					warnings++
					messages = append(messages, "tailscale-offline")
				}
			}
		}
	}

	if warnings == 0 {
		return "0|OK"
	}

	return fmt.Sprintf("%d|%s", warnings, strings.Join(messages, ","))
}
```

---

## 5. Automated Repair: `doctor-fix`

The `doctor-fix` (or `FixCommonIssues`) routine performs three corrections to align local configs with repository defaults.

### Go Action Design (`pkg/diagnostics/fix.go`)

```go
package diagnostics

import (
	"fmt"
	"path/filepath"
)

// FixCommonIssues matches the behavior of legacy `doctor_fix`.
func FixCommonIssues(ctx SystemContext) error {
	rootDir := filepath.Dir(ctx.ExecutablePath())
	home := ctx.HomeDir()

	fmt.Println("Fixing common issues...")

	// 1. Link ~/.xsessionrc to root's config/xsessionrc
	xsessionrcLink := filepath.Join(home, ".xsessionrc")
	xsessionrcTarget := filepath.Join(rootDir, "config", "xsessionrc")
	
	curTarget, err := ctx.ReadLink(xsessionrcLink)
	if err != nil || curTarget != xsessionrcTarget {
		_ = ctx.Remove(xsessionrcLink)
		if err := ctx.Symlink(xsessionrcTarget, xsessionrcLink); err != nil {
			return fmt.Errorf("failed to link xsessionrc: %w", err)
		}
	}

	// 2. Link Applet Files
	appletDir := filepath.Join(home, ".local", "share", "cinnamon", "applets", "remote-studio@neek")
	if err := ctx.MkdirAll(appletDir, 0755); err != nil {
		return fmt.Errorf("failed to create applet directory: %w", err)
	}

	files := []string{"applet.js", "metadata.json"}
	for _, f := range files {
		linkPath := filepath.Join(appletDir, f)
		targetPath := filepath.Join(rootDir, "applet", f)
		curT, err := ctx.ReadLink(linkPath)
		if err != nil || curT != targetPath {
			_ = ctx.Remove(linkPath)
			if err := ctx.Symlink(targetPath, linkPath); err != nil {
				return fmt.Errorf("failed to link applet file %s: %w", f, err)
			}
		}
	}

	// 3. Copy default RustDesk TOML if missing
	rustdeskConfDir := filepath.Join(home, ".config", "rustdesk")
	rustdeskConfFile := filepath.Join(rustdeskConfDir, "RustDesk_default.toml")
	if _, err := ctx.Stat(rustdeskConfFile); err != nil {
		// Does not exist, copy default
		defaultTomlSource := filepath.Join(rootDir, "config", "RustDesk_default.toml")
		data, err := ctx.ReadFile(defaultTomlSource)
		if err != nil {
			return fmt.Errorf("failed to read default RustDesk toml source: %w", err)
		}
		if err := ctx.MkdirAll(rustdeskConfDir, 0755); err != nil {
			return fmt.Errorf("failed to create rustdesk config dir: %w", err)
		}
		// Write with 0644
		tmpFile := filepath.Join(rustdeskConfDir, "RustDesk_default.toml.tmp")
		// (Atomic write protocol similar to status)
		// Simulating write:
		// ioutil.WriteFile/os.WriteFile equivalent through context if extended,
		// or directly. Since this is design:
	}

	fmt.Println("Done.")
	return nil
}
```

---

## 6. CLI Command Integration (`cmd/doctor.go`)

We use the popular `github.com/spf13/cobra` package to parse the subcommands.

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"remote-studio/pkg/config"
	"remote-studio/pkg/diagnostics"
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system health and configurations",
	Long:  `Inspects dependencies, Xorg status, active GPU rendering, and services (RustDesk, Tailscale).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize context
		ctx := diagnostics.NewDefaultSystemContext()
		
		// Load Profiles
		registry, err := config.LoadAllProfiles()
		if err != nil {
			registry = config.NewProfileRegistry()
		}

		// Run engine
		engine := diagnostics.NewDiagnosticsEngine()
		results := engine.RunAll(ctx, registry)

		// Print results matching legacy formatting:
		// printf "%-22s %-4s %s\n" "$1" "$2" "$3"
		fmt.Println("Remote Studio doctor")
		for _, r := range results {
			fmt.Printf("%-22s %-4s %s\n", r.Name, r.Status, r.Description)
		}

		return nil
	},
}

// doctorFixCmd represents the doctor-fix command
var doctorFixCmd = &cobra.Command{
	Use:   "doctor-fix",
	Short: "Automatically repair common configuration issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := diagnostics.NewDefaultSystemContext()
		return diagnostics.FixCommonIssues(ctx)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(doctorFixCmd)
}
```

---

## 7. Verification and Unit Testing Strategy

To isolate the test environment and avoid mutating the runner's machine, the unit tests must define a mock implementation of `SystemContext`.

### A. Mock Context Strategy (`pkg/diagnostics/doctor_test.go`)

```go
type MockSystemContext struct {
	Commands     map[string]string // command name -> mock output
	CommandErr   map[string]error  // command name -> mock error
	Files        map[string][]byte
	Symlinks     map[string]string
	Processes    map[string]bool
	MockHome     string
	MockExecPath string
	HTTPResponse map[string][]byte
}

func (m *MockSystemContext) CommandExists(name string) bool {
	_, exists := m.Commands[name]
	return exists
}

func (m *MockSystemContext) RunCommand(cwd string, name string, args ...string) (string, error) {
	if err, exists := m.CommandErr[name]; exists && err != nil {
		return "", err
	}
	return m.Commands[name], nil
}

// ... other interface implementation methods mapping to mock maps
```

### B. Suggested Test Scenarios

1. **Software Rendering Detection**:
   - Provide a mock `glxinfo` output containing `OpenGL renderer string: llvmpipe (LLVM 12.0.0, 256 bits)`.
   - Verify that `RendererCheck` results in `StatusWarn` and reports `llvmpipe (SW)`.

2. **Display Disconnection**:
   - Provide a mock `xrandr` output that does not contain any active display connection.
   - Verify that `DisplayCheck` returns `StatusWarn` ("no active X display").

3. **Tailscale Offline**:
   - Provide a mock `tailscale status --json` output with `BackendState: "NoNetwork"`.
   - Verify that `TailscaleCheck` detects state correctly and the warning summary contains `tailscale-offline`.

4. **Self-Contained Repair Validation**:
   - Run `FixCommonIssues` with an empty mock directory layout.
   - Verify that `MockSystemContext` records `Symlink` calls to `~/.xsessionrc` and Cinnamon applet paths, and that directories are created.

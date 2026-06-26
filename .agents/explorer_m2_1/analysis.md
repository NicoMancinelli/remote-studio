# Remote Studio: CLI Architecture and Subcommands Design Report

This report analyzes the requirements and designs the command-line interface (CLI) structure for the modernized Go-based `res` control plane. It details the root command and the `version`, `info`, and `log` subcommands, ensuring complete compatibility with legacy shell-based behaviors and terminal outputs.

---

## 1. CLI Architecture & Cobra Command Hierarchy

The modernized `res` control plane will be built using the **Cobra** library (`github.com/spf13/cobra`). The CLI hierarchy maps directly to the legacy subcommands found in `res.sh` while providing structured command routing.

### Proposed Code Layout
- `cmd/res/main.go`: Main entry point containing `main()` and invoking the root CLI execution.
- `pkg/cli/root.go`: Definition of `RootCmd` and CLI initializations.
- `pkg/cli/version.go`: Definition and logic of the `version` subcommand.
- `pkg/cli/log.go`: Definition, log-reading utilities, and logic of the `log` subcommand.
- `pkg/cli/info.go`: Definition, system-probing, state-parsing, and layout logic of the `info` subcommand.

### Root Command Design (`res`)
In legacy `res.sh`, running the script with no arguments initiates the interactive Cinnamon TUI (using `whiptail` or falling back to a text-based menu).
In the Go implementation, `RootCmd` must emulate this behavior by starting the TUI when executed without arguments, rather than showing a generic help page.

```go
// pkg/cli/root.go
package cli

import (
    "os"
    "github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
    Use:   "res",
    Short: "Remote Studio display management suite",
    Long:  `Remote Studio optimizes Linux Mint (Cinnamon) hosts for remote access.`,
    RunE: func(cmd *cobra.Command, args []string) error {
        // If no subcommand is specified, start the interactive TUI
        return StartInteractiveTUI()
    },
}

func Execute() {
    if err := RootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

func init() {
    // Add subcommands
    RootCmd.AddCommand(VersionCmd)
    RootCmd.AddCommand(InfoCmd)
    RootCmd.AddCommand(LogCmd)
}
```

---

## 2. Version Subcommand Design

The `version` subcommand prints the application version and exits.

### Legacy Behavior
In `res.sh`, the version is stored in the `$VERSION` variable (currently `"9.0"`). Running `res version` outputs:
```text
9.0
```

### Cobra Command Blueprint
To preserve this contract, the version command prints the hardcoded version string (or injected variable) followed by a single newline.

```go
// pkg/cli/version.go
package cli

import (
    "fmt"
    "github.com/spf13/cobra"
)

const Version = "9.0"

var VersionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print the Remote Studio version",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println(Version)
    },
}
```

---

## 3. Log Subcommand Design

The `log` subcommand displays log events from the Remote Studio log file.

### Legacy Behavior
1. **Log Location**: Defaults to `~/.remote_studio.log` (resolves to `$HOME/.remote_studio.log`).
2. **Missing Log**: If the file does not exist, the command outputs `"No log file yet."` and exits with code `0`.
3. **Log Tailing**: If the file exists, it tails the last `N` lines (defaulting to `20`).
4. **Custom Line Count**: The command accepts a single optional positional argument specifying the number of lines to print (e.g., `res log 50`).

### Enhanced Go CLI Features
While keeping positional legacy compatibility, we can add standard flags such as `-n`/`--lines` and `-f`/`--follow` (replicating `tail -f`).

### Cobra Command Blueprint
```go
// pkg/cli/log.go
package cli

import (
    "errors"
    "fmt"
    "strconv"
    "github.com/spf13/cobra"
)

var (
    logLines  int
    followLog bool
)

var LogCmd = &cobra.Command{
    Use:   "log [lines]",
    Short: "Print the tail of the Remote Studio log file",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        lines := 20 // Default legacy line count

        // 1. Check positional argument (legacy mode)
        if len(args) == 1 {
            parsed, err := strconv.Atoi(args[0])
            if err != nil || parsed < 0 {
                return fmt.Errorf("invalid line count: %s", args[0])
            }
            lines = parsed
        } else if cmd.Flags().Changed("lines") {
            // 2. Check --lines flag (standard CLI enhancement)
            lines = logLines
        }

        return ReadAndPrintLogs(lines, followLog)
    },
}

func init() {
    LogCmd.Flags().IntVarP(&logLines, "lines", "n", 20, "Number of lines to show")
    LogCmd.Flags().BoolVarP(&followLog, "follow", "f", false, "Follow log output (live updates)")
}
```

### Log File Reading & Rotation Logic
In `res.sh`, log rotation is handled by `log_event()`: when the log file size exceeds 1MB (`1048576` bytes), it is moved to `~/.remote_studio.log.1` and a new log file is created.

When reading logs:
1. **Path Resolution**: Resolve `$HOME/.remote_studio.log`.
2. **Checking Existence**: If the file does not exist, print `No log file yet.` and exit with status 0.
3. **Tailing Algorithm**:
   - Because the log file is capped at 1MB, we can safely read the active log file in memory, split it by newlines, and extract the last `N` lines.
   - *Advanced Enhancement*: If the active log file contains fewer than `N` lines, and the rotated file `~/.remote_studio.log.1` exists, we can read the necessary remaining lines from `~/.remote_studio.log.1` to fulfill the user's request.
4. **Follow Mode (`-f`)**:
   - If `--follow` is set, read the last `N` lines first.
   - Use a file poller or `fsnotify` library to watch for write events on the log file, printing new lines as they are written.

---

## 4. Info Subcommand Design

The `info` subcommand aggregates and displays system health, active configurations, and environment toggles.

### Legacy Behavior
Running `res info` outputs a colored status list:
```text
Remote Studio [Cyan Bold]
  Mode:        mac [Green Bold] (1920x1080)
  Speed Mode:  ON [Green Bold] (or OFF [Dim])
  Theme:       Dark
  Night Shift: ON [Yellow Bold] (or OFF [Dim])
  Caffeine:    ON [Green Bold] (or OFF [Dim])
  IP:          100.1.2.3
  Temp:        ⚠️ 82.5°C [Yellow/Red Bold if > 80]
  RAM:         45.2%
  Latency:     24ms
  RustDesk:    1 user(s)
```

### Component Breakdown & Go Implementation Plan

To replicate this, the Go CLI must resolve the same system and configuration variables:

#### 1. Color Definitions (ANSI Codes)
```go
const (
    ColorCyan   = "\033[1;36m"
    ColorGreen  = "\033[1;32m"
    ColorYellow = "\033[1;33m"
    ColorDim    = "\033[2m"
    ColorReset  = "\033[0m"
)
```

#### 2. Active Mode and Resolution (`curMode`, `curRes`)
- **File**: Read from `~/.res_state`.
- **Legacy Format**: A single line: `width height scaling text_scale cursor 'label'` (e.g. `1920 1080 1 1.0 24 'mac'`).
- **Go Logic**:
  - Open and read `~/.res_state`.
  - Parse space-separated values.
  - Extract the first two fields: `width` and `height`. Format as `${width}x${height}`.
  - Find the string enclosed in single quotes (the 6th field) to extract the profile label (e.g., `"mac"`).
  - If the file is missing, return `curMode = "None"` and `curRes = "N/A"`.

#### 3. Toggle States (`Speed Mode`, `Theme`, `Night Shift`, `Caffeine`)
- **Speed Mode**:
  - Query: `gsettings get org.cinnamon desktop-effects`
  - Output: If `"false"`, return `"ON"` (styled `ColorGreen + "ON"`), else `"OFF"` (styled `ColorDim + "OFF"`).
- **Caffeine**:
  - Query: `gsettings get org.cinnamon.desktop.screensaver lock-enabled`
  - Output: If `"false"`, return `"ON"` (styled `ColorGreen + "ON"`), else `"OFF"` (styled `ColorDim + "OFF"`).
- **Theme**:
  - Query: `gsettings get org.cinnamon.desktop.interface gtk-theme`
  - Output: Read result and remove single quotes. If the string contains `"Dark"`, return `"Dark"`, else `"Light"`.
- **Night Shift**:
  - Query: Execute `xgamma` and capture standard error / standard output.
  - Output format: `"Red  1.000, Green  1.000, Blue  1.000"` (specifically, 4th space-separated field).
  - Go Logic: Parse the red gamma value (field 4). If it does not equal `"1.000"`, return `"ON"` (styled `ColorYellow + "ON"`), else `"OFF"` (styled `ColorDim + "OFF"`).

#### 4. System Diagnostics (`IP`, `Temp`, `RAM`, `Latency`, `RustDesk`)
- **IP Address**:
  - Get Tailscale IP: Execute `tailscale ip -4` and parse the first IPv4 address.
  - Fallback LAN IP: If Tailscale is unavailable, query local network interfaces in Go (`net.InterfaceAddrs()`) and return the first non-loopback IPv4 address.
- **Temperature**:
  - Query: Read sensors or read `/sys/class/hwmon/` directory natively.
  - Command: `sensors` and parse the line with `"Package id 0"`.
  - Format: Parse temperature numeric value (e.g. `45.0` from `+45.0°C`).
  - Alert: If the temperature is `> 80`, prepend the warning symbol `"⚠️ "` to the output.
- **RAM**:
  - Go Logic: Read `/proc/meminfo` directly (avoiding external process execution).
  - Formula: `used = MemTotal - MemAvailable`. Percentage = `used * 100 / MemTotal`.
  - Output: Format to one decimal place followed by `%` (e.g. `45.2%`).
- **Latency (Ping Cache)**:
  - Cache Location: `/tmp/remote-studio/.ping_cache` (or `$XDG_RUNTIME_DIR/remote-studio/.ping_cache`).
  - Read:
    - Line 1: Unix timestamp (seconds).
    - Line 2: Cached ping value in milliseconds.
  - If the cache exists and is newer than 30 seconds, return `${pingVal}ms`.
  - If the cache is stale or missing, return `"…"` (or the stale cached value) and execute a background goroutine to perform:
    - `ping -c 1 -W 1 8.8.8.8`
    - Extract the latency value from the `time=` token.
    - Write the current timestamp and new latency back to the cache file.
- **RustDesk Connections**:
  - Probe: Run `ss -tnp` and find established connections containing `"rustdesk"`.
  - Filter: Extract the foreign address column, filter for unique client IPs, and count the occurrences.

### Cobra Command Blueprint
```go
// pkg/cli/info.go
package cli

import (
    "fmt"
    "github.com/spf13/cobra"
)

var InfoCmd = &cobra.Command{
    Use:   "info",
    Short: "Display Remote Studio status information",
    RunE: func(cmd *cobra.Command, args []string) error {
        return ShowSystemInfo()
    },
}

func ShowSystemInfo() error {
    // 1. Gather status variables
    mode, res := getActiveModeAndResolution()
    speed := getSpeedMode()
    theme := getTheme()
    night := getNightShift()
    caffeine := getCaffeine()
    ip := getIPAddress()
    temp, alert := getTemperature()
    ram := getRAMUsage()
    latency := getLatency()
    users := getRustDeskUsers()

    // 2. Format output matching legacy shell script
    fmt.Printf("%sRemote Studio%s\n", ColorCyan, ColorReset)
    fmt.Printf("  Mode:        %s%s%s (%s)\n", ColorGreen, mode, ColorReset, res)
    fmt.Printf("  Speed Mode:  %s\n", speed)
    fmt.Printf("  Theme:       %s\n", theme)
    fmt.Printf("  Night Shift: %s\n", night)
    fmt.Printf("  Caffeine:    %s\n", caffeine)
    fmt.Printf("  IP:          %s\n", ip)
    fmt.Printf("  Temp:        %s%s\n", alert, temp)
    fmt.Printf("  RAM:         %s\n", ram)
    fmt.Printf("  Latency:     %s\n", latency)
    fmt.Printf("  RustDesk:    %d user(s)\n", users)

    return nil
}
```

---

## 5. Verification & Testing Strategy

To guarantee that the Go implementation behaves exactly like the legacy components, we can apply two levels of testing:

### 1. Bats Mock Verification
The existing Bats test suite (`tests/test_log.bats` and `tests/test_diagnostics.bats`) should be adapted to verify the Go binary instead of `res.sh`.
Because the Bats suite relies on environment exports and mocks (like stubbing out `xgamma` and `gsettings` using `mock_commands.bash`), compiling the Go binary and running it under the same Bats test suite will verify that it parses mock outputs correctly and returns matching exits and outputs.

**Sample Bats Adapter command**:
```bash
# In tests/test_log.bats, replace the script pointer with the compiled Go binary:
SCRIPT="/Users/nico/Library/Mobile Documents/com~apple~CloudDocs/Dev/Code/Github/Remote-Studio/dist/res"
```

### 2. Go Unit Tests
Create unit tests in `pkg/cli/` to test state-file parsing and log-reading algorithms directly:
- **State-file test**: Feed various space-separated strings (with and without single quotes) to the parser and verify they resolve to correct structs.
- **Log tailing test**: Create temporary log files of varying line counts, execute log tailing, and verify output line counts match expectations.
- **Ping cache test**: Validate cache age checks and check that background refreshes are launched asynchronously.

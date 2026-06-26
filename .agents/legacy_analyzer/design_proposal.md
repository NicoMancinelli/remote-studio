# Remote Studio Design Proposal: Go Rewrite Foundation

This document outlines the findings from the analysis of the legacy Remote Studio codebase (`res.sh`, `lib/*.sh`, and `daemon/remote_studio_daemon.py`) and presents a modular, robust Go architecture design to replace the legacy system, meeting the **Go Rewrite Foundation (R1)** requirements.

---

## 1. Legacy Codebase Analysis Findings

### 1.1 CLI Commands & Subcommands
The legacy Bash control plane (`res.sh` and supporting modules in `lib/`) provides the following subcommands and flags:

| Subcommand / Arguments | Options/Flags | Behavior & Implementation Details |
|---|---|---|
| `custom <width> <height> [scale]` | None | Generates modelines using `cvt`, adds and applies the mode via `xrandr` to the primary connected output. Sets Cinnamon settings factor, text-scaling factor, and cursor size, and optionally saves the configuration to `$HOME/.config/remote-studio/profiles.conf`. |
| `status` | `--json` | Gathers system telemetry, temperature, ping, users, RAM, warnings, speed, IP, connection type, resolution, direct address, and codec. Formats the data as a pipe-delimited string (written to `$STATUS_FILE`). If `--json` is specified, prints the output in JSON format. |
| `info` | None | Prints a human-readable summary of screen mode, speed mode, theme, night shift, caffeine state, and telemetry. |
| `log` | `[lines]` | Displays the tail of `~/.remote_studio.log` (default 20 lines). |
| `doctor` | None | Inspects commands (`xrandr`, `glxinfo`), display connection, GPU renderer (warns on `llvmpipe`), service statuses (`rustdesk`, `tailscaled`), git revisions, log size, and backup sizes. |
| `doctor-fix` | None | Automates common setups: symlinks `~/.xsessionrc` and Cinnamon applet files; creates default RustDesk configuration. |
| `self-test` | None | Executes validation tests on CLI availability, paths, profiles, logging, config actions, and doctor status. |
| `init` | None | TUI-based setup using `whiptail` to verify packages, install Tailscale/RustDesk, configure defaults, and link the applet. |
| `tailnet` | `peer`, `doctor`, `hosts`, `exit-node` | Queries Tailscale info: prints IPv4 address; runs `tailscale netcheck` (doctor); lists peers (`hosts`); pings/inspects specific peer (`peer [name]`); shows active exit-node. |
| `rustdesk` | `apply`, `backup`, `diff`, `status`, `log` | Manages RustDesk configurations: backups (`backup`); compares against templates (`diff <preset>`); merges presets (`apply <preset>`) preserving credentials; displays connection health/logs. |
| `xorg` | `[rollback\|PATH]` | Generates `/etc/X11/xorg.conf` using `cvt` modelines derived from profiles (`mac`, `mac15`, `fallback`), dynamically detecting GPU driver (`nvidia`, `amdgpu`, `intel`, or `modesetting`). If `rollback` is chosen, restores from backups. |
| `session` | `start [profile]`, `stop`, `status` | Starts/stops remote sessions. Saves display/effects/screensaver configurations to `~/.config/remote-studio/session.state` and restores them on stop. |
| `update` | None | Performs git pull and re-runs `install.sh install`. |
| `watch` | `[interval]` | A blocking connection watcher that polls `ss` to detect RustDesk users and runs `res session start` / `stop` automatically. |
| `rotate` | `[normal\|left\|right\|inverted]` | Rotates the primary display screen using `xrandr`. |
| `profiles` | None | Lists all built-in and user-defined profiles. |
| `config` | `show`, `get KEY`, `set KEY VALUE` | Views or updates configuration keys in `~/.config/remote-studio/remote-studio.conf`. |
| `version` | None | Outputs the current control plane version (`9.0`). |
| `help`, `-h`, `--help` | None | Prints the CLI help utility. |
| **Shortcuts** | `speed`, `theme`, `night`, `caf`, `privacy`, `clip`, `service`, `audio`, `keys`, `fix`, `reset` | Triggers specific core actions (e.g., toggling screensaver, resetting screen mode, clearing clipboard). |

### 1.2 Status File Path & Text/JSON Conventions
- **Path Resolution**: 
  - If `$XDG_RUNTIME_DIR` is set and writable, the status directory is `$XDG_RUNTIME_DIR/remote-studio`.
  - Otherwise, it falls back to `/tmp/remote-studio-$UID`.
  - The status file is written to `$STATUS_DIR/status`.
- **Pipe-Delimited Conventions**:
  ```
  Mode | Temp | Ping | Users | RAM | WarningCount | WarningMsg | NetSpeed | IP | ConnType | Resolution | DirectAddress | Codec
  ```
  *Example*:
  ```
  mac | 55.0°C | 12ms | 1 | 24.5% | 0 | OK | ↓24KB/s ↑5KB/s | 100.64.12.34/192.168.1.10 | Direct | 2560x1664 | 100.64.12.34:21118 | H264
  ```
- **JSON Output Schema**:
  ```json
  {
    "mode": "mac",
    "temperature": "55.0°C",
    "latency": "12ms",
    "users": 1,
    "ram": "24.5%",
    "warnings": {
      "count": 0,
      "summary": "OK"
    },
    "network": "↓24KB/s ↑5KB/s",
    "ip": "100.64.12.34/192.168.1.10",
    "connection": "Direct",
    "resolution": "2560x1664",
    "direct_address": "100.64.12.34:21118",
    "codec": "H264",
    "status_file": "/run/user/1000/remote-studio/status"
  }
  ```

### 1.3 D-Bus Service Details
- **Bus Name**: `org.remote_studio.Daemon`
- **Object Path**: `/org/remote_studio/Daemon`
- **Interface Name**: `org.remote_studio.Daemon`
- **Properties**:
  - `Status` (type `s`, read-only): Returns the serialized JSON string of the system status. In addition to the keys provided by `res status --json`, it injects `"active_ips"`.
    *Example value*:
    ```json
    {
      "mode": "mac",
      "temperature": "55.0°C",
      ...
      "codec": "H264",
      "status_file": "/run/user/1000/remote-studio/status",
      "active_ips": ["100.64.12.34"]
    }
    ```
- **Methods**:
  - `Refresh()` -> returns `void`: Triggers an immediate network poll and updates/broadcasts the status.
- **Signals**:
  - `StatusChanged(status: s)`: Dispatched when connection status or active users change. The `status` argument contains the JSON status payload containing `"active_ips"`.

### 1.4 Web Servers & Polling Logic
- **WebSocket Server**:
  - Listens on port `9998`, binding to `0.0.0.0`.
  - On client connection: immediately sends a JSON payload with structure `{"type": "status_full", "data": <status_json>}`.
  - Active client connections are stored in memory. When status changes are detected, a broadcast is sent to all connected sockets.
  - Receives action messages from clients:
    - Command invocation: `{"action": "command", "cmd": "..."}` -> triggers execution of `res <cmd>`.
    - Interface scaling: `{"action": "scale", "val": ...}` -> sets Cinnamon desktop interface text-scaling factor.
- **HTTP Server**:
  - Listens on port `9999`, binding to `0.0.0.0`.
  - Serves static dashboard files from the `web` folder.
- **Network Polling & Auto-Session Management**:
  - Periodically polls every **5 seconds**.
  - Polls active TCP connections using `ss -tnp` filtering for established connections by `rustdesk`. Parses remote IP addresses.
  - Transition **0 -> >0 users**:
    - Validates if the connecting IP is trusted (resolves in `127.0.0.1` or matching Tailscale peers from `tailscale status --json`).
    - Resolves the peer OS (`macOS`, `iOS`, `windows`, `linux`).
    - If trusted: sets status to Active, emits D-Bus signal, and triggers auto-session setup if enabled (determines profile based on OS: `macOS` -> `mac`, `iOS` -> `ipad`, others -> `fallback`). Runs `res session start <profile>`.
  - Transition **>0 -> 0 users**:
    - Resets status to Idle, emits D-Bus signal, and stops the active session (`res session stop`).
  - Standard status files are refreshed at the end of each polling cycle.

---

## 2. Proposed Go Architecture Design

We propose a clean, modular Go framework to unify the CLI commands, daemon loop, D-Bus interfaces, and WebSockets.

### 2.1 Package & Folder Structure
Following the `PROJECT.md` conventions, the codebase will be laid out as follows:

```
├── cmd/
│   └── res/
│       └── main.go           # Application entrypoint & CLI command routing
├── pkg/
│   ├── cli/                  # Individual CLI subcommands (info, doctor, rustdesk, etc.)
│   ├── daemon/               # DBus, WebSocket, HTTP server, and polling loops
│   ├── config/               # Profile loading (.conf) and startup config parsing
│   ├── display/              # Display abstraction (X11 & gnome-randr Wayland wrappers)
│   └── services/             # Client wrappers for Tailscale and RustDesk
├── go.mod
└── go.sum
```

### 2.2 Core Modules & Packages

#### `pkg/config`
Responsible for loading configurations (`remote-studio.conf` and profile definitions `profiles.conf`).
- Loads `DEFAULT_PROFILE`, `DEFAULT_SESSION_PROFILE`, `DEFAULT_RUSTDESK_PRESET`, and `AUTO_SESSION`.
- Parses profiles using the pipe-delimited format: `label|width|height|scaling|text_scale|cursor` and returns a structured profile registry map.

#### `pkg/display`
Defines an interface for display backend drivers:
```go
type DisplayBackend interface {
    GetActiveDisplay() (string, error)
    ApplyMode(output string, width, height int, scale float64, textScale float64, cursor int, label string) error
    RotateDisplay(output string, direction string) error
    GetRotation(output string) (string, error)
    ToggleNightShift() (bool, error) // Night-shift or gamma adjustments
}
```
Provides `X11Backend` (wrapping `xrandr`, `xgamma`, `xrdb`, `xclip`) and `WaylandBackend` (wrapping `gnome-randr`, `gsettings`, `wl-copy`).

#### `pkg/services`
Encapsulates host-level daemon integrations:
- **Tailscale**: Wraps CLI command `tailscale status --json` to fetch connection lists, retrieve peer OS details, and check the network health.
- **RustDesk**: Manages safe configuration mergers for INI/TOML options (preserving keys, IDs, passwords, and salt) and restarts the background service.

#### `pkg/daemon`
Implements the core long-running process:
- **D-Bus Server**: Implemented using the `github.com/godbus/dbus/v5` package. Binds to `org.remote_studio.Daemon`, responds to properties/methods, and fires the `StatusChanged` signal.
- **WebSocket Server**: Uses `github.com/gorilla/websocket`. Maintains a map of active client connections, handles inbound control commands, and dispatches JSON updates.
- **HTTP Server**: Serves files in the `web` folder using Go's `http.FileServer`.
- **Poll Loop**: Runs an asynchronous loop with a `time.Ticker` set to 5 seconds. Triggers the transition events and calls CLI functions natively rather than spawning subprocesses.

#### `pkg/cli`
Implements subcommand actions (`Doctor()`, `Info()`, `Session()`, etc.) as clean Go functions, sharing modules and types with the daemon to avoid code duplication.

---

## 3. Key Implementation Details in Go

### 3.1 D-Bus Service Integration
Using `github.com/godbus/dbus/v5`, we will register our object path and export the interface:

```go
package daemon

import (
	"encoding/json"
	"fmt"
	"github.com/godbus/dbus/v5"
)

const (
	BusName    = "org.remote_studio.Daemon"
	ObjectPath = "/org/remote_studio/Daemon"
	Interface  = "org.remote_studio.Daemon"
)

type DBusDaemon struct {
	conn *dbus.Conn
}

// Exported D-Bus method
func (d *DBusDaemon) Refresh() *dbus.Error {
	// Trigger polling logic
	TriggerPoll()
	return nil
}

// GetProperty handles D-Bus property reads
func (d *DBusDaemon) GetProperty(name string) (dbus.Variant, error) {
	if name == Interface+".Status" {
		statusJSON := GetStatusJSON() // Returns JSON string containing active_ips
		return dbus.MakeVariant(statusJSON), nil
	}
	return dbus.Variant{}, fmt.Errorf("property not found")
}

// EmitStatusChanged broadcasts DBus status signals
func (d *DBusDaemon) EmitStatusChanged(statusStr string) error {
	return d.conn.Emit(ObjectPath, Interface+".StatusChanged", statusStr)
}
```

### 3.2 WebSocket Status Schema Integration
On client connect and status changes, a message will be broadcast:
```json
{
  "type": "status_full",
  "data": {
    "mode": "mac",
    "temperature": "55.0°C",
    "latency": "12ms",
    "users": 1,
    "ram": "24.5%",
    "warnings": {
      "count": 0,
      "summary": "OK"
    },
    "network": "↓24KB/s ↑5KB/s",
    "ip": "100.64.12.34/192.168.1.10",
    "connection": "Direct",
    "resolution": "2560x1664",
    "direct_address": "100.64.12.34:21118",
    "codec": "H264",
    "status_file": "/run/user/1000/remote-studio/status"
  }
}
```

### 3.3 Dynamic Network Polling & Transitions
Inside the polling ticker:
```go
func StartPolling(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            ips, err := GetRustDeskConnections()
            if err != nil {
                log.Printf("Error checking connections: %v", err)
                continue
            }
            
            currUsers := len(ips)
            if currUsers > 0 && prevUsers == 0 {
                // Connection transition: Idle -> Active
                HandleSessionConnect(ips)
            } else if currUsers == 0 && prevUsers > 0 {
                // Connection transition: Active -> Idle
                HandleSessionDisconnect()
            }
            prevUsers = currUsers
            
            // Refresh local status file and broadcast ws
            WriteStatusFile()
            BroadcastStatusToWebsockets()
            
        case <-ctx.Done():
            return
        }
    }
}
```

---

## 4. Verification and Packaging Strategy

1. **Unit Testing**:
   Write native Go testing suites under `pkg/*` packages (e.g. `config_test.go`, `display_test.go`) mocking shell utilities and D-Bus interfaces.
2. **E2E Compatibility**:
   Ensure `res daemon` and CLI output conform to the exact string patterns and exit codes expected by existing BATS tests in the `tests/` directory.

# Design Proposal: Go Rewrite of Remote Studio Control Plane

## 1. Legacy Architecture Summary
The Remote Studio control plane consists of two major parts:
1.  **CLI (`res.sh` and files in `lib/`)**:
    *   `res.sh` acts as the entry point, resolving configurations and invoking commands.
    *   `lib/core.sh` implements basic initialization, display detection, and status management.
    *   `lib/config.sh` parses profile files (by sourcing them or parsing key-value pairs).
    *   `lib/diagnostics.sh` contains the `doctor` checks.
    *   `lib/services.sh` manages session processes (such as Xorg, Wayland, or the WM/Xsession).
    *   `lib/tui.sh` provides visual status reporting.
    *   `lib/virtual_display.sh` launches Virtual Display servers.
    *   Command state is persisted in a JSON status file under `/var/run/remote-studio/status.json` or falls back to `/tmp/remote-studio/status.json`.
2.  **Daemon (`daemon/remote_studio_daemon.py`)**:
    *   Exposes a D-Bus interface `org.remote_studio.Daemon` at path `/org/remote_studio/Daemon`.
    *   Methods: `StartSession()`, `StopSession()`, `GetStatus()`, `GetConfig()`, `SetConfig()`.
    *   Signals: Broadcasts `StatusChanged` with a JSON payload of status.
    *   Web Services:
        *   An HTTP dashboard server running on port 9999, serving static dashboard UI (usually from `web/dist/`).
        *   A WebSocket telemetry server running on port 9998, pushing live CPU, memory, display status, and active session telemetry.
    *   Periodic Task: Network polling (pinging 8.8.8.8) to determine connection status.

## 2. Go Architecture Design
We propose a single unified Go binary `res` that supports CLI commands and a background daemon service (`res daemon`).

### A. Directory Structure
```
remote-studio/
├── go.mod
├── go.sum
├── cmd/
│   ├── root.go       (Main CLI registry)
│   ├── status.go     (res status command)
│   ├── info.go       (res info command)
│   ├── log.go        (res log command)
│   ├── doctor.go     (res doctor command)
│   ├── session.go    (res session command)
│   ├── rotate.go     (res rotate command)
│   ├── profiles.go   (res profiles command)
│   ├── config.go     (res config command)
│   └── daemon.go     (res daemon command - D-Bus/WS/HTTP)
├── pkg/
│   ├── config/       (Configuration parsing & Profile utilities)
│   ├── daemon/       (D-Bus registry, WebSocket server, HTTP server, network polling)
│   ├── diagnostics/  (Doctor diagnostics rules)
│   ├── session/      (Session launcher, process tracking, display management)
│   └── status/       (Status file writing, JSON structure matching legacy)
└── main.go           (Entry point)
```

### B. Configuration & Legacy Status Conventions
*   Config directory is searched at `$HOME/.config/remote-studio/` or `/etc/remote-studio/`.
*   Profiles are stored in `profiles.conf` or individual profile files.
*   Status file writing must output JSON matching:
    ```json
    {
      "session_active": false,
      "session_pid": 0,
      "display": ":99",
      "profile": "default",
      "network_status": "connected",
      "cpu_usage": 0.0,
      "memory_usage": 0.0,
      "last_updated": "2026-06-15T14:18:00Z"
    }
    ```
*   Path conventions: First try `/var/run/remote-studio/status.json` (writable by daemon), fall back to `/tmp/remote-studio/status.json`.

### C. D-Bus Service Design
*   Use `github.com/godbus/dbus/v5`.
*   Object Path: `/org/remote_studio/Daemon`
*   Interface: `org.remote_studio.Daemon`
*   Methods:
    *   `StartSession(profile string) (bool, error)`
    *   `StopSession() (bool, error)`
    *   `GetStatus() (string, error)` (Returns JSON status string)
*   Signal:
    *   Name: `StatusChanged`
    *   Type: Broadcasts a single string parameter containing the exact JSON schema of status.

### D. WebSocket and HTTP Servers
*   WebSocket Server: Port 9998. Exposes `/ws`. Periodically sends telemetry every 2 seconds.
*   HTTP Server: Port 9999. Serves static dashboard files from `web/dist/`.
*   Network Polling: Pings 8.8.8.8 every 10 seconds or queries network interfaces.

### E. CLI Interface (`res`)
Use Cobra (`github.com/spf13/cobra`) to parse CLI subcommands:
*   `res status [-j|--json]`: Displays formatting/JSON of active session.
*   `res info`: Prints environment specs.
*   `res log`: Prints logs from `/var/log/remote-studio.log`.
*   `res doctor`: Run checkups on display servers and configuration files.
*   `res session [start|stop|restart|attach]`: Interacts with D-Bus daemon or launches processes locally.
*   `res profiles [list|set]`: Manages user profiles.
*   `res config [get|set]`: Handles key-value configuration values.
*   `res daemon`: Starts the background services.

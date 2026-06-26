# Project: Remote Studio Modernization

## Architecture
The modernized Remote Studio will replace the legacy Python daemon (`daemon/remote_studio_daemon.py`) and Bash CLI control plane (`res.sh` and `lib/*.sh`) with a single statically-linked Go binary (`res`).

The Go binary serves two modes:
1. **CLI Mode**: Recreates all `res.sh` subcommands (`status`, `info`, `log`, `doctor`, `session`, `rotate`, `profiles`, `config`, etc.).
2. **Daemon Mode**: Run via `res daemon`. It runs the DBus session daemon, WebSocket server (port 9998), and HTTP server (port 9999) serving the web dashboard. It listens for network connections (RustDesk/Tailscale) and automatically manages display sessions based on client OS, while exporting status to a status file (`$XDG_RUNTIME_DIR/remote-studio/status` or `/tmp/remote-studio-$UID/status`).

```
               +----------------------------------+
               |           Cinnamon Applet        |
               +----------------------------------+
                 /                              \
     Reads status file                       Invokes CLI
                 v                              v
+-------------------------------------------------------------+
|                        Go Binary (res)                      |
|                                                             |
|  +------------------+  +-----------------+  +------------+  |
|  |   DBus Server    |  |  WebSocket Srv  |  |  HTTP Srv  |  |
|  | (StatusChanged)  |  |   (Port 9998)   |  | (Port 9999)|  |
|  +------------------+  +-----------------+  +------------+  |
|          |                      |                 |         |
|          +-----------+----------+-----------------+         |
|                      |                                      |
|                      v                                      |
|            Core Integration Engines                         |
|   - Wayland / X11 Session Managers                          |
|   - Systemd Sockets / Services                              |
|   - PipeWire Audio Engine                                   |
|   - uinput KVM Input Engine                                 |
|   - VA-API / NVENC Hardware Encoder Checks                 |
|   - TOML Declarative Parser                                 |
+-------------------------------------------------------------+
```

## Code Layout
- `go.mod`, `go.sum`: Go module configuration.
- `cmd/res/main.go`: Main entry point for the Go binary. Handles subcommand routing and runs the daemon.
- `pkg/cli/`: Go implementations of all legacy CLI actions.
- `pkg/daemon/`: Go implementation of the core daemon loop, D-Bus service, WebSockets, and HTTP server.
- `pkg/config/`: TOML configuration parser (`BurntSushi/toml`).
- `pkg/display/`: X11 and Wayland session/display managers.
- `pkg/audio/`: PipeWire virtual audio sink controller.
- `pkg/input/`: Kernel `uinput` virtual KVM implementation.
- `pkg/video/`: VA-API/NVENC dynamic hardware encoding checks.
- `pkg/systemd/`: Systemd helper library (for socket activation, etc.).
- `web/`: Static dashboard assets (served by HTTP server).

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|---|---|---|---|
| 1 | E2E Testing Track | Define test architecture, write 4 tiers of opaque-box E2E tests, publish `TEST_READY.md`. | None | IN_PROGRESS (7032e882) |
| 2 | Go Rewrite Foundation (R1) | Implement single unified Go binary replacing `res.sh` CLI and `remote_studio_daemon.py` (with D-Bus, WebSockets, HTTP, status file, and basic CLI commands). | None | IN_PROGRESS (f945f57e) |
| 3 | Wayland Native Support (R2.1) | Implement Wayland session support alongside X11. | M2 | PLANNED |
| 4 | Systemd Socket Activation (R2.2) | Add systemd socket activation support to `res daemon`. | M2 | PLANNED |
| 5 | PipeWire Audio Sinks (R2.3) | Implement PipeWire virtual audio sinks. | M2 | PLANNED |
| 6 | Kernel uinput KVM (R2.4) | Implement `uinput` virtual input proxying. | M2 | PLANNED |
| 7 | VA-API/NVENC Checks (R2.5) | Dynamic hardware encoding capabilities query. | M2 | PLANNED |
| 8 | TOML Declarative Config (R2.6) | Replace legacy profiles/configs with declarative TOML parsing using `BurntSushi/toml`. | M2 | PLANNED |
| 9 | Final Integration & Test Pass | Pass 100% of E2E tests and perform Phase 2 adversarial coverage hardening. | M1, M2, M3, M4, M5, M6, M7, M8 | PLANNED |

## Interface Contracts
### Daemon ↔ Client Apps (DBus)
- **Bus Name**: `org.remote_studio.Daemon`
- **Object Path**: `/org/remote_studio/Daemon`
- **Interface**: `org.remote_studio.Daemon`
- **Properties**:
  - `Status` (read-only, type `s`): A JSON string representing the daemon/session status (identical to legacy format).
- **Methods**:
  - `Refresh()`: Triggers immediate network polling and status broadcast.
- **Signals**:
  - `StatusChanged(status: s)`: Emitted when connection status or active users change.

### WebSocket Status Schema
- **Type**: `status_full`
- **Data**: JSON object representing current status.
- **Action Commands**:
  - `{"action": "command", "cmd": "<command>"}`: Executes CLI command.
  - `{"action": "scale", "val": <float>}`: Adjusts text-scaling-factor.

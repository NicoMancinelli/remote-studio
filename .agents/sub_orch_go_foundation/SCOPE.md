# Scope: Go Rewrite Foundation (R1)

## Architecture
- **CLI Entry Point (`main.go`, `cmd/`)**: Cobra CLI definitions matching legacy command arguments and flag conventions.
- **Config & Profiles (`pkg/config/`)**: Decoupled package for reading/writing configuration and managing/listing active profiles.
- **Status Management (`pkg/status/`)**: Handles JSON serialization and file persistence under `/var/run/remote-studio/status.json` (falling back to `/tmp/remote-studio/status.json`).
- **Diagnostics (`pkg/diagnostics/`)**: Legacy system environment verification logic (equivalent to `res doctor`).
- **Session Management (`pkg/session/`)**: Local session command executors and process management/display spawning.
- **Daemon (`pkg/daemon/`)**: Serves the D-Bus interface (`org.remote_studio.Daemon`), WebSocket server (9998), HTTP server (9999), and background polling routines.

```
                  ┌───────────────┐
                  │   res CLI     │
                  └───────┬───────┘
                          │ (D-Bus / Direct File)
                          ▼
                  ┌───────────────┐
                  │  res daemon   │
                  └───────┬───────┘
                          ├──────────────────────────┐
                          ▼                          ▼
                  ┌───────────────┐          ┌───────────────┐
                  │  D-Bus Service│          │ WebSocket/HTTP│
                  └───────────────┘          └───────────────┘
```

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 1 | Setup & Foundation Modules | Initialize `go.mod`, implement `pkg/config` and `pkg/status` | None | DONE |
| 2 | CLI Commands Part 1 | Implement `cmd/root.go`, `cmd/config.go`, `cmd/profiles.go`, `cmd/info.go`, `cmd/log.go`, and `pkg/diagnostics/` (`cmd/doctor.go`) | M1 | DONE |
| 3 | CLI Status & Session Commands | Implement `cmd/status.go` (with JSON support) and `cmd/session.go` (local execution and D-Bus proxy mode) | M2 | IN_PROGRESS |
| 4 | Daemon D-Bus Service | Implement D-Bus daemon backend (`pkg/daemon/dbus.go`) supporting methods and broadcasting `StatusChanged` signal | M3 | PLANNED |
| 5 | Daemon Web Services & Polling | Implement WebSocket telemetry (9998), HTTP dashboard (9999), and background network/telemetry poll | M4 | PLANNED |
| 6 | Integration, Testing, E2E | Integrate CLI and daemon in `main.go` and `cmd/daemon.go`, verify compilation, run full `go test ./...` test suite | M5 | PLANNED |

## Interface Contracts
### `pkg/status` ↔ CLI/Daemon
- Status structure:
```go
type SessionStatus struct {
	SessionActive bool      `json:"session_active"`
	SessionPID    int       `json:"session_pid"`
	Display       string    `json:"display"`
	Profile       string    `json:"profile"`
	NetworkStatus string    `json:"network_status"`
	CPUUsage      float64   `json:"cpu_usage"`
	MemoryUsage   float64   `json:"memory_usage"`
	LastUpdated   time.Time `json:"last_updated"`
}
```
- Persistence helper: `WriteStatus(status *SessionStatus) error`, `ReadStatus() (*SessionStatus, error)`
- DBus payload format: The `StatusChanged` signal broadcasts the stringified JSON payload matching this exact schema.

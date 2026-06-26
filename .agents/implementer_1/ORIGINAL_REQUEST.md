## 2026-06-18T08:31:24-04:00
Your task is to continue the modernizing of Remote Studio by rewriting the legacy Python/Bash control plane into Go. You need to implement the remaining CLI commands and the background daemon, ensure compilation, and verify tests pass.

Here is the plan and instructions:
1. First, build and test the current codebase using `go build` and `go test ./...` to ensure it is in a clean starting state.
2. Update the root `main.go` file. Currently it is a stub. It MUST call `cmd.Execute()` (importing `remote-studio/cmd`), so that building the root directory compiles the real CLI.
3. Finish the CLI commands in the `cmd/` package:
   - `status.go`: Implement full stats gathering (mode, temperature, latency, active users, RAM, warnings, net speed, IP, connection type, active display resolution, direct address, rustdesk log codec). Supports `--json` flag matching the legacy schema exactly, writes legacy status file format to `$XDG_RUNTIME_DIR/remote-studio/status` (or fallback) and writes status.json using `pkg/status/WriteStatus` when called without flags or after updating.
   - `session.go`: Supports `res session [start [PROFILE] | stop | status]`. Implements local execution logic (creates state file, runs gsettings, xrandr, powerprofilesctl performance, speed/caffeine mode toggles) and D-Bus proxy mode (if D-Bus daemon is running on session bus, calls D-Bus interface methods instead of local execution).
   - `rotate.go`: Supports `res rotate [normal | left | right | inverted]`.
   - `actions.go` (or individual command files): Supports action subcommands `speed`, `theme`, `night`, `caf`, `privacy`, `clip`, `service`, `audio`, `keys`, `fix`, `reset`.
   - `custom.go`: Supports `res custom <width> <height> [scale]`.
   - Implement `tailnet.go`, `rustdesk.go`, `xorg.go`, `watch.go`, `update.go` by translating legacy Bash logic.
4. Implement `pkg/session/` package to hold session start/stop and action helper functions.
5. Implement `pkg/daemon/` package and the `daemon.go` command:
   - Expose the D-Bus interface `org.remote_studio.Daemon` on path `/org/remote_studio/Daemon` with `Status` property, `Refresh` method, and `StatusChanged` signal.
   - Implement WebSocket server on port 9998 to handle `/ws` connection, broadcast status telemetry every 2 seconds, and execute incoming commands.
   - Implement HTTP server on port 9999 to serve web dashboard static assets from `web/dist` (or `web` directory).
   - Implement background network/telemetry polling and auto-session start/stop.
6. Verify your implementation by running `go build` and `go test ./...` (including E2E tests). Ensure they all compile and pass cleanly.

MANDATORY INTEGRITY WARNING: DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Please report back when complete with your build and test results.

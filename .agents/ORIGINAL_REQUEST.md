# Original User Request

## Initial Request — 2026-06-15T14:16:29Z

Modernize the Remote Studio project by completely rewriting the Python/Bash backend into a unified Go binary, and then implementing 6 massive OS integrations (Wayland, Systemd Sockets, PipeWire, Virtual KVM, VA-API Encoding, TOML config).

Working directory: /Users/nico/Library/Mobile Documents/com~apple~CloudDocs/Dev/Code/Github/Remote-Studio
Integrity mode: demo

## Requirements

### R1. Unified Go Rewrite (Foundation)
Rewrite the entire `res.sh` CLI and `remote_studio_daemon.py` into a single, statically-linked Go binary. The Go binary must flawlessly recreate the existing DBus broadcasting logic, the WebSocket server, and the CLI commands. This MUST be completed first to establish a solid foundation.

### R2. Core OS Enhancements
Build the 6 remaining tracks on top of the new Go foundation: 
- Wayland native support alongside X11.
- Systemd Socket Activation for the Daemon (zero idle resources).
- PipeWire virtual audio sinks isolated from physical speakers.
- Kernel-level `uinput` virtual KVM for perfect remote input proxying.
- VA-API/NVENC dynamic hardware encoding checks.
- Declarative TOML configuration parsing.

### R3. Safe Implementation
Since the agent team will be interacting with low-level Linux APIs (`uinput`, systemd, pipewire), they should rely on established open-source Go libraries (e.g. `godbus/dbus`, `BurntSushi/toml`) where possible to ensure robust core logic.

## Acceptance Criteria

### Testing & Validation
- [ ] `go test ./...` passes for all core logic (especially TOML parsing and DBus mocking).
- [ ] The Go daemon successfully broadcasts the `StatusChanged` DBus signal containing the identical JSON schema as the previous Python daemon.
- [ ] A `.socket` and `.service` systemd unit file pair is provided and correctly triggers the Go daemon on network activity.

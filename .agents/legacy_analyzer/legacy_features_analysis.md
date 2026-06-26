# Remote Studio Feature Analysis Report

This report presents a comprehensive investigation of the legacy Remote Studio codebase (`res.sh`, `lib/*.sh`, `daemon/remote_studio_daemon.py`, `daemon/ebpf_tracker.py`, and related configurations) to identify and document every distinct feature that the modernized Go-based system must support.

---

## Feature 1: Command-Line Interface (CLI) Control Plane & Subcommand Router

### 1. Description
A unified CLI entry point that routes subcommand execution, validates inputs, reads user configuration/profiles, and performs operations. It supports interactive TUI panels and text-only menus when invoked without arguments.

### 2. Legacy Code Locations
- **`res.sh` (Lines 102-174)**: Core command-line parser and dispatch router.
- **`lib/config.sh` (Lines 94-116)**: Help menu definition (`show_help`).
- **`lib/tui.sh` (Lines 610-640)**: Fallback text menu (`show_text_menu`).

### 3. CLI Arguments & Inputs
- **Commands**:
  - `res custom <width> <height> [scale]`
  - `res status [--json]`
  - `res info`
  - `res log [lines]`
  - `res doctor`
  - `res doctor-fix`
  - `res self-test`
  - `res init`
  - `res tailnet [peer <name> | doctor | hosts | exit-node]`
  - `res rustdesk [apply <preset> | backup | diff <preset> | status | log [lines]]`
  - `res xorg [rollback | <path>]`
  - `res session [start [PROFILE] | stop | status]`
  - `res update`
  - `res watch [interval_sec]`
  - `res rotate [normal | left | right | inverted]`
  - `res profiles`
  - `res config [show | get KEY | set KEY VALUE]`
  - `res version`
  - `res <profile_key>` (loads profile directly from registry, e.g., `res mac`)
  - `res speed|theme|night|caf|privacy|clip|service|audio|keys|fix|reset` (runs toggle actions)
- **TUI Invocation**: Executing `res` without arguments launches the interactive `whiptail` dashboard or fallbacks to a terminal-drawn text menu if `whiptail` is missing or the terminal size is smaller than 15 lines or 60 columns.

### 4. Intended Behaviors & Parameters
- **Behavior**: Resolves execution root directories, loads startup configuration files (`~/.config/remote-studio/remote-studio.conf`), parses profile registers, and dispatches to the corresponding backend engine or TUI screen.
- **CLI Outputs**: Outputs human-readable logs to `stdout`/`stderr` or logs to `~/.remote_studio.log`. The `status` subcommand outputs a structured text line or serializes to JSON.

### 5. Potential Edge Cases & System Integrations
- **Wayland vs. X11 Environment**: CLI commands changing display mode must check the session display variable (`DISPLAY` or `WAYLAND_DISPLAY`) and redirect execution to the correct backend driver.
- **Configuration Parsing**: Legacy key-value parser is custom and prone to spacing issues. Go migration must replace this with a declarative TOML parser using `BurntSushi/toml`.

---

## Feature 2: Display Configuration & Custom Resolution Generator

### 1. Description
Dynamically registers new display resolutions (modelines) at runtime and manages Cinnamon desktop interface properties (UI scaling factor, text-scaling factor, cursor size, and X11 DPI) to match incoming client display specifications.

### 2. Legacy Code Locations
- **`lib/engine.sh` (Lines 4-72)**: Dynamic resolution calculation and Cinnamon UI scaling setup (`apply_all`, `apply_profile`).
- **`lib/backend_x11.sh` (Lines 18-48)**: X11 mode generation via `cvt` and application via `xrandr` (`backend_apply_custom_mode`).
- **`lib/backend_wayland.sh` (Lines 30-52)**: Wayland mode registration via `gnome-randr` (`backend_apply_native_mode`).

### 3. Protocols & Status Formats
- **State File (`~/.res_state`)**: Stores the active resolution parameters.
  - *Format*: `width height scaling text_scale cursor 'Label'` (e.g., `2560 1664 1 1.5 48 'MacBook Air 13'`).
- **X11 DPI Resource**: Set using `xrdb -merge`.
  - *Format*: `Xft.dpi: <dpi_value>` (calculated as `96 * scaling`).

### 4. Intended Behaviors & Parameters
- **Parameters**: `width` (int), `height` (int), `scaling` (float), `text_scale` (float), `cursor` (int), `label` (string).
- **Execution Flow**:
  1. Finds the first connected active display output.
  2. Enumerates and purges inactive duplicate custom modes of the same name or resolution.
  3. Under X11: Calls `cvt <width> <height> 60` to get modeline parameters, runs `xrandr --newmode` and `xrandr --addmode`, then switches the output using `xrandr --output <output> --mode <mode_name>`.
  4. Configures Cinnamon settings daemon via `gsettings`.
  5. Computes DPI and updates X11 resource database via `xrdb`.
  6. Writes state parameters to `~/.res_state` and appends an event to the log file.

### 5. Potential Edge Cases & System Integrations
- **Wayland Integration**: Wayland (specifically Mutter/Muffin) does not support dynamic custom modelines on-the-fly via CLI. The Wayland backend must fall back to the closest available native screen resolution using `gnome-randr`.
- **System Integration (Cinnamon Settings)**: Interacts with `org.cinnamon.desktop.interface` gsettings keys:
  - `scaling-factor` (uint32)
  - `text-scaling-factor` (double)
  - `cursor-size` (int32)
- **Headless X11 Buffer limits**: Cannot apply resolutions larger than the X11 virtual buffer declared in `xorg.conf` (default is `3840 2160`).

---

## Feature 3: Session Lifecycle & Environment Toggles Manager

### 1. Description
Manages remote session start and stop sequences. Automatically captures pre-session state, activates speed/caffeine optimization toggles, raises system power efficiency profiles, and safely rolls back all changes upon client disconnect.

### 2. Legacy Code Locations
- **`lib/engine.sh` (Lines 74-149)**: Comfort toggles executor (`do_action`) and session start/stop sequence handlers (`session_start`, `session_stop`).
- **`config/xsessionrc`**: Executed during desktop session startup to restore the last active display state from `~/.res_state`.

### 3. CLI Arguments & State Formats
- **CLI Commands**:
  - `res session start [PROFILE]`
  - `res session stop`
  - `res session status`
- **Session State File (`~/.config/remote-studio/session.state`)**:
  - *Format*: Key-value text format:
    ```ini
    started_at=YYYY-MM-DD HH:MM:SS
    profile=profile_name
    speed=ON|OFF
    caffeine=ON|OFF
    state=width height scaling text_scale cursor 'Label'
    ```

### 4. Intended Behaviors & Parameters
- **Session Start**:
  - Backs up the current wallpaper and display parameters.
  - Forces **Speed Mode** (disables desktop effects/animations/wallpaper for latency reduction).
  - Forces **Caffeine** (disables screensaver locks to avoid connection drops).
  - Configures CPU governor/power profiles to `performance` using `powerprofilesctl`.
- **Session Stop**:
  - Reverts display settings to the pre-session state.
  - Reverts Speed Mode and Caffeine toggles if they were originally disabled.
  - Reverts CPU power profiles to `balanced`.
- **Action Toggles**:
  - `speed`: Toggles Cinnamon `desktop-effects` and `enable-animations`. Sets background picture-options to `"none"` and primary-color to `"#000000"`.
  - `theme`: Toggles theme between `"Mint-Y"` and `"Mint-Y-Dark"`.
  - `night`: Sets warm color temperature (X11 `xgamma` RGB shift, Wayland `night-light-enabled` gsettings).
  - `caf`: Setsscreensaver lock-enabled.
  - `privacy`: Blank monitor (`xset dpms force off`) and locks screensaver.
  - `clip`: Flushes X11/Wayland selections.
  - `audio`: Restarts PulseAudio daemon.
  - `keys`: Resets map to `"us"`.
  - `fix`: Combination of `clip` + `audio` + `keys`.

### 5. Potential Edge Cases & System Integrations
- **PipeWire Integration**: Instead of legacy `pulseaudio -k`, the modernized system must integrate with PipeWire (via `wpctl` or `pw-cli`) to reset audio servers.
- **Power Management Integration**: Integrates with systemd's `powerprofilesctl` service.
- **Display Blanking Integration**: Uses DPMS controls (`xset dpms force off`). Under Wayland, this must use screen power control protocols supported by the compositor.

---

## Feature 4: Autonomous Connection Watcher (Autopilot Engine)

### 1. Description
An autonomous background watcher that polls network sockets or intercepts socket events to identify incoming RustDesk connections. Automatically authenticates connection requests using Tailscale peer definitions, detects peer OS, and starts/stops optimized display sessions dynamically.

### 2. Legacy Code Locations
- **`lib/engine.sh` (Lines 151-173)**: Connection polling loop (`show_watch`).
- **`daemon/remote_studio_daemon.py` (Lines 78-151)**: Watcher connection listener, Tailscale OS parser, and autopilot trigger (`poll_network`).
- **`daemon/ebpf_tracker.py`**: Experimental eBPF-based zero-latency connection tracker.

### 3. CLI Arguments & Protocols
- **CLI Commands**:
  - `res watch [interval_sec]`
- **Socket Polling**: Scans socket tables using `ss -tnp` to find active TCP connections matching `rustdesk` on default port `21118`.
- **eBPF Perf Buffer Event**: Intercepts `tcp_v4_connect` and outputs events when destination port matches `RUSTDESK_PORT`.

### 4. Intended Behaviors & Parameters
- **Authentication & OS Resolution**:
  1. Finds the remote IP of the connection.
  2. If the IP is `127.0.0.1`, it is automatically trusted.
  3. Otherwise, queries Tailscale status via `tailscale status --json`. Checks if the IP is in any peer's `TailscaleIPs` registry.
  4. If the peer matches, extracts its operating system (`OS` field).
  5. If untrusted, logs a warning and ignores the connection.
- **Session Auto-Launch**:
  - Mapped Profile selection:
    - `peer_os == "iOS"` -> profile: `ipad`
    - `peer_os == "macOS"` -> profile: `mac`
    - `peer_os == "windows" | "linux"` -> profile: `fallback`
  - Launches `res session start <profile>` when a trusted connection is established and no session is active.
  - Launches `res session stop` when the user count drops back to `0`.

### 5. Potential Edge Cases & System Integrations
- **Tailscale Integration**: Strictly depends on local `tailscale` daemon socket access and `tailscale status --json` output structure.
- **eBPF / Kernel Integration**: eBPF tracker requires root execution (`CAP_BPF` and `CAP_TRACING` capabilities) and hooks kernel call `tcp_v4_connect`. If kernel headers or BCC compiler toolchains are missing, it must fallback to socket table polling (`ss` parsing).
- **Systemd Integration**: Daemon is managed via systemd user unit `remote-studio.service`.
- **Legacy Bug to Avoid**: The legacy daemon contained a bug where `GLib.timeout_add_seconds` was invoked with `self.poll_interval * 1000` (5000 seconds / ~83 mins) instead of `self.poll_interval` (5 seconds). The modernized Go daemon must utilize correct time intervals.

---

## Feature 5: D-Bus Daemon IPC Service (`org.remote_studio.Daemon`)

### 1. Description
Exposes a session D-Bus interface that allows external system components (like taskbar panel applets or scripts) to query status, receive real-time notifications of connection changes, and trigger manual updates.

### 2. Legacy Code Locations
- **`daemon/remote_studio_daemon.py` (Lines 14-77)**: D-Bus service, methods, properties, and signals registration.

### 3. D-Bus Interface Specification
- **Well-Known Name**: `org.remote_studio.Daemon`
- **Object Path**: `/org/remote_studio/Daemon`
- **Interface**: `org.remote_studio.Daemon`
- **Properties**:
  - `Status` (read-only, type `s`): A JSON string representing the full status of Remote Studio.
- **Methods**:
  - `Refresh()`: Invokes immediate socket polling and status emission.
- **Signals**:
  - `StatusChanged(status: s)`: Emitted when active connection status changes.

### 4. JSON Status Schema
The property `Status` and the signal `StatusChanged` yield a JSON status string conforming to the following schema:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "RemoteStudioStatus",
  "type": "object",
  "properties": {
    "mode": { "type": "string" },
    "temperature": { "type": "string" },
    "latency": { "type": "string" },
    "users": { "type": "integer" },
    "ram": { "type": "string" },
    "warnings": {
      "type": "object",
      "properties": {
        "count": { "type": "integer" },
        "summary": { "type": "string" }
      },
      "required": ["count", "summary"]
    },
    "network": { "type": "string" },
    "ip": { "type": "string" },
    "connection": { "type": "string" },
    "resolution": { "type": "string" },
    "direct_address": { "type": "string" },
    "codec": { "type": "string" },
    "status_file": { "type": "string" },
    "active_ips": {
      "type": "array",
      "items": { "type": "string" }
    }
  },
  "required": [
    "mode", "temperature", "latency", "users", "ram", "warnings", 
    "network", "ip", "connection", "resolution", "direct_address", 
    "codec", "status_file", "active_ips"
  ]
}
```

### 5. Potential Edge Cases & System Integrations
- **D-Bus Session Access**: The daemon runs as a systemd user service and must have access to `DBUS_SESSION_BUS_ADDRESS`. If run in headless sessions, systemd user services might start before the D-Bus session bus is fully initialized.
- **Cinnamon Applet Integration**: The Cinnamon taskbar applet interacts with Remote Studio by reading the status file or querying D-Bus to update UI panels.

---

## Feature 6: Embedded Web Server & WebSocket Control Protocol

### 1. Description
Hosts a local HTTP server that serves the dashboard Web UI and a WebSocket server that broadcasts real-time system status and processes incoming control commands.

### 2. Legacy Code Locations
- **`daemon/remote_studio_daemon.py` (Lines 153-208)**: Threaded HTTP and WebSocket server implementations.
- **`web/src/App.jsx`**: Client-side connection setup, status parsing, and action dispatch.

### 3. Network Ports & Protocols
- **HTTP Server**: Port `9999`. Serves static files from `web/dist`.
- **WebSocket Server**: Port `9998`. Handles client-initiated command actions.
- **WebSocket JSON Messages**:
  - **Broadcast message (Server -> Client)**:
    ```json
    {
      "type": "status_full",
      "data": <status_json_object>
    }
    ```
  - **Execute command (Client -> Server)**:
    ```json
    {
      "action": "command",
      "cmd": "string"
    }
    ```
    *Supported values*: `theme`, `night`, `audio`, `ipad`, `mac`, `reset`, etc.
  - **Adjust Text Scale (Client -> Server)**:
    ```json
    {
      "action": "scale",
      "val": 1.5
    }
    ```

### 4. Intended Behaviors & Parameters
- **Connection Handshake**: Upon client connection, the server immediately sends the current system status payload.
- **Status Broadcast**: The daemon invokes a broadcast on every polling cycle or when connection changes are detected.
- **Command Dispatch**: Client commands are translated into CLI calls (`res cmd` or `gsettings` set).

### 5. Potential Edge Cases & System Integrations
- **Web UI Performance**: The dashboard UI performs constant polling over WebSockets. If the WebSocket server disconnects, it displays an "Offline" status badge.
- **Go Modernization (Systemd Socket Activation)**: The modernized Go binary will replace Python's `websockets` and `SimpleHTTPRequestHandler` with Go's `net/http` and `gorilla/websocket` libraries. It should support systemd socket activation on ports 9998/9999.

---

## Feature 7: RustDesk Configuration Preset Safe-Merger

### 1. Description
Safely merges quality, balanced, or speed template configurations into the active RustDesk TOML configuration. It preserves the host's unique cryptographic identity and security keys, and restarts the RustDesk daemon only if configuration changes are detected.

### 2. Legacy Code Locations
- **`lib/services.sh` (Lines 30-156)**: Safe-merge logic (`merge_rustdesk_config`, `merge_rustdesk_options`) and parser (`show_rustdesk`).
- **`config/RustDesk_*`**: Templates for presets (`quality`, `balanced`, `speed`, `options`).

### 3. CLI Arguments & Files
- **CLI Commands**:
  - `res rustdesk apply <preset>`
  - `res rustdesk backup`
  - `res rustdesk diff <preset>`
  - `res rustdesk status`
  - `res rustdesk log [lines]`
- **Configuration Files**:
  - Active: `$HOME/.config/rustdesk/RustDesk_default.toml`
  - Options: `$HOME/.config/rustdesk/RustDesk2.options.toml`
  - Templates: `config/RustDesk_{quality|balanced|speed}.toml` and `config/RustDesk2.options.toml`.

### 4. Intended Behaviors & Parameters
- **Safe Merge Logic**:
  - Identifies specific key values to preserve: `id`, `key`, `password`, `salt`, `relay-server`, `api-server`.
  - Parses template configuration files.
  - Writes the merged structure back to `RustDesk_default.toml`.
  - Merges options to `RustDesk2.options.toml` (overwrites completely since options have no identity fields).
  - Compares file hashes; restarts systemd unit `rustdesk.service` via sudo only if differences are found.
- **Telemetry Extraction**:
  - Reads the last 50 lines of `$HOME/.local/share/rustdesk/log/rustdesk.log` (or `$HOME/.rustdesk/log/rustdesk.log`).
  - Extracts active codec (e.g. `h264`, `hevc`), FPS, and bitrate for diagnostics.

### 5. Potential Edge Cases & System Integrations
- **TOML Parser Integration**: Legacy code uses custom awk/sed commands to merge files line-by-line. Modern Go binary must parse and write files natively using a Go TOML engine, eliminating parsing errors.
- **Systemd Privileges**: Restarting the service requires `sudo systemctl restart rustdesk`. The modernized Go binary must handle sudo privilege escalation or integrate with DBus systemd unit manager if run with sufficient privileges.

---

## Feature 8: System Health Diagnostics & Automated Integration Testing

### 1. Description
Evaluates system configuration, package dependencies, service states, symlinks, OpenGL rendering paths, and version mismatches. Offers an interactive wizard (`init`) for onboarding and an automated integration test suite (`self-test`).

### 2. Legacy Code Locations
- **`lib/diagnostics.sh` (Lines 4-203)**: Doctor checks, auto-fixes, and self-test verification (`show_doctor`, `doctor_fix`, `show_self_test`).
- **`lib/config.sh` (Lines 41-92)**: Whiptail onboarding wizard (`show_init_wizard`).
- **`lib/tui.sh` (Lines 479-507)**: Interactive diagnostics menu.

### 3. CLI Arguments
- `res doctor`
- `res doctor-fix`
- `res self-test`
- `res init`

### 4. Intended Behaviors & Parameters
- **`doctor`**: Checks dependencies (`xrandr`, `glxinfo`, etc.), checks display presence, inspects GLX renderer (flags a warning if software rendering `llvmpipe` is active), checks `rustdesk` and `tailscaled` service active states, checks log file sizes and backup counts, verifies `/usr/local/bin/res` symlinks, checks Cinnamon applet files.
- **`doctor-fix`**: Automatically links `.xsessionrc`, links Cinnamon applets, copies default RustDesk TOML if missing.
- **`self-test`**: Verification suite executing test scenarios and verifying log write capabilities.
- **`init`**: Guides users through checking dependencies, installing Tailscale, selecting the default resolution profile, and linking Cinnamon applets.

### 5. Potential Edge Cases & System Integrations
- **Software Rendering (llvmpipe)**: Remote headless sessions often lack a physical display. The NVIDIA driver requires an HDMI dummy plug or custom Xorg dummy configurations, otherwise OpenGL acceleration fails and runs on CPU software rendering (`llvmpipe`), which degrades streaming speed.
- **Update Checks**: Connects to GitHub API to check release tags. Must specify short HTTP timeouts to avoid locking up under offline environments.

---

## Feature 9: Xorg Framebuffer Configuration Generator

### 1. Description
Generates a custom `/etc/X11/xorg.conf` configuration incorporating virtual display modelines for core profiles, automatically detecting the GPU hardware driver to support headless hardware-accelerated remote displays.

### 2. Legacy Code Locations
- **`lib/engine.sh` (Lines 184-239)**: Xorg configuration writer (`generate_xorg`, `rollback_xorg`).
- **`lib/virtual_display.sh`**: Dummy display creator (`generate_dummy_conf`, `start_virtual_display`, `stop_virtual_display`).
- **`install.sh` (Lines 42-80)**: System installer and backup/rollback manager.

### 3. CLI Arguments & Formats
- **CLI Commands**:
  - `res xorg [path]`
  - `res xorg rollback`
  - `./install.sh system` (generates and writes configuration to `/etc/X11/xorg.conf`)
- **Backups**: Keeps rotating backups in `$HOME/.config/remote-studio/backups/` (capped at 10 directories).

### 4. Intended Behaviors & Parameters
- **GPU Driver Probe**: Probes system PCI devices via `lspci`. Matches GPU vendors:
  - NVIDIA -> `nvidia`
  - AMD -> `amdgpu`
  - Intel -> `intel`
  - Default -> `modesetting`
- **Configuration Output**: Generates an Xorg configuration defining:
  - Modeline definitions for `mac`, `mac15`, and `fallback` resolutions.
  - Device layout with the resolved driver. (Forces `ConnectedMonitor "DFP"` option if NVIDIA is active to enable OpenGL acceleration on headless dummy plugs).
  - Virtual desktop area capped at `3840x2160`.

### 5. Potential Edge Cases & System Integrations
- **Privilege Requirements**: Installing or updating `/etc/X11/xorg.conf` requires superuser (`sudo`) access.
- **Backup Pruning**: Rotating backup logic must prune entries when count exceeds 10 to prevent disk bloating.
- **LightDM/Display Manager Restart**: Applying a new Xorg configuration requires restarting the display manager (`sudo systemctl restart lightdm`) or rebooting the host.

---

## Modern Integration Features (Planned Go Architecture Extensions)

The modernized Go-based architecture will extend the system's capabilities by introducing native implementations for the following integration layers:

| Target Integration | Scope & System Integration Details | Sourced Milestones |
|---|---|---|
| **Wayland Native Session Manager** | Replaces X11 specific `xrandr` calls with Mutter DBus APIs or `gnome-randr` configurations. Replaces X11 clipboard tools with `wl-clipboard` primitives. | Milestone 3 (R2.1) |
| **Systemd Socket Activation** | Configures `res daemon` to listen on systemd sockets for WebSocket (9998) and HTTP (9999), launching the Go server on-demand upon incoming traffic. | Milestone 4 (R2.2) |
| **PipeWire Audio Sinks** | Replaces legacy PulseAudio restarts with a native PipeWire virtual audio sink manager (`wpctl` or direct client binding) to route remote audio streams efficiently. | Milestone 5 (R2.3) |
| **Kernel `uinput` KVM Input Proxy** | Implements low-level keyboard and mouse event injection by writing directly to `/dev/uinput`, bypassing standard compositor-specific input constraints. | Milestone 6 (R2.4) |
| **VA-API & NVENC Codec Probing** | Dynamically inspects hardware capabilities (Intel/AMD VA-API and NVIDIA NVENC) to report video encoding capabilities for low-latency streaming. | Milestone 7 (R2.5) |
| **TOML Declarative Configuration** | Replaces profile text lists and configuration scripts with standard TOML parsing (`profiles.toml` and `remote-studio.toml`). | Milestone 8 (R2.6) |

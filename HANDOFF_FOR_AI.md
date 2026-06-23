# Remote Studio: Handoff for AI Agents

This file provides comprehensive technical context for AI agents (Gemini CLI, Claude Code, Cursor, etc.) tasked with maintaining or extending the **Remote Studio** project.

## Project Purpose
Remote Studio is a display management suite for Linux Mint (Cinnamon) that optimizes the host for high-quality, low-latency remote access from Apple devices (Mac, iPad, iPhone) using RustDesk and Tailscale. It automates Xorg mode generation, system optimizations, and UI switching.

## Leadership Notes

- Treat `res.sh` and the sourced `lib/*.sh` modules as the product control plane; keep the applet thin and non-blocking.
- Preserve two status contracts: pipe-delimited status file for the Cinnamon applet and `res status --json` for automation.
- Run `make ci` before handing work back. Use `make release-check` before tags or installer/package changes.
- Design direction: dense utility UI, compact panel labels, action groups in menus, details in tooltips or terminal panels.
- Documentation direction: keep `docs/quickstart.md` as the maintained first-run guide; `docs/quick-start.md` is only a compatibility pointer.

## Core Component Logic

### 1. The Control Plane (`res.sh` + `lib/`)
- **Primary Function:** Acts as the single source of truth for display state.
- **TUI/CLI Dual Mode:** Uses `whiptail` for an interactive dashboard but provides a clean CLI for automation.
- **State Management:** Tracks applied modes in `~/.res_state`.
- **Status Exports:** Writes pipe-delimited applet data to `$XDG_RUNTIME_DIR/remote-studio/status` or `/tmp/remote-studio-$UID/status`; `res status --json` prints a JSON snapshot for scripts.
- **Status JSON Fields:** `mode`, `temperature`, `latency`, `users`, `ram`, `warnings`, `network`, `ip`, `connection`, `resolution`, `direct_address`, `codec`, `fps`, `bitrate`, `toggles`, and `status_file`.

### 2. The Cinnamon Applet (`applet/`)
- **Integration:** Symlinked to `~/.local/share/cinnamon/applets/remote-studio@neek/`.
- **Status Updates:** Uses `Gio.FileMonitor` on the status file with a scheduled refresh fallback.
- **Interaction:** Triggers `res.sh` CLI commands for mode switches.

### 3. Profile Management (`config/profiles.conf`)
- **Format:** Simple `KEY=VALUE` pairs defining resolution, scaling, and DPI.
- **Dynamic Xorg:** `res xorg` generates a complete `/etc/X11/xorg.conf` using these profiles to create "Dummy" display outputs that match remote client resolutions exactly (e.g., Retina-matching modes for iPad Pro).

## Development Patterns to Follow

1.  **Strict Shell Hygiene:** Use `res.sh`/`lib/*.sh` for all system-level logic. Avoid duplicating `xrandr` or `gsettings` logic in other components.
2.  **Idempotency:** `install.sh` and `res.sh` commands should be safe to run multiple times.
3.  **Silent CLI:** Ensure all `res.sh` subcommands have a non-interactive mode for the applet and scripts to call.
4.  **Logging:** Log all significant state changes to `~/.remote_studio.log` using the `log_event` function.
5.  **Warnings:** The `get_warning_summary` function in `lib/core.sh` is critical for diagnostics; update it when adding new external dependencies.

## Key Files to Watch

- `res.sh`: Entrypoint, command dispatch, module loading.
- `lib/core.sh`, `lib/engine.sh`, `lib/diagnostics.sh`, `lib/services.sh`, `lib/config.sh`, `lib/tui.sh`: Product logic.
- `applet/applet.js`: Cinnamon UI and asynchronous status polling.
- `install.sh`: Manages the complex symlink environment and Xorg setup.
- `config/xorg.conf`: The template for headless virtual buffer operations.

## Common Tasks for AI

- **Adding a Profile:** Add it to `config/profiles.conf`. You may need to run `res xorg` to update the virtual buffer support.
- **New Toggle:** Implement it in `res.sh` as a CLI flag and update the TUI menu.
- **Applet UI Fixes:** Look for `-symbolic` icon conventions and ensure asynchronous calls don't block the panel.
- **Docs/Product Changes:** Update README feature coverage, the maintained quick start, and ROADMAP status together when behavior or design guidance changes.

## Troubleshooting Context
- If scaling fails: Check `gsettings get org.cinnamon.desktop.interface text-scaling-factor`.
- If mode is missing: Run `res doctor` to check if the current Xorg server was started with the generated `xorg.conf`.
- If status is stale: Check `res status`, `res status --json`, and the `status_file` path reported by JSON.
- If quick-start instructions diverge: Update `docs/quickstart.md`; leave `docs/quick-start.md` as a pointer only.

---
*Note: This file is intended for AI consumption. For human-readable docs, see README.md and REMOTE_STUDIO.md.*

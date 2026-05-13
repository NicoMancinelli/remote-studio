# Changelog

## [Unreleased]

### Added
- `applet.js`: collapsible submenu groups — **Device Presets**, **Performance & Session**,
  **RustDesk**, **System & Security** — replace the flat item list, greatly condensing the
  panel menu. Groups remember their open/closed state for the lifetime of the Cinnamon session.
- `applet.js`: "smart expand" — the Device Presets group auto-opens whenever the current mode
  matches a known profile, so the active checkmark is always immediately visible.
- `applet.js`: reads `DEFAULT_PROFILE` from `~/.config/remote-studio/remote-studio.conf` at
  menu-open time; the **Start Session** item now reflects the real default instead of hardcoding `mac`.
- `lib/core.sh`: `get_ping_cached` / `_refresh_ping_cache` — ping is now non-blocking. The
  result is cached for 30 s and refreshed in the background, eliminating the ≤1 s stall that
  `ping -W 1` caused on every `get_stats` call (dashboard render, `res status`, etc.). While
  the cache is cold the dashboard shows `…` instead of blocking.
- `lib/tui.sh`: `_tui_collect_state` helper — eliminates the 8-line duplication between
  `tui_header` and `tui_title_header`.

### Fixed
- `show_update`: changed `exit 1` → `return 1` on git pull failure — `exit` killed the parent
  shell when invoked from the TUI via `run_panel_command`.
- `applet.js` `_loadProfiles`: duplicate menu items appeared when a profile key existed in both
  the default and user `profiles.conf` files. Now deduplicates by key (last-wins, user overrides
  default), preserving insertion order.

### Changed
- `applet.js`: migrated all `Mainloop.*` calls to `GLib.*` equivalents (`GLib.timeout_add_seconds`,
  `GLib.source_remove`, `GLib.timeout_add`) — `Mainloop` is deprecated in Cinnamon 5.4+.
- `applet.js`: `_addMenuItem` / `_addTerminalItem` replaced by `_subItem` / `_subTerminal` /
  `_subSep` helpers that operate on a submenu group rather than the root menu.

## [8.1] — 2026-05-02


### Added
- Cinnamon applet: `Gio.FileMonitor` file-watch replaces 600 ms polling loop (lower CPU)
- Cinnamon applet: connection quality indicator in panel label (● Direct / ◐ Relayed)
- Cinnamon applet: Copy Direct Address menu item and notification mute/unmute toggle
- TUI profiles menu now loops — switch profiles without returning to the main menu
- TUI quick actions use `DEFAULT_PROFILE` at runtime instead of hardcoding "mac"
- TUI dashboard auto-refreshes every 15 s via `timeout`; Escape/Close still exits
- TUI diagnostics: Automated Self-Test entry wired to `show_self_test`
- TUI system menu: Watch Service submenu (status / enable / disable / journal)
- `res config set-custom`: validates key format (`^[A-Z][A-Z0-9_]*$`)
- `install.sh install`: per-step Linked/Skipped/Copied feedback
- `install.sh`: `~/.xsessionrc` safety check — skips with warning if file is not a symlink
- `res update`: shows version and commit SHA before and after pull
- `install-remote-studio.sh`: `--help` flag with usage, requirements, and post-install steps
- Watch service submenu in TUI System menu for enable/disable/status/log
- ipad13 profile (iPad Pro 13″, 2064×2752)

### Fixed
- `PROFILES["$key"]` was written as `PROFILES[key]` — the literal string "key" was used as the array index, making the PROFILES array effectively empty (all profile commands broken)
- `exit 0` at end of CLI dispatch swallowed function return codes (`res session <invalid>` always exited 0)
- `prune_backups`: pipe-into-while ran in a subshell so the counter never incremented; rewrote with `mapfile`
- `get_warning_summary`: removed `log_event` call that fired every 30 s when applet symlink was wrong, flooding the log
- `show_self_test` log_event check: was running in a `bash -c` subshell where the function isn't defined (always failed); now runs inline
- `merge_rustdesk_options`: removed pointless tmpfile hop — options.toml has no identity fields, plain `cp` is correct
- `tui_dashboard`: Escape key (exit 255) now closes instead of triggering a redraw
- `config/xsessionrc`: added `-r` to `read`, fixed unquoted variables, added `|| true` guards on xrandr/gsettings calls
- `applet.js` `PROFILES_FILE`: falls back to `/usr/share/remote-studio/profiles.conf` when `/usr/local/bin/res` is not a symlink (.deb installs)
- `Makefile` `test` target: was `shellcheck *.sh` (missed all of `lib/`); now covers `res.sh install.sh install-remote-studio.sh lib/*.sh`
- `shellcheck.yml`: pinned `action-shellcheck` to `2.0.0` (was `@master`); added `tests/**` to path triggers; removed `needs: shellcheck` from bats job

### Changed
- `res.sh` modularised into `lib/core.sh`, `lib/engine.sh`, `lib/diagnostics.sh`, `lib/services.sh`, `lib/config.sh`, `lib/tui.sh`
- DPI calculation uses `awk` instead of `bc` (removes `bc` as a runtime dependency)
- `git fetch` in `show_doctor` uses `http.lowSpeedTime=3` and `http.connectTimeout=3` to avoid hanging on offline machines
- README rewritten as a full project homepage (features, install, profiles table, CLI reference, architecture tree, configuration)

## [8.0] — current

- **Session presets**: `res session start <profile>` applies display mode, performance mode, caffeine, and power profile in one step; `res session stop` restores the prior state
- **RustDesk presets**: `balanced`, `quality`, and `low-bandwidth` codec/FPS profiles applied via `res session start`
- **Tailnet doctor**: `res tailnet doctor` summarises DNS, UDP, NAT, DERP, and direct-path status; `res tailnet peer <name>` checks direct vs DERP path to a specific device
- **RustDesk config management**: `res rustdesk apply`, `res rustdesk backup`, and `res rustdesk diff`; identity, key, password, and trusted-device fields are never overwritten
- **Profile validation**: structured error output for malformed profile lines; detects stale xrandr modes and user-override profile files
- **NVIDIA dummy plug support**: `res xorg` generates an NVIDIA-backed `Driver "nvidia"` config with `Option "ConnectedMonitor" "DFP"` instead of the software dummy driver when a GPU is present
- **Dynamic applet**: panel label shows warning count from `res status`; tooltip shows current resolution and Tailnet IP; status polling uses `$XDG_RUNTIME_DIR` instead of `/tmp/res_status`
- **Auto speed mode**: `res session start` activates speed mode automatically based on connection quality
- **Xorg rollback**: `res xorg rollback` restores `/etc/X11/xorg.conf` from the latest backup
- **Stale mode detection**: `res doctor` detects xrandr modes with matching resolutions but incorrect refresh rates
- **Packaging**: `make install`, `make doctor`, `make test`, `make release`; `res version` output; `shellcheck` and GitHub Actions CI workflow
- **User config file**: `~/.config/remote-studio/remote-studio.conf` for defaults such as preferred profile, RustDesk port, and Mac peer name
- **Dry-run support**: `install.sh system --dry-run` previews system changes without writing files

## [7.0]

- Refactored codebase: deduplicated profile handling, fixed xrandr mode bugs, cleaned up command dispatch
- Added `res help` command
- All toggle actions and mode switches logged to `~/.remote_studio.log`
- Improved applet UX: rebuilt menu on each click to reflect current mode with checkmark indicator

## [6.x and earlier]

- TUI dashboard with whiptail menus (`profiles`, `performance`, `diagnostics`, `system`, `dashboard`)
- `res doctor`: checks RustDesk, Tailscale, Xorg, profile symlinks, and active renderer
- `res status`: pipe-delimited output for applet consumption
- `res xorg`: generates Xorg dummy config modelines from `config/profiles.conf`
- Cinnamon applet with live stats panel and device preset menu
- RustDesk low-latency defaults (`config/RustDesk_default.toml`)
- Login-time display restore via `config/xsessionrc` / `~/.xsessionrc`
- `install.sh` user install, system install, backup, and uninstall targets
- Initial project directory structure committed

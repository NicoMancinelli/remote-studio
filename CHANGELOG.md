# Changelog

## [Unreleased]

### Added
- `lib/core.sh`: `get_ping_cached` / `_refresh_ping_cache` — ping is now non-blocking.
  Result is written to `$STATUS_DIR/.ping_cache` (file-based IPC — shell variables written
  in a background `&` subshell are discarded on exit). Cache TTL is 30 s; shows `…` while
  warming instead of blocking for ≤1 s.
- `lib/core.sh`: `_tui_collect_state` helper — eliminates the 8-line duplication between
  `tui_header` and `tui_title_header`.
- `lib/diagnostics.sh` `show_doctor`: exit node status row (`exit-node: none / hostname`).
  Tailscale row now also shows `BackendState` so `tailscale-starting` etc. are visible.
- `lib/core.sh` `get_warning_summary`: catches `NoState`, `Starting`, and `NoNetwork`
  backend states as `tailscale-offline` warnings (only fires when daemon is active, avoiding
  double-counting with the existing IP-missing warning).
- `applet/applet.js`: collapsible submenu groups (Device Presets, Performance & Session,
  RustDesk, System & Security). Groups remember open/closed state per Cinnamon session.
  Smart auto-expand: Device Presets opens when current mode matches a known profile.
  Reads `DEFAULT_PROFILE` from config at menu-open time. All `Mainloop.*` → `GLib.*`.

### Fixed
- `show_update`: `exit 1` → `return 1` on git pull failure (was killing the TUI shell).
- `applet.js` `_loadProfiles`: duplicate entries when a key appeared in both default and
  user profiles.conf. Now deduplicates by key (last-wins, insertion order preserved).
- `lib/core.sh` ping cache: previous implementation wrote to shell variables from a
  background subshell — bash discards these, so the cache was never populated. Fixed by
  writing to a file in `STATUS_DIR` instead.
- `lib/engine.sh` `session_stop`: replaced `awk -F"'"` label extraction with parameter
  expansion on the `rest` field from `read`. Safer when labels contain spaces; avoids a
  forked subshell per stop.

### Changed
- `lib/services.sh` `show_rustdesk status`: produces structured output (session count,
  Direct/Relayed, remote IP, local port, last codec/FPS/bitrate from log) instead of a
  raw `grep` dump.
- `applet/applet.js`: `_addMenuItem`/`_addTerminalItem` → `_subItem`/`_subTerminal`/
  `_subSep` helpers that operate on submenu groups.

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

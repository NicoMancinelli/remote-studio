# Changelog

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

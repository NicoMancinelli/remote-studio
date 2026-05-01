# Remote Studio Roadmap

## Highest Impact

- Replace Xorg dummy software rendering with a GPU-backed path.
  - Done: use a physical HDMI dummy plug on the NVIDIA GPU.
  - Done: build a reliable NVIDIA-backed virtual Xorg configuration.
  - Success signal: `res doctor` reports a non-`llvmpipe` OpenGL renderer.

- Make RustDesk config application explicit and safe.
  - Done: Add `res rustdesk apply`, `res rustdesk backup`, and `res rustdesk diff`.
  - Done: Never overwrite identity, key, password, or trusted-device fields.

- Expand session mode.
  - Done: `res session start mac` applies the Mac profile, performance mode, caffeine, and performance power profile where available.
  - Done: `res session stop` restores captured display, speed, caffeine, and balanced power profile where available.
  - Done: include RustDesk service restart policy and peer-specific Tailscale checks.

## Reliability

- Done: Add `shellcheck` and a GitHub Actions workflow for shell syntax and linting.
- Done: Add dry-run support to `install.sh system`.
- Done: Add rollback support for `/etc/X11/xorg.conf` from the latest backup.
- Done: Add structured profile validation with clear errors for malformed profile lines.
- Done: Detect stale xrandr modes with matching resolutions but bad refresh rates.
- Done: Detect whether `~/.config/remote-studio/profiles.conf` overrides built-in profiles.
- Detect whether Cinnamon loaded the applet from the expected symlink.

## Tailscale

- Done: Add `res tailnet peer <name>` to check direct vs DERP path to a specific device.
- Done: Add `res tailnet doctor` to summarize DNS, UDP, NAT, DERP, and direct-path status.
- Prefer Tailscale IPs in status output, but show LAN IP as a secondary detail.
- Generate the exact RustDesk direct address for the current host.

## RustDesk

- Add config merge logic for `RustDesk_default.toml` and `RustDesk2.toml`.
- Done: Add a command to restart RustDesk only after config changes are staged.
- Done: Detect whether the current connection is direct or relayed when RustDesk exposes enough process/socket detail.
- Done: Add session presets:
  - `balanced`: adaptive, auto codec, 60 FPS target.
  - `quality`: higher image quality for text-heavy static work.
  - `low-bandwidth`: lower resolution and more compression.

## Applet

- Done: Show current resolution and Tailnet IP in the panel tooltip.
- Add one-click `doctor`, `tailnet`, and session start/stop actions.
- Done: highlight warning count from `res status` in the panel label.
- Done: avoid writing `/tmp/res_status`; use a user runtime path such as `$XDG_RUNTIME_DIR/remote-studio/status`.
- Make applet device entries data-driven from `config/profiles.conf`.

## Configuration

- Generate `config/xorg.conf` from profiles during release or install instead of manually editing it.
- Done: Add a `remote-studio.conf` file for defaults such as preferred profile, RustDesk port, and Mac peer name.
- Split built-in profiles from user overrides more explicitly:
  - `config/profiles.conf`
  - `~/.config/remote-studio/profiles.conf`
  - `~/.config/remote-studio/local.conf`

## Packaging

- Done: Add `make install`, `make doctor`, `make test`, and `make release`.
- Add a Debian package or Mint-friendly install target.
- Done: Add version output: `res version`.
- Add changelog entries for profile/config changes.

## Documentation

- Add screenshots of the Cinnamon applet and terminal doctor output.
- Document the 13-inch vs 15-inch MacBook Air profile difference.
- Document the difference between runtime xrandr switching and persistent Xorg config.
- Document the GPU rendering limitation and recommended HDMI dummy plug setup.

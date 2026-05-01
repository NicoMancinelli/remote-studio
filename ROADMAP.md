# Remote Studio Roadmap

Items are grouped by theme and loosely ordered by impact within each section.

---

## Testing

- Add a `bats` test suite (`tests/`) covering core `res.sh` logic: profile loading, `validate_profiles`, `mode_name_for`, `get_warning_summary`, and `session_start`/`session_stop` state file round-trips.
- Extend the GitHub Actions workflow to run `bats` on every push alongside `shellcheck`.
- Add a `make test` alias that runs both `shellcheck` and `bats` locally.

---

## Auto-update & Distribution

- Add `res update` command: pulls latest from the git origin and re-runs `install.sh install` in one step.
- Add a version check to `res doctor`: compare `res version` against the latest tag on GitHub (via `curl` + `gh api`) and report if an update is available.
- Add a GitHub Actions workflow that builds `remote-studio_X.Y_all.deb` and attaches it as a release asset automatically on every version tag push.
- Publish a `curl | bash` one-liner install script (`install-remote-studio.sh`) that clones the repo and runs `install.sh install`.

---

## Incoming Connection Automation

- Detect when a new RustDesk session connects (poll `ss` for new `ESTAB` on port 21118) and auto-apply `res session start` for the default profile.
- Detect when the last session disconnects and auto-run `res session stop`.
- Make auto-session behaviour opt-in via `AUTO_SESSION=true` in `remote-studio.conf`.
- Add `res watch` command that runs the connection-detection loop in the foreground (useful for testing; background mode via systemd unit).

---

## RustDesk Visibility

- Add `res rustdesk status`: show active codec, FPS, and bitrate by parsing RustDesk logs or `/proc` socket details.
- Add `res rustdesk log [N]`: tail the RustDesk service log (replaces manual `journalctl -u rustdesk`).
- Surface codec and FPS in `res status` and the applet tooltip when a session is active.

---

## Tailscale

- Show exit node status in `res tailnet` and `res doctor` (active exit node name or "none").
- Add a warning to `get_warning_summary` when the machine has been offline from the tailnet for more than N minutes.
- Add `res tailnet hosts`: list all tailnet peers with their IPs and online/offline status (thin wrapper around `tailscale status`).

---

## Display & Profiles

- Add `res custom <WxH>` shorthand that applies a resolution without requiring a named profile, and prompts to save it to `~/.config/remote-studio/profiles.conf`.
- Support portrait/landscape rotation toggle (`xrandr --rotate`) as a profile option or standalone `res rotate` command.
- Add a `res profiles list` command that prints all loaded profiles (built-in and user) with their source file, for debugging override priority.
- Add an `ipad13` profile for the 13-inch iPad Pro (2064×2752 or landscape 2752×2064).

---

## Applet

- Show connection quality in the panel label: color the label green for Direct, amber for Relayed, grey for no session.
- Add a `Copy Direct Address` menu item that writes `IP:21118` to the clipboard.
- Add configurable notification suppression: a toggle in the applet menu to silence connect/disconnect popups.
- Replace polling-via-shell-spawn with a GLib file-watch (`Gio.FileMonitor`) on the status file to reduce CPU overhead.

---

## Packaging & Install

- Add a `make deb` GitHub Actions job so the `.deb` is always built on Linux and attached to releases (pairs with the auto-update distribution item above).
- Add an `install.sh rollback` subcommand that restores the previous symlink state from the backup directory.
- Write a `INSTALL.md` with a quick-start guide, prerequisites list, and post-install checklist.

---

## Configuration

- Add `res config set KEY VALUE` / `res config get KEY` commands to read and write `remote-studio.conf` without manually editing it.
- Add `res config show` to print the effective config (defaults merged with user overrides).
- Support a `DEFAULT_SESSION_PROFILE` key in `remote-studio.conf` so `res session start` uses it without an argument.

---

## Documentation & Screenshots

- Capture screenshots of the Cinnamon applet (panel label, open menu, tooltip) and commit them to `docs/screenshots/`.
- Capture `res doctor` terminal output and add it to `docs/screenshots/`.
- Add a `docs/` directory with a quick-start guide for new machines.

# Installing Remote Studio

## Requirements

- Linux Mint (Cinnamon) 21.x or later
- `git`, `xrandr`, `whiptail` (all pre-installed on Linux Mint)
- RustDesk service installed and running
- Tailscale installed and authenticated

## Quick Install

Recommended one-liner:

    curl -fsSL https://raw.githubusercontent.com/NicoMancinelli/remote-studio/master/install-remote-studio.sh | bash

It clones or updates the repo at `~/remote-studio`, then runs `./install.sh install`.

Manual install:

    git clone https://github.com/NicoMancinelli/remote-studio.git ~/remote-studio
    cd ~/remote-studio
    ./install.sh install

For the guided first-run workflow with screenshots, use [docs/quickstart.md](docs/quickstart.md).

## What the installer does

- Symlinks `res` to `/usr/local/bin/res`
- Symlinks the Cinnamon applet into `~/.local/share/cinnamon/applets/remote-studio@neek/`
- Symlinks `config/xsessionrc` to `~/.xsessionrc` for login-time display restore
- Copies `config/profiles.conf` to `~/.config/remote-studio/profiles.conf` if not already present
- Copies `config/remote-studio.conf.example` to `~/.config/remote-studio/remote-studio.conf` if not already present
- Copies `config/RustDesk_default.toml` to `~/.config/rustdesk/` if not already present

## System install (optional)

To write a persistent headless Xorg config (required for operation without a physical monitor):

    ./install.sh system

This generates `/etc/X11/xorg.conf` from your active profiles. Restart LightDM or reboot to apply.

Preview user or system changes without writing files:

    ./install.sh --dry-run install
    ./install.sh --dry-run system

## Post-install checklist

1. Run `res doctor` — all checks should show OK
2. Run `res status --json` and confirm it reports a `status_file` path
3. Add the `remote-studio@neek` applet to your Cinnamon panel
4. Run `res mac` (or your preferred profile) to set the initial display mode
5. Optionally run `res session start mac` before your first remote session

## Updating

    res update

Or manually:

    cd ~/remote-studio && git pull && ./install.sh install

## Uninstalling

    ./install.sh uninstall

## Backup and rollback

    ./install.sh backup
    ./install.sh rollback

Backups are kept under `~/.config/remote-studio/backups/`; rollback restores the newest available backup for user config and `/etc/X11/xorg.conf` when present.

## Debian package (Linux Mint)

Build a `.deb` package:

    make deb
    sudo dpkg -i dist/remote-studio_*.deb

Pre-built `.deb` packages are attached to each [GitHub release](https://github.com/NicoMancinelli/remote-studio/releases).

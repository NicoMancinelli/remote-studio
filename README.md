# Remote Studio

> Linux Mint Cinnamon control layer for RustDesk sessions over Tailscale — optimised for Apple devices.

[![CI](https://github.com/NicoMancinelli/remote-studio/actions/workflows/shellcheck.yml/badge.svg)](https://github.com/NicoMancinelli/remote-studio/actions/workflows/shellcheck.yml)
[![Integration](https://github.com/NicoMancinelli/remote-studio/actions/workflows/integration.yml/badge.svg)](https://github.com/NicoMancinelli/remote-studio/actions/workflows/integration.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/NicoMancinelli/remote-studio)](https://github.com/NicoMancinelli/remote-studio/releases/latest)

Remote Studio manages headless Xorg display modes, device-specific scaling profiles, a Cinnamon panel applet, and low-latency RustDesk display defaults — so your Linux machine looks right when you connect from a MacBook, iPad, or iPhone.

---

## Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Device Profiles](#device-profiles)
- [Usage](#usage)
- [Architecture](#architecture)
- [Configuration](#configuration)
- [Contributing](#contributing)
- [License](#license)

---

## Features

- **One-command profiles** — `res mac`, `res ipad`, `res iphonel` etc. set the right resolution, HiDPI scaling, text scale, and cursor size in a single call
- **Headless Xorg** — generates `/etc/X11/xorg.conf` with dummy/NVIDIA driver config so custom resolutions survive reboots
- **Session lifecycle** — `res session start` / `res session stop` apply a profile before the client connects and restore defaults when they leave
- **RustDesk presets** — `quality`, `balanced`, `speed` TOML presets merged at runtime without touching your identity or password files
- **Tailscale integration** — `res tailnet` shows your address, peer health, and direct vs relayed path detection
- **Interactive TUI** — full whiptail dashboard when run without arguments; plain CLI when called with an argument
- **Cinnamon panel applet** — live connection indicator (● Direct / ◐ Relayed), user count, warnings, and a GUI menu
- **Automatic watch loop** — `res watch` (or the included systemd user unit) detects new RustDesk connections and applies a profile automatically
- **Doctor & self-test** — `res doctor` checks symlinks, Xorg, RustDesk, Tailscale, renderer, and more; `res self-test` runs 9 automated checks
- **Debian package** — pre-built `.deb` attached to every GitHub release; build your own with `make deb`

---

## Requirements

| Dependency | Notes |
| :--- | :--- |
| Linux Mint 21.x+ (Cinnamon) | Other Debian/Ubuntu-based distros may work |
| `bash` ≥ 5, `xrandr`, `whiptail` | Pre-installed on Linux Mint |
| `gsettings` (part of `glib2`) | Pre-installed on Cinnamon |
| [RustDesk](https://rustdesk.com/) | Installed and running as a service |
| [Tailscale](https://tailscale.com/) | Authenticated on your tailnet |

Optional but recommended: an **HDMI dummy plug** in the GPU's HDMI port to activate hardware rendering on headless machines (see [GPU Setup](#gpu-setup--hdmi-dummy-plug) in REMOTE_STUDIO.md).

---

## Installation

### One-liner

```bash
curl -fsSL https://raw.githubusercontent.com/NicoMancinelli/remote-studio/master/install-remote-studio.sh | bash
```

### Manual

```bash
git clone https://github.com/NicoMancinelli/remote-studio.git ~/remote-studio
cd ~/remote-studio
./install.sh install   # symlinks res, applet, and login restore
./install.sh system    # writes /etc/X11/xorg.conf (requires sudo)
```

### Debian package

Download the pre-built `.deb` from the [latest release](https://github.com/NicoMancinelli/remote-studio/releases/latest), or build it yourself:

```bash
make deb                             # requires dpkg-deb (Linux only)
sudo dpkg -i dist/remote-studio_*.deb
```

### Post-install

```bash
res doctor   # all checks should show OK
res mac      # apply your first profile
```

Then add the **`remote-studio@neek`** applet to your Cinnamon panel.

See [INSTALL.md](INSTALL.md) for the full checklist.

---

## Device Profiles

Built-in profiles are defined in [`config/profiles.conf`](config/profiles.conf):

| Command | Device | Resolution | Scaling | Text Scale |
| :--- | :--- | :--- | :--- | :--- |
| `res mac` | MacBook Air 13" | 2560×1664 | 1× | 1.5 |
| `res mac15` | MacBook Air 15" | 2880×1864 | 1× | 1.5 |
| `res ipad` | iPad Pro 11" | 2424×1664 | 2× | 1.1 |
| `res ipad13` | iPad Pro 13" | 2064×2752 | 2× | 1.1 |
| `res iphonel` | iPhone Landscape | 2868×1320 | 2× | 1.2 |
| `res iphonep` | iPhone Portrait | 1320×2868 | 2× | 1.2 |
| `res fallback` | Fallback 1920×1200 | 1920×1200 | 1× | 1.1 |

Add your own in `~/.config/remote-studio/profiles.conf` using the same `key=Label|width|height|scale|text_scale|cursor` format, or interactively via `res custom <width> <height>`.

---

## Usage

### CLI quick-reference

```bash
# Profiles
res mac                   # Apply MacBook Air 13" profile
res ipad                  # Apply iPad Pro 11" profile
res custom 1920 1080      # Apply arbitrary resolution (prompts to save)

# Session lifecycle
res session start mac     # Apply profile + prep for incoming connection
res session stop          # Restore pre-session state

# RustDesk
res rustdesk apply quality    # Merge quality TOML preset
res rustdesk apply balanced   # Merge balanced TOML preset
res rustdesk apply speed      # Merge speed TOML preset

# Network
res tailnet               # Show Tailscale IP + RustDesk direct address
res tailnet doctor        # Check tailnet health

# Xorg
res xorg                  # Generate /etc/X11/xorg.conf from profiles
res xorg rollback         # Restore previous xorg.conf

# Diagnostics
res doctor                # Full system health check
res self-test             # Automated self-test (9 checks)
res status                # Pipe-delimited stats (consumed by applet)
res log [N]               # Tail last N lines of the event log (default 20)

# Toggles
res speed                 # Toggle performance mode (strips wallpaper/animations)
res theme                 # Toggle OLED dark/light theme
res night                 # Toggle night shift (warm gamma)
res caf                   # Toggle caffeine (disable screen lock)
res privacy               # Lock screen and blank monitor
res fix                   # Fix clipboard + audio + keyboard in one shot
res service               # Restart RustDesk service

# Config
res config set KEY VALUE  # Write a key to remote-studio.conf
res config get KEY        # Read a key
res config show           # Print effective config (defaults + overrides)

# Misc
res watch [interval]      # Auto-apply profile on new RustDesk connections
res rotate [normal|left|right|inverted]
res update                # Pull latest and re-run install
res version               # Print version
res help                  # Full command reference
```

### Interactive TUI

Run `res` with no arguments to open the whiptail dashboard:

```
┌─ Remote Studio v8.0 ──────────────────────────────────┐
│ Mode: MacBook Air 13 (2560x1664) | IP: 100.x.x.x     │
├───────────────────────────────────────────────────────┤
│  profiles     Display Profiles                        │
│  quick        Quick Actions                           │
│  performance  Session & Toggles                       │
│  diagnostics  Diagnostics                             │
│  tailnet      Tailscale Network                       │
│  system       System & Tools                          │
│  dashboard    Live Dashboard                          │
│  help         Help                                    │
└───────────────────────────────────────────────────────┘
```

Falls back to a plain numbered text menu when the terminal is too small for whiptail.

---

## Architecture

```
remote-studio/
├── res.sh                        # Entrypoint — CLI dispatch + TUI main loop
├── lib/
│   ├── core.sh                   # Colours, logging, caching, profile helpers
│   ├── engine.sh                 # apply_all, apply_profile, session, xorg
│   ├── diagnostics.sh            # doctor, self-test, info, status, log
│   ├── services.sh               # tailnet, rustdesk config merge
│   ├── config.sh                 # res config, init wizard, help, update
│   └── tui.sh                    # All whiptail TUI panels and menus
├── applet/
│   ├── applet.js                 # Cinnamon panel applet (GJS)
│   └── metadata.json
├── config/
│   ├── profiles.conf             # Built-in device profiles
│   ├── xorg.conf                 # Headless Xorg dummy config template
│   ├── RustDesk_default.toml     # Default RustDesk display settings
│   ├── RustDesk_balanced.toml
│   ├── RustDesk_quality.toml
│   ├── RustDesk_speed.toml
│   ├── logrotate.d/remote-studio
│   └── remote-studio-watch.service   # Systemd user unit for watch loop
├── tests/
│   ├── test_profiles.bats
│   ├── test_config.bats
│   └── test_log.bats
├── package/
│   └── build-deb.sh
├── install.sh                    # Symlink + system installer
├── install-remote-studio.sh      # curl-pipe-bash one-liner
├── Makefile
└── docs/
    └── quick-start.md
```

`res.sh` sources the `lib/` modules at startup, resolving `LIB_DIR` from the repo's own `lib/` directory (development) or `/usr/share/remote-studio/lib` (`.deb` install). The applet reads `/tmp/remote-studio/status` (written every 10 seconds by `res status`) and uses `Gio.FileMonitor` to react instantly to changes.

---

## Configuration

User overrides live in `~/.config/remote-studio/`:

| File | Purpose |
| :--- | :--- |
| `remote-studio.conf` | Key-value config (`DEFAULT_PROFILE`, `AUTO_SESSION`, etc.) |
| `profiles.conf` | Custom device profiles (appended to built-ins) |
| `session.state` | Active session snapshot (managed by `res session`) |
| `recent_profiles` | Last 5 used profiles (shown at top of TUI profiles menu) |

```bash
res config set DEFAULT_PROFILE mac      # Set default profile
res config set AUTO_SESSION true        # Auto-apply profile on connection
res config show                         # View full effective config
```

Runtime state in `$HOME`:

| File | Purpose |
| :--- | :--- |
| `~/.res_state` | Last applied mode (read by applet and login restore) |
| `~/.remote_studio.log` | Event log (auto-rotated at 1 MB; logrotate weekly) |
| `~/.wallpaper_backup` | Saved wallpaper URI when speed mode is active |

---

## Contributing

Bug reports and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, code style, and the PR process.

Run the test suite locally:

```bash
make test    # shellcheck + bats
```

See [RELEASING.md](RELEASING.md) for the release workflow. Security issues — please read [SECURITY.md](SECURITY.md) first.

---

## License

MIT © 2026 Nico Mancinelli. See [LICENSE](LICENSE).

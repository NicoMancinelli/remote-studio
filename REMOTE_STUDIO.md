# Remote Studio Project

A remote display management suite for Linux Mint (Cinnamon), optimized for Apple device connections (MacBook Air, iPad Pro, iPhone) via RustDesk or RDP.

## Architecture

Three components working in sync:

1.  **Core Engine & TUI (`res.sh`)** - Bash script handling xrandr modes, gsettings scaling, diagnostics, Xorg config generation, and system toggles. Interactive TUI (whiptail) when run directly, silent CLI with arguments.

2.  **Cinnamon Applet (`applet/`)** - JavaScript panel applet providing a taskbar dashboard with live stats, device preset indicators, and a GUI menu. Polls status asynchronously via `/tmp/res_status`. Symlinked into `~/.local/share/cinnamon/applets/remote-studio@neek/`.

3.  **Profiles (`config/profiles.conf` + `~/.config/remote-studio/profiles.conf`)** - Device definitions used by runtime switching, login restore, generated Xorg config, and applet state.
4.  **X11 Configuration (`res xorg` -> `/etc/X11/xorg.conf`)** - Dummy driver with a `3840x2160` virtual buffer and generated modelines for the core profiles.
5.  **RustDesk Defaults (`config/RustDesk_default.toml`)** - Balanced display defaults for lower-latency RustDesk over Tailscale.

## Project Layout

```
~/dev/remote-studio/
    res.sh              # Main engine, TUI, and CLI
    REMOTE_STUDIO.md    # This file
    .gitignore
    applet/
        applet.js       # Cinnamon panel applet
        metadata.json   # Applet metadata
    config/
        xsessionrc      # Display restore on login
        profiles.conf   # Built-in device profiles
        xorg.conf       # Headless Xorg dummy display config
        RustDesk_default.toml
        RustDesk2.options.toml
    install.sh          # Symlink and optional system config installer
```

## Symlinks

| System Path | Target |
| :--- | :--- |
| `/usr/local/bin/res` | `res.sh` |
| `~/.local/share/cinnamon/applets/remote-studio@neek/*` | `applet/*` |
| `~/.xsessionrc` | `config/xsessionrc` |

## Install

```
./install.sh install
./install.sh system
./install.sh doctor
./install.sh backup
./install.sh uninstall
```

The default install links `res`, the Cinnamon applet, and the login restore script. The `system` install generates Xorg config from the active profiles, backs up `/etc/X11/xorg.conf`, and replaces it; restart LightDM or reboot for that file to be loaded.

For RustDesk over Tailscale, copy the safe defaults from `config/RustDesk_default.toml` into `~/.config/rustdesk/RustDesk_default.toml`, then set `local-ip-addr` in RustDesk's options to this host's Tailscale IPv4 address. Do not commit real RustDesk key, password, or trusted-device files.

## Runtime State (in `$HOME`)

| File | Role |
| :--- | :--- |
| `~/.res_state` | Last applied resolution and scaling profile |
| `~/.remote_studio.log` | Event log for mode switches and toggle actions |
| `~/.wallpaper_backup` | Saved wallpaper URI when Speed Mode is active |

## CLI Reference

```
res help              Show all commands
res mac               MacBook Air 13 2560x1664 (1x)
res ipad              iPad Pro 2424x1664 (3:2, 2x)
res iphonel           iPhone Landscape 2868x1320 (19.5:9, 2x)
res iphonep           iPhone Portrait 1320x2868 (9:19.5, 2x)
res speed             Toggle performance mode (animations/wallpaper)
res theme             Toggle OLED dark/light theme
res night             Toggle night shift (warm gamma)
res caf               Toggle caffeine (disable screen lock)
res privacy           Lock screen + blank monitor
res fix               Fix clipboard + audio + keyboard
res clip              Flush clipboard only
res audio             Restart PulseAudio only
res keys              Reset keyboard layout (US)
res service           Restart RustDesk service (sudo)
res reset             Reset to 1024x768
res doctor            Check RustDesk, Tailscale, Xorg, profiles, and symlinks
res tailnet           Show this host's Tailscale IP and RustDesk direct address
res xorg [PATH]       Generate Xorg dummy config from profiles
res session start mac Start an optimized RustDesk session
res session stop      Restore state captured before session start
res status            Pipe-delimited stats for applet
```

## TUI Dashboard

Run `res` with no arguments to open the dashboard. The TUI is organized around the operational workflow:

| Menu | Purpose |
| :--- | :--- |
| `profiles` | Apply a saved device profile or enter a custom resolution |
| `performance` | Toggle speed mode, caffeine, theme, night shift, and quick repairs |
| `diagnostics` | Run doctor, tailnet, xrandr, OpenGL, service, and log views |
| `system` | Restart RustDesk, preview/write generated Xorg config, install links, backup configs |
| `dashboard` | Show a scrollable summary of current mode, services, renderer, toggles, and Tailnet address |

## Developer Notes

*   Device profiles are loaded from `config/profiles.conf`, then overridden by `~/.config/remote-studio/profiles.conf`.
*   `res xorg` generates Xorg modelines from the same profile definitions used by `res mac` and the applet.
*   `res doctor` is the first place to check drift between symlinks, RustDesk, Tailscale, Xorg, and the active renderer.
*   `res status` returns: `Mode | Temp | Ping | Users | RAM | WarningCount | WarningText | Traffic | IP`
*   Use `-symbolic` icons in the applet, emojis in the TUI.
*   The applet rebuilds its menu on click to reflect current mode (checkmark indicator).
*   The TUI falls back to a plain text menu when the terminal is too small for whiptail.
*   All toggle actions and mode switches are logged to `~/.remote_studio.log`.

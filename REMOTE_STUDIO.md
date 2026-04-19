# Remote Studio Project

A remote display management suite for Linux Mint (Cinnamon), optimized for Apple device connections (MacBook Air, iPad Pro, iPhone) via RustDesk or RDP.

## Architecture

Three components working in sync:

1.  **Core Engine & TUI (`res.sh`)** - Bash script handling xrandr modes, gsettings scaling, and system toggles. Interactive TUI (whiptail) when run directly, silent CLI with arguments.

2.  **Cinnamon Applet (`applet/`)** - JavaScript panel applet providing a taskbar dashboard with live stats, device preset indicators, and a GUI menu. Polls status asynchronously via `/tmp/res_status`. Symlinked into `~/.local/share/cinnamon/applets/remote-studio@neek/`.

3.  **X11 Configuration (`config/xorg.conf` -> `/etc/X11/xorg.conf`)** - Dummy driver with a `3840x2160` virtual buffer and presets for 13-inch MacBook Air, 15-inch MacBook Air, and 1920x1200 fallback modes.
4.  **RustDesk Defaults (`config/RustDesk_default.toml`)** - Balanced display defaults for lower-latency RustDesk over Tailscale.

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
./install.sh
./install.sh --system
```

The default install links `res`, the Cinnamon applet, and the login restore script. The `--system` install also backs up and replaces `/etc/X11/xorg.conf`; restart LightDM or reboot for that file to be loaded.

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
res status            Pipe-delimited stats for applet
```

## Developer Notes

*   Device profiles are defined once in the `PROFILES` associative array. Add new devices there.
*   `res status` returns: `Mode | Temp | Ping | Users | RAM | Alerts | Traffic | IP`
*   Use `-symbolic` icons in the applet, emojis in the TUI.
*   The applet rebuilds its menu on click to reflect current mode (checkmark indicator).
*   The TUI falls back to a plain text menu when the terminal is too small for whiptail.
*   All toggle actions and mode switches are logged to `~/.remote_studio.log`.

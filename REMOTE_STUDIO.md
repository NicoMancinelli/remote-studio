# Remote Studio Project

A remote display management suite for Linux Mint (Cinnamon), optimized for Apple device connections (MacBook Air, iPad Pro, iPhone) via RustDesk or RDP.

## Architecture

Three components working in sync:

1.  **Core Engine & TUI (`res.sh`)** - Bash script handling xrandr modes, gsettings scaling, and system toggles. Interactive TUI (whiptail) when run directly, silent CLI with arguments.

2.  **Cinnamon Applet (`applet/`)** - JavaScript panel applet providing a taskbar dashboard with live stats, device preset indicators, and a GUI menu. Polls status asynchronously via `/tmp/res_status`. Symlinked into `~/.local/share/cinnamon/applets/remote-studio@neek/`.

3.  **X11 Configuration (`/etc/X11/xorg.conf`)** - Dummy driver with `Virtual 3840 2160` (4K buffer) for headless high-res virtual screens.

## Project Layout

```
~/projects/remote-studio/
    res.sh              # Main engine, TUI, and CLI
    REMOTE_STUDIO.md    # This file
    .gitignore
    applet/
        applet.js       # Cinnamon panel applet
        metadata.json   # Applet metadata
    config/
        xsessionrc      # Display restore on login
```

## Symlinks

| System Path | Target |
| :--- | :--- |
| `/usr/local/bin/res` | `res.sh` |
| `~/.local/share/cinnamon/applets/remote-studio@neek/*` | `applet/*` |
| `~/.xsessionrc` | `config/xsessionrc` |

## Runtime State (in `$HOME`)

| File | Role |
| :--- | :--- |
| `~/.res_state` | Last applied resolution and scaling profile |
| `~/.remote_studio.log` | Event log for mode switches and toggle actions |
| `~/.wallpaper_backup` | Saved wallpaper URI when Speed Mode is active |

## CLI Reference

```
res help              Show all commands
res mac               MacBook Air 2880x1800 (16:10, 1x)
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

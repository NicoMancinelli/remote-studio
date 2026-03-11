# Remote Studio Project

**Purpose:** A comprehensive remote management and display optimization suite for Linux Mint (Cinnamon), specifically tailored for high-performance connections from Apple devices (MacBook Air, iPad Pro, iPhone) via RustDesk or RDP.

## Architecture

The project consists of three primary components that work in sync:

1.  **Core Engine & TUI (`res.sh`)**:
    *   A versatile Bash script that handles all `xrandr` logic and `gsettings` scaling.
    *   **Modes**: Operates as an interactive TUI (Terminal User Interface) when run directly, or as a silent CLI when passed arguments (e.g., `res mac`).
    *   **Capabilities**: Real-time monitoring of CPU temp, RAM, Disk, Latency, and Network Traffic.

2.  **Cinnamon Applet (`applet/`)**:
    *   A native JavaScript panel applet (`applet.js`) that provides a taskbar dashboard.
    *   **Features**: Displays live stats in the panel and provides a GUI menu for all `res.sh` functions.
    *   **Async Logic**: Fetches status updates asynchronously via `/tmp/res_status` to prevent panel freezes.
    *   Symlinked into `~/.local/share/cinnamon/applets/remote-studio@neek/` for Cinnamon.

3.  **X11 Configuration (`/etc/X11/xorg.conf`)**:
    *   Configured with the `dummy` driver to support headless high-resolution virtual screens.
    *   **Critical Setting**: `Virtual 3840 2160` (4K buffer) allows the system to support the high pixel counts of Retina devices.

## Project Layout

```
~/projects/remote-studio/
    res.sh                  # Main engine, TUI, and CLI
    REMOTE_STUDIO.md        # This file
    applet/
        applet.js           # Cinnamon panel applet (symlinked to applet dir)
        metadata.json       # Applet metadata (symlinked to applet dir)
    config/
        xsessionrc          # Display restore on login (symlinked to ~/.xsessionrc)
```

## Symlinks

| System Path | Points To |
| :--- | :--- |
| `/usr/local/bin/res` | `res.sh` (global CLI access) |
| `~/.local/share/cinnamon/applets/remote-studio@neek/applet.js` | `applet/applet.js` |
| `~/.local/share/cinnamon/applets/remote-studio@neek/metadata.json` | `applet/metadata.json` |
| `~/.xsessionrc` | `config/xsessionrc` |

## Runtime State Files (in `$HOME`)

| File | Role |
| :--- | :--- |
| `~/.res_state` | Last applied resolution and scaling profile |
| `~/.remote_studio.log` | Event log for mode switches |
| `~/.remote_studio_env` | Scaling variable exports (sourced in .bashrc) |
| `~/.wallpaper_backup` | Saved wallpaper path when Speed Mode is active |

## Key Features

*   **Retina Optimization**: Pre-configured profiles for MacBook Air (16:10), iPad Pro (3:2), and iPhone (19.5:9).
*   **HiDPI Scaling**: Automatically toggles between `scaling-factor 1` and `2` with precise `text-scaling-factor` adjustments.
*   **Speed Mode**: One-click bandwidth saver (Disables animations/effects, sets solid black wallpaper).
*   **Security**:
    *   **Privacy Shield**: Locks the local session and blanks the physical monitor.
    *   **Intruder Alerts**: Native notifications when new users connect to RustDesk.
    *   **Session Audit**: Logs and displays recent remote IP connections.
*   **Maintenance**: Instant fixes for Clipboard desync, Audio engine crashes, and "Ghost" keys.

## Developer Notes

*   **Execution**: Use `res [arg]` (available system-wide via `/usr/local/bin/res` symlink).
*   **Status Format**: `res status` returns a pipe-delimited string: `Mode | Temp | Ping | Users | RAM | Alerts | Traffic | IP`.
*   **UI Standard**: Always use symbolic icons (`-symbolic`) for the applet and standard emojis for the TUI to maintain uniformity.
*   **Window Logic**: The TUI includes a fallback text menu for small terminal windows (e.g., iPhone SSH).

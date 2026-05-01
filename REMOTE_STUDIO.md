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

## Device Profiles

Profiles are defined in `config/profiles.conf` with the format `key=label|width|height|scaling|text_scale|cursor`.

| Key | Device | Resolution | Scaling | Text Scale |
| :--- | :--- | :--- | :--- | :--- |
| `mac` | MacBook Air 13" | 2560×1664 | 1x | 1.5 |
| `mac15` | MacBook Air 15" | 2880×1864 | 1x | 1.5 |
| `ipad` | iPad Pro 11" | 2424×1664 | 2x | 1.1 |
| `iphonel` | iPhone Landscape | 2868×1320 | 2x | 1.2 |
| `iphonep` | iPhone Portrait | 1320×2868 | 2x | 1.2 |

**Why `mac` and `mac15` use 1x scaling with text-scale 1.5 instead of 2x HiDPI:**
The dummy display runs at the native Retina panel resolution (2560×1664 or 2880×1864). At 2x scaling, Cinnamon would halve the logical resolution to 1280×832, which defeats the purpose of a high-resolution virtual display. Running at 1x scaling preserves the full pixel count while `gsettings text-scaling-factor 1.5` compensates for legibility — it scales fonts and UI chrome without shrinking the logical canvas. The result is equivalent to native HiDPI on a real Retina display.

**Why iPad and iPhone profiles use 2x scaling:**
Those devices have smaller physical screens. The higher pixel density helps readability when the RustDesk stream fills a compact display. The lower logical resolution (half in each dimension) also reduces the bandwidth cost for the remote stream.

**The `mac15` profile** is 320 pixels wider and 200 pixels taller than `mac`. Use it when connecting from a 15-inch MacBook Air to avoid black bars or slight scaling artifacts on the receiving end.

## How Display Modes Work

Remote Studio has two distinct mechanisms for setting display modes, with different persistence characteristics.

**Runtime switching with `xrandr` (`res mac`, `res ipad`, etc.)**

Each profile command calls `xrandr --newmode` and `xrandr --addmode` at runtime, then `xrandr --output ... --mode ...` to activate the mode. Changes take effect immediately with no restart required, but are not persistent — they reset when the X session ends (logout, reboot, or LightDM restart). This is the normal operating path for switching between client devices mid-session.

**Persistent Xorg config (`res xorg` / `install.sh system`)**

`res xorg` generates `/etc/X11/xorg.conf` (or a target path you specify) from the active profile definitions. This file is read by Xorg at startup, so the declared modelines and virtual framebuffer size are available from the moment the display server starts. It is required for headless operation: without it, X11 will not know about your custom resolutions on boot and may fall back to a minimal VGA mode.

Run `res xorg` and then `install.sh system` (or `sudo cp` the output file to `/etc/X11/xorg.conf`) in these situations:

- Setting up a fresh machine for the first time
- After adding or changing profiles in `config/profiles.conf`
- When `res doctor` reports display or modeline issues after a reboot

`install.sh system` automatically backs up the existing `/etc/X11/xorg.conf` before writing. To undo, run `res xorg rollback`.

After writing a new `xorg.conf`, restart LightDM (`sudo systemctl restart lightdm`) or reboot for the change to take effect.

## GPU Setup & HDMI Dummy Plug

By default, a headless Linux machine has no physical monitor attached. Xorg will start, but the NVIDIA driver will not activate hardware acceleration without a display output. In this state, OpenGL rendering falls back to Mesa's `llvmpipe` software rasteriser.

**Detection:**

```
res doctor
```

A software-rendering system reports:

```
renderer: WARN (software-rendering)
```

The full renderer string (visible in the diagnostics TUI or `glxinfo`) will contain `llvmpipe`.

**Fix: HDMI dummy plug**

Insert an HDMI dummy plug (a passive resistor-based adapter that emulates a connected monitor) into the NVIDIA GPU's HDMI port. The driver detects the plug as a connected DFP (digital flat panel) and activates the hardware rendering path.

**Xorg configuration for the dummy plug:**

`res xorg` detects the NVIDIA driver and generates a config that declares the dummy framebuffer as a DFP:

```
Section "Device"
    Identifier "Configured Video Device"
    Driver "nvidia"
    Option "ConnectedMonitor" "DFP"
EndSection
```

Without `ConnectedMonitor "DFP"`, the NVIDIA driver may still refuse to render even with the plug present. The default `config/xorg.conf` in this repo uses `Driver "dummy"` (software), which is the safe fallback for systems without an NVIDIA GPU or a physical display — replace it with the NVIDIA version after confirming the plug is seated.

**Full procedure for a new machine:**

1. Insert the HDMI dummy plug into the NVIDIA GPU.
2. Run `make release` (or `res xorg` followed by `install.sh system`) to regenerate and install `xorg.conf` with the NVIDIA driver config.
3. Restart LightDM: `sudo systemctl restart lightdm`.
4. Reconnect via RustDesk and run `res doctor` — the renderer line should report a non-`llvmpipe` string (e.g. `NVIDIA GeForce ...`).

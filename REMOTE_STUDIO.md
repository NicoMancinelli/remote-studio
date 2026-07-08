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
remote-studio/
    res.sh              # Entrypoint: CLI dispatch + TUI main loop
    lib/
        core.sh         # Colors, logging, profile loading, state, caching
        engine.sh       # Display engine, sessions, actions, xorg generation
        diagnostics.sh  # Doctor, self-test, info, status, log
        services.sh     # Tailscale and RustDesk service helpers
        config.sh       # Config get/set, init wizard, help, update
        tui.sh          # All whiptail TUI panels and menus
    applet/
        applet.js       # Cinnamon panel applet (GJS)
        metadata.json   # Applet metadata
    config/
        profiles.conf           # Built-in device profiles (7 entries)
        xorg.conf               # Static dummy driver Xorg config
        xsessionrc              # Login-time display restore script
        RustDesk_default.toml   # Balanced RustDesk preset
        RustDesk_balanced.toml  # Balanced preset (alias)
        RustDesk_quality.toml   # High quality preset
        RustDesk_speed.toml     # Low bandwidth preset
        RustDesk2.options.toml  # RustDesk options (no identity)
        remote-studio.service  # Systemd user unit
        logrotate.d/remote-studio    # System logrotate config
    tests/
        test_profiles.bats      # Profile format and CLI tests
        test_config.bats        # Config loading and session tests
        test_log.bats           # Log subcommand tests
        helpers/
            mock_commands.bash  # Mock stubs for display/network tools
    package/
        build-deb.sh    # Debian package builder
    install.sh          # Installer (install, system, doctor, backup, rollback, uninstall)
    install-remote-studio.sh  # curl-pipe-bash one-liner installer
    Makefile            # install, doctor, test, release, deb targets
    profiles.conf.example     # User profile template
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

For the full command list, run `res help`. Common subcommands:

| Command | Purpose |
| :--- | :--- |
| `res <profile>` | Apply a built-in profile (`mac`, `mac15`, `ipad`, `ipad13`, `iphonel`, `iphonep`, `fallback`, …) |
| `res custom <W> <H> [scale]` | Apply an arbitrary resolution; offers to save as a profile |
| `res speed` / `theme` / `night` / `caf` / `privacy` | Performance and comfort toggles |
| `res doctor` | Diagnose symlinks, Xorg, RustDesk, Tailscale, renderer, and logs |
| `res session start [profile] \| stop \| status` | Capture/restore display + toggle state |
| `res watch [interval]` | Foreground connection watcher (RustDesk ESTAB polling) |
| `res status` / `res status --json` / `res log [N]` | Pipe-delimited applet status, JSON for automation, log tail |
| `res rustdesk apply <preset>` | Merge RustDesk quality/balanced/speed TOML presets while preserving identity |
| `res tailnet [hosts\|peer NAME\|doctor]` | Tailscale status, peer listing, network diagnostics |

See `res help` for the complete surface.

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
*   `res status` returns: `Mode | Temp | Ping | Users | RAM | WarningCount | WarningText | Traffic | IP | ConnType | Resolution | DirectAddress`
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
| `ipad13` | iPad Pro 13" | 2064×2752 | 2x | 1.1 |
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

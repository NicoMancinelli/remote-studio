# Remote Studio Quick Start

## What you need

- Linux Mint (Cinnamon) with RustDesk and Tailscale installed and running
- A Mac, iPad, or iPhone to connect from

## Install

    git clone https://github.com/NicoMancinelli/remote-studio.git ~/remote-studio
    cd ~/remote-studio
    ./install.sh install
    res doctor

All checks in `res doctor` should show OK before proceeding.

## Set up a headless display (no physical monitor)

If your Linux machine has no monitor attached, generate a persistent Xorg config:

    ./install.sh system

Reboot or restart LightDM. `res doctor` should then report a non-llvmpipe renderer.
See [REMOTE_STUDIO.md](../REMOTE_STUDIO.md#gpu-setup--hdmi-dummy-plug) for GPU and dummy plug details.

## Connect from a Mac

1. Open RustDesk on your Mac
2. Enter the Linux machine's Tailscale IP (shown by `res tailnet`)
3. Run `res session start mac` on the Linux machine before connecting, or enable auto-session:

       res config set AUTO_SESSION true

## Switch display profiles

| Command | Device |
|---|---|
| `res mac` | MacBook Air 13" |
| `res mac15` | MacBook Air 15" |
| `res ipad` | iPad Pro 11" |
| `res ipad13` | iPad Pro 13" |
| `res iphonel` | iPhone Landscape |
| `res iphonep` | iPhone Portrait |
| `res custom 1920 1200` | Any resolution |

## Add the panel applet

In Cinnamon: right-click the panel → Applets → search "remote-studio" → Add.
The applet shows the active profile, connection status, and warning count.

## Diagnostics

    res doctor          # full health check
    res tailnet         # show Tailscale IP and RustDesk direct address
    res tailnet doctor  # network path diagnostics
    res log             # recent event log

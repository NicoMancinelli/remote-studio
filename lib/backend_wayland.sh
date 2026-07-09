#!/usr/bin/env bash

# Remote Studio - Wayland Backend Abstraction
# Uses gnome-randr (Mutter/Muffin compatible) and wl-clipboard.

backend_get_active_display() {
    if command -v gnome-randr >/dev/null 2>&1; then
        gnome-randr 2>/dev/null | grep -B1 "current:" | grep -v "current:" | head -n 1 | awk '{print $1}'
    else
        echo "Wayland-Output"
    fi
}

backend_get_connected_outputs() {
    if command -v gnome-randr >/dev/null 2>&1; then
        gnome-randr 2>/dev/null | grep "^[A-Za-z0-9\-]* " | cut -d" " -f1
    else
        echo "Wayland-Output"
    fi
}

backend_get_current_active_mode() {
    if command -v gnome-randr >/dev/null 2>&1; then
        gnome-randr 2>/dev/null | grep "\bcurrent\b" | head -n 1 | awk '{print $1}'
    else
        echo "1920x1080"
    fi
}

backend_apply_native_mode() {
    local output="$1"
    local target_res="$2"
    if command -v gnome-randr >/dev/null 2>&1; then
        gnome-randr --output "$output" --mode "$target_res" 2>/dev/null
        return $?
    fi
    echo "Warning: gnome-randr not installed. Cannot switch resolution." >&2
    return 1
}

backend_apply_custom_mode() {
    local output="$1"
    # shellcheck disable=SC2034 # mode_name/freq are required for the cross-backend dispatcher signature (see lib/backend_x11.sh).
    local mode_name="$2"
    local width="$3"
    local height="$4"
    # shellcheck disable=SC2034 # freq is required for the cross-backend dispatcher signature (see lib/backend_x11.sh).
    local freq="${5:-60}"
    # Under Mutter Wayland, arbitrary custom modes via command line are generally unsupported
    # without patching mutter or using specific custom EDIDs.
    # We will attempt to fall back to the closest native mode.
    backend_apply_native_mode "$output" "${width}x${height}"
}

backend_rotate_display() {
    local output="$1"
    local dir="$2"
    if command -v gnome-randr >/dev/null 2>&1; then
        gnome-randr --output "$output" --rotate "$dir" 2>/dev/null
        return $?
    fi
    echo "Warning: gnome-randr not installed. Cannot rotate display." >&2
    return 1
}

backend_get_current_rotation() {
    echo "normal" # Not easily parsed from gnome-randr output simply
}

backend_set_dpi() {
    # Handled by gsettings text-scaling-factor in engine.sh
    :
}

backend_get_gamma_state() {
    local val
    val=$(gsettings get org.cinnamon.settings-daemon.plugins.color night-light-enabled 2>/dev/null || echo "false")
    [ "$val" = "true" ] && echo "ON" || echo "OFF"
}

backend_toggle_night_shift() {
    local val
    val=$(gsettings get org.cinnamon.settings-daemon.plugins.color night-light-enabled 2>/dev/null || echo "false")
    if [ "$val" = "true" ]; then
        gsettings set org.cinnamon.settings-daemon.plugins.color night-light-enabled false 2>/dev/null
        echo "OFF"
    else
        gsettings set org.cinnamon.settings-daemon.plugins.color night-light-enabled true 2>/dev/null
        echo "ON"
    fi
}

backend_clear_clipboard() {
    if command -v wl-copy >/dev/null 2>&1; then
        echo -n "" | wl-copy -p
        echo -n "" | wl-copy
    else
        echo "Warning: wl-clipboard not installed." >&2
    fi
}

backend_copy_to_clipboard() {
    if command -v wl-copy >/dev/null 2>&1; then
        echo -n "$1" | wl-copy
    fi
}

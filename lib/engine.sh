#!/bin/bash
# Remote Studio — display engine, sessions, actions, xorg

apply_all() {
    local width=$1; local height=$2; local scaling=$3; local text_scale=$4; local cursor=$5; local label=$6
    local dpi
    dpi=$(echo "96 * $scaling" | bc)
    OUTPUT=$(xrandr | grep " connected" | head -n 1 | cut -f1 -d" ")
    [ -z "$OUTPUT" ] && return 1
    MODE_NAME=$(mode_name_for "$width" "$height")

    # Remove stale modes with same name or resolution
    for m in $(xrandr | awk '{print $1}' | grep -E "^${MODE_NAME}\$|^${width}x${height}(_.*)?$"); do
        xrandr --delmode "$OUTPUT" "$m" 2>/dev/null || true
        xrandr --rmmode "$m" 2>/dev/null || true
    done

    MODE_INFO=$(cvt "$width" "$height" 60 | grep Modeline)
    MODE_PARAMS=$(echo "$MODE_INFO" | cut -d' ' -f3-)
    # shellcheck disable=SC2086
    xrandr --newmode "$MODE_NAME" $MODE_PARAMS 2>/dev/null
    xrandr --addmode "$OUTPUT" "$MODE_NAME" 2>/dev/null
    if xrandr --output "$OUTPUT" --mode "$MODE_NAME"; then
        gsettings set org.cinnamon.desktop.interface scaling-factor "$scaling" 2>/dev/null
        gsettings set org.cinnamon.desktop.interface text-scaling-factor "$text_scale" 2>/dev/null
        gsettings set org.cinnamon.desktop.interface cursor-size "$cursor" 2>/dev/null
        echo "Xft.dpi: $dpi" | xrdb -merge
        echo "$width $height $scaling $text_scale $cursor '$label'" > "$STATE_FILE"
        log_event "Mode: $label"
        return 0
    fi
}

apply_profile() {
    local profile="${PROFILES[$1]}"
    [ -z "$profile" ] && return 1
    IFS='|' read -r label width height scaling text_scale cursor <<< "$profile"
    apply_all "$width" "$height" "$scaling" "$text_scale" "$cursor" "$label"
}

do_action() {
    case "$1" in
        speed) status=$(gsettings get org.cinnamon desktop-effects)
               if [ "$status" == "true" ]; then
                   gsettings get org.cinnamon.desktop.background picture-uri > "$WALLPAPER_BACKUP" 2>/dev/null
                   gsettings set org.cinnamon desktop-effects false; gsettings set org.cinnamon.desktop.interface enable-animations false
                   gsettings set org.cinnamon.desktop.background picture-options "none"; gsettings set org.cinnamon.desktop.background primary-color "#000000"
                   log_event "Speed mode: ON"
               else
                   gsettings set org.cinnamon desktop-effects true; gsettings set org.cinnamon.desktop.interface enable-animations true
                   gsettings set org.cinnamon.desktop.background picture-options "zoom"
                   if [ -f "$WALLPAPER_BACKUP" ]; then gsettings set org.cinnamon.desktop.background picture-uri "$(cat "$WALLPAPER_BACKUP")"; rm -f "$WALLPAPER_BACKUP"; fi
                   log_event "Speed mode: OFF"
               fi ;;
        theme) cur=$(gsettings get org.cinnamon.desktop.interface gtk-theme | tr -d "'")
               if [[ "$cur" == *"Dark"* ]]; then gsettings set org.cinnamon.desktop.interface gtk-theme "Mint-Y"; log_event "Theme: Light"; else gsettings set org.cinnamon.desktop.interface gtk-theme "Mint-Y-Dark"; log_event "Theme: Dark"; fi ;;
        night) gamma=$(xgamma 2>&1 | awk '{print $4}')
               if [[ "$gamma" == "1.000" ]]; then xgamma -rgamma 1.0 -ggamma 0.8 -bgamma 0.6; log_event "Night shift: ON"; else xgamma -gamma 1.0; log_event "Night shift: OFF"; fi ;;
        caf)   cur=$(gsettings get org.cinnamon.desktop.screensaver lock-enabled)
               if [[ "$cur" == "true" ]]; then gsettings set org.cinnamon.desktop.screensaver lock-enabled false; log_event "Caffeine: ON"; else gsettings set org.cinnamon.desktop.screensaver lock-enabled true; log_event "Caffeine: OFF"; fi ;;
        privacy) cinnamon-screensaver-command -l; xset dpms force off; log_event "Privacy shield activated" ;;
        clip)  echo -n "" | xclip -selection primary; echo -n "" | xclip -selection clipboard ;;
        service) sudo systemctl restart rustdesk; log_event "RustDesk service restarted" ;;
        audio) pulseaudio -k; sleep 1; pulseaudio --start ;;
        keys)  setxkbmap us ;;
        fix)   do_action clip; do_action audio; do_action keys; log_event "Fix all: clip+audio+keys" ;;
        reset) apply_all 1024 768 1 1.0 24 "Reset" ;;
    esac
}

session_start() {
    local profile="${1:-$DEFAULT_PROFILE}"
    mkdir -p "$(dirname "$SESSION_FILE")"
    { echo "started_at=$(date '+%Y-%m-%d %H:%M:%S')"; echo "profile=$profile"; echo "speed=$(speed_state)"; echo "caffeine=$(caffeine_state)"; echo "state=$(cat "$STATE_FILE" 2>/dev/null || true)"; } > "$SESSION_FILE"
    apply_profile "$profile" || return 1
    [ "$(speed_state)" = "ON" ] || do_action speed
    [ "$(caffeine_state)" = "ON" ] || do_action caf
    if command -v powerprofilesctl >/dev/null 2>&1; then powerprofilesctl set performance 2>/dev/null || true; fi
    log_event "Session start: $profile"
}

session_stop() {
    if [ -f "$SESSION_FILE" ]; then
        previous_state=$(grep '^state=' "$SESSION_FILE" | sed 's/^state=//')
        if [ -n "$previous_state" ]; then
            echo "$previous_state" > "$STATE_FILE"
            read -r width height scaling text_scale cursor rest <<< "$previous_state"
            label=$(echo "$previous_state" | awk -F"'" '{print $2}')
            apply_all "$width" "$height" "$scaling" "$text_scale" "$cursor" "${label:-Restored}"
        fi
        grep -q '^speed=OFF$' "$SESSION_FILE" && [ "$(speed_state)" = "ON" ] && do_action speed
        grep -q '^caffeine=OFF$' "$SESSION_FILE" && [ "$(caffeine_state)" = "ON" ] && do_action caf
        rm -f "$SESSION_FILE"
    fi
    if command -v powerprofilesctl >/dev/null 2>&1; then powerprofilesctl set balanced 2>/dev/null || true; fi
    log_event "Session stop"
}

show_session() {
    case "${1:-status}" in
        start) session_start "${2:-mac}" ;;
        stop) session_stop ;;
        status) [ -f "$SESSION_FILE" ] && cat "$SESSION_FILE" || echo "No active session." ;;
        *) echo "Usage: res session start [PROFILE] | stop | status"; return 1 ;;
    esac
}

show_watch() {
    local interval=${1:-5}
    local prev_users=0
    log_event "Watch: started (interval=${interval}s)"
    trap 'log_event "Watch: stopped (signal)"; exit 0' SIGTERM SIGINT
    while true; do
        local users
        users=$(ss -tnp 2>/dev/null | grep -c -i "rustdesk.*ESTAB" || true)
        if [ "$users" -gt 0 ] && [ "$prev_users" -eq 0 ]; then
            log_event "Watch: session connected ($users user(s))"
            if [ "${AUTO_SESSION:-false}" = "true" ]; then
                session_start "$DEFAULT_PROFILE"
            fi
        elif [ "$users" -eq 0 ] && [ "$prev_users" -gt 0 ]; then
            log_event "Watch: session disconnected"
            if [ "${AUTO_SESSION:-false}" = "true" ]; then
                session_stop
            fi
        fi
        prev_users=$users
        sleep "$interval"
    done
}

show_rotate() {
    local dir="${1:-normal}"
    OUTPUT=$(xrandr | grep " connected" | head -n 1 | cut -f1 -d" ")
    [ -z "$OUTPUT" ] && { echo "No connected display."; return 1; }
    xrandr --output "$OUTPUT" --rotate "$dir"
    log_event "Rotate: $dir"
    echo "Rotated $OUTPUT to $dir"
}

generate_xorg() {
    local out="${1:-}"; local lines=(); local mode_names=()
    for key in mac mac15 fallback; do
        [ -n "${PROFILES[$key]:-}" ] || continue
        # shellcheck disable=SC2034
        IFS='|' read -r _label w h s _ts _c <<< "${PROFILES[$key]}"
        local m="${w}x${h}_60.00"
        local mi
        mi=$(cvt "$w" "$h" 60 | grep Modeline)
        local mp
        mp=$(echo "$mi" | cut -d' ' -f3-)
        lines+=("    Modeline \"$m\" $mp"); mode_names+=("\"$m\"")
    done
    {
        echo 'Section "Device"'; echo '    Identifier "Configured Video Device"'; echo '    Driver "nvidia"'; echo '    Option "ConnectedMonitor" "DFP"'; echo 'EndSection'; echo
        echo 'Section "Monitor"'; echo '    Identifier "Configured Monitor"'; printf '%s\n' "${lines[@]}"; echo '    Option "PreferredMode" "2560x1664_60.00"'; echo 'EndSection'; echo
        echo 'Section "Screen"'; echo '    Identifier "Default Screen"'; echo '    Monitor "Configured Monitor"'; echo '    Device "Configured Video Device"'; echo '    DefaultDepth 24'; echo '    SubSection "Display"'; echo '        Depth 24'; echo "        Modes ${mode_names[*]} \"1024x768\""; echo '        Virtual 3840 2160'; echo '    EndSubSection'; echo 'EndSection'
    } > "${out:-/dev/stdout}"
}

rollback_xorg() {
    local backup_root="$HOME/.config/remote-studio/backups"
    [ -d "$backup_root" ] || { echo "Error: No backup directory found at $backup_root"; return 1; }
    local latest
    latest=$(find "$backup_root" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort -r | head -n 1)
    if [ -z "$latest" ] || [ ! -f "$latest/xorg.conf" ]; then
        echo "Error: No xorg.conf found in the latest backup: $latest"
        return 1
    fi
    echo "Restoring /etc/X11/xorg.conf from $latest/xorg.conf..."
    sudo cp "$latest/xorg.conf" /etc/X11/xorg.conf
    echo "Rollback complete. Restart LightDM or reboot to apply."
}

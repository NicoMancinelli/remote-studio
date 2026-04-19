#!/bin/bash

# ==============================================================================
# RUSTDESK REMOTE STUDIO V8.0
# ==============================================================================

STATE_FILE="$HOME/.res_state"
WALLPAPER_BACKUP="$HOME/.wallpaper_backup"
LOG_FILE="$HOME/.remote_studio.log"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_PROFILES="$ROOT_DIR/config/profiles.conf"
USER_PROFILES="$HOME/.config/remote-studio/profiles.conf"

# Colors
RED='\033[1;31m'
GREEN='\033[1;32m'
CYAN='\033[1;36m'
YELLOW='\033[1;33m'
DIM='\033[2m'
NC='\033[0m'

declare -A PROFILES=()

load_profiles_file() {
    local file=$1
    [ -f "$file" ] || return 0
    while IFS='=' read -r key value; do
        [[ "$key" =~ ^[[:space:]]*# ]] && continue  # skip comments
        [[ -z "$key" || -z "$value" ]] && continue   # skip empty lines
        key=$(echo "$key" | xargs)  # trim whitespace
        PROFILES[$key]="$value"
    done < "$file"
}

load_profiles_file "$DEFAULT_PROFILES"
load_profiles_file "$USER_PROFILES"

# ------------------------------------------------------------------------------
# CORE ENGINE
# ------------------------------------------------------------------------------

log_event() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"; }

mode_name_for() {
    echo "remote-studio-${1}x${2}-60"
}

get_tailnet_ip() {
    if command -v tailscale >/dev/null 2>&1; then
        tailscale ip -4 2>/dev/null | head -n 1
    fi
}

get_primary_ip() {
    local tailnet_ip
    tailnet_ip=$(get_tailnet_ip)
    if [ -n "$tailnet_ip" ]; then
        echo "$tailnet_ip"
    else
        hostname -I | awk '{print $1}'
    fi
}

get_stats() {
    IP_ADDR=$(get_primary_ip)
    TEMP=$(sensors 2>/dev/null | grep "Package id 0" | awk '{print $4}' | tr -d '+')
    RAM=$(free -m | awk 'NR==2{printf "%.1f%%", $3*100/$2 }')
    USERS=$(ss -tnp | grep -i "rustdesk" | grep -i "ESTAB" | wc -l)
    PING_RAW=$(ping -c 1 -W 1 8.8.8.8 2>/dev/null | grep 'time=' | awk -F'time=' '{print $2}' | cut -d' ' -f1 | cut -d'.' -f1)
    [ -z "$PING_RAW" ] && PING_STAT="Offline" || PING_STAT="${PING_RAW}ms"
    [[ "${TEMP%.*}" -gt 80 ]] 2>/dev/null && THERMAL_ALERT="⚠️ " || THERMAL_ALERT=""
}

get_net_speed() {
    local IFACE=$(ip route get 8.8.8.8 | awk '{print $5}')
    local R1=$(cat /sys/class/net/$IFACE/statistics/rx_bytes); local T1=$(cat /sys/class/net/$IFACE/statistics/tx_bytes)
    sleep 0.5
    local R2=$(cat /sys/class/net/$IFACE/statistics/rx_bytes); local T2=$(cat /sys/class/net/$IFACE/statistics/tx_bytes)
    local RX=$(( ($R2 - $R1) / 512 )); local TX=$(( ($T2 - $T1) / 512 ))
    echo "↓${RX}KB/s ↑${TX}KB/s"
}

apply_all() {
    local width=$1; local height=$2; local scaling=$3; local text_scale=$4; local cursor=$5; local label=$6
    local dpi=$(echo "96 * $scaling" | bc)
    OUTPUT=$(xrandr | grep " connected" | head -n 1 | cut -f1 -d" ")
    [ -z "$OUTPUT" ] && return 1
    MODE_INFO=$(cvt "$width" "$height" 60 | grep Modeline)
    MODE_NAME=$(mode_name_for "$width" "$height")
    MODE_PARAMS=$(echo "$MODE_INFO" | cut -d' ' -f3-)
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
                   if [ -f "$WALLPAPER_BACKUP" ]; then
                       gsettings set org.cinnamon.desktop.background picture-uri "$(cat "$WALLPAPER_BACKUP")"
                       rm -f "$WALLPAPER_BACKUP"
                   fi
                   log_event "Speed mode: OFF"
               fi ;;
        theme) cur=$(gsettings get org.cinnamon.desktop.interface gtk-theme | tr -d "'")
               if [[ "$cur" == *"Dark"* ]]; then
                   gsettings set org.cinnamon.desktop.interface gtk-theme "Mint-Y"; log_event "Theme: Light"
               else
                   gsettings set org.cinnamon.desktop.interface gtk-theme "Mint-Y-Dark"; log_event "Theme: Dark"
               fi ;;
        night) gamma=$(xgamma 2>&1 | awk '{print $4}')
               if [[ "$gamma" == "1.000" ]]; then
                   xgamma -rgamma 1.0 -ggamma 0.8 -bgamma 0.6; log_event "Night shift: ON"
               else
                   xgamma -gamma 1.0; log_event "Night shift: OFF"
               fi ;;
        caf)   cur=$(gsettings get org.cinnamon.desktop.screensaver lock-enabled)
               if [[ "$cur" == "true" ]]; then
                   gsettings set org.cinnamon.desktop.screensaver lock-enabled false; log_event "Caffeine: ON"
               else
                   gsettings set org.cinnamon.desktop.screensaver lock-enabled true; log_event "Caffeine: OFF"
               fi ;;
        privacy) cinnamon-screensaver-command -l; xset dpms force off; log_event "Privacy shield activated" ;;
        clip)  echo -n "" | xclip -selection primary; echo -n "" | xclip -selection clipboard ;;
        service) sudo systemctl restart rustdesk; log_event "RustDesk service restarted" ;;
        audio) pulseaudio -k; sleep 1; pulseaudio --start ;;
        keys)  setxkbmap us ;;
        fix)   do_action clip; do_action audio; do_action keys; log_event "Fix all: clip+audio+keys" ;;
        reset) apply_all 1024 768 1 1.0 24 "Reset" ;;
    esac
}

get_toggle_states() {
    SPD=$(gsettings get org.cinnamon desktop-effects 2>/dev/null); [ "$SPD" == "false" ] && S_ST="ON" || S_ST="OFF"
    CAF=$(gsettings get org.cinnamon.desktop.screensaver lock-enabled 2>/dev/null); [ "$CAF" == "false" ] && C_ST="ON" || C_ST="OFF"
    THM=$(gsettings get org.cinnamon.desktop.interface gtk-theme 2>/dev/null | tr -d "'"); [[ "$THM" == *"Dark"* ]] && T_ST="Dark" || T_ST="Light"
    GAMMA=$(xgamma 2>&1 | awk '{print $4}'); [[ "$GAMMA" != "1.000" ]] && N_ST="ON" || N_ST="OFF"
}

show_info() {
    local cur_mode="None" cur_res="N/A"
    if [ -f "$STATE_FILE" ]; then
        cur_mode=$(awk -F"'" '{print $2}' "$STATE_FILE")
        read w h _ <<< "$(cat "$STATE_FILE")"
        cur_res="${w}x${h}"
    fi
    get_toggle_states
    get_stats
    echo -e "${CYAN}Remote Studio${NC}"
    echo -e "  Mode:        ${GREEN}${cur_mode}${NC} (${cur_res})"
    echo -e "  Speed Mode:  $([ "$S_ST" == "ON" ] && echo "${GREEN}ON${NC}" || echo "${DIM}OFF${NC}")"
    echo -e "  Theme:       ${T_ST}"
    echo -e "  Night Shift: $([ "$N_ST" == "ON" ] && echo "${YELLOW}ON${NC}" || echo "${DIM}OFF${NC}")"
    echo -e "  Caffeine:    $([ "$C_ST" == "ON" ] && echo "${GREEN}ON${NC}" || echo "${DIM}OFF${NC}")"
    echo -e "  IP:          ${IP_ADDR}"
    echo -e "  Temp:        ${THERMAL_ALERT}${TEMP}"
    echo -e "  RAM:         ${RAM}"
    echo -e "  Latency:     ${PING_STAT}"
    echo -e "  RustDesk:    ${USERS} user(s)"
}

show_log() {
    local lines=${1:-20}
    if [ -f "$LOG_FILE" ]; then
        tail -n "$lines" "$LOG_FILE"
    else
        echo "No log file yet."
    fi
}

show_help() {
    echo "Remote Studio - RustDesk display management"
    echo ""
    echo "Usage: res [command]"
    echo ""
    echo "Device Profiles:"
    for key in $(echo "${!PROFILES[@]}" | tr ' ' '\n' | sort); do
        IFS='|' read -r label width height scaling _ _ <<< "${PROFILES[$key]}"
        printf "  %-12s %s (%dx%d @%sx)\n" "$key" "$label" "$width" "$height" "$scaling"
    done
    echo ""
    echo "Custom Resolution:"
    echo "  custom W H [S] Set arbitrary WxH resolution (S=scaling, default 1)"
    echo ""
    echo "Actions:"
    echo "  speed        Toggle performance mode (animations/wallpaper)"
    echo "  theme        Toggle OLED dark/light theme"
    echo "  night        Toggle night shift (warm gamma)"
    echo "  caf          Toggle caffeine (disable screen lock)"
    echo "  privacy      Lock screen + blank monitor"
    echo "  fix          Fix clipboard + audio + keyboard"
    echo "  clip         Flush clipboard only"
    echo "  audio        Restart PulseAudio only"
    echo "  keys         Reset keyboard layout (US)"
    echo "  service      Restart RustDesk service (sudo)"
    echo "  reset        Reset to 1024x768"
    echo "  doctor       Check RustDesk, Tailscale, Xorg, profiles, and symlinks"
    echo "  tailnet      Show this host's Tailscale IPv4 and RustDesk direct address"
    echo "  xorg [PATH]  Generate Xorg dummy config from profiles"
    echo ""
    echo "Info:"
    echo "  info         Show current state and toggle status"
    echo "  status       Print system stats (pipe-delimited, for applet)"
    echo "  log [N]      Show last N log entries (default 20)"
    echo "  help         Show this help"
    echo ""
    echo "Custom profiles: $USER_PROFILES"
    echo "  Format: key=label|width|height|scaling|text_scale|cursor"
    echo ""
    echo "No arguments launches the interactive TUI."
}

generate_xorg() {
    local out="${1:-}"
    local lines=()
    local mode_names=()
    local key label width height scaling text_scale cursor mode_name mode_info mode_params

    for key in mac mac15 fallback; do
        [ -n "${PROFILES[$key]:-}" ] || continue
        IFS='|' read -r label width height scaling text_scale cursor <<< "${PROFILES[$key]}"
        mode_name="${width}x${height}_60.00"
        mode_info=$(cvt "$width" "$height" 60 | grep Modeline)
        mode_params=$(echo "$mode_info" | cut -d' ' -f3-)
        lines+=("    Modeline \"$mode_name\" $mode_params")
        mode_names+=("\"$mode_name\"")
    done

    {
        echo 'Section "Device"'
        echo '    Identifier  "Configured Video Device"'
        echo '    Driver      "dummy"'
        echo '    VideoRam    512000'
        echo 'EndSection'
        echo
        echo 'Section "Monitor"'
        echo '    Identifier  "Configured Monitor"'
        printf '%s\n' "${lines[@]}"
        echo '    Option "PreferredMode" "2560x1664_60.00"'
        echo 'EndSection'
        echo
        echo 'Section "Screen"'
        echo '    Identifier  "Default Screen"'
        echo '    Monitor     "Configured Monitor"'
        echo '    Device      "Configured Video Device"'
        echo '    DefaultDepth 24'
        echo '    SubSection "Display"'
        echo '        Depth 24'
        echo "        Modes ${mode_names[*]} \"1024x768\""
        echo '        Virtual 3840 2160'
        echo '    EndSubSection'
        echo 'EndSection'
    } > "${out:-/dev/stdout}"
}

doctor_check() {
    local name=$1 status=$2 detail=$3
    printf "%-22s %-4s %s\n" "$name" "$status" "$detail"
}

show_doctor() {
    local output current mode rustdesk_state tailscale_state tailnet_ip renderer profile_link applet_link
    echo "Remote Studio doctor"

    command -v xrandr >/dev/null 2>&1 && doctor_check "xrandr" "OK" "$(command -v xrandr)" || doctor_check "xrandr" "MISS" "install x11-xserver-utils"
    command -v cvt >/dev/null 2>&1 && doctor_check "cvt" "OK" "$(command -v cvt)" || doctor_check "cvt" "MISS" "install xserver-xorg-core"
    command -v bc >/dev/null 2>&1 && doctor_check "bc" "OK" "$(command -v bc)" || doctor_check "bc" "MISS" "install bc"

    current=$(xrandr 2>/dev/null | awk '/ connected/ {out=$1} /\*/ {print out " " $1; exit}')
    [ -n "$current" ] && doctor_check "display" "OK" "$current" || doctor_check "display" "WARN" "no active X display detected"

    renderer=$(glxinfo -B 2>/dev/null | awk -F': ' '/OpenGL renderer string/ {print $2}')
    if [ -n "$renderer" ]; then
        case "$renderer" in
            *llvmpipe*) doctor_check "renderer" "WARN" "$renderer; software rendering" ;;
            *) doctor_check "renderer" "OK" "$renderer" ;;
        esac
    else
        doctor_check "renderer" "WARN" "glxinfo unavailable or no GL context"
    fi

    rustdesk_state=$(systemctl is-active rustdesk 2>/dev/null)
    [ "$rustdesk_state" = "active" ] && doctor_check "rustdesk" "OK" "service active" || doctor_check "rustdesk" "WARN" "service state: ${rustdesk_state:-unknown}"

    tailscale_state=$(systemctl is-active tailscaled 2>/dev/null)
    tailnet_ip=$(get_tailnet_ip)
    if [ "$tailscale_state" = "active" ] && [ -n "$tailnet_ip" ]; then
        doctor_check "tailscale" "OK" "$tailnet_ip"
    else
        doctor_check "tailscale" "WARN" "service=${tailscale_state:-unknown} ip=${tailnet_ip:-none}"
    fi

    if [ -f "$HOME/.config/rustdesk/RustDesk_default.toml" ]; then
        grep -q "codec-preference = 'auto'" "$HOME/.config/rustdesk/RustDesk_default.toml" \
            && doctor_check "rustdesk codec" "OK" "auto codec preference" \
            || doctor_check "rustdesk codec" "WARN" "expected codec-preference = 'auto'"
    else
        doctor_check "rustdesk config" "WARN" "missing RustDesk_default.toml"
    fi

    profile_link=$(readlink -f "$HOME/.xsessionrc" 2>/dev/null)
    [ "$profile_link" = "$ROOT_DIR/config/xsessionrc" ] && doctor_check "xsessionrc" "OK" "$profile_link" || doctor_check "xsessionrc" "WARN" "${profile_link:-not linked}"

    applet_link=$(readlink -f "$HOME/.local/share/cinnamon/applets/remote-studio@neek/applet.js" 2>/dev/null)
    [ "$applet_link" = "$ROOT_DIR/applet/applet.js" ] && doctor_check "applet link" "OK" "$applet_link" || doctor_check "applet link" "WARN" "${applet_link:-not linked}"

    if [ -f "$STATE_FILE" ]; then
        doctor_check "state" "OK" "$(cat "$STATE_FILE")"
    else
        doctor_check "state" "WARN" "no $STATE_FILE yet"
    fi
}

show_tailnet() {
    local ip
    ip=$(get_tailnet_ip)
    if [ -z "$ip" ]; then
        echo "Tailscale IPv4 not available."
        return 1
    fi
    echo "Tailscale IP: $ip"
    echo "RustDesk direct: $ip:21118"
}

# ------------------------------------------------------------------------------
# INTERFACES
# ------------------------------------------------------------------------------

if [ -n "$1" ]; then
    case "$1" in
        custom)
            if [ -z "$2" ] || [ -z "$3" ]; then
                echo "Usage: res custom WIDTH HEIGHT [SCALING]"
                echo "  e.g. res custom 1920 1080"
                echo "  e.g. res custom 2560 1440 2"
                exit 1
            fi
            local_scaling="${4:-1}"
            local_text=$(echo "scale=1; 1.0 * $local_scaling" | bc)
            local_cursor=$(( 24 * local_scaling ))
            apply_all "$2" "$3" "$local_scaling" "$local_text" "$local_cursor" "Custom ${2}x${3}"
            ;;
        status) get_stats; net=$(get_net_speed); cur="NONE"; [ -f "$STATE_FILE" ] && cur=$(awk -F"'" '{print $2}' "$STATE_FILE")
                echo "$cur | $TEMP | $PING_STAT | $USERS | $RAM | $THERMAL_ALERT | $net | $IP_ADDR" ;;
        info) show_info ;;
        log) show_log "$2" ;;
        doctor) show_doctor ;;
        tailnet) show_tailnet ;;
        xorg) generate_xorg "$2" ;;
        help|-h|--help) show_help ;;
        speed|theme|night|caf|privacy|clip|service|audio|keys|fix|reset) do_action "$1" ;;
        *) # Check if it's a profile name (including user-defined)
           if [ -n "${PROFILES[$1]}" ]; then
               apply_profile "$1"
           else
               echo "Unknown command: $1"
               echo "Run 'res help' for usage."
               exit 1
           fi ;;
    esac
    exit 0
fi

get_current_mode() {
    if [ -f "$STATE_FILE" ]; then
        awk -F"'" '{print $2}' "$STATE_FILE"
    else
        echo "None"
    fi
}

get_current_resolution() {
    if [ -f "$STATE_FILE" ]; then
        read -r w h _ < "$STATE_FILE"
        echo "${w}x${h}"
    else
        xrandr 2>/dev/null | awk '/\*/ {print $1; exit}'
    fi
}

get_active_display() {
    xrandr 2>/dev/null | awk '/ connected/ {out=$1} /\*/ {print out " " $1; exit}'
}

get_renderer_summary() {
    local renderer
    renderer=$(glxinfo -B 2>/dev/null | awk -F': ' '/OpenGL renderer string/ {print $2}')
    [ -n "$renderer" ] && echo "$renderer" || echo "unknown"
}

run_panel_command() {
    local title=$1
    shift
    local tmp
    tmp=$(mktemp)
    {
        echo "$ $*"
        echo
        "$@"
    } > "$tmp" 2>&1
    whiptail --title "$title" --scrolltext --textbox "$tmp" 24 90
    rm -f "$tmp"
}

confirm_action() {
    local message=$1
    whiptail --title "Confirm" --yesno "$message" 10 70
}

tui_header() {
    local mode res display renderer tailnet rustdesk_state tailscale_state
    mode=$(get_current_mode)
    res=$(get_current_resolution)
    display=$(get_active_display)
    renderer=$(get_renderer_summary)
    tailnet=$(get_tailnet_ip)
    rustdesk_state=$(systemctl is-active rustdesk 2>/dev/null || echo "unknown")
    tailscale_state=$(systemctl is-active tailscaled 2>/dev/null || echo "unknown")
    get_stats
    get_toggle_states

    cat <<EOF
Mode:       $mode ($res)
Display:    ${display:-not detected}
Tailnet:    ${tailnet:-unavailable}  RustDesk direct: ${tailnet:-<tailscale-ip>}:21118
Services:   rustdesk=$rustdesk_state  tailscaled=$tailscale_state  users=$USERS
System:     temp=${THERMAL_ALERT}${TEMP:-unknown}  ram=$RAM  ping=$PING_STAT
Toggles:    speed=$S_ST  caffeine=$C_ST  theme=$T_ST  night=$N_ST
Renderer:   $renderer
EOF
}

show_dashboard() {
    local tmp
    tmp=$(mktemp)
    tui_header > "$tmp"
    whiptail --title "Remote Studio Dashboard" --scrolltext --textbox "$tmp" 20 90
    rm -f "$tmp"
}

tui_profiles() {
    local entries=()
    local key label width height scaling text_scale cursor current choice
    current=$(get_current_mode)

    for key in $(printf '%s\n' "${!PROFILES[@]}" | sort); do
        IFS='|' read -r label width height scaling text_scale cursor <<< "${PROFILES[$key]}"
        entries+=("$key" "$label ${width}x${height} scale=${scaling} cursor=${cursor}")
    done
    entries+=("custom" "Enter arbitrary resolution and scaling")

    choice=$(whiptail --title "Display Profiles" --backtitle "Current: $current" --menu "Apply a profile" 24 90 14 "${entries[@]}" 3>&1 1>&2 2>&3)
    [ -z "$choice" ] && return 0
    [ "$choice" = "custom" ] && { tui_custom_resolution; return 0; }

    if apply_profile "$choice"; then
        whiptail --title "Profile Applied" --msgbox "Applied ${PROFILES[$choice]}" 9 80
    else
        whiptail --title "Profile Failed" --msgbox "Could not apply profile: $choice" 9 80
    fi
}

tui_custom_resolution() {
    local width height scaling text_scale cursor
    width=$(whiptail --title "Custom Resolution" --inputbox "Width" 9 50 "1920" 3>&1 1>&2 2>&3) || return 0
    height=$(whiptail --title "Custom Resolution" --inputbox "Height" 9 50 "1200" 3>&1 1>&2 2>&3) || return 0
    scaling=$(whiptail --title "Custom Resolution" --inputbox "Cinnamon scaling factor" 9 50 "1" 3>&1 1>&2 2>&3) || return 0
    case "$width:$height:$scaling" in
        *[!0-9:.]*|:*|*:|*::*|*.*:*)
            whiptail --title "Invalid Input" --msgbox "Use numeric width, height, and scaling." 9 60
            return 1
            ;;
    esac
    text_scale="$scaling"
    cursor=$(awk "BEGIN { printf \"%d\", 24 * $scaling }")
    apply_all "$width" "$height" "$scaling" "$text_scale" "$cursor" "Custom ${width}x${height}" \
        && whiptail --title "Custom Resolution" --msgbox "Applied ${width}x${height}." 8 60 \
        || whiptail --title "Custom Resolution" --msgbox "Could not apply ${width}x${height}." 8 60
}

tui_performance() {
    local choice
    while true; do
        get_toggle_states
        choice=$(whiptail --title "Performance & Comfort" --menu "Tune the active remote session" 20 82 10 \
            "speed" "Toggle performance mode (currently $S_ST)" \
            "caf" "Toggle caffeine / screen lock (currently $C_ST)" \
            "theme" "Toggle dark/light theme (currently $T_ST)" \
            "night" "Toggle warm gamma (currently $N_ST)" \
            "fix" "Fix clipboard, audio, and keyboard layout" \
            "audio" "Restart PulseAudio" \
            "keys" "Reset keyboard layout to US" \
            "back" "Return to dashboard" 3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            speed|caf|theme|night|fix|audio|keys)
                do_action "$choice"
                whiptail --title "Action Complete" --msgbox "Ran: $choice" 8 50
                ;;
        esac
    done
}

tui_diagnostics() {
    local choice
    while true; do
        choice=$(whiptail --title "Diagnostics" --menu "Inspect the remote desktop stack" 20 86 10 \
            "doctor" "Full Remote Studio health report" \
            "tailnet" "Show Tailscale IP and RustDesk direct address" \
            "info" "Show current state and toggles" \
            "xrandr" "Show active Xorg display modes" \
            "gl" "Show OpenGL renderer details" \
            "services" "Show RustDesk and Tailscale service state" \
            "log" "Show Remote Studio event log" \
            "back" "Return to dashboard" 3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            doctor) run_panel_command "Doctor" "$0" doctor ;;
            tailnet) run_panel_command "Tailnet" "$0" tailnet ;;
            info) run_panel_command "Info" "$0" info ;;
            xrandr) run_panel_command "xrandr" xrandr --verbose ;;
            gl) run_panel_command "OpenGL Renderer" glxinfo -B ;;
            services) run_panel_command "Services" systemctl status rustdesk tailscaled --no-pager ;;
            log) run_panel_command "Remote Studio Log" "$0" log 80 ;;
        esac
    done
}

tui_system() {
    local choice tmp
    while true; do
        choice=$(whiptail --title "System & Config" --menu "System-level operations" 20 86 10 \
            "service" "Restart RustDesk service" \
            "xorg-preview" "Preview generated Xorg dummy config" \
            "xorg-write" "Write generated Xorg config to config/xorg.conf" \
            "install" "Run user-level install" \
            "backup" "Run install.sh backup" \
            "privacy" "Lock screen and blank monitor" \
            "reset" "Reset display to 1024x768" \
            "back" "Return to dashboard" 3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            service)
                confirm_action "Restart RustDesk now?" && do_action service
                ;;
            xorg-preview)
                run_panel_command "Generated Xorg Config" "$0" xorg
                ;;
            xorg-write)
                confirm_action "Regenerate config/xorg.conf from profiles?" && "$0" xorg "$ROOT_DIR/config/xorg.conf" \
                    && whiptail --title "Xorg Config" --msgbox "Updated config/xorg.conf." 8 60
                ;;
            install)
                run_panel_command "Install" "$ROOT_DIR/install.sh" install
                ;;
            backup)
                run_panel_command "Backup" "$ROOT_DIR/install.sh" backup
                ;;
            privacy|reset)
                confirm_action "Run $choice now?" && do_action "$choice"
                ;;
        esac
    done
}

show_text_menu() {
    local choice
    while true; do
        clear
        echo -e "${CYAN}Remote Studio${NC}"
        tui_header
        echo
        echo "1) Profiles       2) Performance    3) Diagnostics"
        echo "4) System         5) Logs           6) Tailnet"
        echo "7) Doctor         8) Help           9) Exit"
        echo
        read -r -p "Select [1-9]: " choice
        case "$choice" in
            1) show_help; read -r -p "Run profile key: " key; [ -n "$key" ] && apply_profile "$key" ;;
            2) do_action speed ;;
            3|7) show_doctor; read -r -p "Press enter to continue..." _ ;;
            4) do_action service ;;
            5) show_log 40; read -r -p "Press enter to continue..." _ ;;
            6) show_tailnet; read -r -p "Press enter to continue..." _ ;;
            8) show_help; read -r -p "Press enter to continue..." _ ;;
            9) exit 0 ;;
        esac
    done
}

if ! command -v whiptail >/dev/null 2>&1 || [ "$(tput lines 2>/dev/null || echo 0)" -lt 18 ] || [ "$(tput cols 2>/dev/null || echo 0)" -lt 70 ]; then
    show_text_menu
fi

while true; do
    mode=$(get_current_mode)
    res=$(get_current_resolution)
    tailnet=$(get_tailnet_ip)
    users=$(ss -tnp 2>/dev/null | grep -i "rustdesk" | grep -i "ESTAB" | wc -l)

    choice=$(whiptail --title "Remote Studio" --backtitle "Mode: $mode ($res) | Tailnet: ${tailnet:-none} | Users: $users" --menu "$(tui_header)" 24 92 9 \
        "profiles" "Apply display profile or custom resolution" \
        "performance" "Session speed, comfort, and quick fixes" \
        "diagnostics" "Doctor, Tailscale, xrandr, renderer, logs" \
        "system" "RustDesk service, Xorg config, install, backups" \
        "dashboard" "Show dashboard as a scrollable report" \
        "tailnet" "Print RustDesk direct address" \
        "doctor" "Run health checks" \
        "help" "Command reference" \
        "exit" "Quit" 3>&1 1>&2 2>&3) || exit 0

    case "$choice" in
        profiles) tui_profiles ;;
        performance) tui_performance ;;
        diagnostics) tui_diagnostics ;;
        system) tui_system ;;
        dashboard) show_dashboard ;;
        tailnet) run_panel_command "Tailnet" "$0" tailnet ;;
        doctor) run_panel_command "Doctor" "$0" doctor ;;
        help) run_panel_command "Help" "$0" help ;;
        exit) exit 0 ;;
    esac
done

#!/bin/bash

# ==============================================================================
# RUSTDESK REMOTE STUDIO V8.0
# ==============================================================================

STATE_FILE="$HOME/.res_state"
WALLPAPER_BACKUP="$HOME/.wallpaper_backup"
LOG_FILE="$HOME/.remote_studio.log"
USER_PROFILES="$HOME/.config/remote-studio/profiles.conf"

# Colors
RED='\033[1;31m'
GREEN='\033[1;32m'
CYAN='\033[1;36m'
YELLOW='\033[1;33m'
DIM='\033[2m'
NC='\033[0m'

# Built-in device profiles: label|width|height|scaling|text_scale|cursor
declare -A PROFILES=(
    [mac]="MacBook Air 13|2560|1664|1|1.5|48"
    [ipad]="iPad Pro 11\"|2424|1664|2|1.1|48"
    [iphonel]="iPhone Landscape|2868|1320|2|1.2|64"
    [iphonep]="iPhone Portrait|1320|2868|2|1.2|64"
)

# Load user-defined profiles from ~/.config/remote-studio/profiles.conf
# Format: key=label|width|height|scaling|text_scale|cursor
if [ -f "$USER_PROFILES" ]; then
    while IFS='=' read -r key value; do
        [[ "$key" =~ ^[[:space:]]*# ]] && continue  # skip comments
        [[ -z "$key" || -z "$value" ]] && continue   # skip empty lines
        key=$(echo "$key" | xargs)  # trim whitespace
        PROFILES[$key]="$value"
    done < "$USER_PROFILES"
fi

# ------------------------------------------------------------------------------
# CORE ENGINE
# ------------------------------------------------------------------------------

log_event() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"; }

get_stats() {
    IP_ADDR=$(hostname -I | awk '{print $1}')
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
    MODE_NAME="remote-studio-${width}x${height}-60"
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

# FALLBACK TEXT MENU
show_text_menu() {
    while true; do
        clear
        get_stats
        get_toggle_states
        CUR="None"; [ -f "$STATE_FILE" ] && CUR=$(awk -F"'" '{print $2}' "$STATE_FILE")
        echo -e "${CYAN}=========================================================${NC}"
        echo -e "${YELLOW} REMOTE STUDIO (TEXT MODE)${NC}  ${DIM}Mode: ${CUR}${NC}"
        echo -e "${CYAN} IP: ${NC}$IP_ADDR ${CYAN}| Users: ${NC}$USERS ${CYAN}| ${NC}${RED}${THERMAL_ALERT}${NC}${TEMP} ${CYAN}| ${NC}${RAM}"
        echo -e "${CYAN}=========================================================${NC}"
        echo " 1) MacBook Air | 2) iPad Pro | 3) iPhone L | 4) iPhone P"
        echo -e " 5) Speed $([ "$S_ST" == "ON" ] && echo "${GREEN}[ON]${NC} " || echo "[OFF]") | 6) Theme [${T_ST}] | 7) Caffeine $([ "$C_ST" == "ON" ] && echo "${GREEN}[ON]${NC} " || echo "[OFF]") | 8) Night $([ "$N_ST" == "ON" ] && echo "${YELLOW}[ON]${NC} " || echo "[OFF]")"
        echo " 9) Fix All     | 10) Privacy  | 11) Reset   | 12) Exit"
        echo -e "${CYAN}=========================================================${NC}"
        read -p "Select [1-12]: " C
        case $C in
            1) apply_profile mac ;; 2) apply_profile ipad ;;
            3) apply_profile iphonel ;; 4) apply_profile iphonep ;;
            5) do_action speed ;; 6) do_action theme ;; 7) do_action caf ;; 8) do_action night ;;
            9) do_action fix ;; 10) do_action privacy ;; 11) do_action reset ;;
            12) exit 0 ;;
        esac
    done
}

# TUI Loop
while true; do
    get_stats
    get_toggle_states
    S_ICO="[🚀]"; [ "$S_ST" == "OFF" ] && S_ICO="[🎨]"
    C_ICO="[☕]"; [ "$C_ST" == "OFF" ] && C_ICO="[OFF]"
    T_ICO="[🌙]"; [ "$T_ST" == "Light" ] && T_ICO="[☀️]"
    N_ICO="[🌙]"; [ "$N_ST" == "OFF" ] && N_ICO="[OFF]"
    CUR="None"; [ -f "$STATE_FILE" ] && CUR=$(awk -F"'" '{print $2}' "$STATE_FILE")

    # DYNAMIC WHIPTAIL SIZE
    T_H=$(tput lines); T_W=$(tput cols)
    W_H=$(( T_H - 4 )); W_W=$(( T_W - 8 ))
    [ $W_H -gt 22 ] && W_H=22; [ $W_W -gt 78 ] && W_W=78

    WCHOICE=$(whiptail --title "STUDIO | $IP_ADDR | 👥 $USERS | $PING_STAT" --backtitle "Remote Studio (Mode: $CUR)" --menu "Control Dashboard" $W_H $W_W 13 \
    "1" "💻 MacBook Air 13 (2560x1664)" \
    "2" "📱 iPad Pro 11\" (3:2)" \
    "3" "📱 iPhone Landscape (19.5:9)" \
    "4" "📱 iPhone Portrait (9:19.5)" \
    " " "─── PERFORMANCE & COMFORT ───" \
    "5" "Toggle Performance Mode $S_ICO" \
    "6" "Toggle OLED Dark Theme  $T_ICO" \
    "7" "Toggle Night Shift      $N_ICO" \
    "8" "Toggle Caffeine Mode    $C_ICO" \
    "  " "─── SYSTEM TOOLS ───" \
    "9" "Security Privacy Shield (Lock)" \
    "10" "Fix Clipboard / Audio / Keys" \
    "11" "Restart RustDesk Service" \
    "12" "Standard Mode Reset" \
    "13" "Exit" 3>&1 1>&2 2>&3)

    RET=$?
    if [ $RET -eq 255 ]; then show_text_menu; elif [ $RET -ne 0 ]; then exit 0; fi

    case $WCHOICE in
        1) apply_profile mac ;; 2) apply_profile ipad ;;
        3) apply_profile iphonel ;; 4) apply_profile iphonep ;;
        5) do_action speed ;; 6) do_action theme ;; 7) do_action night ;; 8) do_action caf ;;
        9) do_action privacy ;; 10) do_action fix ;; 11) do_action service ;; 12) do_action reset ;;
        13) exit 0 ;;
        *) ;; # section headers, ignore
    esac
done

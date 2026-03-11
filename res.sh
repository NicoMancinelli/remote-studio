#!/bin/bash

# ==============================================================================
# RUSTDESK REMOTE STUDIO V7.0
# ==============================================================================

STATE_FILE="$HOME/.res_state"
WALLPAPER_BACKUP="$HOME/.wallpaper_backup"
LOG_FILE="$HOME/.remote_studio.log"

# Colors for Text Menu
RED='\033[1;31m'
CYAN='\033[1;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Device profiles: name|width|height|scaling|text_scale|cursor
declare -A PROFILES=(
    [mac]="MacBook Air|2880|1800|1|1.5|48"
    [ipad]="iPad Pro 11\"|2424|1664|2|1.1|48"
    [iphonel]="iPhone Landscape|2868|1320|2|1.2|64"
    [iphonep]="iPhone Portrait|1320|2868|2|1.2|64"
)

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
    MODE_NAME=$(echo "$MODE_INFO" | awk '{print $2}' | tr -d '"')
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
                   # Save wallpaper before disabling
                   gsettings get org.cinnamon.desktop.background picture-uri > "$WALLPAPER_BACKUP" 2>/dev/null
                   gsettings set org.cinnamon desktop-effects false; gsettings set org.cinnamon.desktop.interface enable-animations false
                   gsettings set org.cinnamon.desktop.background picture-options "none"; gsettings set org.cinnamon.desktop.background primary-color "#000000"
               else
                   gsettings set org.cinnamon desktop-effects true; gsettings set org.cinnamon.desktop.interface enable-animations true
                   gsettings set org.cinnamon.desktop.background picture-options "zoom"
                   # Restore wallpaper if saved
                   if [ -f "$WALLPAPER_BACKUP" ]; then
                       gsettings set org.cinnamon.desktop.background picture-uri "$(cat "$WALLPAPER_BACKUP")"
                       rm -f "$WALLPAPER_BACKUP"
                   fi
               fi ;;
        theme) cur=$(gsettings get org.cinnamon.desktop.interface gtk-theme | tr -d "'")
               [[ "$cur" == *"Dark"* ]] && gsettings set org.cinnamon.desktop.interface gtk-theme "Mint-Y" || gsettings set org.cinnamon.desktop.interface gtk-theme "Mint-Y-Dark" ;;
        night) gamma=$(xgamma 2>&1 | awk '{print $4}'); [[ "$gamma" == "1.000" ]] && xgamma -rgamma 1.0 -ggamma 0.8 -bgamma 0.6 || xgamma -gamma 1.0 ;;
        caf)   cur=$(gsettings get org.cinnamon.desktop.screensaver lock-enabled)
               [[ "$cur" == "true" ]] && gsettings set org.cinnamon.desktop.screensaver lock-enabled false || gsettings set org.cinnamon.desktop.screensaver lock-enabled true ;;
        privacy) cinnamon-screensaver-command -l; xset dpms force off ;;
        clip)  echo -n "" | xclip -selection primary; echo -n "" | xclip -selection clipboard ;;
        service) sudo systemctl restart rustdesk ;;
        audio) pulseaudio -k; sleep 1; pulseaudio --start ;;
        keys)  setxkbmap us ;;
        fix)   do_action clip; do_action audio; do_action keys ;;
        reset) apply_all 1024 768 1 1.0 24 "Reset" ;;
    esac
}

# ------------------------------------------------------------------------------
# INTERFACES
# ------------------------------------------------------------------------------

if [ -n "$1" ]; then
    case "$1" in
        mac|ipad|iphonel|iphonep) apply_profile "$1" ;;
        status) get_stats; net=$(get_net_speed); cur="NONE"; [ -f "$STATE_FILE" ] && cur=$(awk -F"'" '{print $2}' "$STATE_FILE")
                echo "$cur | $TEMP | $PING_STAT | $USERS | $RAM | $THERMAL_ALERT | $net | $IP_ADDR" ;;
        *) do_action "$1" ;;
    esac
    exit 0
fi

# FALLBACK TEXT MENU
show_text_menu() {
    while true; do
        clear
        get_stats
        echo -e "${CYAN}=========================================================${NC}"
        echo -e "${YELLOW} RUSTDESK CONTROL STUDIO (TEXT MODE) ${NC}"
        echo -e "${CYAN} IP: ${NC}$IP_ADDR | ${CYAN}👥 USERS: ${NC}$USERS | ${RED}${THERMAL_ALERT}${TEMP}${NC}"
        echo -e "${CYAN}=========================================================${NC}"
        echo " 1) MacBook Air | 2) iPad Pro | 3) iPhone L | 4) iPhone P"
        echo " 5) Speed Mode  | 6) OLED Theme | 7) Caffeine | 8) Night Shift"
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
    SPD=$(gsettings get org.cinnamon desktop-effects); [ "$SPD" == "false" ] && S_ST="[🚀]" || S_ST="[🎨]"
    CAF=$(gsettings get org.cinnamon.desktop.screensaver lock-enabled); [ "$CAF" == "false" ] && C_ST="[☕]" || C_ST="[OFF]"
    THM=$(gsettings get org.cinnamon.desktop.interface gtk-theme | tr -d "'"); [[ "$THM" == *"Dark"* ]] && T_ST="[🌙]" || T_ST="[☀️]"
    GAMMA=$(xgamma 2>&1 | awk '{print $4}'); [[ "$GAMMA" == "1.000" ]] && N_ST="[OFF]" || N_ST="[🌙]"
    CUR="None"; [ -f "$STATE_FILE" ] && CUR=$(awk -F"'" '{print $2}' "$STATE_FILE")

    # DYNAMIC WHIPTAIL SIZE
    T_H=$(tput lines); T_W=$(tput cols)
    W_H=$(( T_H - 4 )); W_W=$(( T_W - 8 ))
    [ $W_H -gt 22 ] && W_H=22; [ $W_W -gt 78 ] && W_W=78

    WCHOICE=$(whiptail --title "STUDIO | $IP_ADDR | 👥 $USERS | $PING_STAT" --backtitle "Remote Studio (Mode: $CUR)" --menu "Control Dashboard" $W_H $W_W 13 \
    "1" "💻 MacBook Air M4 (16:10)" \
    "2" "📱 iPad Pro 11\" (3:2)" \
    "3" "📱 iPhone Landscape (19.5:9)" \
    "4" "📱 iPhone Portrait (9:19.5)" \
    " " "─── PERFORMANCE & COMFORT ───" \
    "5" "Toggle Performance Mode $S_ST" \
    "6" "Toggle OLED Dark Theme  $T_ST" \
    "7" "Toggle Night Shift      $N_ST" \
    "8" "Toggle Caffeine Mode    $C_ST" \
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

#!/bin/bash

# ==============================================================================
# RUSTDESK REMOTE STUDIO V8.0
# ==============================================================================

VERSION="8.0"
STATE_FILE="$HOME/.res_state"
WALLPAPER_BACKUP="$HOME/.wallpaper_backup"
LOG_FILE="$HOME/.remote_studio.log"
SESSION_FILE="$HOME/.config/remote-studio/session.state"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_PROFILES="$ROOT_DIR/config/profiles.conf"
USER_PROFILES="$HOME/.config/remote-studio/profiles.conf"
if [ -n "${XDG_RUNTIME_DIR:-}" ] && [ -w "$XDG_RUNTIME_DIR" ]; then
    STATUS_DIR="$XDG_RUNTIME_DIR/remote-studio"
else
    STATUS_DIR="/tmp/remote-studio-${UID:-$(id -u)}"
fi
STATUS_FILE="$STATUS_DIR/status"
USER_CONFIG="$HOME/.config/remote-studio/remote-studio.conf"
RECENT_PROFILES_FILE="$HOME/.config/remote-studio/recent_profiles"
_WARN_CACHE=""
_WARN_CACHE_TS=0

# Load user config if exists
if [ -f "$USER_CONFIG" ]; then
    # shellcheck source=/dev/null
    source "$USER_CONFIG"
fi

DEFAULT_PROFILE="${DEFAULT_PROFILE:-mac}"
DEFAULT_RUSTDESK_PRESET="${DEFAULT_RUSTDESK_PRESET:-default}"

# Colors
# shellcheck disable=SC2034
RED='\033[1;31m'
GREEN='\033[1;32m'
CYAN='\033[1;36m'
YELLOW='\033[1;33m'
DIM='\033[2m'
NC='\033[0m'

declare -A PROFILES=()

get_current_mode() {
    [ -f "$STATE_FILE" ] && awk -F"'" '{print $2}' "$STATE_FILE" || echo "None"
}

get_current_resolution() {
    if [ -f "$STATE_FILE" ]; then
        read -r w h _ <<< "$(cat "$STATE_FILE")"
        echo "${w}x${h}"
    else
        echo "N/A"
    fi
}

validate_profiles() {
    local file=$1; local errs=0
    [ -f "$file" ] || return 0
    while IFS='=' read -r key value; do
        [[ "$key" =~ ^[[:space:]]*# ]] && continue
        [[ -z "$key" || -z "$value" ]] && continue
        if [[ ! "$value" =~ ^[^\|]+\|[0-9]+\|[0-9]+\|[0-9.]+\|[0-9.]+\|[0-9]+$ ]]; then
            echo "Error: Malformed profile line in $file: $key=$value" >&2
            errs=$((errs + 1))
        fi
    done < "$file"
    return $errs
}

load_profiles_file() {
    local file=$1
    [ -f "$file" ] || return 0
    validate_profiles "$file" || echo "Warning: $file has validation errors." >&2
    while IFS='=' read -r key value; do
        [[ "$key" =~ ^[[:space:]]*# ]] && continue
        [[ -z "$key" || -z "$value" ]] && continue
        key=$(echo "$key" | xargs)
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

get_lan_ip() { hostname -I 2>/dev/null | awk '{print $1}'; }

get_primary_ip() {
    local tailnet_ip
    tailnet_ip=$(get_tailnet_ip)
    if [ -n "$tailnet_ip" ]; then
        echo "$tailnet_ip"
    else
        hostname -I 2>/dev/null | awk '{print $1}'
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

get_stats() {
    IP_ADDR=$(get_primary_ip)
    TEMP=$(sensors 2>/dev/null | grep "Package id 0" | awk '{print $4}' | tr -d '+')
    RAM=$(free -m | awk 'NR==2{printf "%.1f%%", $3*100/$2 }')
    USERS=$(ss -tnp 2>/dev/null | grep -i "rustdesk" | grep -ic "ESTAB")

    # Connection Path Detection
    if [ "$USERS" -gt 0 ]; then
        if ss -tnp 2>/dev/null | grep -i "rustdesk" | grep -i "ESTAB" | grep -q ":21118"; then
            RUSTDESK_CONN_TYPE="Direct"
        else
            RUSTDESK_CONN_TYPE="Relayed"
        fi
    else
        RUSTDESK_CONN_TYPE="None"
    fi

    PING_RAW=$(ping -c 1 -W 1 8.8.8.8 2>/dev/null | grep 'time=' | awk -F'time=' '{print $2}' | cut -d' ' -f1 | cut -d'.' -f1)
    [ -z "$PING_RAW" ] && PING_STAT="Offline" || PING_STAT="${PING_RAW}ms"
    [[ "${TEMP%.*}" -gt 80 ]] 2>/dev/null && THERMAL_ALERT="⚠️ " || THERMAL_ALERT=""
}

check_auto_speed() {
    [ -n "$PING_RAW" ] || return 0
    local speed
    speed=$(speed_state)
    if [ "$PING_RAW" -gt 100 ] && [ "$speed" = "OFF" ]; then
        log_event "Auto-Speed: ON (Latency ${PING_RAW}ms)"
        do_action speed
        notify-send -u normal "Remote Studio" "High latency (${PING_RAW}ms): Speed Mode enabled"
    fi
}

get_warning_summary() {
    local warnings=0 messages=() renderer rustdesk_state tailscale_state tailnet_ip current
    local applet_dir applet_ok
    renderer=$(get_renderer_summary 2>/dev/null || true)
    rustdesk_state=$(systemctl is-active rustdesk 2>/dev/null || echo "unknown")
    tailscale_state=$(systemctl is-active tailscaled 2>/dev/null || echo "unknown")
    tailnet_ip=$(get_tailnet_ip)
    current=$(xrandr 2>/dev/null | awk '/ connected/ {out=$1} /\*/ {print out " " $1; exit}')

    if [[ "$renderer" == *llvmpipe* ]]; then warnings=$((warnings + 1)); messages+=("software-rendering"); fi
    if [ "$rustdesk_state" != "active" ]; then warnings=$((warnings + 1)); messages+=("rustdesk-${rustdesk_state:-unknown}"); fi
    if [ "$tailscale_state" != "active" ] || [ -z "$tailnet_ip" ]; then warnings=$((warnings + 1)); messages+=("tailscale"); fi
    if [ -z "$current" ]; then warnings=$((warnings + 1)); messages+=("display"); fi

    applet_dir="$HOME/.local/share/cinnamon/applets/remote-studio@neek"
    applet_ok=1
    for f in applet.js metadata.json; do
        if [ "$(readlink "$applet_dir/$f" 2>/dev/null)" != "$ROOT_DIR/applet/$f" ]; then
            applet_ok=0; break
        fi
    done
    if [ "$applet_ok" -eq 0 ]; then
        warnings=$((warnings + 1)); messages+=("applet-symlink")
        log_event "Warning: applet-symlink missing or incorrect"
    fi

    local ts_status
    ts_status=$(tailscale status --json 2>/dev/null | grep -o '"BackendState":"[^"]*"' | cut -d'"' -f4 || true)
    if [ "$ts_status" = "NeedsLogin" ] || [ "$ts_status" = "Stopped" ]; then
        warnings=$((warnings + 1)); messages+=("tailscale-${ts_status,,}")
    fi

    if [ "$warnings" -eq 0 ]; then
        echo "0|OK"
    else
        local IFS=','
        echo "$warnings|${messages[*]}"
    fi
}

get_net_speed() {
    local IFACE
    IFACE=$(ip route get 8.8.8.8 2>/dev/null | awk '{print $5; exit}')
    if [ -z "$IFACE" ] || [ ! -r "/sys/class/net/$IFACE/statistics/rx_bytes" ]; then echo "n/a"; return 0; fi
    local R1 T1 R2 T2 RX TX
    R1=$(cat "/sys/class/net/$IFACE/statistics/rx_bytes")
    T1=$(cat "/sys/class/net/$IFACE/statistics/tx_bytes")
    sleep 0.5
    R2=$(cat "/sys/class/net/$IFACE/statistics/rx_bytes")
    T2=$(cat "/sys/class/net/$IFACE/statistics/tx_bytes")
    RX=$(( (R2 - R1) / 512 ))
    TX=$(( (T2 - T1) / 512 ))
    echo "↓${RX}KB/s ↑${TX}KB/s"
}

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

speed_state() { [ "$(gsettings get org.cinnamon desktop-effects 2>/dev/null)" = "false" ] && echo "ON" || echo "OFF"; }
caffeine_state() { [ "$(gsettings get org.cinnamon.desktop.screensaver lock-enabled 2>/dev/null)" = "false" ] && echo "ON" || echo "OFF"; }

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
        read -r w h _ <<< "$(cat "$STATE_FILE")"
        cur_res="${w}x${h}"
    fi
    get_toggle_states; get_stats
    echo -e "${CYAN}Remote Studio${NC}"
    echo -e "  Mode:        ${GREEN}${cur_mode}${NC} (${cur_res})"
    echo -e "  Speed Mode:  $([ "$S_ST" == "ON" ] && echo "${GREEN}ON${NC}" || echo "${DIM}OFF${NC}")"
    echo -e "  Theme:       ${T_ST}"
    echo -e "  Night Shift: $([ "$N_ST" == "ON" ] && echo "${YELLOW}ON${NC}" || echo "${DIM}OFF${NC}")"
    echo -e "  Caffeine:    $([ "$C_ST" == "ON" ] && echo "${GREEN}ON${NC}" || echo "${DIM}OFF${NC}")"
    echo -e "  IP:          ${IP_ADDR}"
    check_auto_speed
    echo -e "  Temp:        ${THERMAL_ALERT}${TEMP}"
    echo -e "  RAM:         ${RAM}"
    echo -e "  Latency:     ${PING_STAT}"
    echo -e "  RustDesk:    ${USERS} user(s)"
}

show_status() {
    local cur net warning_data warning_count warning_text line res tailnet_ip lan_ip combined_ip rustdesk_direct
    get_stats; net=$(get_net_speed); cur=$(get_current_mode); res=$(get_current_resolution)
    warning_data=$(get_warning_summary); warning_count=${warning_data%%|*}; warning_text=${warning_data#*|}
    tailnet_ip=$(get_tailnet_ip)
    lan_ip=$(get_lan_ip)
    if [ -n "$tailnet_ip" ]; then
        combined_ip="${tailnet_ip}/${lan_ip}"
        rustdesk_direct="${tailnet_ip}:21118"
    else
        combined_ip="${lan_ip}"
        rustdesk_direct="${lan_ip}:21118"
    fi
    mkdir -p "$STATUS_DIR"
    check_auto_speed
    line="$cur | $TEMP | $PING_STAT | $USERS | $RAM | $warning_count | $warning_text | $net | $combined_ip | $RUSTDESK_CONN_TYPE | $res | $rustdesk_direct"
    printf '%s\n' "$line" > "$STATUS_FILE"; printf '%s\n' "$line"
}

show_log() { local lines=${1:-20}; if [ -f "$LOG_FILE" ]; then tail -n "$lines" "$LOG_FILE"; else echo "No log file yet."; fi; }

show_help() {
    echo "Remote Studio - RustDesk display management"
    echo "Usage: res [command]"
    echo ""; echo "Device Profiles:"
    for key in $(sorted_profile_keys); do
        IFS='|' read -r label width height scaling _ _ <<< "${PROFILES[$key]}"
        printf "  %-12s %s (%dx%d @%sx)\n" "$key" "$label" "$width" "$height" "$scaling"
    done
    echo ""; echo "Actions:"
    echo "  speed, theme, night, caf, privacy, fix, reset, service, audio, keys"
    echo "  doctor, doctor-fix"
    echo "  tailnet, tailnet peer <name>, tailnet hosts, tailnet doctor"
    echo "  rustdesk [apply <preset>|backup|diff <preset>|status|log [lines]]"
    echo "  xorg [PATH], session [start|stop|status]"
    echo "  custom <W> <H> [scale]   Apply arbitrary resolution (offers save-as-profile)"
    echo "  rotate [normal|left|right|inverted]"
    echo "  watch [interval_sec]     Foreground connection watcher"
    echo "  update                   Pull latest code and re-run install"
    echo "  profiles                 List all profiles and sources"
    echo "  config [show|get K|set K V]"
    echo "  version, help"
}

generate_xorg() {
    local out="${1:-}"; local lines=(); local mode_names=()
    for key in mac mac15 fallback; do
        [ -n "${PROFILES[$key]:-}" ] || continue
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

doctor_check() { printf "%-22s %-4s %s\n" "$1" "$2" "$3"; }
show_doctor() {
    local c r rs tip
    echo "Remote Studio doctor"
    if command -v xrandr >/dev/null 2>&1; then
        doctor_check "xrandr" "OK" "$(command -v xrandr)"
    else
        doctor_check "xrandr" "MISS" "install x11-xserver-utils"
    fi
    if command -v glxinfo >/dev/null 2>&1; then
        doctor_check "glxinfo" "OK" "$(command -v glxinfo)"
    else
        doctor_check "glxinfo" "MISS" "install mesa-utils"
    fi
    c=$(xrandr 2>/dev/null | awk '/ connected/ {out=$1} /\*/ {print out " " $1; exit}')
    if [ -n "$c" ]; then
        doctor_check "display" "OK" "$c"
    else
        doctor_check "display" "WARN" "no active X display"
    fi
    r=$(get_renderer_summary 2>/dev/null)
    if [[ "$r" == *llvmpipe* ]]; then
        doctor_check "renderer" "WARN" "$r (SW)"
    else
        doctor_check "renderer" "OK" "$r"
    fi
    rs=$(systemctl is-active rustdesk 2>/dev/null)
    if [ "$rs" = "active" ]; then
        doctor_check "rustdesk" "OK" "active"
    else
        doctor_check "rustdesk" "WARN" "$rs"
    fi
    tip=$(get_tailnet_ip)
    if [ -n "$tip" ]; then
        doctor_check "tailscale" "OK" "$tip"
    else
        doctor_check "tailscale" "WARN" "no tailnet IP"
    fi
    git -C "$ROOT_DIR" fetch --quiet 2>/dev/null || true
    local head upstream
    head=$(git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || true)
    upstream=$(git -C "$ROOT_DIR" rev-parse '@{u}' 2>/dev/null || true)
    if [ -z "$head" ] || [ -z "$upstream" ]; then
        doctor_check "update" "INFO" "cannot check (no remote)"
    elif [ "$head" = "$upstream" ]; then
        doctor_check "update" "OK" "up to date"
    else
        doctor_check "update" "WARN" "update available (res update)"
    fi
}

doctor_fix() {
    local applet_target="$HOME/.local/share/cinnamon/applets/remote-studio@neek"
    echo "Fixing common issues..."
    [ "$(readlink -f "$HOME/.xsessionrc")" != "$ROOT_DIR/config/xsessionrc" ] && ln -sf "$ROOT_DIR/config/xsessionrc" "$HOME/.xsessionrc"
    mkdir -p "$applet_target"; for f in applet.js metadata.json; do [ "$(readlink -f "$applet_target/$f")" != "$ROOT_DIR/applet/$f" ] && ln -sf "$ROOT_DIR/applet/$f" "$applet_target/$f"; done
    [ ! -f "$HOME/.config/rustdesk/RustDesk_default.toml" ] && { mkdir -p "$HOME/.config/rustdesk"; cp "$ROOT_DIR/config/RustDesk_default.toml" "$HOME/.config/rustdesk/RustDesk_default.toml"; }
    echo "Done."
}

show_tailnet() {
    local ip
    ip=$(get_tailnet_ip)
    [ -z "$ip" ] && { echo "Tailscale IPv4 unavailable."; return 1; }
    echo "Tailscale IP: $ip"
    echo "RustDesk direct: $ip:21118"
    local exit_status
    exit_status=$(tailscale exit-node list 2>/dev/null | grep "selected" | awk '{print $1}' || true)
    [ -n "$exit_status" ] && echo "Exit node: $exit_status" || echo "Exit node: none"
}

show_tailnet_hosts() {
    echo "Tailnet peers:"
    tailscale status --peers=true 2>/dev/null | tail -n +2 | awk '{printf "  %-20s %s\n", $2, $1}'
}

merge_rustdesk_config() {
    local source=$1 target=$2
    [ -f "$target" ] || { cp "$source" "$target"; return 0; }
    local preserve=("id" "key" "password" "salt" "relay-server" "api-server")
    local tmp_preserve tmp_new val field line
    tmp_preserve=$(mktemp)
    for field in "${preserve[@]}"; do
        val=$(grep "^$field =" "$target" || true)
        [ -n "$val" ] && echo "$val" >> "$tmp_preserve"
    done
    tmp_new=$(mktemp)
    cp "$source" "$tmp_new"
    while read -r line; do
        field=$(echo "$line" | cut -d' ' -f1)
        if grep -q "^$field =" "$tmp_new"; then
            sed -i "s/^$field =.*/$line/" "$tmp_new"
        else
            echo "$line" >> "$tmp_new"
        fi
    done < "$tmp_preserve"
    cp "$tmp_new" "$target"
    rm "$tmp_preserve" "$tmp_new"
}

merge_rustdesk_options() {
    local source=$1 target=$2
    [ -f "$target" ] || { cp "$source" "$target"; return 0; }
    local tmp_new
    tmp_new=$(mktemp)
    cp "$source" "$tmp_new"
    cp "$tmp_new" "$target"
    rm "$tmp_new"
}

show_rustdesk() {
    local config_file="$HOME/.config/rustdesk/RustDesk_default.toml"
    local options_file="$HOME/.config/rustdesk/RustDesk2.options.toml"
    local options_source="$ROOT_DIR/config/RustDesk2.options.toml"
    local preset=${2:-$DEFAULT_RUSTDESK_PRESET}
    local source_file="$ROOT_DIR/config/RustDesk_${preset}.toml"
    case "$1" in
        backup) [ -f "$config_file" ] && { cp "$config_file" "${config_file}.bak.$(date +%F_%T)"; echo "Backed up."; } || echo "No config."; ;;
        diff) [ -f "$config_file" ] && [ -f "$source_file" ] && diff --color=always -u "$config_file" "$source_file" || echo "Missing files (preset: $preset)."; ;;
        apply)
            [ -f "$source_file" ] || { echo "No template $source_file."; return 1; }
            mkdir -p "$(dirname "$config_file")"
            [ -f "$config_file" ] && cp "$config_file" "${config_file}.pre-apply"
            merge_rustdesk_config "$source_file" "$config_file"
            echo "Merged $preset (Identity preserved)."
            if [ -f "$options_source" ]; then
                merge_rustdesk_options "$options_source" "$options_file"
                echo "Merged RustDesk2.options (options only, no identity)."
            fi
            if [ -f "${config_file}.pre-apply" ]; then
                if cmp -s "$config_file" "${config_file}.pre-apply"; then
                    echo "Configuration unchanged. Skipping restart."
                else
                    echo "Configuration changed. Restarting rustdesk..."
                    sudo systemctl restart rustdesk
                fi
            else
                sudo systemctl restart rustdesk
            fi
            ;;
        status)
            local log_file="$HOME/.local/share/rustdesk/log/rustdesk.log"
            [ -f "$log_file" ] || log_file="$HOME/.rustdesk/log/rustdesk.log"
            if [ -f "$log_file" ]; then
                grep -E "(codec|fps|bitrate|connected|disconnected)" "$log_file" | tail -n 20
            else
                echo "RustDesk log not found."
            fi
            local users
            users=$(ss -tnp 2>/dev/null | grep -c -i "rustdesk.*ESTAB" || true)
            echo "Active sessions: $users"
            ;;
        log)
            local nlines=${2:-50}
            journalctl -u rustdesk -n "$nlines" --no-pager 2>/dev/null || echo "journalctl unavailable."
            ;;
        *) echo "Usage: res rustdesk [apply <preset>|backup|diff <preset>|status|log [lines]]"; ;;
    esac
}

show_tailnet_peer() {
    local peer=$1; if [ -z "$peer" ]; then tailscale status --peers=true | head -n 20; return; fi
    echo "Checking $peer..."; tailscale ping "$peer"; echo; tailscale status | grep -i "$peer"
}

show_tailnet_doctor() {
    echo "Tailnet Doctor"
    tailscale netcheck
}

show_session() {
    case "${1:-status}" in
        start) session_start "${2:-mac}" ;;
        stop) session_stop ;;
        status) [ -f "$SESSION_FILE" ] && cat "$SESSION_FILE" || echo "No active session." ;;
        *) echo "Usage: res session start [PROFILE] | stop | status"; return 1 ;;
    esac
}

show_update() {
    if ! git -C "$ROOT_DIR" pull --ff-only; then
        echo "Error: git pull failed. Ensure this is a git repo with a clean working tree." >&2
        exit 1
    fi
    "$ROOT_DIR/install.sh" install
    res version
    log_event "Self-update complete: $VERSION"
}

show_watch() {
    local interval=${1:-5}
    local prev_users=0
    log_event "Watch: started (interval=${interval}s)"
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

show_profiles_list() {
    printf "%-12s %-30s %s\n" "KEY" "LABEL" "SOURCE"
    while IFS='=' read -r key value; do
        [[ "$key" =~ ^[[:space:]]*# ]] && continue; [[ -z "$key" || -z "$value" ]] && continue
        IFS='|' read -r label _ _ _ _ _ <<< "$value"
        printf "%-12s %-30s %s\n" "$(echo "$key" | xargs)" "$(echo "$label" | xargs)" "$DEFAULT_PROFILES"
    done < "$DEFAULT_PROFILES"
    if [ -f "$USER_PROFILES" ]; then
        while IFS='=' read -r key value; do
            [[ "$key" =~ ^[[:space:]]*# ]] && continue; [[ -z "$key" || -z "$value" ]] && continue
            IFS='|' read -r label _ _ _ _ _ <<< "$value"
            printf "%-12s %-30s %s\n" "$(echo "$key" | xargs)" "$(echo "$label" | xargs)" "$USER_PROFILES (override)"
        done < "$USER_PROFILES"
    fi
}

show_config() {
    case "${1:-show}" in
        show)
            echo "# Effective remote-studio config"
            echo "DEFAULT_PROFILE=${DEFAULT_PROFILE}"
            echo "DEFAULT_RUSTDESK_PRESET=${DEFAULT_RUSTDESK_PRESET}"
            echo "AUTO_SESSION=${AUTO_SESSION:-false}"
            [ -f "$USER_CONFIG" ] && echo "# User config: $USER_CONFIG" || echo "# No user config file"
            ;;
        get)
            [ -z "$2" ] && { echo "Usage: res config get KEY"; return 1; }
            grep "^${2}=" "$USER_CONFIG" 2>/dev/null | tail -1 | cut -d'=' -f2- || echo "(not set)"
            ;;
        set)
            [ -z "$2" ] || [ -z "$3" ] && { echo "Usage: res config set KEY VALUE"; return 1; }
            mkdir -p "$(dirname "$USER_CONFIG")"
            if grep -q "^${2}=" "$USER_CONFIG" 2>/dev/null; then
                sed -i "s/^${2}=.*/${2}=${3}/" "$USER_CONFIG"
            else
                echo "${2}=${3}" >> "$USER_CONFIG"
            fi
            echo "Set ${2}=${3} in $USER_CONFIG"
            log_event "Config set: ${2}=${3}"
            ;;
        *) echo "Usage: res config [show|get KEY|set KEY VALUE]"; return 1 ;;
    esac
}

if [ -n "$1" ]; then
    case "$1" in
        custom)
            [ -z "$2" ] || [ -z "$3" ] && { echo "Usage: res custom <width> <height> [scale]"; exit 1; }
            local_s="${4:-1}"
            local_ts=$(awk "BEGIN { printf \"%.1f\", $local_s > 1.0 ? 1.5 : 1.0 }")
            local_cursor=$(awk "BEGIN { printf \"%d\", 24 * $local_s }")
            if apply_all "$2" "$3" "$local_s" "$local_ts" "$local_cursor" "Custom ${2}x${3}"; then
                if [ -t 0 ]; then
                    read -r -p "Save as profile? [y/N] " ans
                    if [[ "$ans" =~ ^[Yy]$ ]]; then
                        read -r -p "Profile key (e.g. 'work'): " pkey
                        mkdir -p "$(dirname "$USER_PROFILES")"
                        echo "${pkey}=Custom ${2}x${3}|${2}|${3}|${local_s}|${local_ts}|${local_cursor}" >> "$USER_PROFILES"
                        echo "Saved to $USER_PROFILES"
                        log_event "Profile saved: $pkey ${2}x${3}"
                    fi
                fi
            fi
            ;;
        status) show_status ;;
        info) show_info ;;
        log) show_log "$2" ;;
        doctor) show_doctor ;;
        doctor-fix) doctor_fix ;;
        tailnet)
            if [ "$2" = "peer" ]; then
                show_tailnet_peer "$3"
            elif [ "$2" = "doctor" ]; then
                show_tailnet_doctor
            elif [ "$2" = "hosts" ]; then
                show_tailnet_hosts
            else
                show_tailnet
            fi
            ;;
        rustdesk) show_rustdesk "$2" "$3" ;;
        xorg) if [ "$2" = "rollback" ]; then rollback_xorg; else generate_xorg "$2"; fi ;;
        session) show_session "$2" "$3" ;;
        update) show_update ;;
        watch) show_watch "${2:-5}" ;;
        rotate) show_rotate "${2:-normal}" ;;
        profiles) show_profiles_list ;;
        config) show_config "$2" "$3" "$4" ;;
        version) echo "$VERSION" ;;
        help|-h|--help) show_help ;;
        speed|theme|night|caf|privacy|clip|service|audio|keys|fix|reset) do_action "$1" ;;
        *)
            if [ -n "${PROFILES[$1]}" ]; then
                apply_profile "$1" && record_recent_profile "$1"
            else
                echo "Unknown command: $1"; exit 1
            fi
            ;;
    esac
    exit 0
fi

get_warning_summary_cached() {
    local now
    now=$(date +%s)
    if [ -z "$_WARN_CACHE" ] || [ $(( now - _WARN_CACHE_TS )) -gt 30 ]; then
        _WARN_CACHE=$(get_warning_summary)
        _WARN_CACHE_TS=$now
    fi
    printf '%s' "$_WARN_CACHE"
}

tui_header() {
    local mode res_str ip wdata wcount wmsg renderer rustdesk_st session_st
    mode=$(get_current_mode)
    res_str=$(get_current_resolution)
    ip=$(get_tailnet_ip)
    wdata=$(get_warning_summary_cached); wcount=${wdata%%|*}; wmsg=${wdata#*|}
    renderer=$(get_renderer_summary 2>/dev/null | sed 's/.*NVIDIA.*/NVIDIA/;s/.*AMD.*/AMD/;s/.*Intel.*/Intel/;s/.*llvmpipe.*/SW-render/')
    rustdesk_st=$(systemctl is-active rustdesk 2>/dev/null || echo "?")
    session_st="$([ -f "$SESSION_FILE" ] && echo "active" || echo "idle")"
    printf 'Mode: %s (%s)  |  IP: %s  |  Session: %s\nRustDesk: %s  |  Renderer: %s  |  Warnings: %s' \
        "$mode" "$res_str" "${ip:-none}" "$session_st" \
        "$rustdesk_st" "$renderer" \
        "$wcount$([ "$wcount" -gt 0 ] && printf ' (%s)' "$wmsg" || true)"
}

# ------------------------------------------------------------------------------
# TUI
# ------------------------------------------------------------------------------

run_panel_command() {
    local title=$1; shift
    local tmp lines cols
    tmp=$(mktemp)
    lines=$(tput lines 2>/dev/null || echo 24)
    cols=$(tput cols 2>/dev/null || echo 90)
    lines=$(( lines > 6 ? lines - 2 : 22 ))
    cols=$(( cols > 10 ? cols - 4 : 86 ))
    { echo "$ $*"; echo; "$@"; } > "$tmp" 2>&1
    whiptail --title "$title" --scrolltext --textbox "$tmp" "$lines" "$cols"
    rm -f "$tmp"
}
confirm_action() { whiptail --title "Confirm" --yesno "$1" 10 70; }

tui_quick() {
    local choice
    while true; do
        choice=$(whiptail --title "Quick Actions" \
            --menu "Common workflows:" \
            22 80 10 \
            "mac-quality"     "Start Mac session + apply Quality preset" \
            "mac-balanced"    "Start Mac session + apply Balanced preset" \
            "mac-speed"       "Start Mac session + apply Speed preset" \
            "ipad-balanced"   "Start iPad session + apply Balanced preset" \
            "stop-reset"      "Stop session + reset display" \
            "fix-and-restart" "Fix clipboard/audio/keys + restart RustDesk" \
            "back"            "Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            mac-quality)
                session_start mac && show_rustdesk apply quality
                whiptail --msgbox "Mac session started with Quality preset." 7 60
                ;;
            mac-balanced)
                session_start mac && show_rustdesk apply balanced
                whiptail --msgbox "Mac session started with Balanced preset." 7 60
                ;;
            mac-speed)
                session_start mac && show_rustdesk apply speed
                whiptail --msgbox "Mac session started with Speed preset." 7 60
                ;;
            ipad-balanced)
                session_start ipad && show_rustdesk apply balanced
                whiptail --msgbox "iPad session started with Balanced preset." 7 60
                ;;
            stop-reset)
                session_stop && do_action reset
                whiptail --msgbox "Session stopped, display reset." 7 55
                ;;
            fix-and-restart)
                do_action fix && do_action service
                whiptail --msgbox "Fixed clipboard/audio/keys, RustDesk restarted." 7 65
                ;;
        esac
    done
}

tui_dashboard() {
    local lines cols body mode res_str renderer rustdesk_st tailscale_st session_info recent_log
    while true; do
        lines=$(tput lines 2>/dev/null || echo 24)
        cols=$(tput cols 2>/dev/null || echo 90)
        lines=$(( lines > 6 ? lines - 2 : 22 ))
        cols=$(( cols > 10 ? cols - 4 : 86 ))
        get_stats
        get_toggle_states
        mode=$(get_current_mode)
        res_str=$(get_current_resolution)
        renderer=$(get_renderer_summary 2>/dev/null)
        rustdesk_st=$(systemctl is-active rustdesk 2>/dev/null || echo "unknown")
        tailscale_st=$(systemctl is-active tailscaled 2>/dev/null || echo "unknown")
        session_info="$([ -f "$SESSION_FILE" ] && grep '^profile=' "$SESSION_FILE" | cut -d= -f2 || echo "none")"
        body="Remote Studio v${VERSION}

DISPLAY
  Mode:        ${mode} (${res_str})
  Renderer:    ${renderer}
  Speed:${S_ST}  Caffeine:${C_ST}  Theme:${T_ST}  Night:${N_ST}

SESSION
  Active:      ${session_info}
  Users:       ${USERS} connected  (${RUSTDESK_CONN_TYPE:-N/A})

NETWORK
  IP:          ${IP_ADDR}
  Latency:     ${PING_STAT}   Temp: ${THERMAL_ALERT}${TEMP}   RAM: ${RAM}

SERVICES
  RustDesk:    ${rustdesk_st}
  Tailscale:   ${tailscale_st}"
        recent_log=""
        if [ -f "$LOG_FILE" ]; then
            recent_log=$(tail -n 3 "$LOG_FILE" 2>/dev/null | sed 's/^/  /')
        fi
        [ -n "$recent_log" ] && body="${body}

RECENT EVENTS
${recent_log}"
        if ! whiptail --title "Dashboard — Remote Studio v${VERSION}" \
            --yes-button "Refresh" --no-button "Close" \
            --yesno "$body" "$lines" "$cols"; then
            return 0
        fi
        # "Refresh" selected — loop and redraw
        _WARN_CACHE=""  # invalidate cache so next render is fresh
    done
}

tui_log_viewer() {
    local choice filter tmp tlines tcols
    while true; do
        choice=$(whiptail --title "Event Log" \
            --menu "View ~/.remote_studio.log:" \
            18 70 6 \
            "tail-20"   "Last 20 entries" \
            "tail-80"   "Last 80 entries" \
            "tail-200"  "Last 200 entries" \
            "filter"    "Search / filter" \
            "all"       "Full log" \
            "back"      "Return" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back)      return 0 ;;
            tail-20)   run_panel_command "Log (last 20)"  show_log 20 ;;
            tail-80)   run_panel_command "Log (last 80)"  show_log 80 ;;
            tail-200)  run_panel_command "Log (last 200)" show_log 200 ;;
            filter)
                filter=$(whiptail --inputbox "Pattern to grep for (case-insensitive):" 9 60 "" 3>&1 1>&2 2>&3) || continue
                [ -z "$filter" ] && continue
                tmp=$(mktemp)
                if [ -f "$LOG_FILE" ]; then
                    grep -i "$filter" "$LOG_FILE" | tail -n 200 > "$tmp"
                fi
                if [ ! -s "$tmp" ]; then
                    whiptail --msgbox "No matches for '$filter'." 7 55
                else
                    tlines=$(tput lines 2>/dev/null || echo 24); tcols=$(tput cols 2>/dev/null || echo 90)
                    tlines=$(( tlines > 6 ? tlines - 2 : 22 )); tcols=$(( tcols > 10 ? tcols - 4 : 86 ))
                    whiptail --title "Log filter: $filter" --scrolltext --textbox "$tmp" "$tlines" "$tcols"
                fi
                rm -f "$tmp"
                ;;
            all)       run_panel_command "Full Log" cat "$LOG_FILE" ;;
        esac
    done
}

sorted_profile_keys() {
    local preferred=(mac mac15 ipad ipad13 iphonel iphonep fallback)
    local seen=()
    # Emit preferred keys that actually exist
    for k in "${preferred[@]}"; do
        if [ -n "${PROFILES[$k]+x}" ]; then
            printf '%s\n' "$k"
            seen+=("$k")
        fi
    done
    # Emit remaining keys (user-added) in alpha order
    for k in $(printf '%s\n' "${!PROFILES[@]}" | sort); do
        local found=0
        for s in "${seen[@]}"; do [ "$s" = "$k" ] && found=1 && break; done
        [ "$found" -eq 0 ] && printf '%s\n' "$k"
    done
}

record_recent_profile() {
    local key=$1
    [ -z "$key" ] && return 0
    mkdir -p "$(dirname "$RECENT_PROFILES_FILE")"
    local tmp; tmp=$(mktemp)
    # New entry first, then existing entries minus this one, capped at 5 lines
    {
        printf '%s\n' "$key"
        [ -f "$RECENT_PROFILES_FILE" ] && grep -v "^${key}\$" "$RECENT_PROFILES_FILE" || true
    } | head -n 5 > "$tmp"
    mv "$tmp" "$RECENT_PROFILES_FILE"
}

get_recent_profiles() {
    [ -f "$RECENT_PROFILES_FILE" ] && cat "$RECENT_PROFILES_FILE" || true
}

tui_profiles() {
    local entries=() current choice key label w h s src marker recent_keys recent_count=0 rk
    current=$(get_current_mode)
    recent_keys=$(get_recent_profiles)
    if [ -n "$recent_keys" ]; then
        while IFS= read -r rk; do
            [ -z "$rk" ] && continue
            [ -z "${PROFILES[$rk]+x}" ] && continue  # profile no longer exists
            IFS='|' read -r label w h s _ _ <<< "${PROFILES[$rk]}"
            marker=""; [ "$label" = "$current" ] && marker="✓ "
            entries+=("$rk" "★ ${marker}${label} ${w}x${h}")
            recent_count=$((recent_count + 1))
        done <<< "$recent_keys"
        [ "$recent_count" -gt 0 ] && entries+=("" "─── all profiles ───")
    fi
    for key in $(sorted_profile_keys); do
        IFS='|' read -r label w h s _ _ <<< "${PROFILES[$key]}"
        if [ -f "$USER_PROFILES" ] && grep -q "^${key}=" "$USER_PROFILES" 2>/dev/null; then
            src="[user]"
        else
            src="[built-in]"
        fi
        marker=""; [ "$label" = "$current" ] && marker="✓ "
        entries+=("$key" "${marker}${label} ${w}x${h} x${s} ${src}")
    done
    entries+=("custom"  "  Enter arbitrary resolution")
    entries+=("manage"  "  Manage user profiles")
    choice=$(whiptail --title "Profiles" --backtitle "Active: $current" \
        --menu "Select a profile to apply:" 24 90 18 "${entries[@]}" \
        3>&1 1>&2 2>&3) || return 0
    [ -z "$choice" ] && return 0  # ignore the divider line
    case "$choice" in
        custom) tui_custom_resolution; return 0 ;;
        manage) tui_manage_profiles;   return 0 ;;
        *)
            if apply_profile "$choice"; then
                record_recent_profile "$choice"
                IFS='|' read -r label _ <<< "${PROFILES[$choice]}"
                whiptail --msgbox "Applied: $label" 7 50
            else
                whiptail --msgbox "Failed to apply profile '$choice'." 7 50
            fi
            ;;
    esac
}

tui_manage_profiles() {
    [ -f "$USER_PROFILES" ] || { whiptail --msgbox "No user profiles at:\n$USER_PROFILES" 8 60; return 0; }
    local entries=() choice key value label w h s
    while IFS='=' read -r key value; do
        [[ "$key" =~ ^# ]] && continue; [[ -z "$key" ]] && continue
        IFS='|' read -r label w h s _ _ <<< "$value"
        entries+=("$key" "$label ${w}x${h} x${s}")
    done < "$USER_PROFILES"
    [ ${#entries[@]} -eq 0 ] && { whiptail --msgbox "No user profiles found." 7 50; return 0; }
    entries+=("back" "Return")
    choice=$(whiptail --title "User Profiles" --menu "Select a profile to manage:" \
        22 70 12 "${entries[@]}" 3>&1 1>&2 2>&3) || return 0
    [ "$choice" = "back" ] && return 0
    # Action submenu
    local action
    action=$(whiptail --title "Manage: $choice" --menu "What would you like to do?" \
        12 55 3 \
        "edit"   "Edit (change resolution)" \
        "delete" "Delete" \
        "back"   "Cancel" \
        3>&1 1>&2 2>&3) || return 0
    case "$action" in
        back)   return 0 ;;
        delete)
            if whiptail --yesno "Delete user profile '$choice'?" 8 50; then
                sed -i "/^${choice}=/d" "$USER_PROFILES"
                log_event "User profile deleted: $choice"
                whiptail --msgbox "Deleted '$choice'." 7 50
            fi
            ;;
        edit)
            local cur_val cur_w cur_h cur_s new_w new_h new_s new_ts new_cursor new_label tmp_profiles
            cur_val=$(grep "^${choice}=" "$USER_PROFILES" | cut -d= -f2-)
            IFS='|' read -r _ cur_w cur_h cur_s _ _ <<< "$cur_val"
            new_w=$(whiptail --inputbox "Width (current: ${cur_w}):" 9 50 "$cur_w" 3>&1 1>&2 2>&3) || return 0
            [[ "$new_w" =~ ^[0-9]+$ ]] || { whiptail --msgbox "Width must be a positive integer." 7 50; return 0; }
            new_h=$(whiptail --inputbox "Height (current: ${cur_h}):" 9 50 "$cur_h" 3>&1 1>&2 2>&3) || return 0
            [[ "$new_h" =~ ^[0-9]+$ ]] || { whiptail --msgbox "Height must be a positive integer." 7 50; return 0; }
            new_s=$(whiptail --inputbox "Scaling (current: ${cur_s}):" 9 50 "$cur_s" 3>&1 1>&2 2>&3) || return 0
            [[ "$new_s" =~ ^[12]$ ]] || { whiptail --msgbox "Scaling must be 1 or 2." 7 50; return 0; }
            new_ts=$(awk "BEGIN { printf \"%.1f\", ($new_s > 1) ? 1.5 : 1.0 }")
            new_cursor=$(awk "BEGIN { printf \"%d\", 24 * $new_s }")
            new_label="Custom ${new_w}x${new_h}"
            tmp_profiles=$(mktemp)
            while IFS= read -r line; do
                if [[ "$line" =~ ^${choice}= ]]; then
                    printf '%s=%s|%s|%s|%s|%s|%s\n' "$choice" "$new_label" "$new_w" "$new_h" "$new_s" "$new_ts" "$new_cursor"
                else
                    printf '%s\n' "$line"
                fi
            done < "$USER_PROFILES" > "$tmp_profiles"
            mv "$tmp_profiles" "$USER_PROFILES"
            log_event "User profile edited: $choice -> ${new_w}x${new_h}"
            whiptail --msgbox "Updated '$choice' to ${new_w}x${new_h}." 7 55
            ;;
    esac
}

tui_custom_resolution() {
    local w h s ts cursor label pkey
    w=$(whiptail --inputbox "Width (pixels):" 9 50 "1920" 3>&1 1>&2 2>&3) || return 0
    [[ "$w" =~ ^[0-9]+$ ]] || { whiptail --msgbox "Width must be a positive integer." 7 50; return 0; }
    h=$(whiptail --inputbox "Height (pixels):" 9 50 "1200" 3>&1 1>&2 2>&3) || return 0
    [[ "$h" =~ ^[0-9]+$ ]] || { whiptail --msgbox "Height must be a positive integer." 7 50; return 0; }
    s=$(whiptail --inputbox "Scaling factor (1 or 2):" 9 50 "1" 3>&1 1>&2 2>&3) || return 0
    [[ "$s" =~ ^[12]$ ]] || { whiptail --msgbox "Scaling must be 1 or 2." 7 50; return 0; }
    ts=$(awk "BEGIN { printf \"%.1f\", ($s > 1) ? 1.5 : 1.0 }")
    cursor=$(awk "BEGIN { printf \"%d\", 24 * $s }")
    label="Custom ${w}x${h}"
    if apply_all "$w" "$h" "$s" "$ts" "$cursor" "$label"; then
        if whiptail --yesno "Applied ${w}x${h}.\n\nSave as a named user profile?" 9 60; then
            pkey=$(whiptail --inputbox "Profile key (e.g. 'work', 'tv'):" 9 50 "" 3>&1 1>&2 2>&3) || return 0
            if [[ "$pkey" =~ ^[a-z][a-z0-9_-]*$ ]]; then
                mkdir -p "$(dirname "$USER_PROFILES")"
                echo "${pkey}=${label}|${w}|${h}|${s}|${ts}|${cursor}" >> "$USER_PROFILES"
                log_event "Custom profile saved: $pkey ${w}x${h}"
                whiptail --msgbox "Saved as '$pkey'." 7 50
            else
                whiptail --msgbox "Invalid key — use only a-z, 0-9, _, - and start with a letter." 8 64
            fi
        fi
    else
        whiptail --msgbox "Failed to apply ${w}x${h}." 7 50
    fi
}

tui_config() {
    local choice key val lines cols
    lines=$(tput lines 2>/dev/null || echo 24)
    cols=$(tput cols 2>/dev/null || echo 90)
    lines=$(( lines > 6 ? lines - 2 : 22 ))
    cols=$(( cols > 10 ? cols - 4 : 86 ))
    while true; do
        local _auto_val
        _auto_val=$(show_config get AUTO_SESSION 2>/dev/null || echo "false")
        choice=$(whiptail --title "Config" --menu "Remote Studio Configuration" "$lines" "$cols" 6 \
            "show"        "Show Effective Config" \
            "set-profile" "Default Profile         [${DEFAULT_PROFILE}]" \
            "set-preset"  "Default RustDesk Preset [${DEFAULT_RUSTDESK_PRESET}]" \
            "set-auto"    "AUTO_SESSION            [${_auto_val}]" \
            "set-custom"  "Set Arbitrary Key=Value" \
            "back"        "Return" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            show) run_panel_command "Config" show_config show ;;
            set-profile)
                val=$(whiptail --inputbox "DEFAULT_PROFILE (current: ${DEFAULT_PROFILE})" 9 60 "${DEFAULT_PROFILE}" 3>&1 1>&2 2>&3) || continue
                [ -n "$val" ] && show_config set DEFAULT_PROFILE "$val" && DEFAULT_PROFILE="$val" && whiptail --msgbox "Set DEFAULT_PROFILE=$val" 7 50
                ;;
            set-preset)
                val=$(whiptail --inputbox "DEFAULT_RUSTDESK_PRESET (current: ${DEFAULT_RUSTDESK_PRESET})" 9 60 "${DEFAULT_RUSTDESK_PRESET}" 3>&1 1>&2 2>&3) || continue
                [ -n "$val" ] && show_config set DEFAULT_RUSTDESK_PRESET "$val" && DEFAULT_RUSTDESK_PRESET="$val" && whiptail --msgbox "Set DEFAULT_RUSTDESK_PRESET=$val" 7 50
                ;;
            set-auto)
                if [ "$_auto_val" = "true" ]; then
                    show_config set AUTO_SESSION false && whiptail --msgbox "AUTO_SESSION=false" 7 50
                else
                    show_config set AUTO_SESSION true && whiptail --msgbox "AUTO_SESSION=true" 7 50
                fi
                ;;
            set-custom)
                local ck cv
                ck=$(whiptail --inputbox "Config key:" 9 50 "" 3>&1 1>&2 2>&3) || continue
                [ -z "$ck" ] && continue
                cv=$(whiptail --inputbox "Value for ${ck}:" 9 50 "" 3>&1 1>&2 2>&3) || continue
                show_config set "$ck" "$cv" && whiptail --msgbox "Set ${ck}=${cv}" 7 55
                ;;
        esac
    done
}

tui_performance() {
    local choice current_rotation profile p_entries key label w h s
    while true; do
        get_toggle_states
        current_rotation=$(xrandr 2>/dev/null | grep " connected" | grep -o "normal\|left\|right\|inverted" | head -1 || echo "normal")
        choice=$(whiptail --title "Performance & Session" \
            --menu "Session: $([ -f "$SESSION_FILE" ] && grep '^profile=' "$SESSION_FILE" | cut -d= -f2 || echo idle)" \
            24 86 10 \
            "session-start" "Start Session (choose profile)" \
            "session-stop"  "Stop Session & Restore State" \
            "speed"         "Toggle Speed Mode         [$S_ST]" \
            "caf"           "Toggle Caffeine           [$C_ST]" \
            "theme"         "Toggle Theme              [$T_ST]" \
            "night"         "Toggle Night Shift        [$N_ST]" \
            "rotate"        "Rotate Display            [$current_rotation]" \
            "privacy"       "Lock Screen & Blank Monitor" \
            "fix"           "Fix Clipboard / Audio / Keys" \
            "back"          "Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            session-start)
                p_entries=()
                for key in $(sorted_profile_keys); do
                    IFS='|' read -r label w h s _ _ <<< "${PROFILES[$key]}"
                    p_entries+=("$key" "$label ${w}x${h}")
                done
                profile=$(whiptail --title "Start Session" \
                    --menu "Select profile for this session:" \
                    20 70 10 "${p_entries[@]}" \
                    3>&1 1>&2 2>&3) || continue
                session_start "$profile"
                whiptail --msgbox "Session started: $profile" 7 50
                ;;
            session-stop)
                session_stop
                whiptail --msgbox "Session stopped. State restored." 7 50
                ;;
            rotate)
                local rot_choice
                rot_choice=$(whiptail --title "Rotate Display" \
                    --menu "Select orientation:" 12 52 4 \
                    "normal"   "Normal (landscape)" \
                    "left"     "Left (portrait CW)" \
                    "right"    "Right (portrait CCW)" \
                    "inverted" "Inverted" \
                    3>&1 1>&2 2>&3) || continue
                show_rotate "$rot_choice"
                ;;
            privacy) do_action privacy ;;
            *) do_action "$choice" ;;
        esac
    done
}

tui_tailnet() {
    local choice n
    while true; do
        choice=$(whiptail --title "Tailnet" \
            --menu "Tailscale network tools:" \
            20 72 8 \
            "address"   "Show Tailscale IP & RustDesk Direct" \
            "hosts"     "List All Tailnet Peers" \
            "peer"      "Check Specific Peer (ping + path)" \
            "doctor"    "Tailnet Path Doctor (netcheck)" \
            "exit-node" "Show Exit Node Status" \
            "back"      "Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back)      return 0 ;;
            address)   run_panel_command "Tailnet Address" show_tailnet ;;
            hosts)     run_panel_command "Tailnet Hosts"   show_tailnet_hosts ;;
            peer)
                n=$(whiptail --inputbox "Peer hostname or Tailscale IP:" 9 55 "" 3>&1 1>&2 2>&3) || continue
                [ -n "$n" ] && run_panel_command "Peer: $n" show_tailnet_peer "$n"
                ;;
            doctor)    run_panel_command "Tailnet Doctor" show_tailnet_doctor ;;
            exit-node)
                local en
                en=$(tailscale exit-node list 2>/dev/null | grep -i "selected" | awk '{print $1}' || true)
                whiptail --msgbox "Exit node: ${en:-none}" 7 50
                ;;
        esac
    done
}

tui_diagnostics() {
    local choice n
    while true; do
        choice=$(whiptail --title "Diagnostics" \
            --menu "Inspect system health:" \
            24 88 10 \
            "doctor"          "Full Health Report" \
            "fix-all"         "Auto-Repair Issues" \
            "tailnet"         "Tailscale Tools" \
            "rustdesk-status" "RustDesk Session Status" \
            "profiles"        "List All Profiles (with source)" \
            "watch-status"    "Session / Watcher State" \
            "log"             "Event Log Viewer" \
            "back"            "Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back)             return 0 ;;
            doctor)           run_panel_command "Doctor" show_doctor ;;
            fix-all)          run_panel_command "Repair" doctor_fix ;;
            tailnet)          tui_tailnet ;;
            rustdesk-status)  run_panel_command "RustDesk Status" show_rustdesk status ;;
            profiles)         run_panel_command "Profiles" show_profiles_list ;;
            watch-status)
                if [ -f "$SESSION_FILE" ]; then
                    run_panel_command "Session State" cat "$SESSION_FILE"
                else
                    whiptail --msgbox "No active session state." 7 50
                fi
                ;;
            log)              tui_log_viewer ;;
        esac
    done
}

tui_rustdesk() {
    local choice
    while true; do
        choice=$(whiptail --title "RustDesk" \
            --menu "Config, presets, and service:" \
            22 88 10 \
            "status"         "Session Status (codec / FPS)" \
            "log"            "Service Log (50 lines)" \
            "backup"         "Backup Config" \
            "diff"           "Diff Active vs Template" \
            "apply-quality"  "Apply Quality Preset" \
            "apply-balanced" "Apply Balanced Preset" \
            "apply-speed"    "Apply Speed Preset" \
            "service"        "Restart RustDesk Service" \
            "back"           "System Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            apply-*)
                local p=${choice#apply-}
                confirm_action "Safe-merge '$p' preset and restart RustDesk?" && \
                    run_panel_command "RustDesk — $p" show_rustdesk apply "$p"
                ;;
            *) run_panel_command "RustDesk — $choice" show_rustdesk "$choice" ;;
        esac
    done
}

tui_system() {
    local choice
    while true; do
        choice=$(whiptail --title "System & Tools" \
            --menu "Installation, Xorg, and maintenance:" \
            24 88 12 \
            "rustdesk"      "RustDesk Tools" \
            "config"        "Configuration" \
            "update"        "Update Remote Studio (git pull)" \
            "xorg-preview"  "Preview Generated Xorg Config" \
            "xorg-write"    "Write /etc/X11/xorg.conf (sudo)" \
            "xorg-rollback" "Rollback /etc/X11/xorg.conf" \
            "install"       "Re-run User Install" \
            "backup"        "Full Config Backup" \
            "reset"         "Reset Display (1024x768)" \
            "back"          "Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back)          return 0 ;;
            rustdesk)      tui_rustdesk ;;
            config)        tui_config ;;
            update)
                confirm_action "Pull latest from git and reinstall?" && \
                    run_panel_command "Update" show_update
                ;;
            xorg-preview)  run_panel_command "Xorg Config Preview" generate_xorg ;;
            xorg-write)
                confirm_action "Write generated Xorg config to /etc/X11/xorg.conf?\n(Requires sudo. Restart LightDM to apply.)" && \
                    run_panel_command "Write Xorg" "$ROOT_DIR/install.sh" system
                ;;
            xorg-rollback)
                confirm_action "Restore /etc/X11/xorg.conf from latest backup?" && \
                    run_panel_command "Xorg Rollback" rollback_xorg
                ;;
            install)  run_panel_command "Install" "$ROOT_DIR/install.sh" install ;;
            backup)   run_panel_command "Backup" "$ROOT_DIR/install.sh" backup ;;
            reset)    do_action reset ;;
        esac
    done
}

show_text_menu() {
    local choice
    while true; do
        clear
        echo -e "${CYAN}=== Remote Studio v${VERSION} ===${NC}"
        tui_header
        echo ""
        echo "  1) Profiles       4) System"
        echo "  2) Performance    5) Dashboard"
        echo "  3) Diagnostics    6) Tailnet"
        echo "  7) Quick Actions  8) Help"
        echo "  0) Exit"
        echo ""
        read -r -p "Select: " choice
        case "$choice" in
            1) tui_profiles ;;
            2) tui_performance ;;
            3) tui_diagnostics ;;
            4) tui_system ;;
            5) tui_dashboard ;;
            6) tui_tailnet ;;
            7) tui_quick ;;
            8) show_help | "${PAGER:-less}" ;;
            0|q|Q) exit 0 ;;
        esac
    done
}

[ "$(tput lines 2>/dev/null || echo 0)" -lt 18 ] && show_text_menu
while true; do
    _m=$(get_current_mode)
    _r=$(get_current_resolution)
    _t=$(get_tailnet_ip)
    _u=$(ss -tnp 2>/dev/null | grep -ic "rustdesk.*ESTAB" || true)
    _wdata=$(get_warning_summary_cached); _w=${_wdata%%|*}
    choice=$(whiptail \
        --title "Remote Studio v${VERSION}" \
        --backtitle "Mode: $_m ($_r)  |  IP: ${_t:-none}  |  Users: $_u  |  Warnings: $_w" \
        --menu "$(tui_header)" 24 92 10 \
        "profiles"    "Display Profiles" \
        "quick"       "Quick Actions" \
        "performance" "Session & Toggles" \
        "diagnostics" "Diagnostics" \
        "tailnet"     "Tailscale Network" \
        "system"      "System & Tools" \
        "dashboard"   "Live Dashboard" \
        "help"        "Help" \
        "exit"        "Quit" \
        3>&1 1>&2 2>&3) || exit 0
    case "$choice" in
        profiles)    tui_profiles ;;
        quick)       tui_quick ;;
        performance) tui_performance ;;
        diagnostics) tui_diagnostics ;;
        tailnet)     tui_tailnet ;;
        system)      tui_system ;;
        dashboard)   tui_dashboard ;;
        help)        run_panel_command "Help" show_help ;;
        exit)        exit 0 ;;
    esac
done

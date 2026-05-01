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
    for key in $(echo "${!PROFILES[@]}" | tr ' ' '\n' | sort); do
        IFS='|' read -r label width height scaling _ _ <<< "${PROFILES[$key]}"
        printf "  %-12s %s (%dx%d @%sx)\n" "$key" "$label" "$width" "$height" "$scaling"
    done
    echo ""; echo "Actions:"
    echo "  speed, theme, night, caf, privacy, fix, reset, service, audio, keys"
    echo "  doctor, doctor-fix, tailnet, tailnet peer <name>, rustdesk [apply|backup|diff]"
    echo "  xorg [PATH], session [start|stop|status]"
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
        *) echo "Usage: res rustdesk [apply <preset>|backup|diff <preset>]"; ;;
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

handle_custom() {
    local w=$1 h=$2 s="${3:-1}"
    [ -z "$w" ] || [ -z "$h" ] && exit 1
    apply_all "$w" "$h" "$s" "$s" "$(awk "BEGIN { printf \"%d\", 24 * $s }")" "Custom ${w}x${h}"
}

if [ -n "$1" ]; then
    case "$1" in
        custom) handle_custom "$2" "$3" "$4" ;;
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
            else
                show_tailnet
            fi
            ;;
        rustdesk) show_rustdesk "$2" "$3" ;;
        xorg) if [ "$2" = "rollback" ]; then rollback_xorg; else generate_xorg "$2"; fi ;;
        session) show_session "$2" "$3" ;;
        version) echo "$VERSION" ;;
        help|-h|--help) show_help ;;
        speed|theme|night|caf|privacy|clip|service|audio|keys|fix|reset) do_action "$1" ;;
        *)
            if [ -n "${PROFILES[$1]}" ]; then
                apply_profile "$1"
            else
                echo "Unknown command: $1"; exit 1
            fi
            ;;
    esac
    exit 0
fi

# ------------------------------------------------------------------------------
# TUI
# ------------------------------------------------------------------------------

run_panel_command() {
    local title=$1; shift
    local tmp
    tmp=$(mktemp)
    { echo "$ $*"; echo; "$@"; } > "$tmp" 2>&1
    whiptail --title "$title" --scrolltext --textbox "$tmp" 24 90
    rm -f "$tmp"
}
confirm_action() { whiptail --title "Confirm" --yesno "$1" 10 70; }

tui_profiles() {
    local entries=() current choice
    current=$(get_current_mode)
    # shellcheck disable=SC2034
    for key in $(printf '%s\n' "${!PROFILES[@]}" | sort); do IFS='|' read -r label w h s ts c <<< "${PROFILES[$key]}"; entries+=("$key" "$label ${w}x${h} scale=${s}"); done
    entries+=("custom" "Enter arbitrary resolution")
    choice=$(whiptail --title "Profiles" --backtitle "Current: $current" --menu "Select Profile" 24 90 14 "${entries[@]}" 3>&1 1>&2 2>&3) || return 0
    [ "$choice" = "custom" ] && { tui_custom_resolution; return 0; }
    apply_profile "$choice" && whiptail --msgbox "Applied $choice" 8 50
}

tui_custom_resolution() {
    local w h s
    w=$(whiptail --inputbox "Width" 9 50 "1920" 3>&1 1>&2 2>&3) || return 0
    h=$(whiptail --inputbox "Height" 9 50 "1200" 3>&1 1>&2 2>&3) || return 0
    s=$(whiptail --inputbox "Scaling" 9 50 "1" 3>&1 1>&2 2>&3) || return 0
    apply_all "$w" "$h" "$s" "$s" "$(awk "BEGIN { printf \"%d\", 24 * $s }")" "Custom ${w}x${h}"
}

tui_performance() {
    local choice
    while true; do
        get_toggle_states; choice=$(whiptail --title "Performance" --menu "Adjust Session" 20 82 10 "speed" "Toggle Speed Mode ($S_ST)" "caf" "Toggle Caffeine ($C_ST)" "theme" "Toggle Theme ($T_ST)" "night" "Toggle Night Shift ($N_ST)" "fix" "Fix Common Issues" "session-start" "Start Mac Session" "session-stop" "Stop Session" "back" "Main Menu" 3>&1 1>&2 2>&3) || return 0
        case "$choice" in back) return 0 ;; session-start) session_start mac ;; session-stop) session_stop ;; *) do_action "$choice" ;; esac
    done
}

tui_diagnostics() {
    local choice n
    while true; do
        choice=$(whiptail --title "Diagnostics" --menu "Inspect Stack" 20 86 11 "doctor" "Health Report" "fix-all" "Auto-Repair Issues" "tailnet" "Show Address" "peer" "Check Peer" "tailnet-doctor" "Tailnet Doctor" "log" "Event Log" "back" "Main Menu" 3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            doctor) run_panel_command "Doctor" show_doctor ;;
            fix-all) run_panel_command "Repair" doctor_fix ;;
            tailnet) run_panel_command "Tailnet" show_tailnet ;;
            peer) n=$(whiptail --inputbox "Peer name" 9 50 3>&1 1>&2 2>&3); [ -n "$n" ] && run_panel_command "Peer: $n" show_tailnet_peer "$n" ;;
            tailnet-doctor) run_panel_command "Tailnet Doctor" show_tailnet_doctor ;;
            log) run_panel_command "Log" show_log 80 ;;
        esac
    done
}

tui_rustdesk() {
    local choice
    while true; do
        choice=$(whiptail --title "RustDesk" --menu "Config & Service" 20 86 12 "backup" "Backup Config" "diff" "Diff vs Template" "apply-quality" "Apply Quality Preset" "apply-balanced" "Apply Balanced Preset" "apply-speed" "Apply Speed Preset" "service" "Restart Service" "back" "System Menu" 3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            apply-*)
                local p=${choice#apply-}
                confirm_action "Safe-merge $p preset and restart?" && run_panel_command "RustDesk Apply $p" show_rustdesk apply "$p"
                ;;
            *) run_panel_command "RustDesk" show_rustdesk "$choice" ;;
        esac
    done
}

tui_system() {
    local choice
    while true; do
        choice=$(whiptail --title "System" --menu "Operations" 20 86 10 "rustdesk" "RustDesk Tools" "xorg-preview" "Preview Xorg" "install" "User Install" "backup" "Full Backup" "reset" "Reset (1024x768)" "back" "Main Menu" 3>&1 1>&2 2>&3) || return 0
        case "$choice" in back) return 0 ;; rustdesk) tui_rustdesk ;; xorg-preview) run_panel_command "Xorg" generate_xorg ;; install) run_panel_command "Install" "$ROOT_DIR/install.sh" install ;; backup) run_panel_command "Backup" "$ROOT_DIR/install.sh" backup ;; reset) do_action reset ;; esac
    done
}

show_text_menu() {
    local choice
    while true; do
        clear; echo -e "${CYAN}Remote Studio${NC}"; tui_header; echo "1) Profiles 2) Performance 3) Diagnostics 4) System 5) Exit"
        read -r -p "Select: " choice; case "$choice" in 1) tui_profiles ;; 2) tui_performance ;; 3) tui_diagnostics ;; 4) tui_system ;; 5) exit 0 ;; esac
    done
}

[ "$(tput lines 2>/dev/null || echo 0)" -lt 18 ] && show_text_menu
while true; do
    m=$(get_current_mode); r=$(get_current_resolution); t=$(get_tailnet_ip)
    u=$(ss -tnp 2>/dev/null | grep -i "rustdesk" | grep -ic "ESTAB")
    choice=$(whiptail --title "Remote Studio" --backtitle "Mode: $m ($r) | Tailnet: ${t:-none} | Users: $u" --menu "$(tui_header)" 24 92 7 "profiles" "Display Profiles" "performance" "Session Adjust" "diagnostics" "Diagnostics" "system" "System & Tools" "help" "Help" "exit" "Quit" 3>&1 1>&2 2>&3) || exit 0
    case "$choice" in profiles) tui_profiles ;; performance) tui_performance ;; diagnostics) tui_diagnostics ;; system) tui_system ;; help) run_panel_command "Help" show_help ;; exit) exit 0 ;; esac
done

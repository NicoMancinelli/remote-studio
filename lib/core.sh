#!/bin/bash
# Remote Studio — core helpers, profile loading, state, colors

# Colors
# shellcheck disable=SC2034
RED='\033[1;31m'
GREEN='\033[1;32m'
CYAN='\033[1;36m'
WHITE='\033[1;37m'
BOLD='\033[1m'
YELLOW='\033[1;33m'
DIM='\033[2m'
NC='\033[0m'

log_event() {
    if [ -f "$LOG_FILE" ]; then
        local size
        size=$(stat -c%s "$LOG_FILE" 2>/dev/null || stat -f%z "$LOG_FILE" 2>/dev/null || echo 0)
        if [ "$size" -gt 1048576 ]; then
            mv "$LOG_FILE" "${LOG_FILE}.1"
        fi
    fi
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

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

# Ping cache: result is written to a file so a background subshell can
# communicate it back to any future caller (shell variables written in
# a background & subshell are discarded when it exits).
_ping_cache_file() { printf '%s/.ping_cache' "$STATUS_DIR"; }

_refresh_ping_cache() {
    local result f
    f="$(_ping_cache_file)"
    mkdir -p "$(dirname "$f")" 2>/dev/null || true
    result=$(ping -c 1 -W 1 8.8.8.8 2>/dev/null \
        | grep 'time=' | awk -F'time=' '{print $2}' | cut -d' ' -f1 | cut -d'.' -f1)
    printf '%s\n%s\n' "$(date +%s)" "${result}" > "$f" 2>/dev/null || true
}

get_ping_cached() {
    local now ts val f
    f="$(_ping_cache_file)"
    now=$(date +%s)
    if [ -f "$f" ]; then
        { IFS= read -r ts && IFS= read -r val; } < "$f"
        if [ "$(( now - ${ts:-0} ))" -le 30 ]; then
            printf '%s' "$val"
            return 0
        fi
    fi
    # Cache cold or stale — kick off background refresh; return stale value (or empty)
    _refresh_ping_cache &
    printf '%s' "${val:-}"
}

get_stats() {
    IP_ADDR=$(get_primary_ip)
    TEMP=$(sensors 2>/dev/null | grep "Package id 0" | awk '{print $4}' | tr -d '+')
    RAM=$(free -m | awk 'NR==2{printf "%.1f%%", $3*100/$2 }')
    USERS=$(ss -tnp 2>/dev/null | awk '/ESTAB/ && /rustdesk/{print $5}' | cut -d: -f1 | sort -u | wc -l)

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

    PING_RAW=$(get_ping_cached)
    [ -z "$PING_RAW" ] && PING_STAT="…" || PING_STAT="${PING_RAW}ms"
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
    current=$(xrandr 2>/dev/null | awk '/ connected/ {out=$1} /\*/ {print out " " $1; exit}')

    # Single tailscale subprocess: extract both IP and BackendState from one JSON call
    local ts_json ts_ip ts_state
    ts_json=$(tailscale status --json 2>/dev/null)
    ts_ip=$(printf '%s' "$ts_json" | grep -o '"TailscaleIPs":\["[^"]*"' | grep -o '[0-9][0-9.]*' | head -1)
    ts_state=$(printf '%s' "$ts_json" | grep -o '"BackendState":"[^"]*"' | cut -d'"' -f4)

    if [[ "$renderer" == *llvmpipe* ]]; then warnings=$((warnings + 1)); messages+=("software-rendering"); fi
    if [ "$rustdesk_state" != "active" ]; then warnings=$((warnings + 1)); messages+=("rustdesk-${rustdesk_state:-unknown}"); fi
    if [ "$tailscale_state" != "active" ] || [ -z "$ts_ip" ]; then warnings=$((warnings + 1)); messages+=("tailscale"); fi
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
    fi

    # Auth / connectivity states (only when daemon is active, to avoid double-counting)
    if [ "$tailscale_state" = "active" ]; then
        case "$ts_state" in
            NeedsLogin|Stopped)
                warnings=$((warnings + 1)); messages+=("tailscale-${ts_state,,}") ;;
            NoState|Starting|NoNetwork)
                warnings=$((warnings + 1)); messages+=("tailscale-offline") ;;
        esac
    fi

    if [ "$warnings" -eq 0 ]; then
        echo "0|OK"
    else
        local IFS=','
        echo "$warnings|${messages[*]}"
    fi
}

get_warning_summary_cached() {
    local now
    now=$(date +%s)
    if [ -z "$_WARN_CACHE" ] || [ $(( now - _WARN_CACHE_TS )) -gt 30 ]; then
        _WARN_CACHE=$(get_warning_summary)
        _WARN_CACHE_TS=$now
    fi
    printf '%s' "$_WARN_CACHE"
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

get_toggle_states() {
    SPD=$(gsettings get org.cinnamon desktop-effects 2>/dev/null); [ "$SPD" == "false" ] && S_ST="ON" || S_ST="OFF"
    CAF=$(gsettings get org.cinnamon.desktop.screensaver lock-enabled 2>/dev/null); [ "$CAF" == "false" ] && C_ST="ON" || C_ST="OFF"
    THM=$(gsettings get org.cinnamon.desktop.interface gtk-theme 2>/dev/null | tr -d "'"); [[ "$THM" == *"Dark"* ]] && T_ST="Dark" || T_ST="Light"
    GAMMA=$(xgamma 2>&1 | awk '{print $4}'); [[ "$GAMMA" != "1.000" ]] && N_ST="ON" || N_ST="OFF"
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
        PROFILES["$key"]="$value"
    done < "$file"
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

speed_state() { [ "$(gsettings get org.cinnamon desktop-effects 2>/dev/null)" = "false" ] && echo "ON" || echo "OFF"; }
caffeine_state() { [ "$(gsettings get org.cinnamon.desktop.screensaver lock-enabled 2>/dev/null)" = "false" ] && echo "ON" || echo "OFF"; }

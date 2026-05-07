#!/bin/bash
# ==============================================================================
# Remote Studio — entrypoint
# ==============================================================================
set -uo pipefail
IFS=$'\n\t'

VERSION="8.1"

# ---- Resolve script root and library directory ----
ROOT_DIR="$(cd "$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")" && pwd)"
if [ -d "$ROOT_DIR/lib" ]; then
    LIB_DIR="$ROOT_DIR/lib"
elif [ -d "/usr/share/remote-studio/lib" ]; then
    LIB_DIR="/usr/share/remote-studio/lib"
else
    echo "ERROR: cannot locate Remote Studio lib/ directory" >&2
    exit 1
fi

# ---- Path constants (consumed by sourced lib modules) ----
# shellcheck disable=SC2034
{
STATE_FILE="$HOME/.res_state"
WALLPAPER_BACKUP="$HOME/.wallpaper_backup"
LOG_FILE="$HOME/.remote_studio.log"
SESSION_FILE="$HOME/.config/remote-studio/session.state"
DEFAULT_PROFILES="$ROOT_DIR/config/profiles.conf"
[ -f "$DEFAULT_PROFILES" ] || DEFAULT_PROFILES="/usr/share/remote-studio/profiles.conf"
USER_PROFILES="$HOME/.config/remote-studio/profiles.conf"
USER_CONFIG="$HOME/.config/remote-studio/remote-studio.conf"
RECENT_PROFILES_FILE="$HOME/.config/remote-studio/recent_profiles"
if [ -n "${XDG_RUNTIME_DIR:-}" ] && [ -w "$XDG_RUNTIME_DIR" ]; then
    STATUS_DIR="$XDG_RUNTIME_DIR/remote-studio"
else
    STATUS_DIR="/tmp/remote-studio-${UID:-$(id -u)}"
fi
STATUS_FILE="$STATUS_DIR/status"
}

# ---- Load user config ----
# shellcheck source=/dev/null
[ -f "$USER_CONFIG" ] && source "$USER_CONFIG"
DEFAULT_PROFILE="${DEFAULT_PROFILE:-mac}"
DEFAULT_RUSTDESK_PRESET="${DEFAULT_RUSTDESK_PRESET:-default}"

# ---- Cache state ----
_WARN_CACHE=""
_WARN_CACHE_TS=0

# ---- Profile registry ----
declare -A PROFILES=()

# ---- Source library modules (order matters: core first) ----
# shellcheck source=lib/core.sh disable=SC1091
source "$LIB_DIR/core.sh"
# shellcheck source=lib/engine.sh disable=SC1091
source "$LIB_DIR/engine.sh"
# shellcheck source=lib/diagnostics.sh disable=SC1091
source "$LIB_DIR/diagnostics.sh"
# shellcheck source=lib/services.sh disable=SC1091
source "$LIB_DIR/services.sh"
# shellcheck source=lib/config.sh disable=SC1091
source "$LIB_DIR/config.sh"
# shellcheck source=lib/tui.sh disable=SC1091
source "$LIB_DIR/tui.sh"

# ---- Load profiles (now that load_profiles_file is defined) ----
load_profiles_file "$DEFAULT_PROFILES"
load_profiles_file "$USER_PROFILES"

# ---- CLI dispatch ----
if [ -n "${1:-}" ]; then
    case "$1" in
        custom)
            { [ -z "${2:-}" ] || [ -z "${3:-}" ]; } && { echo "Usage: res custom <width> <height> [scale]"; exit 1; }
            local_s="${4:-1}"
            local_ts=$(awk "BEGIN { printf \"%.1f\", $local_s > 1.0 ? 1.5 : 1.0 }")
            local_cursor=$(awk "BEGIN { printf \"%d\", 24 * $local_s }")
            if apply_all "$2" "$3" "$local_s" "$local_ts" "$local_cursor" "Custom ${2}x${3}"; then
                if [ -t 0 ]; then
                    read -r -p "Save as profile? [y/N] " ans
                    if [[ "$ans" =~ ^[Yy]$ ]]; then
                        read -r -p "Profile key (e.g. 'work'): " pkey
                        if [[ ! "$pkey" =~ ^[a-z][a-z0-9_-]*$ ]]; then
                            echo "Invalid key '$pkey': use lowercase letters, digits, hyphens, underscores (e.g. 'work')"
                        else
                            mkdir -p "$(dirname "$USER_PROFILES")"
                            echo "${pkey}=Custom ${2}x${3}|${2}|${3}|${local_s}|${local_ts}|${local_cursor}" >> "$USER_PROFILES"
                            echo "Saved to $USER_PROFILES"
                            log_event "Profile saved: $pkey ${2}x${3}"
                        fi
                    fi
                fi
            fi
            ;;
        status) show_status ;;
        info) show_info ;;
        log) show_log "${2:-20}" ;;
        doctor) show_doctor ;;
        doctor-fix) doctor_fix ;;
        self-test) show_self_test ;;
        init) show_init_wizard ;;
        tailnet)
            if [ "${2:-}" = "peer" ]; then
                show_tailnet_peer "${3:-}"
            elif [ "${2:-}" = "doctor" ]; then
                show_tailnet_doctor
            elif [ "${2:-}" = "hosts" ]; then
                show_tailnet_hosts
            else
                show_tailnet
            fi
            ;;
        rustdesk) show_rustdesk "${2:-}" "${3:-}" ;;
        xorg) if [ "${2:-}" = "rollback" ]; then rollback_xorg; else generate_xorg "${2:-}"; fi ;;
        session) show_session "${2:-}" "${3:-}" ;;
        update) show_update ;;
        watch) show_watch "${2:-5}" ;;
        rotate) show_rotate "${2:-normal}" ;;
        profiles) show_profiles_list ;;
        config) show_config "${2:-}" "${3:-}" "${4:-}" ;;
        version) echo "$VERSION" ;;
        help|-h|--help) show_help ;;
        speed|theme|night|caf|privacy|clip|service|audio|keys|fix|reset) do_action "$1" ;;
        *)
            if [ -n "${PROFILES[$1]:-}" ]; then
                apply_profile "$1" && record_recent_profile "$1"
            else
                echo "Unknown command: $1"; exit 1
            fi
            ;;
    esac
    exit
fi

# ---- TUI main loop ----
# Graceful fallback: text menu if whiptail is not installed
if ! command -v whiptail >/dev/null 2>&1; then
    echo "Note: whiptail not installed - using text menu (apt install whiptail for the full TUI)"
    show_text_menu
fi

# Calculate dynamic TUI dimensions
_T_LINES=$(tput lines 2>/dev/null || echo 24)
_T_COLS=$(tput cols 2>/dev/null || echo 90)

# If terminal is too small, fallback to text menu
if [ "$_T_LINES" -lt 15 ] || [ "$_T_COLS" -lt 60 ]; then
    show_text_menu
fi

while true; do
    _m=$(get_current_mode)
    _r=$(get_current_resolution)
    _t=$(get_tailnet_ip)
    _u=$(ss -tnp 2>/dev/null | grep -ic "rustdesk.*ESTAB" || true)
    _wdata=$(get_warning_summary_cached); _w=${_wdata%%|*}
    
    # Adjust whiptail height to fit terminal but cap at 24
    _W_H=$(( _T_LINES > 24 ? 24 : _T_LINES - 2 ))
    _W_W=$(( _T_COLS > 92 ? 92 : _T_COLS - 2 ))

    choice=$(whiptail \
        --title "Remote Studio v${VERSION}" \
        --backtitle "Mode: $_m ($_r)  |  IP: ${_t:-none}  |  Users: $_u  |  Warnings: $_w" \
        --menu "$(tui_header)" "$_W_H" "$_W_W" 10 \
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

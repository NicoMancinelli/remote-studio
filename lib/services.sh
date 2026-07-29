#!/bin/bash
# Remote Studio — Tailscale and RustDesk service helpers
#
# All show_tailnet_* functions gracefully degrade when the tailscale
# binary is missing or when LAN mode is active (in which case the
# tailnet is intentionally absent). The user gets a clear "not in
# use" message instead of a cryptic "command not found" error.

show_tailnet() {
    if lan_mode_active; then
        # In LAN mode the tailnet is intentionally absent. Surface the
        # LAN IP instead so the user can still copy/paste the address.
        local lan
        lan=$(hostname -I 2>/dev/null | awk '{print $1}')
        echo "LAN mode active — tailnet disabled."
        echo "LAN IP: ${lan:-unknown}"
        echo "RustDesk direct: ${lan:-0.0.0.0}:21118"
        return 0
    fi
    if ! command -v tailscale >/dev/null 2>&1; then
        echo "tailscale: command not found"
        echo "Install with: curl -fsSL https://tailscale.com/install.sh | sh"
        echo "Or enable LAN mode: res config set-toml lan_enabled true"
        return 1
    fi
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
    if lan_mode_active; then
        echo "LAN mode active — tailnet peers unavailable."
        echo "Use 'arp -a' or your router's admin UI to list LAN hosts."
        return 0
    fi
    if ! command -v tailscale >/dev/null 2>&1; then
        echo "tailscale: command not found"
        return 1
    fi
    echo "Tailnet peers:"
    tailscale status --peers=true 2>/dev/null | tail -n +2 | awk '{printf "  %-20s %s\n", $2, $1}'
}

show_tailnet_peer() {
    if lan_mode_active; then
        echo "LAN mode active — 'res tailnet peer' requires Tailscale."
        return 0
    fi
    if ! command -v tailscale >/dev/null 2>&1; then
        echo "tailscale: command not found"
        return 1
    fi
    local peer=$1; if [ -z "$peer" ]; then tailscale status --peers=true | head -n 20; return; fi
    echo "Checking $peer..."; tailscale ping "$peer"; echo; tailscale status | grep -i "$peer"
}

show_tailnet_doctor() {
    if lan_mode_active; then
        echo "LAN mode active — tailnet diagnostics skipped."
        echo "Use 'res doctor' for general system health instead."
        return 0
    fi
    if ! command -v tailscale >/dev/null 2>&1; then
        echo "tailscale: command not found"
        return 1
    fi
    echo "Tailnet Doctor"
    tailscale netcheck
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
            awk -v field="$field" -v replacement="$line" '
                $0 ~ ("^" field " =") { print replacement; next }
                { print }
            ' "$tmp_new" > "${tmp_new}.awk" && mv "${tmp_new}.awk" "$tmp_new"
        else
            echo "$line" >> "$tmp_new"
        fi
    done < "$tmp_preserve"
    cp "$tmp_new" "$target"
    rm "$tmp_preserve" "$tmp_new"
}

merge_rustdesk_options() {
    local source=$1 target=$2
    # RustDesk2.options.toml has no identity fields — safe to overwrite entirely
    cp "$source" "$target"
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
            local users conn_type remote_ip local_port
            users=$(ss -tnp 2>/dev/null | awk '
                function host(addr) {
                    sub(/^\[/, "", addr)
                    sub(/\]:[0-9]+$/, "", addr)
                    sub(/:[0-9]+$/, "", addr)
                    return addr
                }
                /ESTAB/ && /rustdesk/ { print host($5) }
            ' | sort -u | wc -l)
            echo "Active sessions : $users"

            if [ "$users" -gt 0 ]; then
                if ss -tnp 2>/dev/null | grep -i rustdesk | grep -qi ":21118"; then
                    conn_type="Direct"
                else
                    conn_type="Relayed (via DERP)"
                fi
                echo "Connection type : $conn_type"

                remote_ip=$(ss -tnp 2>/dev/null \
                    | awk '
                        function host(addr) {
                            sub(/^\[/, "", addr)
                            sub(/\]:[0-9]+$/, "", addr)
                            sub(/:[0-9]+$/, "", addr)
                            return addr
                        }
                        /ESTAB/ && /rustdesk/ { print host($5); exit }
                    ')
                local_port=$(ss -tnp 2>/dev/null \
                    | awk '/ESTAB/ && /rustdesk/ {split($4,a,":"); print a[length(a)]; exit}')
                [ -n "$remote_ip" ]   && echo "Remote IP       : $remote_ip"
                [ -n "$local_port" ]  && echo "Local port      : $local_port"
            fi

            local log_file="$HOME/.local/share/rustdesk/log/rustdesk.log"
            [ -f "$log_file" ] || log_file="$HOME/.rustdesk/log/rustdesk.log"
            if [ -f "$log_file" ]; then
                echo ""
                echo "-- Recent codec/perf events (last 50 log lines) --"
                # Extract the last mention of each key metric. The shared
                # helpers below keep the extraction logic in one place;
                # `res status` (show_status) reuses them too.
                local codec fps bitrate
                codec=$(get_rustdesk_codec "$log_file")
                fps=$(get_rustdesk_fps "$log_file")
                bitrate=$(get_rustdesk_bitrate "$log_file")
                [ -n "$codec"   ] && echo "  Codec   : $codec"
                [ -n "$fps"     ] && echo "  FPS     : $fps"
                [ -n "$bitrate" ] && echo "  Bitrate : $bitrate"
                [ -z "$codec$fps$bitrate" ] && echo "  (no codec/fps/bitrate found in last 50 lines)"
            else
                echo "(RustDesk log not found — check ~/.local/share/rustdesk/log/)"
            fi
            ;;
        log)
            local nlines=${2:-50}
            journalctl -u rustdesk -n "$nlines" --no-pager 2>/dev/null || echo "journalctl unavailable."
            ;;
        *) echo "Usage: res rustdesk [apply <preset>|backup|diff <preset>|status|log [lines]]" ;;
    esac
}

# RustDesk log metric extractors shared by `res rustdesk status` and
# `res status` (show_status). Each function takes the RustDesk log file
# path and returns the last matching log line for that metric, or empty
# if no match is found. The shapes are stable across rustdesk 1.4.x.
#
#   get_rustdesk_codec   "$log_file"   -> e.g. "VIDEO: H264"
#   get_rustdesk_fps     "$log_file"   -> e.g. "FPS: 60"
#   get_rustdesk_bitrate "$log_file"   -> e.g. "Bitrate: 2048 kbps"

_rustdesk_log_last_match() {
    # $1 = log file, $2 = case-insensitive pattern
    local log_file="${1:-}" pattern="${2:-}"
    [ -z "$log_file" ] || [ -z "$pattern" ] && return 0
    [ -f "$log_file" ] || return 0
    tail -n 50 "$log_file" 2>/dev/null | grep -i "$pattern" | tail -1 || true
}

get_rustdesk_codec() {
    local line
    line=$(_rustdesk_log_last_match "$1" 'codec')
    # Extract the codec token (e.g. H264, H265, VP8, VP9, AV1) from the line.
    printf '%s' "$line" | grep -oE '[A-Za-z0-9_-]+(264|265|VP[89]|AV1)[A-Za-z0-9_-]*' | head -1 || true
}

get_rustdesk_fps() {
    local line
    line=$(_rustdesk_log_last_match "$1" 'fps')
    # Trim leading non-numeric noise so the applet panel rendered label
    # stays compact. Examples that survive: "FPS: 60", "60 fps",
    # "measured fps: 30.5".
    printf '%s' "$line" | sed -nE 's/.*[Ff][Pp][Ss][^0-9]*([0-9]+(\.[0-9]+)?).*/\1/p' | head -1 || true
}

get_rustdesk_bitrate() {
    local line
    line=$(_rustdesk_log_last_match "$1" 'bitrate')
    # Extract the numeric value and the unit (kbps/mbps) so the applet
    # can present a single compact label. Examples: "Bitrate: 2048 kbps"
    # leaves "2048 kbps" behind.
    printf '%s' "$line" | sed -nE 's/.*[Bb]itrate[^0-9]*([0-9]+(\.[0-9]+)?[[:space:]]*(kbps|mbps|Kbps|Mbps)).*/\1/p' | head -1 || true
}

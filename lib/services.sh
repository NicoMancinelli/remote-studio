#!/bin/bash
# Remote Studio — Tailscale and RustDesk service helpers

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

show_tailnet_peer() {
    local peer=$1; if [ -z "$peer" ]; then tailscale status --peers=true | head -n 20; return; fi
    echo "Checking $peer..."; tailscale ping "$peer"; echo; tailscale status | grep -i "$peer"
}

show_tailnet_doctor() {
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
            users=$(ss -tnp 2>/dev/null | awk '/ESTAB/ && /rustdesk/{print $5}' \
                | cut -d: -f1 | sort -u | wc -l)
            echo "Active sessions : $users"

            if [ "$users" -gt 0 ]; then
                if ss -tnp 2>/dev/null | grep -i rustdesk | grep -qi ":21118"; then
                    conn_type="Direct"
                else
                    conn_type="Relayed (via DERP)"
                fi
                echo "Connection type : $conn_type"

                remote_ip=$(ss -tnp 2>/dev/null \
                    | awk '/ESTAB/ && /rustdesk/ {split($5,a,":"); print a[1]; exit}')
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
                # Extract the last mention of each key metric
                local last50
                last50=$(tail -n 50 "$log_file" 2>/dev/null)
                local codec fps bitrate
                codec=$(printf '%s' "$last50" | grep -i 'codec'   | tail -1)
                fps=$(printf '%s' "$last50"   | grep -i 'fps'     | tail -1)
                bitrate=$(printf '%s' "$last50"| grep -i 'bitrate' | tail -1)
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
        *) echo "Usage: res rustdesk [apply <preset>|backup|diff <preset>|status|log [lines]]"; ;;
    esac
}

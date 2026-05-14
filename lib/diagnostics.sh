#!/bin/bash
# Remote Studio — doctor, self-test, info, status, log

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
    if ! command -v rustdesk >/dev/null 2>&1 && ! systemctl list-unit-files rustdesk.service >/dev/null 2>&1; then
        doctor_check "rustdesk" "MISS" "not installed (download from rustdesk.com)"
    else
        rs=$(systemctl is-active rustdesk 2>/dev/null || echo "inactive")
        if [ "$rs" = "active" ]; then
            doctor_check "rustdesk" "OK" "active"
        else
            doctor_check "rustdesk" "WARN" "$rs"
        fi
    fi
    if ! command -v tailscale >/dev/null 2>&1; then
        doctor_check "tailscale" "MISS" "not installed (curl -fsSL https://tailscale.com/install.sh | sh)"
    else
        tip=$(get_tailnet_ip)
        local ts_backend
        ts_backend=$(tailscale status --json 2>/dev/null | grep -o '"BackendState":"[^"]*"' | cut -d'"' -f4 || true)
        if [ -n "$tip" ]; then
            doctor_check "tailscale" "OK" "$tip (${ts_backend:-unknown})"
        else
            doctor_check "tailscale" "WARN" "no tailnet IP — state: ${ts_backend:-unknown} (tailscale up?)"
        fi
        local exit_node
        exit_node=$(tailscale exit-node list 2>/dev/null | awk '/selected/ {print $1}' | head -1 || true)
        doctor_check "exit-node" "INFO" "${exit_node:-none}"
    fi
    git -C "$ROOT_DIR" \
        -c http.lowSpeedLimit=1000 \
        -c http.lowSpeedTime=3 \
        -c http.connectTimeout=3 \
        fetch --quiet 2>/dev/null || true
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

    # Log file size
    if [ -f "$LOG_FILE" ]; then
        local lsize
        lsize=$(stat -c%s "$LOG_FILE" 2>/dev/null || stat -f%z "$LOG_FILE" 2>/dev/null || echo 0)
        if [ "$lsize" -gt 524288 ]; then
            doctor_check "log-size" "WARN" "$((lsize / 1024)) KB (rotates at 1024 KB)"
        else
            doctor_check "log-size" "OK" "$((lsize / 1024)) KB"
        fi
    else
        doctor_check "log-size" "INFO" "no log yet"
    fi

    # Backup directory size (cap at 10 entries enforced by install.sh)
    local backup_root="$HOME/.config/remote-studio/backups"
    if [ -d "$backup_root" ]; then
        local bcount
        bcount=$(find "$backup_root" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
        if [ "$bcount" -gt 10 ]; then
            doctor_check "backups" "WARN" "$bcount entries (recommended: <= 10)"
        else
            doctor_check "backups" "OK" "$bcount entries"
        fi
    fi

    # Stale state file: STATE_FILE references a profile that no longer exists
    if [ -f "$STATE_FILE" ]; then
        local state_label
        state_label=$(awk -F"'" '{print $2}' "$STATE_FILE" 2>/dev/null)
        if [ -n "$state_label" ]; then
            local found=0 k plabel
            for k in "${!PROFILES[@]}"; do
                IFS='|' read -r plabel _ _ _ _ _ <<< "${PROFILES[$k]}"
                [ "$plabel" = "$state_label" ] && found=1 && break
            done
            if [ "$found" -eq 0 ] && [[ "$state_label" != Custom* ]]; then
                doctor_check "state" "WARN" "active mode '$state_label' no longer in profiles"
            else
                doctor_check "state" "OK" "$state_label"
            fi
        fi
    fi

    # /usr/local/bin/res symlink validity
    if [ -L /usr/local/bin/res ]; then
        local target
        target=$(readlink -f /usr/local/bin/res 2>/dev/null)
        if [ "$target" = "$ROOT_DIR/res.sh" ]; then
            doctor_check "symlink" "OK" "/usr/local/bin/res -> $ROOT_DIR/res.sh"
        else
            doctor_check "symlink" "WARN" "/usr/local/bin/res -> $target (expected $ROOT_DIR/res.sh)"
        fi
    elif [ -e /usr/local/bin/res ]; then
        doctor_check "symlink" "WARN" "/usr/local/bin/res exists but is not a symlink"
    else
        doctor_check "symlink" "INFO" "/usr/local/bin/res not installed"
    fi

    # Cinnamon applet loaded
    if pgrep -x cinnamon >/dev/null 2>&1; then
        local applet_dir="$HOME/.local/share/cinnamon/applets/remote-studio@neek"
        if [ -L "$applet_dir/applet.js" ] || [ -f "$applet_dir/applet.js" ]; then
            doctor_check "applet" "OK" "files present at $applet_dir"
        else
            doctor_check "applet" "WARN" "files missing at $applet_dir"
        fi
    else
        doctor_check "applet" "INFO" "cinnamon not running"
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

show_self_test() {
    local pass=0 fail=0
    echo "Remote Studio self-test"
    echo

    _t() {
        local name=$1; shift
        if "$@" >/dev/null 2>&1; then
            printf "  [PASS] %s\n" "$name"; pass=$((pass + 1))
        else
            printf "  [FAIL] %s\n" "$name"; fail=$((fail + 1))
        fi
    }

    _t "res command on PATH"        command -v res
    _t "ROOT_DIR exists"            test -d "$ROOT_DIR"
    _t "profiles file readable"     test -r "$DEFAULT_PROFILES"
    _t "PROFILES populated"         test "${#PROFILES[@]}" -gt 0
    local _probe="self-test-probe-$$"
    log_event "$_probe"
    if grep -q "$_probe" "$LOG_FILE" 2>/dev/null; then
        printf "  [PASS] log_event writes log\n"; pass=$((pass + 1))
    else
        printf "  [FAIL] log_event writes log\n"; fail=$((fail + 1))
    fi
    _t "status output writable"     bash -c "$ROOT_DIR/res.sh status > /dev/null"
    _t "version reports"            bash -c "$ROOT_DIR/res.sh version | grep -q ."
    _t "doctor exits 0"             bash -c "$ROOT_DIR/res.sh doctor > /dev/null"
    _t "config show exits 0"        bash -c "$ROOT_DIR/res.sh config show > /dev/null"

    echo
    echo "Result: $pass passed, $fail failed"
    [ "$fail" -eq 0 ] && return 0 || return 1
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

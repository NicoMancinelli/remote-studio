#!/bin/bash
# Remote Studio — whiptail TUI panels and text-mode menu

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

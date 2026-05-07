#!/bin/bash
# Remote Studio — whiptail TUI panels and text-mode menu

# ---- Helpers ----

tui_header() {
    local mode res_str ip wdata wcount wmsg renderer rustdesk_st session_st
    mode=$(get_current_mode)
    res_str=$(get_current_resolution)
    ip=$(get_tailnet_ip)
    wdata=$(get_warning_summary_cached); wcount=${wdata%%|*}; wmsg=${wdata#*|}
    renderer=$(get_renderer_summary 2>/dev/null | sed 's/.*NVIDIA.*/NVIDIA/;s/.*AMD.*/AMD/;s/.*Intel.*/Intel/;s/.*llvmpipe.*/SW-render/')
    rustdesk_st=$(systemctl is-active rustdesk 2>/dev/null || echo "?")
    session_st="$([ -f "$SESSION_FILE" ] && echo "active" || echo "idle")"
    
    # ANSI colors for text menu
    local c_mode="${GREEN}${mode}${NC}"
    local c_ip="${CYAN}${ip:-none}${NC}"
    local c_session="${DIM}${session_st}${NC}"
    [ "$session_st" = "active" ] && c_session="${GREEN}active${NC}"
    local c_rd="${GREEN}${rustdesk_st}${NC}"
    [ "$rustdesk_st" != "active" ] && c_rd="${RED}${rustdesk_st}${NC}"
    local c_warn="${DIM}0${NC}"
    [ "$wcount" -gt 0 ] && c_warn="${RED}${wcount} (${wmsg})${NC}"

    printf '  Mode: %-28b | IP: %-25b | Session: %b\n' "$c_mode" "$c_ip" "$c_session"
    printf '  GPU:  %-28b | RD: %-25b | Warnings: %b' "$renderer" "$c_rd" "$c_warn"
}

# Centered header for whiptail menus
tui_title_header() {
    local mode res_str ip wdata wcount wmsg renderer rustdesk_st session_st
    mode=$(get_current_mode)
    res_str=$(get_current_resolution)
    ip=$(get_tailnet_ip)
    wdata=$(get_warning_summary_cached); wcount=${wdata%%|*}; wmsg=${wdata#*|}
    renderer=$(get_renderer_summary 2>/dev/null | sed 's/.*NVIDIA.*/NVIDIA/;s/.*AMD.*/AMD/;s/.*Intel.*/Intel/;s/.*llvmpipe.*/SW-render/')
    rustdesk_st=$(systemctl is-active rustdesk 2>/dev/null || echo "?")
    session_st="$([ -f "$SESSION_FILE" ] && echo "active" || echo "idle")"

    printf 'Device: %s (%s)  |  IP: %s\nSession: %s  |  Renderer: %s  |  Warnings: %s' \
        "$mode" "$res_str" "${ip:-none}" "$session_st" "$renderer" "$wcount"
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

# ---- Panels ----

tui_quick() {
    local choice def_label
    def_label="${PROFILES[${DEFAULT_PROFILE}]%%|*}"
    [ -z "$def_label" ] && def_label="$DEFAULT_PROFILE"
    while true; do
        choice=$(whiptail --title "Quick Actions" \
            --backtitle "$(tui_title_header)" \
            --menu "Common workflows (default: ${def_label}):" \
            22 84 10 \
            "default-quality"  "Start ${def_label} session + apply Quality preset" \
            "default-balanced" "Start ${def_label} session + apply Balanced preset" \
            "default-speed"    "Start ${def_label} session + apply Speed preset" \
            "ipad-balanced"    "Start iPad session + apply Balanced preset" \
            "stop-reset"       "Stop session + reset display" \
            "fix-and-restart"  "Fix clipboard/audio/keys + restart RustDesk" \
            "back"             "Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            default-quality)
                session_start "$DEFAULT_PROFILE" && show_rustdesk apply quality
                ;;
            default-balanced)
                session_start "$DEFAULT_PROFILE" && show_rustdesk apply balanced
                ;;
            default-speed)
                session_start "$DEFAULT_PROFILE" && show_rustdesk apply speed
                ;;
            ipad-balanced)
                session_start ipad && show_rustdesk apply balanced
                ;;
            stop-reset)
                session_stop && do_action reset
                ;;
            fix-and-restart)
                do_action fix && do_action service
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
        
        body="[ DISPLAY ]
  Mode:        ${mode} (${res_str})
  Renderer:    ${renderer}
  Toggles:     Speed:${S_ST} | Caffeine:${C_ST} | Theme:${T_ST} | Night:${N_ST}

[ SESSION ]
  Active:      ${session_info}
  Users:       ${USERS} connected ($RUSTDESK_CONN_TYPE)

[ SYSTEM ]
  IP Address:  ${IP_ADDR}
  Telemetry:   Latency:${PING_STAT} | Temp:${TEMP} | RAM:${RAM}
  Services:    RustDesk:${rustdesk_st} | Tailscale:${tailscale_st}"

        recent_log=""
        if [ -f "$LOG_FILE" ]; then
            recent_log=$(tail -n 4 "$LOG_FILE" 2>/dev/null | sed 's/^/  /')
        fi
        [ -n "$recent_log" ] && body="${body}

[ RECENT EVENTS ]
${recent_log}"

        local _exit
        timeout 15 whiptail --title "Remote Studio Dashboard (v${VERSION})" \
            --backtitle "Auto-refreshing every 15 seconds" \
            --yes-button "Refresh" --no-button "Close" \
            --yesno "$body" "$lines" "$cols"
        _exit=$?
        if [ "$_exit" -eq 0 ] || [ "$_exit" -eq 124 ]; then
            _WARN_CACHE=""
        else
            return 0
        fi
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
    local current choice
    while true; do
        local entries=() key label w h s src marker recent_keys recent_count=0 rk
        current=$(get_current_mode)
        recent_keys=$(get_recent_profiles)
        
        if [ -n "$recent_keys" ]; then
            while IFS= read -r rk; do
                [ -z "$rk" ] && continue
                [ -z "${PROFILES[$rk]+x}" ] && continue
                IFS='|' read -r label w h s _ _ <<< "${PROFILES[$rk]}"
                marker="  "; [ "$label" = "$current" ] && marker="✓ "
                entries+=("$rk" "★ ${marker}${label} (${w}x${h})")
                recent_count=$((recent_count + 1))
            done <<< "$recent_keys"
            [ "$recent_count" -gt 0 ] && entries+=("" "──────────────── all profiles ────────────────")
        fi

        for key in $(sorted_profile_keys); do
            IFS='|' read -r label w h s _ _ <<< "${PROFILES[$key]}"
            src="[built-in]"
            grep -q "^${key}=" "$USER_PROFILES" 2>/dev/null && src="[user]"
            marker="  "; [ "$label" = "$current" ] && marker="✓ "
            
            # Icon based on key prefix
            local icon="  "
            case "$key" in
                mac*) icon="💻" ;;
                ipad*) icon="📱" ;;
                iphone*) icon="📱" ;;
                *) icon="🖥️ " ;;
            esac
            
            entries+=("$key" "${icon} ${marker}${label} (${w}x${h} @${s}x) ${src}")
        done
        
        entries+=("" "──────────────── actions ────────────────")
        entries+=("custom"  "  + Create Custom Resolution")
        entries+=("manage"  "  ⚙ Manage User Profiles")
        entries+=("back"    "  ↩ Return to Main Menu")
        
        choice=$(whiptail --title "Display Profiles" \
            --backtitle "$(tui_title_header)" \
            --menu "Select a profile to apply to the current session:" \
            24 90 18 "${entries[@]}" \
            3>&1 1>&2 2>&3) || return 0
        
        [ -z "$choice" ] && continue
        case "$choice" in
            back)   return 0 ;;
            custom) tui_custom_resolution ;;
            manage) tui_manage_profiles ;;
            *)
                if apply_profile "$choice"; then
                    record_recent_profile "$choice"
                    return 0
                else
                    whiptail --msgbox "Failed to apply profile '$choice'." 7 50
                fi
                ;;
        esac
    done
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
                cp "$USER_PROFILES" "${USER_PROFILES}.bak" 2>/dev/null || true
                sed -i "/^${choice}=/d" "$USER_PROFILES" || {
                    whiptail --msgbox "Delete failed — profiles unchanged." 7 55 3>&1 1>&2 2>&3
                    return 1
                }
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
            cp "$USER_PROFILES" "${USER_PROFILES}.bak" 2>/dev/null || true
            while IFS= read -r line; do
                if [[ "$line" =~ ^${choice}= ]]; then
                    printf '%s=%s|%s|%s|%s|%s|%s\n' "$choice" "$new_label" "$new_w" "$new_h" "$new_s" "$new_ts" "$new_cursor"
                else
                    printf '%s\n' "$line"
                fi
            done < "$USER_PROFILES" > "$tmp_profiles" || { rm -f "$tmp_profiles"; whiptail --msgbox "Edit failed — profiles unchanged." 7 55 3>&1 1>&2 2>&3; return 1; }
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
                ck=$(whiptail --inputbox "Config key (e.g. MY_SETTING):" 9 58 "" 3>&1 1>&2 2>&3) || continue
                [[ "$ck" =~ ^[A-Z][A-Z0-9_]*$ ]] || { whiptail --msgbox "Invalid key — use A-Z, 0-9, _ and start with a letter." 8 64; continue; }
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
            --backtitle "$(tui_title_header)" \
            --menu "Manage session state and comfort toggles:" \
            24 86 10 \
            "session-start" "Start New Session (apply profile)" \
            "session-stop"  "Stop Session (restore previous state)" \
            "speed"         "Toggle Speed Mode         [$S_ST]" \
            "caf"           "Toggle Caffeine           [$C_ST]" \
            "theme"         "Toggle Theme              [$T_ST]" \
            "night"         "Toggle Night Shift        [$N_ST]" \
            "rotate"        "Rotate Display            [$current_rotation]" \
            "privacy"       "Privacy Lock (Blank Monitor)" \
            "fix"           "Repair Toolkit (Clip/Audio/Keys)" \
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
                    --menu "Select profile to apply:" \
                    20 70 10 "${p_entries[@]}" \
                    3>&1 1>&2 2>&3) || continue
                session_start "$profile"
                ;;
            session-stop) session_stop ;;
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
        choice=$(whiptail --title "Tailnet Tools" \
            --backtitle "$(tui_title_header)" \
            --menu "Tailscale network management:" \
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
        choice=$(whiptail --title "Diagnostics & Health" \
            --backtitle "$(tui_title_header)" \
            --menu "Inspect system health and network connectivity:" \
            24 88 10 \
            "doctor"          "🩺 Full Health Report" \
            "self-test"       "🧪 Automated Integration Test" \
            "log"             "📜 View Event Logs" \
            "tailnet"         "🌐 Tailscale Network Tools" \
            "rustdesk-status" "📡 RustDesk Connection Details" \
            "profiles-list"   "📋 List Defined Profiles" \
            "fix-all"         "🛠️ Run Automated Repair" \
            "back"            "↩ Return to Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back)             return 0 ;;
            doctor)           run_panel_command "Doctor" show_doctor ;;
            self-test)        run_panel_command "Self-Test" show_self_test ;;
            log)              tui_log_viewer ;;
            tailnet)          tui_tailnet ;;
            rustdesk-status)  run_panel_command "RustDesk Status" show_rustdesk status ;;
            profiles-list)    run_panel_command "Profile Registry" show_profiles_list ;;
            fix-all)          run_panel_command "Repair" doctor_fix ;;
        esac
    done
}

tui_rustdesk() {
    local choice
    while true; do
        choice=$(whiptail --title "RustDesk Tools" \
            --backtitle "$(tui_title_header)" \
            --menu "Config, presets, and service management:" \
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
        choice=$(whiptail --title "System & Maintenance" \
            --backtitle "$(tui_title_header)" \
            --menu "Internal configuration and Xorg management:" \
            24 88 12 \
            "rustdesk"      "📡 RustDesk Tools & Presets" \
            "config"        "⚙️ Remote Studio Configuration" \
            "watch-service" "👁️ Connection Watcher Service" \
            "xorg-mgmt"     "🖥️ Xorg Config Management" \
            "update"        "🔄 Self-Update (git pull)" \
            "install"       "📦 Re-run Installer" \
            "backup"        "💾 System Backup" \
            "reset"         "🆘 Emergency Display Reset" \
            "back"          "↩ Return to Main Menu" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back)          return 0 ;;
            rustdesk)      tui_rustdesk ;;
            config)        tui_config ;;
            watch-service) tui_watch_service ;;
            xorg-mgmt)     tui_xorg_mgmt ;;
            update)        run_panel_command "Update" show_update ;;
            install)       run_panel_command "Install" "$ROOT_DIR/install.sh" install ;;
            backup)        run_panel_command "Backup" "$ROOT_DIR/install.sh" backup ;;
            reset)         do_action reset ;;
        esac
    done
}

tui_watch_service() {
    local ws_choice ws_st
    while true; do
        ws_st=$(systemctl --user is-active remote-studio-watch 2>/dev/null || echo "inactive")
        ws_choice=$(whiptail --title "Watch Service" \
            --menu "Auto-applies profile when RustDesk connects  [${ws_st}]" \
            16 70 6 \
            "status"  "Show Service Status" \
            "enable"  "Enable & Start" \
            "disable" "Stop & Disable" \
            "log"     "View Journal (last 50 lines)" \
            "back"    "Return" \
            3>&1 1>&2 2>&3) || return 0
        case "$ws_choice" in
            back) return 0 ;;
            status) run_panel_command "Watch Status" systemctl --user status remote-studio-watch ;;
            enable) confirm_action "Enable watcher?" && { systemctl --user enable remote-studio-watch; systemctl --user start remote-studio-watch; } ;;
            disable) confirm_action "Disable watcher?" && { systemctl --user stop remote-studio-watch; systemctl --user disable remote-studio-watch; } ;;
            log) run_panel_command "Watch Log" journalctl --user -u remote-studio-watch -n 50 --no-pager ;;
        esac
    done
}

tui_xorg_mgmt() {
    local choice
    while true; do
        choice=$(whiptail --title "Xorg Management" \
            --menu "Configure GPU-backed virtual displays:" 16 70 5 \
            "preview"  "Preview Generated Config" \
            "write"    "Write to /etc/X11/xorg.conf (sudo)" \
            "rollback" "Restore Latest Backup" \
            "back"     "Return" \
            3>&1 1>&2 2>&3) || return 0
        case "$choice" in
            back) return 0 ;;
            preview) run_panel_command "Xorg Preview" generate_xorg ;;
            write) confirm_action "Overwrite /etc/X11/xorg.conf? (Requires sudo)" && run_panel_command "Xorg Write" "$ROOT_DIR/install.sh" system ;;
            rollback) confirm_action "Restore Xorg from backup?" && run_panel_command "Xorg Rollback" rollback_xorg ;;
        esac
    done
}

show_text_menu() {
    local choice
    while true; do
        clear
        echo -e "${CYAN}┌────────────────────────────────────────────────────────────┐${NC}"
        echo -e "${CYAN}│${NC}           ${WHITE}${BOLD}Remote Studio v${VERSION}${NC}           ${CYAN}│${NC}"
        echo -e "${CYAN}└────────────────────────────────────────────────────────────┘${NC}"
        tui_header
        echo -e "\n${DIM}──────────────────────────────────────────────────────────────${NC}"
        printf "  ${CYAN}1)${NC} Profiles         ${CYAN}4)${NC} System\n"
        printf "  ${CYAN}2)${NC} Performance      ${CYAN}5)${NC} Dashboard\n"
        printf "  ${CYAN}3)${NC} Diagnostics      ${CYAN}6)${NC} Tailnet\n"
        printf "  ${CYAN}7)${NC} Quick Actions    ${CYAN}8)${NC} Help\n"
        printf "  ${CYAN}0)${NC} Exit\n"
        echo -e "${DIM}──────────────────────────────────────────────────────────────${NC}"
        echo ""
        read -r -p "Selection: " choice
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
            *) echo -e "${RED}Invalid selection.${NC}"; sleep 1 ;;
        esac
    done
}

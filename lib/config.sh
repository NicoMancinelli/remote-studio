#!/bin/bash
# Remote Studio — config, init wizard, help, update, profile listing

show_config() {
    case "${1:-show}" in
        show)
            echo "# Effective remote-studio config"
            echo "DEFAULT_PROFILE=${DEFAULT_PROFILE}"
            echo "DEFAULT_SESSION_PROFILE=${DEFAULT_SESSION_PROFILE:-${DEFAULT_PROFILE}}"
            echo "DEFAULT_RUSTDESK_PRESET=${DEFAULT_RUSTDESK_PRESET}"
            echo "AUTO_SESSION=${AUTO_SESSION:-false}"
            [ -f "$USER_CONFIG" ] && echo "# User config: $USER_CONFIG" || echo "# No user config file"
            ;;
        get)
            [ -z "${2:-}" ] && { echo "Usage: res config get KEY"; return 1; }
            grep "^${2}=" "$USER_CONFIG" 2>/dev/null | tail -1 | cut -d'=' -f2- || printf ""
            ;;
        set)
            { [ -z "${2:-}" ] || [ -z "${3:-}" ]; } && { echo "Usage: res config set KEY VALUE"; return 1; }
            local key="$2" val="$3"
            mkdir -p "$(dirname "$USER_CONFIG")"
            if grep -q "^${key}=" "$USER_CONFIG" 2>/dev/null; then
                awk -v k="$key" -v v="$val" '$0 ~ ("^" k "=") { print k "=" v; next } { print }' \
                    "$USER_CONFIG" > "${USER_CONFIG}.tmp" && mv "${USER_CONFIG}.tmp" "$USER_CONFIG"
            else
                echo "${key}=${val}" >> "$USER_CONFIG"
            fi
            echo "Set ${key}=${val} in $USER_CONFIG"
            log_event "Config set: ${key}=${val}"
            ;;
        *) echo "Usage: res config [show|get KEY|set KEY VALUE]"; return 1 ;;
    esac
}

show_init_wizard() {
    if ! command -v whiptail >/dev/null 2>&1; then
        echo "Init wizard requires whiptail - falling back to res doctor."
        show_doctor
        return 0
    fi

    whiptail --title "Welcome to Remote Studio v${VERSION}" --msgbox \
        "This wizard will check your setup and guide you through initial configuration.\n\nWe'll verify:\n  1. Required commands\n  2. Tailscale status\n  3. RustDesk service\n  4. Display profile selection\n  5. Cinnamon applet" 16 70

    # Step 1: dependency check
    local missing=()
    for cmd in xrandr whiptail gsettings; do
        command -v "$cmd" >/dev/null 2>&1 || missing+=("$cmd")
    done
    if [ "${#missing[@]}" -gt 0 ]; then
        whiptail --msgbox "Missing required commands:\n  ${missing[*]}\n\nInstall them via apt and run 'res init' again." 12 60
        return 1
    fi

    # Step 2: tailscale
    if ! command -v tailscale >/dev/null 2>&1; then
        whiptail --yesno "Tailscale is not installed.\n\nInstall it now? (runs the official installer)" 10 60 && \
            sh -c "curl -fsSL https://tailscale.com/install.sh | sh"
    fi

    # Step 3: rustdesk
    if ! command -v rustdesk >/dev/null 2>&1 && ! systemctl list-unit-files rustdesk.service >/dev/null 2>&1; then
        whiptail --msgbox "RustDesk is not installed.\n\nDownload it from https://rustdesk.com and re-run 'res init'." 10 65
    fi

    # Step 4: default profile selection
    local p_entries=() picked key label
    for key in $(sorted_profile_keys); do
        IFS='|' read -r label _ _ _ _ _ <<< "${PROFILES[$key]}"
        p_entries+=("$key" "$label")
    done
    picked=$(whiptail --title "Default Profile" --menu \
        "Select the device you'll connect from most often:" \
        20 60 10 "${p_entries[@]}" 3>&1 1>&2 2>&3) || picked="mac"
    show_config set DEFAULT_PROFILE "$picked"

    # Step 5: applet check
    local applet_dir="$HOME/.local/share/cinnamon/applets/remote-studio@neek"
    if [ -L "$applet_dir/applet.js" ] || [ -f "$applet_dir/applet.js" ]; then
        whiptail --msgbox "Setup complete!\n\nDefault profile: $picked\nApplet: installed\n\nRight-click your Cinnamon panel and add 'Remote Studio' to enable it." 12 65
    else
        if whiptail --yesno "Setup complete except for the Cinnamon applet.\n\nRun ./install.sh install now?" 10 65; then
            "$ROOT_DIR/install.sh" install
        fi
    fi
}

show_help() {
    echo "Remote Studio - RustDesk display management"
    echo "Usage: res [command]"
    printf ""; echo "Device Profiles:"
    for key in $(sorted_profile_keys); do
        IFS='|' read -r label width height scaling _ _ <<< "${PROFILES[$key]}"
        printf "  %-12s %s (%dx%d @%sx)\n" "$key" "$label" "$width" "$height" "$scaling"
    done
    printf ""; echo "Actions:"
    echo "  speed, theme, night, caf, privacy, fix, reset, service, audio, keys"
    echo "  doctor, doctor-fix, self-test, init"
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

show_update() {
    local before after new_version
    before=$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")
    echo "Current: v${VERSION} (${before})"
    if ! git -C "$ROOT_DIR" pull --ff-only; then
        echo "Error: git pull failed. Ensure this is a git repo with a clean working tree." >&2
        return 1
    fi
    after=$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown")
    "$ROOT_DIR/install.sh" install
    if [ "$before" = "$after" ]; then
        echo "Already up to date — v${VERSION} (${after})."
    else
        new_version=$(grep '^VERSION=' "$ROOT_DIR/res.sh" 2>/dev/null | head -1 | cut -d'"' -f2 || echo "?")
        echo "Updated: ${before} -> ${after} (v${new_version})"
    fi
    log_event "Self-update: ${before} -> ${after}"
}

show_profiles_list() {
    local cur_mode
    cur_mode=$(get_current_mode)
    local sorted_keys
    mapfile -t sorted_keys < <(printf '%s\n' "${!PROFILES[@]}" | sort)
    printf "%-12s %-26s %-14s %s\n" "KEY" "LABEL" "RESOLUTION" "SOURCE"
    printf "%-12s %-26s %-14s %s\n" "---" "-----" "----------" "------"
    local k label w h scale src active_marker
    for k in "${sorted_keys[@]}"; do
        IFS='|' read -r label w h scale _ _ <<< "${PROFILES[$k]}"
        src="default"
        grep -q "^${k}=" "$USER_PROFILES" 2>/dev/null && src="user"
        active_marker=""
        [ "$label" = "$cur_mode" ] && active_marker=" *"
        printf "%-12s %-26s %-14s %s%s\n" "$k" "$label" "${w}x${h}@${scale}" "$src" "$active_marker"
    done
}

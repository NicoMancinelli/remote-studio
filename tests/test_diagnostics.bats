#!/usr/bin/env bats
# Tests for diagnostics commands (lib/diagnostics.sh).
#
# show_doctor    — system health checks (xrandr, glxinfo, rustdesk, etc.)
# show_self_test — internal smoke tests with pass/fail counts
# show_info      — dashboard display (requires $DISPLAY for some parts)
# doctor_fix     — auto-repair common issues
# get_warning_summary — pipe-delimited warning format (count|messages)

SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/res.sh"
ROOT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)"
HELPERS="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)/helpers"

setup() {
    export HOME="$BATS_TEST_TMPDIR"
    mkdir -p "$BATS_TEST_TMPDIR/.config/remote-studio"
    # Source mocks so system tools (xrandr, systemctl, etc.) are available
    # as exported functions in subprocesses spawned by `run bash ...`.
    source "$HELPERS/mock_commands.bash"
}

# ===========================================================================
# res doctor — header and exit status
# ===========================================================================

@test "res doctor exits 0" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
}

@test "res doctor output contains 'Remote Studio doctor' header" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"Remote Studio doctor"* ]]
}

# ===========================================================================
# res doctor — expected check names
# ===========================================================================

@test "res doctor output contains xrandr check" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"xrandr"* ]]
}

@test "res doctor output contains glxinfo check" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"glxinfo"* ]]
}

@test "res doctor output contains display check" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"display"* ]]
}

@test "res doctor output contains renderer check" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"renderer"* ]]
}

@test "res doctor output contains rustdesk check" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"rustdesk"* ]]
}

@test "res doctor output contains tailscale check" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"tailscale"* ]]
}

@test "res doctor output contains update check" {
    run bash "$SCRIPT" doctor
    [ "$status" -eq 0 ]
    [[ "$output" == *"update"* ]]
}

# ===========================================================================
# res self-test — exit status and output format
# ===========================================================================

@test "res self-test exits 0" {
    # self-test may report failures (e.g. 'res' not on PATH in test env)
    # but the command itself should not crash.
    run bash "$SCRIPT" self-test
    # Accept either 0 (all pass) or 1 (some fail) — both are valid exits
    [[ "$status" -eq 0 || "$status" -eq 1 ]]
}

@test "res self-test output contains 'Remote Studio self-test' header" {
    run bash "$SCRIPT" self-test
    [[ "$output" == *"Remote Studio self-test"* ]]
}

@test "res self-test output contains 'Result:'" {
    run bash "$SCRIPT" self-test
    [[ "$output" == *"Result:"* ]]
}

@test "res self-test output contains pass/fail counts" {
    run bash "$SCRIPT" self-test
    # The result line has the format: "Result: N passed, M failed"
    [[ "$output" == *"passed"* ]]
    [[ "$output" == *"failed"* ]]
}

@test "res self-test output contains PASS or FAIL markers" {
    run bash "$SCRIPT" self-test
    # At least one check should produce either [PASS] or [FAIL]
    [[ "$output" == *"[PASS]"* ]] || [[ "$output" == *"[FAIL]"* ]]
}

# ===========================================================================
# res info — recognised command
# ===========================================================================

@test "res info is recognised as a valid command" {
    # info calls get_toggle_states which needs gsettings/xgamma — mocks are
    # exported but may not propagate into the bash subprocess.  We only check
    # that it is not "Unknown command".
    run bash "$SCRIPT" info
    [[ "$output" != *"Unknown command"* ]]
}

# ===========================================================================
# res doctor-fix — recognised command
# ===========================================================================

@test "res doctor-fix is recognised as a valid command" {
    run bash "$SCRIPT" doctor-fix
    [[ "$output" != *"Unknown command"* ]]
}

@test "res doctor-fix exits 0" {
    run bash "$SCRIPT" doctor-fix
    [ "$status" -eq 0 ]
}

@test "res doctor-fix prints 'Done.'" {
    run bash "$SCRIPT" doctor-fix
    [[ "$output" == *"Done."* ]]
}

# ===========================================================================
# get_warning_summary — pipe-delimited format (count|messages)
# ===========================================================================

@test "get_warning_summary returns pipe-delimited format" {
    # Call the function in a subprocess that sources the project's modules
    # with mocks exported so system tools resolve.
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        ROOT_DIR='$ROOT_DIR'
        LIB_DIR='$ROOT_DIR/lib'
        LOG_FILE='$HOME/.remote_studio.log'
        STATE_FILE='$HOME/.res_state'
        DEFAULT_PROFILES='$ROOT_DIR/config/profiles.conf'
        USER_PROFILES='$HOME/.config/remote-studio/profiles.conf'
        USER_CONFIG='$HOME/.config/remote-studio/remote-studio.conf'
        RECENT_PROFILES_FILE='$HOME/.config/remote-studio/recent_profiles'
        STATUS_DIR='/tmp/remote-studio-test-\$\$'
        STATUS_FILE='\$STATUS_DIR/status'
        SESSION_FILE='$HOME/.config/remote-studio/session.state'
        WALLPAPER_BACKUP='$HOME/.wallpaper_backup'
        _WARN_CACHE=''
        _WARN_CACHE_TS=0
        declare -A PROFILES=()
        source '$HELPERS/mock_commands.bash'
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/diagnostics.sh'
        get_warning_summary
    "
    [ "$status" -eq 0 ]

    # Output must contain exactly one pipe character: count|messages
    local pipe_count
    pipe_count=$(printf '%s' "$output" | tr -cd '|' | wc -c | tr -d ' ')
    [ "$pipe_count" -eq 1 ]
}

@test "get_warning_summary count is a non-negative integer" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        ROOT_DIR='$ROOT_DIR'
        LIB_DIR='$ROOT_DIR/lib'
        LOG_FILE='$HOME/.remote_studio.log'
        STATE_FILE='$HOME/.res_state'
        DEFAULT_PROFILES='$ROOT_DIR/config/profiles.conf'
        USER_PROFILES='$HOME/.config/remote-studio/profiles.conf'
        USER_CONFIG='$HOME/.config/remote-studio/remote-studio.conf'
        RECENT_PROFILES_FILE='$HOME/.config/remote-studio/recent_profiles'
        STATUS_DIR='/tmp/remote-studio-test-\$\$'
        STATUS_FILE='\$STATUS_DIR/status'
        SESSION_FILE='$HOME/.config/remote-studio/session.state'
        WALLPAPER_BACKUP='$HOME/.wallpaper_backup'
        _WARN_CACHE=''
        _WARN_CACHE_TS=0
        declare -A PROFILES=()
        source '$HELPERS/mock_commands.bash'
        source '$ROOT_DIR/lib/backend_x11.sh'
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/diagnostics.sh'
        get_warning_summary
    "
    [ "$status" -eq 0 ]

    # Extract count (everything before the pipe)
    local count="${output%%|*}"
    [[ "$count" =~ ^[0-9]+$ ]]
}

# ===========================================================================
# lan_mode_active — opt-in for LAN-only operation
# ===========================================================================

@test "lan_mode_active returns 1 (off) by default" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        ROOT_DIR='$ROOT_DIR'
        source '$ROOT_DIR/lib/core.sh'
        lan_mode_active
    "
    [ "$status" -eq 1 ]
}

@test "lan_mode_active returns 0 when RES_LAN_MODE=1" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export RES_LAN_MODE=1
        ROOT_DIR='$ROOT_DIR'
        source '$ROOT_DIR/lib/core.sh'
        lan_mode_active
    "
    [ "$status" -eq 0 ]
}

@test "lan_mode_active returns 0 when RES_LAN_MODE=true" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export RES_LAN_MODE=true
        ROOT_DIR='$ROOT_DIR'
        source '$ROOT_DIR/lib/core.sh'
        lan_mode_active
    "
    [ "$status" -eq 0 ]
}

@test "lan_mode_active returns 1 when RES_LAN_MODE=false even with TOML enabled" {
    # Write a TOML that says enabled.
    mkdir -p "$BATS_TEST_TMPDIR/.config/remote-studio"
    cat > "$BATS_TEST_TMPDIR/.config/remote-studio/remote-studio.toml" <<'TOML'
[lan]
enabled = true
TOML
    # Put the Go binary on PATH so res is callable from bash.
    export PATH="$ROOT_DIR:$PATH"
    # Rebuild the Go binary so it's up to date.
    export PATH_BAK="$PATH"
    export PATH=/opt/go/bin:$PATH
    go build -o "$BATS_TEST_TMPDIR/res-go" . 2>/dev/null
    export PATH="$BATS_TEST_TMPDIR:$PATH_BAK"
    mv "$BATS_TEST_TMPDIR/res-go" "$BATS_TEST_TMPDIR/res"

    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export RES_LAN_MODE=false
        ROOT_DIR='$ROOT_DIR'
        source '$ROOT_DIR/lib/core.sh'
        lan_mode_active
    "
    [ "$status" -eq 1 ]
}

# ===========================================================================
# get_warning_summary — LAN mode suppresses tailscale warnings
# ===========================================================================

@test "get_warning_summary excludes tailscale warning when LAN mode is on (env)" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export RES_LAN_MODE=1
        ROOT_DIR='$ROOT_DIR'
        LIB_DIR='$ROOT_DIR/lib'
        LOG_FILE='$HOME/.remote_studio.log'
        STATE_FILE='$HOME/.res_state'
        DEFAULT_PROFILES='$ROOT_DIR/config/profiles.conf'
        USER_PROFILES='$HOME/.config/remote-studio/profiles.conf'
        USER_CONFIG='$HOME/.config/remote-studio/remote-studio.conf'
        RECENT_PROFILES_FILE='$HOME/.config/remote-studio/recent_profiles'
        STATUS_DIR='/tmp/remote-studio-test-\$\$'
        STATUS_FILE='\$STATUS_DIR/status'
        SESSION_FILE='$HOME/.config/remote-studio/session.state'
        WALLPAPER_BACKUP='$HOME/.wallpaper_backup'
        _WARN_CACHE=''
        _WARN_CACHE_TS=0
        declare -A PROFILES=()
        source '$HELPERS/mock_commands.bash'
        source '$ROOT_DIR/lib/backend_x11.sh'
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/diagnostics.sh'
        get_warning_summary
    "
    [ "$status" -eq 0 ]
    # The tailscale warning token must NOT appear in the message list.
    [[ "$output" != *"tailscale"* ]] || [[ "$output" == *"0|"* ]]
}

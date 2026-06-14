#!/usr/bin/env bats
# Tests for the engine module — xorg generation, display actions, sessions,
# profile application, and rotate.
#
# Many engine functions depend on xrandr/gsettings/glxinfo; we source
# tests/helpers/mock_commands.bash where needed so the suite can run in
# headless CI without a real X display.

SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/res.sh"
MOCK_HELPER="$(cd "$(dirname "$BATS_TEST_FILENAME")" && pwd)/helpers/mock_commands.bash"

setup() {
    export HOME="$BATS_TEST_TMPDIR"
    mkdir -p "$BATS_TEST_TMPDIR/.config/remote-studio"
}

# ===========================================================================
# generate_xorg output — Section structure
# ===========================================================================

@test "res xorg output contains Section 'Device'" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'Section "Device"'* ]]
}

@test "res xorg output contains Section 'Monitor'" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'Section "Monitor"'* ]]
}

@test "res xorg output contains Section 'Screen'" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'Section "Screen"'* ]]
}

@test "res xorg output contains matching EndSection for each Section" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    local sections endsections
    sections=$(echo "$output" | grep -c '^Section ')
    endsections=$(echo "$output" | grep -c '^EndSection')
    [ "$sections" -eq "$endsections" ]
}

# ===========================================================================
# generate_xorg output — driver detection fallback
# ===========================================================================

@test "res xorg uses 'modesetting' driver when lspci finds no known GPU" {
    # In a headless Mac CI there is no lspci output matching nvidia/amd/intel,
    # so generate_xorg should fall back to modesetting.
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'Driver "modesetting"'* ]]
}

# ===========================================================================
# generate_xorg output — PreferredMode from mac profile
# ===========================================================================

@test "res xorg output contains PreferredMode derived from mac profile" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    # mac profile is 2560x1664, so PreferredMode should be 2560x1664_60.00
    [[ "$output" == *'PreferredMode'* ]]
    [[ "$output" == *'2560x1664_60.00'* ]]
}

@test "res xorg PreferredMode line is inside the Monitor section" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    # Extract the Monitor section and check that PreferredMode lives in it
    local in_monitor=0
    while IFS= read -r line; do
        [[ "$line" == 'Section "Monitor"' ]] && in_monitor=1
        if [ "$in_monitor" -eq 1 ] && [[ "$line" == *"PreferredMode"* ]]; then
            # Found inside Monitor — pass
            return 0
        fi
        [[ "$line" == "EndSection" ]] && in_monitor=0
    done <<< "$output"
    echo "PreferredMode not found inside Monitor section"
    false
}

# ===========================================================================
# generate_xorg output — Modeline entries
# ===========================================================================

@test "res xorg output contains at least one Modeline entry" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *"Modeline"* ]]
}

@test "res xorg output has Modeline for mac resolution 2560x1664" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'Modeline "2560x1664_60.00"'* ]]
}

@test "res xorg output has Modeline for fallback resolution 1920x1200" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'Modeline "1920x1200_60.00"'* ]]
}

# ===========================================================================
# generate_xorg output — Screen section details
# ===========================================================================

@test "res xorg Screen section has DefaultDepth 24" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *"DefaultDepth 24"* ]]
}

@test "res xorg Screen section references Configured Monitor and Device" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'Monitor "Configured Monitor"'* ]]
    [[ "$output" == *'Device "Configured Video Device"'* ]]
}

@test "res xorg Screen section includes a 1024x768 fallback mode" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *'"1024x768"'* ]]
}

@test "res xorg Screen section includes Virtual framebuffer size" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *"Virtual 3840 2160"* ]]
}

# ===========================================================================
# xorg file output — writing to a path
# ===========================================================================

@test "res xorg writes a file when a path argument is supplied" {
    local out="$BATS_TEST_TMPDIR/xorg_test.conf"
    run bash "$SCRIPT" xorg "$out"
    [ "$status" -eq 0 ]
    [ -f "$out" ]
}

@test "xorg file output contains all three Section types" {
    local out="$BATS_TEST_TMPDIR/xorg_sections.conf"
    run bash "$SCRIPT" xorg "$out"
    [ "$status" -eq 0 ]
    grep -q 'Section "Device"'  "$out"
    grep -q 'Section "Monitor"' "$out"
    grep -q 'Section "Screen"'  "$out"
}

@test "xorg file output contains Modeline entries" {
    local out="$BATS_TEST_TMPDIR/xorg_modelines.conf"
    run bash "$SCRIPT" xorg "$out"
    [ "$status" -eq 0 ]
    grep -q 'Modeline' "$out"
}

@test "xorg file output is non-empty and valid (no empty file)" {
    local out="$BATS_TEST_TMPDIR/xorg_nonempty.conf"
    run bash "$SCRIPT" xorg "$out"
    [ "$status" -eq 0 ]
    [ -s "$out" ]   # -s checks file is non-empty
}

# ===========================================================================
# do_action toggles — recognised commands (headless-safe)
# ===========================================================================
# These actions invoke gsettings/xrandr/xgamma under the hood.  Without a
# display they may fail, but they must NOT produce "Unknown command".

@test "res speed is a recognised command" {
    run bash "$SCRIPT" speed
    [[ "$output" != *"Unknown command"* ]]
}

@test "res theme is a recognised command" {
    run bash "$SCRIPT" theme
    [[ "$output" != *"Unknown command"* ]]
}

@test "res night is a recognised command" {
    run bash "$SCRIPT" night
    [[ "$output" != *"Unknown command"* ]]
}

@test "res caf is a recognised command" {
    run bash "$SCRIPT" caf
    [[ "$output" != *"Unknown command"* ]]
}

@test "res reset is a recognised command" {
    run bash "$SCRIPT" reset
    [[ "$output" != *"Unknown command"* ]]
}

# ===========================================================================
# do_action toggles — behaviour with mocked commands
# ===========================================================================

@test "res speed succeeds with mocked gsettings" {
    [ -n "${DISPLAY:-}" ] || skip "no X display"
    source "$MOCK_HELPER"
    run bash -c "source '$MOCK_HELPER'; bash '$SCRIPT' speed"
    # With mocks loaded in the parent env the child script still picks them up
    # via export -f.  Verify no crash.
    [[ "$output" != *"Unknown command"* ]]
}

@test "res night succeeds with mocked xgamma" {
    [ -n "${DISPLAY:-}" ] || skip "no X display"
    source "$MOCK_HELPER"
    run bash -c "source '$MOCK_HELPER'; bash '$SCRIPT' night"
    [[ "$output" != *"Unknown command"* ]]
}

# ===========================================================================
# apply_profile with invalid profile
# ===========================================================================

@test "applying a nonexistent profile exits non-zero" {
    run bash "$SCRIPT" __nonexistent_profile_xyz__
    [ "$status" -ne 0 ]
}

@test "applying a nonexistent profile prints 'Unknown command'" {
    run bash "$SCRIPT" __nonexistent_profile_xyz__
    [[ "$output" == *"Unknown command"* ]]
}



# ===========================================================================
# session lifecycle — start with invalid profile
# ===========================================================================

@test "session start with nonexistent profile fails gracefully" {
    run bash "$SCRIPT" session start __bad_profile__
    # set -u causes an "unbound variable" error when PROFILES[__bad_profile__]
    # is accessed, so the script exits non-zero.
    [ "$status" -ne 0 ]
}

@test "session start with nonexistent profile writes SESSION_FILE before failing" {
    run bash "$SCRIPT" session start __bad_profile__
    local session_file="$BATS_TEST_TMPDIR/.config/remote-studio/session.state"
    # session_start writes the session file *before* calling apply_profile.
    # With set -u, the unbound variable error in apply_profile aborts the
    # script before the cleanup rm -f runs, so the file is left behind.
    [ -f "$session_file" ]
    grep -q 'profile=__bad_profile__' "$session_file"
}

@test "session start with valid profile name is recognised" {
    run bash "$SCRIPT" session start mac
    # In headless CI, apply_profile fails (no xrandr output) but the
    # subcommand itself must be recognised — not "Unknown command".
    [[ "$output" != *"Unknown command"* ]]
}

@test "session status reports no active session in clean HOME" {
    run bash "$SCRIPT" session status
    [ "$status" -eq 0 ]
    [[ "$output" == *"No active session"* ]]
}

@test "session stop exits 0 when no session is active" {
    run bash "$SCRIPT" session stop
    [ "$status" -eq 0 ]
}

@test "session with unknown sub-subcommand exits non-zero" {
    run bash "$SCRIPT" session __invalid__
    [ "$status" -ne 0 ]
}

@test "session with unknown sub-subcommand prints usage" {
    run bash "$SCRIPT" session __invalid__
    [[ "$output" == *"Usage"* ]]
}

# ===========================================================================
# session lifecycle — round-trip (display required)
# ===========================================================================

@test "session start writes session.state file" {
    [ -n "${DISPLAY:-}" ] || skip "no X display"
    local state_file="$BATS_TEST_TMPDIR/.res_state"
    printf "1280 800 1 1.0 24 'Mac'\n" > "$state_file"
    run bash "$SCRIPT" session start mac
    [ "$status" -eq 0 ]
    local session_file="$BATS_TEST_TMPDIR/.config/remote-studio/session.state"
    [ -f "$session_file" ]
    grep -q 'profile=mac' "$session_file"
}

@test "session start then stop removes session.state" {
    [ -n "${DISPLAY:-}" ] || skip "no X display"
    local session_file="$BATS_TEST_TMPDIR/.config/remote-studio/session.state"
    mkdir -p "$(dirname "$session_file")"
    printf 'started_at=2024-01-01 00:00:00\nprofile=mac\nspeed=OFF\ncaffeine=OFF\nstate=1280 800 1 1.0 24 '"'"'Mac'"'"'\n' > "$session_file"
    run bash "$SCRIPT" session stop
    [ ! -f "$session_file" ]
}

# ===========================================================================
# rotate command
# ===========================================================================

@test "res rotate is a recognised command" {
    run bash "$SCRIPT" rotate
    [[ "$output" != *"Unknown command"* ]]
}

@test "res rotate with explicit direction is recognised" {
    run bash "$SCRIPT" rotate left
    [[ "$output" != *"Unknown command"* ]]
}

@test "res rotate succeeds with a display" {
    [ -n "${DISPLAY:-}" ] || skip "no X display"
    run bash "$SCRIPT" rotate normal
    [ "$status" -eq 0 ]
    [[ "$output" == *"Rotated"* ]]
}

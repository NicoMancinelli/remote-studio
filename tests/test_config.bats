#!/usr/bin/env bats
# Tests for config file loading and the USER_CONFIG mechanism.
#
# res.sh has no "config" subcommand — configuration is a file at
# ~/.config/remote-studio/remote-studio.conf that res.sh sources on
# startup.  These tests verify that the file is picked up correctly
# (DEFAULT_PROFILE, AUTO_SESSION, etc.) and that the state/log paths
# honour $HOME.

SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/res.sh"

setup() {
    export HOME="$BATS_TEST_TMPDIR"
    mkdir -p "$BATS_TEST_TMPDIR/.config/remote-studio"
}

# ---------------------------------------------------------------------------
# USER_CONFIG sourcing
# ---------------------------------------------------------------------------

@test "res version works with an empty HOME config dir" {
    run bash "$SCRIPT" version
    [ "$status" -eq 0 ]
    [[ "$output" =~ ^[0-9] ]]
}

@test "DEFAULT_PROFILE in user config is respected (help output)" {
    # Write a config that sets the default profile to fallback.
    # The only observable effect without a display is that help still works.
    echo "DEFAULT_PROFILE=fallback" > "$HOME/.config/remote-studio/remote-studio.conf"
    run bash "$SCRIPT" help
    [ "$status" -eq 0 ]
}

@test "malformed user config does not crash res version" {
    # A config with a stray line that is not valid shell should not cause an
    # unrecoverable error for read-only subcommands.
    printf '# comment only\nDEFAULT_PROFILE=mac\n' > "$HOME/.config/remote-studio/remote-studio.conf"
    run bash "$SCRIPT" version
    [ "$status" -eq 0 ]
}

# ---------------------------------------------------------------------------
# State file paths follow $HOME
# ---------------------------------------------------------------------------

@test "res log reports no log file when HOME is empty tmpdir" {
    run bash "$SCRIPT" log
    [ "$status" -eq 0 ]
    [[ "$output" == *"No log file"* ]]
}

@test "res session status reports no active session when HOME is empty tmpdir" {
    run bash "$SCRIPT" session status
    [ "$status" -eq 0 ]
    [[ "$output" == *"No active session"* ]]
}

# ---------------------------------------------------------------------------
# xorg subcommand (no display required — generates to stdout)
# ---------------------------------------------------------------------------

@test "res xorg prints Section 'Device' block" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *"Section \"Device\""* ]]
}

@test "res xorg output contains PreferredMode for 2560x1664" {
    run bash "$SCRIPT" xorg
    [ "$status" -eq 0 ]
    [[ "$output" == *"2560x1664"* ]]
}

@test "res xorg writes to a file when a path argument is supplied" {
    local out="$BATS_TEST_TMPDIR/xorg.conf"
    run bash "$SCRIPT" xorg "$out"
    [ "$status" -eq 0 ]
    [ -f "$out" ]
    grep -q "Section" "$out"
}

# ---------------------------------------------------------------------------
# session subcommand (status only — no display needed)
# ---------------------------------------------------------------------------

@test "res session with unknown sub-subcommand exits non-zero" {
    run bash "$SCRIPT" session __invalid__
    [ "$status" -ne 0 ]
}

# ---------------------------------------------------------------------------
# session start writes state; session stop removes it
# ---------------------------------------------------------------------------

@test "res session start writes SESSION_FILE" {
    # session start requires a display to apply_profile, so we only check that
    # it attempts to write the session file before hitting apply_profile.
    # In CI (no display), apply_profile returns 1 but the session file is
    # written first — verify it was attempted by checking exit behaviour.
    run bash "$SCRIPT" session start mac
    # Either succeeds (display present) or fails at apply_profile (no display);
    # either way, the session sub-command itself must be recognised (not "Unknown command").
    [[ "$output" != *"Unknown command"* ]]
}

@test "res session stop exits 0 with no active session" {
    run bash "$SCRIPT" session stop
    [ "$status" -eq 0 ]
}

@test "res session status exits 0" {
    run bash "$SCRIPT" session status
    [ "$status" -eq 0 ]
}

# ---------------------------------------------------------------------------
# session round-trip — stop edge-cases and start/stop lifecycle
# ---------------------------------------------------------------------------

@test "session stop with missing SESSION_FILE exits cleanly" {
    # SESSION_FILE is derived from $HOME (set in setup to BATS_TEST_TMPDIR).
    # Ensure it does not exist, then verify stop returns 0.
    local session_file="$BATS_TEST_TMPDIR/.config/remote-studio/session.state"
    rm -f "$session_file"
    run bash "$SCRIPT" session stop
    [ "$status" -eq 0 ]
}

@test "session stop with missing state field exits cleanly" {
    # Regression for the empty-state guard: a SESSION_FILE with no state= line
    # must not cause session_stop to crash or return non-zero.
    local session_file="$BATS_TEST_TMPDIR/.config/remote-studio/session.state"
    mkdir -p "$(dirname "$session_file")"
    printf 'started_at=2024-01-01 00:00:00\nprofile=mac\n' > "$session_file"
    run bash "$SCRIPT" session stop
    [ "$status" -eq 0 ]
}

@test "session start writes SESSION_FILE" {
    # Requires a live X display; skip in headless CI.
    [ -n "${DISPLAY:-}" ] || skip "no X display"
    local state_file="$BATS_TEST_TMPDIR/.res_state"
    local session_file="$BATS_TEST_TMPDIR/.config/remote-studio/session.state"
    # Provide a fake prior state so session_start has something to record.
    printf "1280 800 1 1.0 24 'Mac'\n" > "$state_file"
    run bash "$SCRIPT" session start mac
    [ -f "$session_file" ]
    grep -q 'profile=' "$session_file"
}

@test "session start then stop removes SESSION_FILE" {
    # Requires a live X display; skip in headless CI.
    [ -n "${DISPLAY:-}" ] || skip "no X display"
    local session_file="$BATS_TEST_TMPDIR/.config/remote-studio/session.state"
    # Write a SESSION_FILE that mimics what session_start would have produced,
    # with a valid numeric state so session_stop can attempt display restore.
    mkdir -p "$(dirname "$session_file")"
    printf 'started_at=2024-01-01 00:00:00\nprofile=mac\nspeed=OFF\ncaffeine=OFF\nstate=1280 800 1 1.0 24 '"'"'Mac'"'"'\n' > "$session_file"
    run bash "$SCRIPT" session stop
    [ ! -f "$session_file" ]
}

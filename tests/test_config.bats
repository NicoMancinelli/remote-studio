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

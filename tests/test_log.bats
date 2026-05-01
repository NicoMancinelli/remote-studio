#!/usr/bin/env bats
# Tests for the log subcommand.

SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/res.sh"

setup() {
    export HOME="$BATS_TEST_TMPDIR"
}

# ---------------------------------------------------------------------------
# show_log — no log file present
# ---------------------------------------------------------------------------

@test "res log exits 0 when no log file exists" {
    run bash "$SCRIPT" log
    [ "$status" -eq 0 ]
    [[ "$output" == *"No log file"* ]]
}

@test "res log with numeric argument exits 0 when no log file exists" {
    run bash "$SCRIPT" log 50
    [ "$status" -eq 0 ]
    [[ "$output" == *"No log file"* ]]
}

# ---------------------------------------------------------------------------
# show_log — log file present
# ---------------------------------------------------------------------------

@test "res log tails the log file when it exists" {
    # Pre-populate the log file that res.sh will look for: $HOME/.remote_studio.log
    local log="$BATS_TEST_TMPDIR/.remote_studio.log"
    printf '[2026-01-01 00:00:00] Event one\n' >  "$log"
    printf '[2026-01-01 00:00:01] Event two\n' >> "$log"
    printf '[2026-01-01 00:00:02] Event three\n' >> "$log"

    run bash "$SCRIPT" log
    [ "$status" -eq 0 ]
    [[ "$output" == *"Event"* ]]
}

@test "res log limits output to the requested number of lines" {
    local log="$BATS_TEST_TMPDIR/.remote_studio.log"
    # Write 30 lines
    for i in $(seq 1 30); do
        echo "[2026-01-01 00:00:$(printf '%02d' "$i")] Line $i" >> "$log"
    done

    # Ask for only 5 lines — output should not contain the first line
    run bash "$SCRIPT" log 5
    [ "$status" -eq 0 ]
    # "Line 1" should NOT appear (it's outside the last-5 window)
    [[ "$output" != *"Line 1 "* ]] || {
        echo "Expected at most 5 lines but got output containing early lines"
        false
    }
    # "Line 30" should appear
    [[ "$output" == *"Line 30"* ]]
}

@test "res log default shows up to 20 lines" {
    local log="$BATS_TEST_TMPDIR/.remote_studio.log"
    # Write exactly 25 lines
    for i in $(seq 1 25); do
        echo "[2026-01-01 00:00:$(printf '%02d' "$i")] Event $i" >> "$log"
    done

    run bash "$SCRIPT" log
    [ "$status" -eq 0 ]
    # Line 25 is in the last-20 window
    [[ "$output" == *"Event 25"* ]]
    # Line 5 is NOT in the last-20 window (25 - 20 = 5 is the boundary line;
    # lines 1-5 are excluded)
    [[ "$output" != *"Event 1 "* ]] || {
        echo "Default tail of 20 should not include line 1 of 25"
        false
    }
}

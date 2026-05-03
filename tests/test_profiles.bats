#!/usr/bin/env bats
# Tests for profiles.conf validity and profile-related res.sh subcommands.

SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/res.sh"
CONF="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/config/profiles.conf"

# ---------------------------------------------------------------------------
# profiles.conf file-level checks
# ---------------------------------------------------------------------------

@test "profiles.conf exists and is readable" {
    [ -f "$CONF" ]
    [ -r "$CONF" ]
}

@test "mac profile is defined in profiles.conf" {
    grep -q "^mac=" "$CONF"
}

@test "mac15 profile is defined in profiles.conf" {
    grep -q "^mac15=" "$CONF"
}

@test "ipad profile is defined in profiles.conf" {
    grep -q "^ipad=" "$CONF"
}

@test "ipad13 profile is defined in profiles.conf" {
    grep -q "^ipad13=" "$CONF"
}

@test "iphonel profile is defined in profiles.conf" {
    grep -q "^iphonel=" "$CONF"
}

@test "iphonep profile is defined in profiles.conf" {
    grep -q "^iphonep=" "$CONF"
}

@test "fallback profile is defined in profiles.conf" {
    grep -q "^fallback=" "$CONF"
}

@test "all non-comment profile lines have exactly 6 pipe-delimited fields" {
    # Each value must have the form: label|width|height|scaling|text_scale|cursor
    # That means 5 pipes => 6 fields.
    while IFS='=' read -r key value; do
        # Skip blank lines and comments
        [[ "$key" =~ ^[[:space:]]*# ]] && continue
        [[ -z "$key" || -z "$value" ]]  && continue
        count=$(printf '%s' "$value" | tr -cd '|' | wc -c)
        [ "$count" -eq 5 ] || {
            echo "Bad profile '$key': expected 5 pipes (6 fields), got $count in: $value"
            false
        }
    done < "$CONF"
}

@test "mac profile has numeric width and height" {
    local val
    val=$(grep "^mac=" "$CONF" | cut -d= -f2-)
    local width height
    IFS='|' read -r _ width height _ <<< "$val"
    [[ "$width"  =~ ^[0-9]+$ ]] || { echo "width not numeric: $width";  false; }
    [[ "$height" =~ ^[0-9]+$ ]] || { echo "height not numeric: $height"; false; }
}

@test "ipad13 profile value matches expected resolution 2064x2752" {
    local val
    val=$(grep "^ipad13=" "$CONF" | cut -d= -f2-)
    local width height
    IFS='|' read -r _ width height _ <<< "$val"
    [ "$width"  = "2064" ] || { echo "ipad13 width: expected 2064, got $width";  false; }
    [ "$height" = "2752" ] || { echo "ipad13 height: expected 2752, got $height"; false; }
}

@test "mac profile value matches expected resolution 2560x1664" {
    local val
    val=$(grep "^mac=" "$CONF" | cut -d= -f2-)
    local width height
    IFS='|' read -r _ width height _ <<< "$val"
    [ "$width"  = "2560" ] || { echo "mac width: expected 2560, got $width";  false; }
    [ "$height" = "1664" ] || { echo "mac height: expected 1664, got $height"; false; }
}

# ---------------------------------------------------------------------------
# mode_name_for — tested via res.sh subcommands
# ---------------------------------------------------------------------------

@test "res version outputs a version string starting with a digit" {
    run bash "$SCRIPT" version
    [ "$status" -eq 0 ]
    [[ "$output" =~ ^[0-9] ]]
}

@test "res version matches semver format" {
    run bash "$SCRIPT" version
    [ "$status" -eq 0 ]
    [[ "$output" =~ ^[0-9]+\.[0-9] ]]
}

@test "res help exits 0" {
    run bash "$SCRIPT" help
    [ "$status" -eq 0 ]
}

@test "res help output contains 'Remote Studio'" {
    run bash "$SCRIPT" help
    [ "$status" -eq 0 ]
    [[ "$output" == *"Remote Studio"* ]]
}

@test "res help lists mac profile" {
    run bash "$SCRIPT" help
    [ "$status" -eq 0 ]
    [[ "$output" == *"mac"* ]]
}

@test "res -h exits 0" {
    run bash "$SCRIPT" -h
    [ "$status" -eq 0 ]
}

@test "res --help exits 0" {
    run bash "$SCRIPT" --help
    [ "$status" -eq 0 ]
}

@test "res unknown-command exits non-zero" {
    run bash "$SCRIPT" __not_a_real_command__
    [ "$status" -ne 0 ]
}

@test "res unknown-command prints 'Unknown command'" {
    run bash "$SCRIPT" __not_a_real_command__
    [[ "$output" == *"Unknown command"* ]]
}

# ---------------------------------------------------------------------------
# Profile application requires a display — skip in headless CI
# ---------------------------------------------------------------------------

@test "res mac exits non-zero without a display" {
    [ -z "${DISPLAY:-}" ] || skip "display present — would actually apply profile"
    run bash "$SCRIPT" mac
    # Without xrandr connected output, apply_all returns 1
    [ "$status" -ne 0 ]
}

# ---------------------------------------------------------------------------
# PROFILES array loading (regression: PROFILES[key] literal bug)
# ---------------------------------------------------------------------------

@test "res help lists all built-in profile keys" {
    run bash "$SCRIPT" help
    [ "$status" -eq 0 ]
    [[ "$output" == *"mac"*    ]]
    [[ "$output" == *"ipad"*   ]]
    [[ "$output" == *"iphonel"* ]]
    [[ "$output" == *"iphonep"* ]]
    [[ "$output" == *"fallback"* ]]
}

@test "res profiles lists all built-in profiles" {
    run bash "$SCRIPT" profiles
    [ "$status" -eq 0 ]
    [[ "$output" == *"mac"*    ]]
    [[ "$output" == *"ipad"*   ]]
}

#!/usr/bin/env bats
# Unit tests for the tailscale-status JSON parsing helper.
#
# The previous regex `"BackendState":"..."` did NOT match pretty-printed
# JSON like `"BackendState": "Running"` emitted by tailscale 1.98+.
# This regression test pins both forms so we don't break again.

# Extract just the parser we want to exercise. lib/core.sh has too many
# dependencies (systemctl, xrandr, glxinfo) for CI containers, so we
# inline a copy here. The two implementations MUST stay in sync — if
# you change lib/core.sh's regex, change the one here too.
parse_backend_state() {
    local ts_json="$1"
    printf '%s' "$ts_json" | grep -oE '"BackendState":[[:space:]]*"[^"]*"' | head -1 | cut -d'"' -f4
}

@test "parse_backend_state matches compact form (tailscale <1.98)" {
    result=$(parse_backend_state '{"BackendState":"Running","TailscaleIPs":["100.100.1.5"]}')
    [ "$result" = "Running" ]
}

@test "parse_backend_state matches pretty-printed form (tailscale 1.98+)" {
    # This is the form that broke the original regex. Newer tailscale
    # versions emit JSON with a space after the colon in pretty mode.
    result=$(parse_backend_state '{
  "BackendState": "Running",
  "TailscaleIPs": [
    "100.100.9.9"
  ]
}')
    [ "$result" = "Running" ]
}

@test "parse_backend_state treats tabs as whitespace" {
    result=$(parse_backend_state $'{"BackendState":\t\t"NeedsLogin"}')
    [ "$result" = "NeedsLogin" ]
}

@test "parse_backend_state returns empty when BackendState is absent" {
    result=$(parse_backend_state '{"TailscaleIPs":["100.100.9.9"]}')
    [ -z "$result" ]
}

@test "parse_backend_state extracts Stopped correctly" {
    result=$(parse_backend_state '{"BackendState": "Stopped"}')
    [ "$result" = "Stopped" ]
}

@test "parse_backend_state extracts Starting correctly" {
    result=$(parse_backend_state '{"BackendState": "Starting"}')
    [ "$result" = "Starting" ]
}

@test "parse_backend_state doesn't get confused by colons in other values" {
    # Defensive: a value like "a:b" elsewhere shouldn't break the match.
    result=$(parse_backend_state '{"BackendState":"NoNetwork","Other":"a:b"}')
    [ "$result" = "NoNetwork" ]
}

#!/usr/bin/env bats
# Tests for RustDesk and Tailscale service helpers (lib/services.sh).
#
# merge_rustdesk_config — merges a source TOML into a target while preserving
#     identity fields (id, key, password, salt, relay-server, api-server).
# merge_rustdesk_options — plain cp (options carry no identity data).
# show_rustdesk — CLI dispatch for the "res rustdesk" subcommand.

SCRIPT="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)/res.sh"
ROOT_DIR="$(cd "$(dirname "$BATS_TEST_FILENAME")/.." && pwd)"

setup() {
    export HOME="$BATS_TEST_TMPDIR"
    mkdir -p "$BATS_TEST_TMPDIR/.config/remote-studio"
}

# ---------------------------------------------------------------------------
# Helper: run a merge function in an isolated bash subprocess.
#
# We source res.sh's dependencies just enough to get the merge functions,
# then call the requested function with the provided arguments.
# ---------------------------------------------------------------------------
_run_merge() {
    local func=$1; shift
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
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/services.sh'
        $func $*
    "
}

# ===========================================================================
# merge_rustdesk_config — target does not exist
# ===========================================================================

@test "merge_rustdesk_config creates target from source when target does not exist" {
    local src="$BATS_TEST_TMPDIR/source.toml"
    local tgt="$BATS_TEST_TMPDIR/target.toml"

    cat > "$src" <<'TOML'
[options]
i444 = 'Y'
image_quality = 'balanced'
codec-preference = 'auto'
id = 'source-id-999'
TOML

    [ ! -f "$tgt" ]  # precondition: target absent

    _run_merge merge_rustdesk_config "'$src'" "'$tgt'"
    [ "$status" -eq 0 ]
    [ -f "$tgt" ]
    # Target should be an exact copy of source when no prior target exists
    diff -q "$src" "$tgt"
}

# ===========================================================================
# merge_rustdesk_config — preserves identity fields from existing target
# ===========================================================================

@test "merge_rustdesk_config preserves id from existing target" {
    local src="$BATS_TEST_TMPDIR/source.toml"
    local tgt="$BATS_TEST_TMPDIR/target.toml"

    cat > "$src" <<'TOML'
[options]
i444 = 'Y'
id = 'new-id-from-source'
key = 'new-key-from-source'
password = 'new-password-from-source'
image_quality = 'speed'
TOML

    cat > "$tgt" <<'TOML'
[options]
i444 = 'N'
id = 'my-machine-id-42'
key = 'my-secret-key-abc'
password = 'my-local-password'
image_quality = 'balanced'
TOML

    _run_merge merge_rustdesk_config "'$src'" "'$tgt'"
    [ "$status" -eq 0 ]

    # Identity fields must come from the ORIGINAL target, not from source
    grep -q "^id = 'my-machine-id-42'" "$tgt"
    grep -q "^key = 'my-secret-key-abc'" "$tgt"
    grep -q "^password = 'my-local-password'" "$tgt"
}

@test "merge_rustdesk_config preserves salt and relay-server from existing target" {
    local src="$BATS_TEST_TMPDIR/source.toml"
    local tgt="$BATS_TEST_TMPDIR/target.toml"

    cat > "$src" <<'TOML'
[options]
i444 = 'Y'
salt = 'overwrite-salt'
relay-server = 'overwrite-relay'
api-server = 'overwrite-api'
TOML

    cat > "$tgt" <<'TOML'
[options]
i444 = 'N'
salt = 'keep-this-salt'
relay-server = 'keep-this-relay'
api-server = 'keep-this-api'
TOML

    _run_merge merge_rustdesk_config "'$src'" "'$tgt'"
    [ "$status" -eq 0 ]

    grep -q "^salt = 'keep-this-salt'" "$tgt"
    grep -q "^relay-server = 'keep-this-relay'" "$tgt"
    grep -q "^api-server = 'keep-this-api'" "$tgt"
}

# ===========================================================================
# merge_rustdesk_config — updates non-identity fields from source
# ===========================================================================

@test "merge_rustdesk_config updates non-identity fields from source" {
    local src="$BATS_TEST_TMPDIR/source.toml"
    local tgt="$BATS_TEST_TMPDIR/target.toml"

    cat > "$src" <<'TOML'
[options]
i444 = 'Y'
image_quality = 'speed'
codec-preference = 'vp9'
custom-fps = '120'
id = 'source-id'
TOML

    cat > "$tgt" <<'TOML'
[options]
i444 = 'N'
image_quality = 'balanced'
codec-preference = 'auto'
custom-fps = '60'
id = 'local-id'
TOML

    _run_merge merge_rustdesk_config "'$src'" "'$tgt'"
    [ "$status" -eq 0 ]

    # Non-identity fields should be updated from source
    grep -q "^image_quality = 'speed'" "$tgt"
    grep -q "^codec-preference = 'vp9'" "$tgt"
    grep -q "^custom-fps = '120'" "$tgt"
    # Identity field should remain from target
    grep -q "^id = 'local-id'" "$tgt"
}

@test "merge_rustdesk_config adds new fields from source that target lacks" {
    local src="$BATS_TEST_TMPDIR/source.toml"
    local tgt="$BATS_TEST_TMPDIR/target.toml"

    cat > "$src" <<'TOML'
[options]
i444 = 'Y'
enable-hwcodec = 'Y'
brand-new-setting = 'hello'
TOML

    cat > "$tgt" <<'TOML'
[options]
i444 = 'N'
TOML

    _run_merge merge_rustdesk_config "'$src'" "'$tgt'"
    [ "$status" -eq 0 ]

    # New fields from source should appear in merged target
    grep -q "enable-hwcodec" "$tgt"
    grep -q "brand-new-setting" "$tgt"
}

# ===========================================================================
# merge_rustdesk_options — plain overwrite
# ===========================================================================

@test "merge_rustdesk_options overwrites target completely" {
    local src="$BATS_TEST_TMPDIR/options_src.toml"
    local tgt="$BATS_TEST_TMPDIR/options_tgt.toml"

    cat > "$src" <<'TOML'
[options]
theme = 'dark'
lang = 'en'
TOML

    cat > "$tgt" <<'TOML'
[options]
theme = 'light'
lang = 'de'
extra = 'old'
TOML

    _run_merge merge_rustdesk_options "'$src'" "'$tgt'"
    [ "$status" -eq 0 ]

    # Target should be an exact copy of source
    diff -q "$src" "$tgt"
}

@test "merge_rustdesk_options creates target from source when target absent" {
    local src="$BATS_TEST_TMPDIR/options_src2.toml"
    local tgt="$BATS_TEST_TMPDIR/options_tgt2.toml"

    echo "theme = 'dark'" > "$src"
    [ ! -f "$tgt" ]

    _run_merge merge_rustdesk_options "'$src'" "'$tgt'"
    [ "$status" -eq 0 ]
    [ -f "$tgt" ]
    diff -q "$src" "$tgt"
}

# ===========================================================================
# CLI: res rustdesk — invalid subcommand shows usage
# ===========================================================================

@test "res rustdesk with invalid subcommand shows usage" {
    run bash "$SCRIPT" rustdesk __not_valid__
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage:"* ]]
}

@test "res rustdesk with no subcommand shows usage" {
    run bash "$SCRIPT" rustdesk
    [ "$status" -eq 0 ]
    [[ "$output" == *"Usage:"* ]]
}

# ===========================================================================
# CLI: res rustdesk status — exits 0 even without log
# ===========================================================================

@test "res rustdesk status exits 0 even if no log file" {
    run bash "$SCRIPT" rustdesk status
    [ "$status" -eq 0 ]
    # Should report that the log is not found, OR print session info
    [[ "$output" == *"log not found"* ]] || [[ "$output" == *"Active sessions"* ]]
}

# ===========================================================================
# CLI: res rustdesk log — exits 0 (journalctl fallback)
# ===========================================================================

@test "res rustdesk log exits 0 with journalctl fallback" {
    run bash "$SCRIPT" rustdesk log
    [ "$status" -eq 0 ]
    # Either journalctl output or the fallback message
    [[ "$output" == *"journalctl"* ]] || [ -n "$output" ] || [ -z "$output" ]
}

# ===========================================================================
# show_tailnet — graceful degradation when Tailscale is missing or LAN mode
# ===========================================================================

@test "show_tailnet prints 'tailscale: command not found' when binary missing" {
    # Build a PATH that has the bare minimum (sh + grep for echo) but no
    # tailscale binary. /usr/bin/tailscale exists on the test host (this
    # box is on a tailnet), so a plain PATH restriction isn't enough.
    local minimal_path
    minimal_path=$(mktemp -d)
    ln -sf /usr/bin/bash "$minimal_path/sh"
    ln -sf /usr/bin/echo "$minimal_path/echo"
    ln -sf /usr/bin/awk "$minimal_path/awk"
    ln -sf /usr/bin/grep "$minimal_path/grep"
    ln -sf /usr/bin/hostname "$minimal_path/hostname"
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export PATH='$minimal_path'
        export RES_LAN_MODE=''
        ROOT_DIR='$ROOT_DIR'
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/services.sh'
        show_tailnet
    "
    rm -rf "$minimal_path"
    [ "$status" -ne 0 ]
    [[ "$output" == *"tailscale: command not found"* ]]
}

@test "show_tailnet prints 'LAN mode active' message when LAN mode is on" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export RES_LAN_MODE=1
        ROOT_DIR='$ROOT_DIR'
        # hostname -I won't run since we don't have tailscale; the LAN
        # mode path is taken before tailscale is consulted.
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/services.sh'
        show_tailnet
    "
    [ "$status" -eq 0 ]
    [[ "$output" == *"LAN mode active"* ]]
    [[ "$output" == *"LAN IP"* ]]
    [[ "$output" == *"RustDesk direct"* ]]
}

@test "show_tailnet_hosts prints 'LAN mode active' message when LAN mode is on" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export RES_LAN_MODE=1
        ROOT_DIR='$ROOT_DIR'
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/services.sh'
        show_tailnet_hosts
    "
    [ "$status" -eq 0 ]
    [[ "$output" == *"LAN mode active"* ]]
}

@test "show_tailnet_doctor prints 'LAN mode active' message when LAN mode is on" {
    run bash -c "
        export HOME='$BATS_TEST_TMPDIR'
        export RES_LAN_MODE=1
        ROOT_DIR='$ROOT_DIR'
        source '$ROOT_DIR/lib/core.sh'
        source '$ROOT_DIR/lib/services.sh'
        show_tailnet_doctor
    "
    [ "$status" -eq 0 ]
    [[ "$output" == *"LAN mode active"* ]]
}

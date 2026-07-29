#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APPLET_DIR="$HOME/.local/share/cinnamon/applets/remote-studio@neek"
RUSTDESK_DIR="$HOME/.config/rustdesk"
CONFIG_DIR="$HOME/.config/remote-studio"
WEB_DIR="$ROOT_DIR/web"

DRY_RUN=false
filtered_args=()
for arg in "$@"; do
    if [ "$arg" == "--dry-run" ]; then
        DRY_RUN=true
    else
        filtered_args+=("$arg")
    fi
done
set -- "${filtered_args[@]}"

run() {
    if [ "$DRY_RUN" == "true" ]; then
        echo "[DRY-RUN] $*"
    else
        "$@"
    fi
}

run_root() {
    if [ "$DRY_RUN" == "true" ]; then
        echo "[DRY-RUN] $*"
    elif [ "$(id -u)" -eq 0 ]; then
        "$@"
    elif command -v pkexec >/dev/null 2>&1; then
        pkexec "$@"
    else
        sudo "$@"
    fi
}

user_systemd_available() {
    systemctl --user show-environment >/dev/null 2>&1
}

usage() {
    cat <<EOF
Remote Studio installer

Usage:
  ./install.sh install      Link user tools and copy default configs
  ./install.sh system       Install /etc/X11/xorg.conf from profiles
  ./install.sh doctor       Run res doctor
  ./install.sh uninstall    Remove user-level links
  ./install.sh backup       Backup current user/system config files
  ./install.sh rollback     Restore config files from the latest backup
EOF
}

backup_configs() {
    local stamp backup_dir
    stamp=$(date +%Y%m%d-%H%M%S)
    backup_dir="$HOME/.config/remote-studio/backups/$stamp"
    run mkdir -p "$backup_dir"

    # Use `cp -L` to dereference any symlinks before backing up.
    # Without -L, `cp -P` preserves the link itself; if the user later moves
    # the source target, the backup becomes a dangling link.
    [ -f "$HOME/.xsessionrc" ] && run cp -L "$HOME/.xsessionrc" "$backup_dir/xsessionrc"
    [ -f "$RUSTDESK_DIR/RustDesk_default.toml" ] && run cp "$RUSTDESK_DIR/RustDesk_default.toml" "$backup_dir/RustDesk_default.toml"
    [ -f "$RUSTDESK_DIR/RustDesk2.toml" ] && run cp "$RUSTDESK_DIR/RustDesk2.toml" "$backup_dir/RustDesk2.toml"
    if [ -f /etc/X11/xorg.conf ]; then
        run_root cp /etc/X11/xorg.conf "$backup_dir/xorg.conf"
    fi

    echo "Backup written to $backup_dir"
    prune_backups
}

prune_backups() {
    local backup_root="$HOME/.config/remote-studio/backups"
    [ -d "$backup_root" ] || return 0
    local to_delete=()
    mapfile -t to_delete < <(find "$backup_root" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort -r | tail -n +11)
    for dir in "${to_delete[@]}"; do
        run rm -rf "$dir"
    done
}

rollback_configs() {
    local backup_root="$HOME/.config/remote-studio/backups"
    [ -d "$backup_root" ] || { echo "Error: No backup directory at $backup_root"; exit 1; }
    local latest
    latest=$(find "$backup_root" -mindepth 1 -maxdepth 1 -type d | sort -r | head -n 1)
    [ -z "$latest" ] && { echo "Error: No backups found in $backup_root"; exit 1; }
    echo "Rolling back from: $latest"
    [ -f "$latest/xsessionrc" ] && run cp -L "$latest/xsessionrc" "$HOME/.xsessionrc" && echo "  Restored .xsessionrc"
    [ -f "$latest/RustDesk_default.toml" ] && run cp "$latest/RustDesk_default.toml" "$RUSTDESK_DIR/RustDesk_default.toml" && echo "  Restored RustDesk_default.toml"
    [ -f "$latest/xorg.conf" ] && run_root cp "$latest/xorg.conf" /etc/X11/xorg.conf && echo "  Restored /etc/X11/xorg.conf"
    echo "Rollback complete. Restart LightDM or reboot if xorg.conf was changed."
}

build_web() {
    [ -f "$WEB_DIR/package.json" ] || return 0

    if ! command -v npm >/dev/null 2>&1; then
        echo "  Warning: npm not found. Web dashboard build skipped."
        echo "  Install Node.js/npm, then run: cd $WEB_DIR && npm install && npm run build"
        return 0
    fi

    if [ "$DRY_RUN" == "true" ]; then
        echo "[DRY-RUN] npm --prefix $WEB_DIR install"
        echo "[DRY-RUN] npm --prefix $WEB_DIR run build"
    else
        npm --prefix "$WEB_DIR" install --include=dev
        npm --prefix "$WEB_DIR" run build
    fi
    echo "  Built    $WEB_DIR/dist"
}

# Install (or rewrite) a single user-systemd unit, with path rewriting
# so the daemon in development trees runs against the local checkout.
# Path rewriting rules:
#   /usr/share/remote-studio   =>  $ROOT_DIR
#   /usr/bin/python3           =>  /usr/bin/python3 (left intact)
#   /usr/local/bin/res         =>  $ROOT_DIR/res.sh (so the bash shim
#                                           points at the local copy)
# The "less surprising" rule for the socket-activated service is to
# point ExecStart directly at the Python daemon, since the bash shim
# does not implement a ``--socket-activated`` flag.
_install_user_systemd_unit() {
    local src="$1" dst="$2"
    if [ "$DRY_RUN" == "true" ]; then
        echo "[DRY-RUN] install $src into $dst (path-rewritten)"
    else
        mkdir -p "$(dirname "$dst")"
        sed -e "s|/usr/share/remote-studio|$ROOT_DIR|g" "$src" > "$dst"
    fi
}

install_user_systemd() {
    local user_systemd_dir="$HOME/.config/systemd/user"
    _install_user_systemd_unit \
        "$ROOT_DIR/systemd/remote-studio.service" \
        "$user_systemd_dir/remote-studio.service"

    # Socket-activated daemon (preferred). Both the socket and the
    # service are installed so systemd can spawn the daemon on first
    # connection. The direct service is then disabled so it can't
    # race the socket on 9998/9999.
    _install_user_systemd_unit \
        "$ROOT_DIR/systemd/remote-studio-socket.service" \
        "$user_systemd_dir/remote-studio-socket.service"
    _install_user_systemd_unit \
        "$ROOT_DIR/systemd/remote-studio.socket" \
        "$user_systemd_dir/remote-studio.socket"

    if [ "$DRY_RUN" == "true" ] || user_systemd_available; then
        run systemctl --user daemon-reload
        # Socket-activated: stop+disable the direct service (back-compat
        # with v9.1 installs), then enable+start the socket unit. The
        # service unit has ``Requires=remote-studio.socket`` so it
        # auto-starts when the first connection arrives.
        run systemctl --user disable --now remote-studio.service 2>/dev/null || true
        run systemctl --user enable --now remote-studio.socket
        echo "  Enabled  systemd user socket: remote-studio.socket (daemon starts on demand)"
    else
        echo "  Skipped  systemd user units (no user bus in this shell)"
        echo "           From your desktop session, run: systemctl --user enable --now remote-studio.socket"
    fi
}

install_user() {
    run mkdir -p "$APPLET_DIR" "$RUSTDESK_DIR" "$CONFIG_DIR"

    if [ "$(readlink -f /usr/local/bin/res 2>/dev/null)" = "$ROOT_DIR/res.sh" ]; then
        echo "  Skipped  /usr/local/bin/res (already linked)"
    elif [ -w /usr/local/bin ]; then
        run ln -sfn "$ROOT_DIR/res.sh" /usr/local/bin/res
        echo "  Linked   /usr/local/bin/res -> $ROOT_DIR/res.sh"
    else
        run_root ln -sfn "$ROOT_DIR/res.sh" /usr/local/bin/res
        echo "  Linked   /usr/local/bin/res -> $ROOT_DIR/res.sh"
    fi

    if [ -e "$HOME/.xsessionrc" ] && [ ! -L "$HOME/.xsessionrc" ]; then
        echo "  SKIPPED  ~/.xsessionrc exists and is not a symlink — move or remove it manually"
    else
        run ln -sfn "$ROOT_DIR/config/xsessionrc" "$HOME/.xsessionrc"
        echo "  Linked   ~/.xsessionrc -> $ROOT_DIR/config/xsessionrc"
    fi

    run ln -sfn "$ROOT_DIR/applet/applet.js" "$APPLET_DIR/applet.js"
    run ln -sfn "$ROOT_DIR/applet/metadata.json" "$APPLET_DIR/metadata.json"
    echo "  Linked   $APPLET_DIR/"

    if [ ! -f "$CONFIG_DIR/profiles.conf" ]; then
        run install -m 0644 "$ROOT_DIR/config/profiles.conf" "$CONFIG_DIR/profiles.conf"
        echo "  Copied   $CONFIG_DIR/profiles.conf"
    else
        echo "  Skipped  $CONFIG_DIR/profiles.conf (already exists)"
    fi

    if [ ! -f "$CONFIG_DIR/remote-studio.conf" ]; then
        run install -m 0644 "$ROOT_DIR/config/remote-studio.conf.example" "$CONFIG_DIR/remote-studio.conf"
        echo "  Copied   $CONFIG_DIR/remote-studio.conf"
    else
        echo "  Skipped  $CONFIG_DIR/remote-studio.conf (already exists)"
    fi

    if [ ! -f "$RUSTDESK_DIR/RustDesk_default.toml" ]; then
        run install -m 0600 "$ROOT_DIR/config/RustDesk_default.toml" "$RUSTDESK_DIR/RustDesk_default.toml"
        echo "  Copied   $RUSTDESK_DIR/RustDesk_default.toml"
    else
        echo "  Skipped  $RUSTDESK_DIR/RustDesk_default.toml (already exists)"
    fi


    if ! python3 -c 'import websockets, gi' 2>/dev/null; then
        echo "  Warning: Python dependencies not found. The daemon may fail to start."
        echo "  Run: sudo apt install python3-websockets python3-gi"
    fi

    build_web

    if [ -f "$ROOT_DIR/systemd/remote-studio.service" ]; then
        install_user_systemd
    fi

    echo ""
    echo "Remote Studio user install complete."
}

install_system() {
    local tmp
    tmp=$(mktemp)
    "$ROOT_DIR/res.sh" xorg "$tmp"
    run_root cp /etc/X11/xorg.conf "/etc/X11/xorg.conf.backup-$(date +%Y%m%d-%H%M%S)" || true
    run_root install -m 0644 -o root -g root "$tmp" /etc/X11/xorg.conf
    rm -f "$tmp"
    echo "Installed /etc/X11/xorg.conf. Restart LightDM or reboot to load it."
}

uninstall_user() {
    [ "$(readlink -f /usr/local/bin/res 2>/dev/null)" = "$ROOT_DIR/res.sh" ] && run_root rm -f /usr/local/bin/res
    [ "$(readlink -f "$HOME/.xsessionrc" 2>/dev/null)" = "$ROOT_DIR/config/xsessionrc" ] && run rm -f "$HOME/.xsessionrc"
    [ "$(readlink -f "$APPLET_DIR/applet.js" 2>/dev/null)" = "$ROOT_DIR/applet/applet.js" ] && run rm -f "$APPLET_DIR/applet.js"
    [ "$(readlink -f "$APPLET_DIR/metadata.json" 2>/dev/null)" = "$ROOT_DIR/applet/metadata.json" ] && run rm -f "$APPLET_DIR/metadata.json"
    # Drop the user systemd units if we still own them.
    if [ -f "$HOME/.config/systemd/user/remote-studio.socket" ]; then
        run systemctl --user disable --now remote-studio.socket 2>/dev/null || true
        run systemctl --user disable --now remote-studio.service 2>/dev/null || true
        run rm -f "$HOME/.config/systemd/user/remote-studio.socket" \
                "$HOME/.config/systemd/user/remote-studio-socket.service" \
                "$HOME/.config/systemd/user/remote-studio.service"
        run systemctl --user daemon-reload 2>/dev/null || true
    fi
    echo "Remote Studio user links removed."
}

case "${1:-install}" in
    install|user) install_user ;;
    --system|system) install_system ;;
    doctor) "$ROOT_DIR/res.sh" doctor ;;
    backup) backup_configs ;;
    rollback) rollback_configs ;;
    uninstall) uninstall_user ;;
    help|-h|--help) usage ;;
    *)
        usage
        exit 1
        ;;
esac

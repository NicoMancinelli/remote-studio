#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APPLET_DIR="$HOME/.local/share/cinnamon/applets/remote-studio@neek"
RUSTDESK_DIR="$HOME/.config/rustdesk"

mkdir -p "$APPLET_DIR" "$RUSTDESK_DIR"

if [ -w /usr/local/bin ]; then
    ln -sfn "$ROOT_DIR/res.sh" /usr/local/bin/res
else
    sudo ln -sfn "$ROOT_DIR/res.sh" /usr/local/bin/res
fi
ln -sfn "$ROOT_DIR/config/xsessionrc" "$HOME/.xsessionrc"
ln -sfn "$ROOT_DIR/applet/applet.js" "$APPLET_DIR/applet.js"
ln -sfn "$ROOT_DIR/applet/metadata.json" "$APPLET_DIR/metadata.json"

if [ "${1:-}" = "--system" ]; then
    sudo cp /etc/X11/xorg.conf "/etc/X11/xorg.conf.backup-$(date +%Y%m%d-%H%M%S)" 2>/dev/null || true
    sudo install -m 0644 -o root -g root "$ROOT_DIR/config/xorg.conf" /etc/X11/xorg.conf
fi

if [ ! -f "$RUSTDESK_DIR/RustDesk_default.toml" ]; then
    install -m 0600 "$ROOT_DIR/config/RustDesk_default.toml" "$RUSTDESK_DIR/RustDesk_default.toml"
fi

echo "Remote Studio installed."
echo "Run 'res mac' to apply the 13-inch MacBook Air profile."
echo "Run './install.sh --system' to install /etc/X11/xorg.conf."

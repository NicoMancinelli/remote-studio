#!/bin/bash
# build-deb.sh — Build a .deb package for Remote Studio
# Usage: bash package/build-deb.sh
# Requires: dpkg-deb (standard on Debian/Ubuntu/Linux Mint)

set -euo pipefail
umask 022

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---------------------------------------------------------------------------
# Version
# ---------------------------------------------------------------------------
VERSION="$(grep '^VERSION=' "$ROOT_DIR/res.sh" | head -1 | cut -d'"' -f2)"
if [ -z "$VERSION" ]; then
    echo "ERROR: could not extract VERSION from res.sh" >&2
    exit 1
fi

PACKAGE="remote-studio"
ARCH="all"
PKG_DIR="$ROOT_DIR/dist/${PACKAGE}_${VERSION}_${ARCH}"
DEB_OUT="$ROOT_DIR/dist/${PACKAGE}_${VERSION}_${ARCH}.deb"

# ---------------------------------------------------------------------------
# Maintainer — use GitHub noreply address to avoid embedding personal email
# in distributed .deb packages
# ---------------------------------------------------------------------------
MAINTAINER="Nico Mancinelli <nicomancinelli@users.noreply.github.com>"

echo "Building ${PACKAGE} ${VERSION} (${ARCH})"
echo "  Maintainer : $MAINTAINER"
echo "  Staging    : $PKG_DIR"
echo "  Output     : $DEB_OUT"
echo

# ---------------------------------------------------------------------------
# Clean staging area
# ---------------------------------------------------------------------------
rm -rf "$PKG_DIR"
mkdir -p "$PKG_DIR"

# ---------------------------------------------------------------------------
# Install destination paths (inside staging tree)
# ---------------------------------------------------------------------------

copy_file() {
    local src=$1 dst=$2 mode=$3
    mkdir -p "$(dirname "$dst")"
    cp "$src" "$dst"
    chmod "$mode" "$dst"
}

copy_config() {
    local name=$1 mode=${2:-0644}
    copy_file "$ROOT_DIR/config/$name" "$PKG_DIR/usr/share/remote-studio/config/$name" "$mode"
    copy_file "$ROOT_DIR/config/$name" "$PKG_DIR/usr/share/remote-studio/$name" "$mode"
}

# Main application tree. Keep res.sh under /usr/share/remote-studio so ROOT_DIR
# resolves beside config/, lib/, applet/, and install.sh after package install.
copy_file "$ROOT_DIR/res.sh" "$PKG_DIR/usr/share/remote-studio/res.sh" 0755
copy_file "$ROOT_DIR/install.sh" "$PKG_DIR/usr/share/remote-studio/install.sh" 0755
mkdir -p "$PKG_DIR/usr/local/bin"
ln -s /usr/share/remote-studio/res.sh "$PKG_DIR/usr/local/bin/res"

# Cinnamon applet
copy_file "$ROOT_DIR/applet/applet.js" "$PKG_DIR/usr/share/remote-studio/applet/applet.js" 0644
copy_file "$ROOT_DIR/applet/metadata.json" "$PKG_DIR/usr/share/remote-studio/applet/metadata.json" 0644
copy_file "$ROOT_DIR/applet/applet.js" "$PKG_DIR/usr/share/cinnamon/applets/remote-studio@neek/applet.js" 0644
copy_file "$ROOT_DIR/applet/metadata.json" "$PKG_DIR/usr/share/cinnamon/applets/remote-studio@neek/metadata.json" 0644

# Shared data — config templates. The config/ copies keep package installs
# source-layout compatible; the flat copies preserve older fallback paths.
copy_config profiles.conf
copy_config remote-studio.conf.example
copy_config RustDesk_default.toml
copy_config RustDesk_balanced.toml
copy_config RustDesk_quality.toml
copy_config RustDesk_speed.toml
copy_config RustDesk2.options.toml
copy_config xorg.conf
copy_config xsessionrc

# Library modules
for libfile in "$ROOT_DIR/lib/"*.sh; do
    copy_file "$libfile" "$PKG_DIR/usr/share/remote-studio/lib/$(basename "$libfile")" 0644
done

# Logrotate config
copy_file "$ROOT_DIR/config/logrotate.d/remote-studio" "$PKG_DIR/etc/logrotate.d/remote-studio" 0644

# Systemd user unit
copy_file "$ROOT_DIR/config/remote-studio-watch.service" "$PKG_DIR/usr/lib/systemd/user/remote-studio-watch.service" 0644

# ---------------------------------------------------------------------------
# DEBIAN/control
# ---------------------------------------------------------------------------
mkdir -p "$PKG_DIR/DEBIAN"

cat > "$PKG_DIR/DEBIAN/control" <<EOF
Package: $PACKAGE
Version: $VERSION
Architecture: $ARCH
Maintainer: $MAINTAINER
Depends: bash, x11-xserver-utils, whiptail, cinnamon
Recommends: tailscale, rustdesk
Section: misc
Priority: optional
Description: Linux Mint Cinnamon control layer for RustDesk sessions over Tailscale
 Remote Studio manages headless Xorg display modes, device-specific scaling
 profiles, a Cinnamon panel applet, and low-latency RustDesk display defaults.
 .
 After installation, run: res doctor
EOF

# ---------------------------------------------------------------------------
# DEBIAN/postinst
# ---------------------------------------------------------------------------
cat > "$PKG_DIR/DEBIAN/postinst" <<'POSTINST'
#!/bin/bash
set -e

chmod 755 /usr/local/bin/res
chmod 755 /usr/share/remote-studio/res.sh
chmod 755 /usr/share/remote-studio/install.sh

echo ""
echo "Remote Studio installed successfully."
echo "Run 'res doctor' to verify your setup."
echo ""
POSTINST
chmod 0755 "$PKG_DIR/DEBIAN/postinst"

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
mkdir -p "$ROOT_DIR/dist"
dpkg-deb --root-owner-group --build "$PKG_DIR" "$DEB_OUT"

echo ""
echo "Package built: $DEB_OUT"

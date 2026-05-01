#!/bin/bash
# build-deb.sh — Build a .deb package for Remote Studio
# Usage: bash package/build-deb.sh
# Requires: dpkg-deb (standard on Debian/Ubuntu/Linux Mint)

set -euo pipefail

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
# Maintainer — prefer git config, fall back to a generic value
# ---------------------------------------------------------------------------
GIT_NAME="$(git -C "$ROOT_DIR" config user.name 2>/dev/null || true)"
GIT_EMAIL="$(git -C "$ROOT_DIR" config user.email 2>/dev/null || true)"
if [ -n "$GIT_NAME" ] && [ -n "$GIT_EMAIL" ]; then
    MAINTAINER="$GIT_NAME <$GIT_EMAIL>"
else
    MAINTAINER="Remote Studio <remote-studio>"
fi

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

# /usr/local/bin/res
install -D -m 0755 "$ROOT_DIR/res.sh" \
    "$PKG_DIR/usr/local/bin/res"

# Cinnamon applet
install -D -m 0644 "$ROOT_DIR/applet/applet.js" \
    "$PKG_DIR/usr/share/cinnamon/applets/remote-studio@neek/applet.js"
install -D -m 0644 "$ROOT_DIR/applet/metadata.json" \
    "$PKG_DIR/usr/share/cinnamon/applets/remote-studio@neek/metadata.json"

# Shared data — config templates
install -D -m 0644 "$ROOT_DIR/config/profiles.conf" \
    "$PKG_DIR/usr/share/remote-studio/profiles.conf"
install -D -m 0644 "$ROOT_DIR/config/RustDesk_default.toml" \
    "$PKG_DIR/usr/share/remote-studio/RustDesk_default.toml"
install -D -m 0644 "$ROOT_DIR/config/RustDesk_balanced.toml" \
    "$PKG_DIR/usr/share/remote-studio/RustDesk_balanced.toml"
install -D -m 0644 "$ROOT_DIR/config/RustDesk_quality.toml" \
    "$PKG_DIR/usr/share/remote-studio/RustDesk_quality.toml"
install -D -m 0644 "$ROOT_DIR/config/RustDesk_speed.toml" \
    "$PKG_DIR/usr/share/remote-studio/RustDesk_speed.toml"
install -D -m 0644 "$ROOT_DIR/config/xorg.conf" \
    "$PKG_DIR/usr/share/remote-studio/xorg.conf"

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
dpkg-deb --build "$PKG_DIR" "$DEB_OUT"

echo ""
echo "Package built: $DEB_OUT"

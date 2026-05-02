#!/bin/bash
# One-liner install for Remote Studio
# Usage: curl -fsSL https://raw.githubusercontent.com/NicoMancinelli/remote-studio/master/install-remote-studio.sh | bash
# Note: chmod +x this file after download if running directly.
set -euo pipefail

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    cat <<'EOF'
install-remote-studio.sh — One-liner installer for Remote Studio

Usage:
  # Pipe from curl (recommended):
  curl -fsSL https://raw.githubusercontent.com/NicoMancinelli/remote-studio/master/install-remote-studio.sh | bash

  # Run directly:
  bash install-remote-studio.sh [--help]

What it does:
  1. Clones (or updates) the repo to ~/remote-studio
  2. Runs ./install.sh install to symlink res, the applet, and login restore

After install:
  res doctor    Verify your setup
  res mac       Apply the MacBook Air 13" profile
  res           Open the interactive TUI

Requirements:
  Linux Mint 21+ (Cinnamon), git, RustDesk, Tailscale
EOF
    exit 0
fi

REPO="https://github.com/NicoMancinelli/remote-studio.git"
DEST="$HOME/remote-studio"

if [ -d "$DEST/.git" ]; then
    echo "Updating existing install at $DEST..."
    git -C "$DEST" pull --ff-only
else
    echo "Cloning Remote Studio to $DEST..."
    git clone "$REPO" "$DEST"
fi

echo "Running installer..."
bash "$DEST/install.sh" install

echo ""
echo "Remote Studio installed. Run 'res doctor' to verify."

#!/bin/bash
# One-liner install for Remote Studio
# Usage: curl -fsSL https://raw.githubusercontent.com/NicoMancinelli/remote-studio/master/install-remote-studio.sh | bash
# Note: chmod +x this file after download if running directly.
set -euo pipefail

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

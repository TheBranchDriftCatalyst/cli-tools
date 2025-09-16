#!/bin/bash

# CLI Tools PATH Installer
# Adds this project's bin and shell directories to PATH via zshrc

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ZSHRC_PATH="$HOME/.zshrc"
MARKER_START="# CLI-TOOLS-PATH-START"
MARKER_END="# CLI-TOOLS-PATH-END"

echo "🔧 Installing CLI Tools to PATH..."

# Check if already installed
if grep -q "$MARKER_START" "$ZSHRC_PATH" 2>/dev/null; then
    echo "⚠️  CLI Tools PATH already installed. Run uninstall.sh first to reinstall."
    exit 1
fi

# Convert absolute path to use ~ syntax relative to HOME
RELATIVE_PATH="${SCRIPT_DIR#$HOME/}"

# Create the PATH addition block
PATH_BLOCK="
$MARKER_START
# Added by CLI Tools installer - $(date)
export PATH=\"~/$RELATIVE_PATH/bin:\$PATH\"
export PATH=\"~/$RELATIVE_PATH/shell:\$PATH\"
$MARKER_END"

# Add to zshrc
echo "$PATH_BLOCK" >> "$ZSHRC_PATH"

echo "✅ CLI Tools added to PATH!"
echo "📋 Added paths:"
echo "   - ~/$RELATIVE_PATH/bin"
echo "   - ~/$RELATIVE_PATH/shell"
echo ""
echo "🔄 Reload your shell or run: source ~/.zshrc"
echo "🧪 Test with: catalystTest"
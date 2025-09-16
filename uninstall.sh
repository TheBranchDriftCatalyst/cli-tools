#!/bin/bash

# CLI Tools PATH Uninstaller
# Removes this project's PATH additions from zshrc

ZSHRC_PATH="$HOME/.zshrc"
MARKER_START="# CLI-TOOLS-PATH-START"
MARKER_END="# CLI-TOOLS-PATH-END"

echo "🗑️  Uninstalling CLI Tools from PATH..."

# Check if installed
if ! grep -q "$MARKER_START" "$ZSHRC_PATH" 2>/dev/null; then
    echo "⚠️  CLI Tools PATH not found in zshrc. Nothing to uninstall."
    exit 1
fi

# Get the real file path (resolve symlink)
REAL_ZSHRC=$(readlink -f "$ZSHRC_PATH" 2>/dev/null || realpath "$ZSHRC_PATH" 2>/dev/null || echo "$ZSHRC_PATH")

# Create backup
cp "$REAL_ZSHRC" "$REAL_ZSHRC.bak.$(date +%Y%m%d_%H%M%S)"
echo "📋 Backup created: $REAL_ZSHRC.bak.$(date +%Y%m%d_%H%M%S)"

# Remove the CLI Tools block using sed on the real file
sed -i '' "/$MARKER_START/,/$MARKER_END/d" "$REAL_ZSHRC"

echo "✅ CLI Tools removed from PATH!"
echo "🔄 Reload your shell or run: source ~/.zshrc"
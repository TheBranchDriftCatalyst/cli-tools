#!/usr/bin/env bash
set -euo pipefail

# bash <(curl -fsSL https://raw.githubusercontent.com/TheBranchDriftCatalyst/cli-tools/main/scripts/install.sh)
# bash <(curl -fsSL https://raw.githubusercontent.com/TheBranchDriftCatalyst/cli-tools/main/scripts/install.sh) --dry-run


# ---------------
# 🔧 Config
# ---------------
REPO="TheBranchDriftCatalyst/cli-tools"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.catalyst/bin}"
TMP_DIR="$(mktemp -d)"
GITHUB_BASE="https://raw.githubusercontent.com/$REPO"
RELEASE_BASE="https://raw.githubusercontent.com/$REPO/main/releases"
DRY_RUN=true

# ---------------
# 🎨 UX Helpers
# ---------------
bold()   { printf "\033[1m%s\033[0m\n" "$1"; }
green()  { printf "\033[32m%s\033[0m\n" "$1"; }
info()   { bold "💡 $1"; }
yellow() { printf "\033[33m%s\033[0m\n" "$1"; }
done_msg() { green "✅ $1"; }
fail()   { printf "\033[31m❌ %s\033[0m\n" "$1"; exit 1; }
log_dryrun() { $DRY_RUN && yellow "🧪 [dry-run] $1"; }

# ---------------
# 🧰 Argument Parsing
# ---------------
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    *) fail "Unknown argument: $arg" ;;
  esac
done

# ---------------
# 🧼 Cleanup trap
# ---------------
cleanup() {
  [[ "$DRY_RUN" == false ]] && rm -rf "$TMP_DIR"
}
trap cleanup EXIT

# ---------------
# 🚀 Main
# ---------------
info "Fetching latest version..."
VERSION=$(curl -fsSL "$GITHUB_BASE/main/VERSION" | tr -d '[:space:]' | sed 's/^v//')
[[ -n "$VERSION" ]] || fail "Could not fetch version"

TARBALL_NAME="cli-tools-${VERSION}.tar.gz"
TARBALL_URL="$RELEASE_BASE/$TARBALL_NAME"

info "Installing cli-tools version $VERSION"
info "Downloading: $TARBALL_URL"

if [[ "$DRY_RUN" == true ]]; then
  log_dryrun "Would curl $TARBALL_URL → $TMP_DIR/$TARBALL_NAME"
  log_dryrun "Would extract tarball to $INSTALL_DIR"
else
  curl -fsSL "$TARBALL_URL" -o "$TMP_DIR/$TARBALL_NAME" || fail "Failed to download tarball"
  mkdir -p "$INSTALL_DIR"
  tar -xzf "$TMP_DIR/$TARBALL_NAME" -C "$INSTALL_DIR"
  chmod +x "$INSTALL_DIR"/*
  done_msg "Installed to $INSTALL_DIR"
fi

# Optional symlink step
if command -v sudo &>/dev/null && [[ -d "/usr/local/bin" ]]; then
  info "Linking binaries to /usr/local/bin"

  for f in "$INSTALL_DIR"/*; do
    dest="/usr/local/bin/$(basename "$f")"
    if [[ "$DRY_RUN" == true ]]; then
      log_dryrun "Would link $f → $dest"
    else
      sudo ln -sf "$f" "$dest"
    fi
  done

  [[ "$DRY_RUN" == false ]] && done_msg "Linked to /usr/local/bin"
fi

done_msg "Installation ${DRY_RUN:+(dry-run) }complete."

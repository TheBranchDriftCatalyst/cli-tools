#!/usr/bin/env bash

# ---------------
# ✨ Config
# ---------------
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../" && pwd)"
BIN_DIR="$PROJECT_ROOT/bin"
RELEASE_DIR="$PROJECT_ROOT/releases"
VERSION_FILE="$PROJECT_ROOT/VERSION"
TIMESTAMP=$(date +"%Y-%m-%d %H:%M:%S")
DATESTAMP=$(date +"%Y%m%d-%H%M%S")
GIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

if [[ "$GIT_BRANCH" != "main" ]]; then
  fail "This script must be run from the main branch"
fi

# ---------------
# 🎨 UX Helpers
# ---------------
bold()   { printf "\033[1m%s\033[0m\n" "$1"; }
green()  { printf "\033[32m%s\033[0m\n" "$1"; }
yellow() { printf "\033[33m%s\033[0m\n" "$1"; }
red()    { printf "\033[31m%s\033[0m\n" "$1"; }
info()   { bold "🔧 $1"; }
done_msg() { green "✅ $1"; }
skip_msg() { yellow "⏭️  $1"; }
fail()   { red "❌ $1"; exit 1; }

# Output this for logging
info "Project root: $PROJECT_ROOT"
info "Bin directory: $BIN_DIR"
info "Release directory: $RELEASE_DIR"
info "Version file: $VERSION_FILE"

# ---------------
# 🔧 Runtime Flags
# ---------------
BUMP_PART="patch"
DRY_RUN=false
NEW_VERSION=""

# ---------------
# 📦 Parse Args
# ---------------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --major) BUMP_PART="major"; shift ;;
    --minor) BUMP_PART="minor"; shift ;;
    --patch) BUMP_PART="patch"; shift ;;
    --dry-run) DRY_RUN=true; shift ;;
    *) fail "Unknown option: $1" ;;
  esac
done

bump_version() {
  if [[ ! -f "$VERSION_FILE" ]]; then
    echo "0.1.0" > "$VERSION_FILE"
  fi

  VERSION=$(<"$VERSION_FILE")
  VERSION="${VERSION#v}"  # remove leading 'v' if present
  VERSION="$(echo -n "$VERSION" | tr -d '[:space:]')"  # trim whitespace

  [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "Invalid version format: '$VERSION'"

  IFS='.' read -r major minor patch <<< "$VERSION" || fail "Failed to parse version string: '$VERSION'"
  [[ -n "$major" && -n "$minor" && -n "$patch" ]] || fail "Empty parsed version components"

  info "Trying to bump version: $VERSION (part: $BUMP_PART)"

  case "$BUMP_PART" in
    major)
      [[ "$major" =~ ^[0-9]+$ ]] || fail "major is not a number: $major"
      ((major++)); minor=0; patch=0 ;;
    minor)
      [[ "$minor" =~ ^[0-9]+$ ]] || fail "minor is not a number: $minor"
      ((minor++)); patch=0 ;;
    patch)
      [[ "$patch" =~ ^[0-9]+$ ]] || fail "patch is not a number: $patch"
      ((patch++)) ;;
    *)
      fail "Unknown bump type: $BUMP_PART"
      ;;
  esac

  info "Bumping version: $VERSION → ${major}.${minor}.${patch} (${BUMP_PART})"

  NEW_VERSION="${major}.${minor}.${patch}"

  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] Would bump version: $VERSION → $NEW_VERSION"
  else
    echo "$NEW_VERSION" > "$VERSION_FILE"
    info "Bumped version: $VERSION → $NEW_VERSION"
  fi
}

commit_version_bump() {
  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] Would commit version bump"
    return
  fi

  git add "$VERSION_FILE"
  if git diff --cached --quiet; then
    skip_msg "No changes to commit."
  else
    git commit -m "chore(release): bump version to $NEW_VERSION [$TIMESTAMP]"
    done_msg "Committed version bump"
  fi
}

create_tarball() {
  TAR_NAME="catalyst-cli-${NEW_VERSION}.tar.gz"
  mkdir -p "$RELEASE_DIR"

  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] Would create tarball $RELEASE_DIR/$TAR_NAME from $BIN_DIR"
  else
    tar -czf "$RELEASE_DIR/$TAR_NAME" -C "$BIN_DIR" .
    done_msg "Created tarball: $RELEASE_DIR/$TAR_NAME"
  fi
}

tag_and_push() {
  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] Would tag commit with v$NEW_VERSION and push to origin"
    return
  fi

  git tag "v$NEW_VERSION"
  git push origin "$GIT_BRANCH"
  git push origin "v$NEW_VERSION"
  done_msg "Pushed tag v$NEW_VERSION to origin"
}

commit_new_code() {
  # TODO: add commit message here as well, or ask to get from the user
  if [[ "$DRY_RUN" == true ]]; then
    info "[dry-run] Would commit new code"
    return
  fi

  git add .
  if git diff --cached --quiet; then
    skip_msg "No changes to commit."
  else
    git commit -m "chore: update code [$TIMESTAMP]"
    done_msg "Committed new code"
  fi
}

# ---------------
# 🚀 Main
# ---------------
cd "$PROJECT_ROOT"
info "Starting build process (bump: $BUMP_PART, dry-run: $DRY_RUN)"
commit_new_code
bump_version
[[ -n "$NEW_VERSION" ]] || fail "NEW_VERSION not set. Version bump failed."
commit_version_bump
create_tarball
tag_and_push
done_msg "Build complete."

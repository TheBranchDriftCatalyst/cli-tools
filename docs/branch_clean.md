# branch_clean - Git Branch Management Tool

A comprehensive tool for managing git branches with interactive selection, PR integration, and smart cleanup features.

---

## Table of Contents

- [Interactive Mode](#interactive-mode)
- [PR Caching](#pr-caching)
- [Branch Deletion](#branch-deletion)
- [Quick Reference](#quick-reference)

---

## Interactive Mode

### Usage

```bash
# Enter interactive mode
branch_clean -i
# or
branch_clean --interactive

# Combine with other filters
branch_clean -i --locals-only           # Only local branches
branch_clean -i --author "@protecht"    # Filter by author
branch_clean -i --sort merged --desc    # Sort by merge status
```

### Keyboard Shortcuts (fzf)

| Key | Action |
|-----|--------|
| `↑` `↓` | Navigate branches |
| `TAB` | Select/deselect current branch |
| `Ctrl+A` | Select all branches |
| `Ctrl+D` | Deselect all branches |
| `ENTER` | Confirm selection |
| `ESC` | Cancel and exit |
| Type text | Fuzzy filter branches |

### Requirements

- **fzf** - Install with: `brew install fzf`
- **git** - Required
- **gh** + **jq** - Optional, for PR info

---

## PR Caching

### Flag: `--cache-pr`

Caches GitHub PR data locally to avoid repeated API calls and speed up subsequent runs.

### Usage

```bash
# First run - fetches from GitHub and caches
branch_clean --cache-pr

# Subsequent runs - uses cached data (fast!)
branch_clean --cache-pr

# Works with all other flags
branch_clean --cache-pr --locals-only
branch_clean --cache-pr --interactive
branch_clean --cache-pr --author "@protecht"
```

### Cache Details

- **Location**: `${XDG_CACHE_HOME:-$HOME/.cache}/branch_clean/`
- **Filename**: `pr_cache_<repo_hash>.json` (unique per repository)
- **Default TTL**: 3600 seconds (1 hour)

### Performance

| Scenario | Without Cache | With Cache |
|----------|---------------|------------|
| First run | ~6 seconds | ~6 seconds |
| Subsequent runs | ~6 seconds | <1 second |

### Cache Management

```bash
# View cache files
ls -lh ~/.cache/branch_clean/

# Clear cache for all repos
rm -rf ~/.cache/branch_clean/

# Check cache age
stat ~/.cache/branch_clean/pr_cache_*.json
```

### Configuration

Edit these variables in the script:

```bash
PR_CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/branch_clean"
PR_CACHE_TTL=3600  # Cache TTL in seconds (default: 1 hour)
```

---

## Branch Deletion

### Interactive Deletion

```bash
# Basic interactive deletion
branch_clean -i

# Dry run (preview only)
branch_clean -i --dry-run

# With filters
branch_clean -i --locals-only
branch_clean -i --remotes-only
branch_clean -i --author "@your-name"
```

### Workflow

1. **Selection Phase** - Use fzf to select branches
2. **Confirmation Table** - Review selected branches
3. **Deletion Phase** - Branches are deleted with summary

### Safety Features

1. **Current Branch Protection** - Never deletes the branch you're on
2. **Base Branch Protection** - Prevents deletion of main/master/develop
3. **Confirmation Prompt** - Requires explicit `y` confirmation
4. **Dry Run Mode** - Test deletions without making changes

### Branch Types

**Local Branches:**
```bash
git branch -D <branch-name>
```

**Remote Tracking Branches:**
```bash
git branch -D --remote origin/<branch-name>
```
Note: This deletes the local tracking reference, NOT the branch on the remote server.

### Dry Run Example

```bash
$ branch_clean -i --dry-run --locals-only
```

Output:
```
[DRY RUN] Would run: git branch -D feat/DEV-8032/adjudicator-admin-cleanup
[DRY RUN] Would run: git branch -D feat/DEV-8100/test-branch
```

---

## Quick Reference

```bash
# List branches (default view)
branch_clean

# Interactive mode
branch_clean -i

# With PR caching
branch_clean --cache-pr

# Dry run deletion
branch_clean -i --dry-run

# Filter by scope
branch_clean --locals-only
branch_clean --remotes-only

# Filter by author
branch_clean --author "@name"

# Sort options
branch_clean --sort merged --desc
branch_clean --sort pr_state

# Combine options
branch_clean -i --dry-run --cache-pr --locals-only --author "@name"
```

### Common Workflows

**Clean up merged branches:**
```bash
branch_clean -i --locals-only --sort merged --desc
```

**Remove your old branches:**
```bash
branch_clean -i --author "@your-email.com" --dry-run
branch_clean -i --author "@your-email.com"
```

**Clean stale remote tracking refs:**
```bash
branch_clean -i --remotes-only
```

---

## Tips

- Use `--dry-run` first to preview deletions
- Cache PR data with `--cache-pr` for speed
- Use `--debug` to see detailed operations
- Filter by `--locals-only` or `--remotes-only` to be precise

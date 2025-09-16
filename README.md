# CLI Tools Repository

This repository contains several Go-based command-line utilities and shell scripts.

## Status

### Completed Tasks
- [x] Create Deployment script for go binaries (using Taskfile and air-go)
- [x] Audit the bin folder and move shells to shell folder (Added PATH-based installer and organize task)

### Remaining TODO
- [ ] Finish up move to common src folder
- [ ] Clean up the bin folder (remove binaries and move shell scripts to shell src)
- [ ] Update deployment for better approach
  - [ ] Do we shim path
  - [ ] Or do we symlink to bin folder
- [ ] Finish implementing AIR dev mode for go code

# Taskfile

This project uses [Task](https://taskfile.dev/) as the build system. Available tasks:

## Core Tasks
- `task build` - Builds all Go CLI commands from `cmd/` into `./bin/`
- `task install` - Adds `./bin/` and `./shell/` to PATH via zshrc
- `task uninstall` - Removes CLI tools from PATH
- `task permissions` - Sets executable permissions on binaries and shell scripts
- `task clean:caches` - Removes `.task` cache directory

## Development Workflow
1. Create new Go CLI in `cmd/<name>/main.go`
2. Run `task build` to compile
3. Run `task install` to add to PATH (one-time setup)
4. Use `air` for development with hot reload (configured via `.air.toml`)

## Installation
```bash
# Build and install to PATH
task build && task install

# Or just install (includes build)
task install
```

## Variables
- `CMD_SRC` - Source directory for Go commands (`./cmd`)
- `LOCAL_BIN` - Local binary directory (`./bin`)
- `REAL_BIN` - System binary directory (`~/bin`)

# Go CLI Tools

This repository contains several Go-based command-line utilities:

## Available Tools

### catalystTest
Simple Hello World test utility for verifying the build system.
```bash
catalystTest
# Output: Hello, World!
```

### multiProcCLI
Terminal UI for running multiple processes simultaneously with tabbed log viewing.
```bash
multiProcCLI "command1" "command2 --flag" "command3"
```
**Features:**
- Tabbed interface for multiple processes
- Real-time log viewing with timestamps
- Vim-like navigation (`j`/`k` for scroll, `g`/`G` for top/bottom)
- Auto-scroll with manual override

### stor
File storage utility that moves files to current directory and creates symlinks back to original location.
```bash
stor /path/to/file
# Moves file to current directory and creates symlink at original location
# Creates manifest.yaml to track operations
```

### unstor
Reverses `stor` operations using manifest files.
```bash
unstor manifest.yaml
# Restores all files listed in manifest back to original locations
```

## Usage Patterns
- Build: `task build`
- Install: `task install` (adds to PATH via zshrc)
- Uninstall: `task uninstall` (removes from PATH)
- Development: `air` for hot reload during development

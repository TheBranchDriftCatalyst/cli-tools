# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a CLI tools repository that manages a collection of Go-based command-line utilities and shell scripts. The project uses a hybrid approach with both compiled Go binaries and shell scripts, organized in a structured deployment system.

## Build System

The project uses [Task](https://taskfile.dev/) as the primary build system with Go workspaces:

- `task build` - Builds all Go CLI commands from `cmd/` and copies scripts from `scripts/` to `./bin/`
- `task install` - Installs CLI tools to PATH via zshrc
- `task permissions` - Sets executable permissions on binaries and shell scripts
- `task clean:caches` - Removes `.task` cache directory

For development with live reload:
- `air` - Uses Air for hot reloading during Go development (configured via `.air.toml`)

## Architecture

### Directory Structure
```
@cli-tools/
├── cmd/                    # Go CLI applications
│   ├── catalystTest/
│   ├── clean_branches/
│   ├── multiProcCLI/
│   ├── stor/
│   ├── unstor/
│   └── wipctl/            # Has its own go.mod (multi-module)
├── scripts/               # Shell scripts (bash, zsh, perl)
├── bin/                   # Build output (binaries + copied scripts)
├── tests/                 # Test infrastructure
├── go.mod                 # Root Go module
├── go.work                # Go workspace (multi-module support)
└── Taskfile.yaml          # Build system
```

### Go CLI Tools
The main Go applications are:

1. **catalystTest** (`cmd/catalystTest/`) - Simple Hello World test utility
2. **multiProcCLI** (`cmd/multiProcCLI/`) - Terminal UI for running multiple processes with tabbed log viewing
3. **stor** (`cmd/stor/`) - File storage utility that moves files and creates symlinks back
4. **unstor** (`cmd/unstor/`) - Reverses `stor` operations using manifest files
5. **clean_branches** (`cmd/clean_branches/`) - Interactive git branch cleanup tool
6. **wipctl** (`cmd/wipctl/`) - Work-in-progress control tool (separate go.mod)

### Key Dependencies
- `github.com/gizak/termui/v3` - Terminal UI library for multiProcCLI
- `github.com/charmbracelet/bubbletea` - TUI framework for clean_branches
- `gopkg.in/yaml.v3` - YAML processing for stor/unstor manifest files

## Development Workflow

1. Create new Go CLI in `cmd/<name>/main.go`
2. Run `task build` to compile
3. Run `task install` to install system-wide
4. Use `air` for development with hot reload

The build system automatically:
- Compiles each `cmd/*/main.go` to `bin/<dirname>`
- Copies scripts from `scripts/` to `bin/`
- Sets proper permissions

## Multi-Module Support

This project uses Go workspaces (`go.work`) to support multiple modules:
- Root module: `github.com/TheBranchDriftCatalyst/cli-tools`
- wipctl module: `github.com/TheBranchDriftCatalyst/cli-tools/cmd/wipctl`

## Testing

- `task test:go` - Run Go tests
- `task test:shell` - Run shell script tests
- `task test:all` - Run all tests
- `task lint` - Run all linters (golangci-lint + shellcheck)

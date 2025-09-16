# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a CLI tools repository that manages a collection of Go-based command-line utilities and shell scripts. The project uses a hybrid approach with both compiled Go binaries and shell scripts, organized in a structured deployment system.

## Build System

The project uses [Task](https://taskfile.dev/) as the primary build system with Go modules:

- `task build` - Builds all Go CLI commands from `cmd/` into `./bin/`
- `task deploy` - Symlinks built binaries to `~/bin/` for system-wide access
- `task permissions` - Sets executable permissions on binaries and shell scripts
- `task clean:caches` - Removes `.task` cache directory

For development with live reload:
- `air` - Uses Air for hot reloading during Go development (configured via `.air.toml`)

## Architecture

### Directory Structure
- `cmd/` - Go CLI applications, each in its own subdirectory with `main.go`
- `bin/` - Built Go binaries and shell scripts for distribution
- `common/` - Shared Go utilities (currently minimal)
- `shell/` - Shell scripts (being phased out in favor of `bin/`)

### Go CLI Tools
The main Go applications are:

1. **catalystTest** (`cmd/catalystTest/`) - Simple Hello World test utility
2. **multiProcCLI** (`cmd/multiProcCLI/`) - Terminal UI for running multiple processes simultaneously with tabbed log viewing
3. **stor** (`cmd/stor/`) - File storage utility that moves files to current directory and creates symlinks back to original location
4. **unstor** (`cmd/unstor/`) - Reverses `stor` operations using manifest files

### Key Dependencies
- `github.com/gizak/termui/v3` - Terminal UI library for multiProcCLI
- `gopkg.in/yaml.v3` - YAML processing for stor/unstor manifest files

## Development Workflow

1. Create new Go CLI in `cmd/<name>/main.go`
2. Run `task build` to compile
3. Run `task deploy` to install system-wide
4. Use `air` for development with hot reload

The build system automatically:
- Compiles each `cmd/*/main.go` to `bin/<dirname>`
- Sets proper permissions
- Creates symlinks to `~/bin/` for PATH access

## Testing

No formal test framework is currently configured. Test by running built binaries directly.
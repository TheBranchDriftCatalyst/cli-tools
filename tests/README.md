# Test Organization

This directory contains all tests for the CLI tools project, organized by type and purpose.

## Directory Structure

```
tests/
├── README.md                    # This file
├── test-config.yaml             # Test configuration
├── run-tests.sh                 # Unified test runner
├── go/
│   └── testhelpers/
│       └── helpers.go           # Go test helpers and utilities
├── shell/
│   ├── test_shell_scripts.sh    # Shell script test framework
│   ├── test_dexec.bats          # BATS tests for dexec
│   └── test_git_scripts.bats    # BATS tests for git utilities
├── integration/                 # Integration test scenarios
└── fixtures/                    # Test data and fixtures
```

## Running Tests

### Quick Start
```bash
# Run all tests
make test

# Or with the test runner directly
./tests/run-tests.sh all
```

### Specific Test Types
```bash
# Go unit tests only
make test-go

# Shell script tests only
make test-shell

# BATS tests only (requires bats-core)
make test-bats

# Integration tests only
make test-integration
```

### Advanced Options
```bash
# With coverage report
make test-coverage

# Fast tests (skip slow ones)
make test-fast

# Verbose output
make test-verbose

# With linting
./tests/run-tests.sh -l all
```

## Test Types

### 1. Go Unit Tests
- Located alongside source code in `cmd/*/main_test.go`
- Use the test helpers in `tests/go/testhelpers/`
- Cover core functionality, error handling, and edge cases
- Run with: `make test-go`

### 2. Shell Script Tests
- Basic framework: `tests/shell/test_shell_scripts.sh`
- BATS tests: `tests/shell/*.bats`
- Test script existence, permissions, help text, and functionality
- Run with: `make test-shell` or `make test-bats`

### 3. Integration Tests
- End-to-end workflow testing
- Test installation, uninstallation, and tool interactions
- Run with: `make test-integration`

## Adding New Tests

### For Go Applications
1. Create `*_test.go` file alongside your source
2. Use `testhelpers.NewTestEnvironment(t)` for isolated testing
3. Add package to test runner if needed

### For Shell Scripts
1. Add basic tests to `tests/shell/test_shell_scripts.sh`
2. For complex scenarios, create a new `.bats` file
3. Use the helper functions for common operations

## Configuration

Test behavior is controlled by:
- `tests/test-config.yaml` - Test configuration and settings
- `tests/run-tests.sh` - Main test runner with CLI options
- `Makefile` - Convenient make targets
- `Taskfile.yaml` - Task runner integration

## CI/CD Integration

Tests are automatically run in GitHub Actions:
- On push to main/develop branches
- On pull requests
- Multiple Go versions and OS platforms
- Coverage reporting and linting

See `.github/workflows/test.yml` for details.
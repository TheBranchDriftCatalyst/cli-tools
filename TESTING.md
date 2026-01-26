# Testing Guide for CLI Tools

This repository includes comprehensive test coverage for both Go CLI applications and shell scripts, with a well-organized test structure and unified test runner.

## Test Organization

```
├── src/go/cmd/                   # Go source code with co-located tests
│   ├── catalystTest/
│   │   ├── main.go
│   │   └── main_test.go
│   ├── multiProcCLI/
│   │   ├── main.go
│   │   └── main_test.go
│   └── ... (other CLI tools)
├── tests/                        # Centralized test infrastructure
│   ├── run-tests.sh             # Unified test runner
│   ├── test-config.yaml         # Test configuration
│   ├── go/
│   │   └── testhelpers/         # Go test utilities
│   ├── shell/
│   │   ├── test_shell_scripts.sh # Shell test framework
│   │   ├── test_dexec.bats      # BATS tests for dexec
│   │   └── test_git_scripts.bats # BATS tests for git scripts
│   └── integration/             # Integration test scenarios
├── Makefile                     # Make targets for testing
├── Taskfile.yaml               # Task runner integration
└── .github/workflows/test.yml  # CI/CD pipeline
```

## Running Tests

### Prerequisites

1. **Go 1.21+** - for Go application tests
2. **Task** - for running test commands (`brew install go-task/tap/go-task`)
3. **BATS** (optional) - for advanced shell testing (`brew install bats-core`)
4. **shellcheck** (optional) - for shell script linting (`brew install shellcheck`)
5. **golangci-lint** (optional) - for Go linting (`brew install golangci-lint`)

### Available Test Commands

The project supports both Make and Task runners for convenience:

#### Using Make (Recommended)
```bash
# Run all tests
make test

# Specific test types
make test-go              # Go tests only
make test-shell           # Shell script tests only
make test-bats            # BATS tests only
make test-integration     # Integration tests only

# Advanced options
make test-coverage        # Go tests with coverage
make test-fast           # Skip slow tests
make test-verbose        # Verbose output

# Linting
make lint                # All linters
make lint-go             # Go code only
make lint-shell          # Shell scripts only
```

#### Using Task
```bash
task test                # All tests
task test:go             # Go tests only
task test:shell          # Shell tests only
task test:coverage       # With coverage
```

#### Direct Test Runner
```bash
./tests/run-tests.sh [OPTIONS] [SUITE...]

# Examples
./tests/run-tests.sh all              # All tests
./tests/run-tests.sh -v go shell      # Verbose Go and shell tests
./tests/run-tests.sh -c -l go         # Go tests with coverage and linting
```

## Test Types

### 1. Go Unit Tests

Each Go CLI application has comprehensive unit tests covering:

- **Core functionality** - Main logic and algorithms
- **Error handling** - Edge cases and failure scenarios
- **Input validation** - Command-line argument parsing
- **File operations** - Creating, reading, and manipulating files
- **Concurrency** - Thread-safe operations where applicable

Example test structure:
```go
func TestMainFunction(t *testing.T) {
    // Test setup
    tmpDir := setupTestEnv(t)
    defer cleanup(tmpDir)

    // Test execution
    result := runFunction(input)

    // Assertions
    assert.Equal(t, expected, result)
}
```

### 2. Shell Script Tests

#### Basic Framework (`test_shell_scripts.sh`)

Tests all shell scripts for:
- **Existence and permissions** - Scripts are present and executable
- **Help/usage output** - Scripts respond to `--help` and `-h` flags
- **Error handling** - Graceful failure when dependencies missing
- **Basic functionality** - Core operations work as expected

#### BATS Tests

More sophisticated testing using the BATS framework:

**`test_dexec.bats`**:
- Docker dependency checking
- FZF dependency checking
- Graceful failure modes
- Script structure validation

**`test_git_scripts.bats`**:
- Git repository integration
- Git operation validation
- Remote repository handling

### 3. Integration Tests

Integration tests verify:
- **End-to-end workflows** - Complete user scenarios
- **Tool interactions** - How different CLI tools work together
- **Installation process** - `task install` works correctly
- **Cross-platform compatibility** - Tests run on different OS

## Writing New Tests

### Adding Go Tests

1. Create a `*_test.go` file alongside your Go source
2. Follow Go testing conventions:
   ```go
   package main

   import "testing"

   func TestYourFunction(t *testing.T) {
       // Test implementation
   }
   ```
3. Add the test path to `Taskfile.yaml` under `test:go`

### Adding Shell Script Tests

1. **Basic tests**: Add to `test_shell_scripts.sh`
   ```bash
   test_your_script() {
       local script_path="$1"
       run_test "your_test_name" \
           "'$script_path' your_args" \
           0 \
           "Description of what this tests"
   }
   ```

2. **BATS tests**: Create `test_your_script.bats`
   ```bash
   @test "description of test" {
       run ./bin/your_script
       [ "$status" -eq 0 ]
       [[ "$output" =~ "expected pattern" ]]
   }
   ```

## CI/CD Pipeline

The GitHub Actions workflow (`.github/workflows/test.yml`) automatically:

1. **Tests multiple Go versions** (1.21, 1.22)
2. **Runs all test suites** in parallel
3. **Generates coverage reports**
4. **Performs linting** on both Go and shell code
5. **Tests installation process**
6. **Runs integration tests**

### Workflow Triggers

- **Push to main/develop** - Full test suite
- **Pull requests** - Full test suite
- **Manual dispatch** - Can be triggered manually

## Test Coverage

Current test coverage includes:

### Go Applications
- ✅ **catalystTest** - Simple hello world functionality
- ✅ **multiProcCLI** - Process management, UI components, concurrency
- ✅ **stor** - File operations, symlink creation, manifest handling
- ✅ **unstor** - File restoration, path expansion, error handling
- ✅ **clean_branches** - Git operations, filtering, sorting, TUI

### Shell Scripts
- ✅ **git-undo** - Git reset operations
- ✅ **dexec** - Docker container selection and execution
- ✅ **Git utilities** - Various git helper scripts
- ✅ **General utilities** - File operations, system tools

## Troubleshooting Tests

### Common Issues

1. **Tests fail with "command not found"**
   - Ensure `task build` has been run
   - Check that `bin/` directory contains built binaries

2. **BATS tests not running**
   - Install bats-core: `brew install bats-core`
   - Verify installation: `bats --version`

3. **Coverage report not generated**
   - Ensure all Go tests pass first
   - Check that `go tool cover` is available

4. **Shell tests fail on different OS**
   - Check shebang compatibility (`#!/bin/bash` vs `#!/usr/bin/env bash`)
   - Verify required dependencies are available

### Debugging Tests

```bash
# Run specific test with verbose output
go test -v ./src/go/cmd/catalystTest/

# Run single BATS test
bats -t test_dexec.bats

# Debug shell script test
bash -x test_shell_scripts.sh
```

## Contributing

When adding new CLI tools:

1. **Add unit tests** for all Go applications
2. **Add shell tests** for all shell scripts
3. **Update Taskfile.yaml** to include new test paths
4. **Update this documentation** with new test descriptions
5. **Ensure CI passes** before merging

## Future Improvements

- [ ] Performance benchmarking tests
- [ ] Cross-platform testing (Windows, macOS, Linux)
- [ ] Fuzzing tests for input validation
- [ ] Load testing for multiProcCLI
- [ ] Database integration tests (if applicable)
- [ ] Security vulnerability scanning
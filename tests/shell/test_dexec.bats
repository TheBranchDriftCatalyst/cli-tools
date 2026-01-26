#!/usr/bin/env bats
# BATS tests for dexec script
# Install bats: brew install bats-core

setup() {
    # Save original PATH
    export ORIGINAL_PATH="$PATH"

    # Set up test environment
    export TEST_DIR="$(mktemp -d)"
    cd "$TEST_DIR"
}

teardown() {
    # Restore PATH
    export PATH="$ORIGINAL_PATH"

    # Clean up
    cd /
    rm -rf "$TEST_DIR"
}

@test "dexec script exists and is executable" {
    [ -f "./bin/dexec" ]
    [ -x "./bin/dexec" ]
}

@test "dexec shows help with --help" {
    run ./bin/dexec --help
    [ "$status" -eq 0 ]
    [[ "$output" =~ "Usage:" ]] || [[ "$output" =~ "dexec" ]]
}

@test "dexec fails gracefully when docker is not available" {
    # Remove docker from PATH
    export PATH="/tmp"

    run ./bin/dexec
    [ "$status" -ne 0 ]
    [[ "$output" =~ "docker" ]] || [[ "$output" =~ "command not found" ]]
}

@test "dexec fails gracefully when fzf is not available" {
    # Create a fake docker that succeeds
    echo '#!/bin/bash
echo "docker available"
exit 0' > "$TEST_DIR/docker"
    chmod +x "$TEST_DIR/docker"

    # Set PATH to only include our test dir (no fzf)
    export PATH="$TEST_DIR"

    run ./bin/dexec
    [ "$status" -ne 0 ]
}

@test "dexec checks for required dependencies" {
    # Test with neither docker nor fzf
    export PATH="/tmp"

    run ./bin/dexec
    [ "$status" -ne 0 ]
}

@test "dexec script has proper shebang" {
    head -1 ./bin/dexec | grep -q "^#!.*zsh"
}

@test "dexec script uses set -o pipefail for safety" {
    grep -q "set -o pipefail" ./bin/dexec
}
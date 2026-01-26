#!/bin/bash
# Shell script test framework for CLI tools
# Usage: ./test_shell_scripts.sh

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Test results array
declare -a TEST_RESULTS=()

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Test runner function
run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_exit_code="${3:-0}"
    local test_description="${4:-}"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    log_info "Running test: $test_name"
    if [[ -n "$test_description" ]]; then
        echo "  Description: $test_description"
    fi

    # Create temporary directory for test
    local temp_dir
    temp_dir=$(mktemp -d)
    local original_dir="$PWD"

    # Capture output and exit code
    local output
    local exit_code

    cd "$temp_dir" || exit 1

    if output=$(eval "$test_command" 2>&1); then
        exit_code=0
    else
        exit_code=$?
    fi

    cd "$original_dir" || exit 1
    rm -rf "$temp_dir"

    # Check result
    if [[ $exit_code -eq $expected_exit_code ]]; then
        log_success "$test_name"
        PASSED_TESTS=$((PASSED_TESTS + 1))
        TEST_RESULTS+=("PASS: $test_name")
        return 0
    else
        log_error "$test_name (expected exit code $expected_exit_code, got $exit_code)"
        echo "  Output: $output"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        TEST_RESULTS+=("FAIL: $test_name")
        return 1
    fi
}

# Test that script exists and is executable
test_script_exists() {
    local script_path="$1"
    local script_name="$2"

    run_test "script_exists_$script_name" \
        "test -f '$script_path' && test -x '$script_path'" \
        0 \
        "Check if $script_name exists and is executable"
}

# Test script help/usage
test_script_help() {
    local script_path="$1"
    local script_name="$2"

    # Test --help flag
    run_test "help_flag_$script_name" \
        "'$script_path' --help" \
        0 \
        "Check if $script_name supports --help flag"

    # Test -h flag
    run_test "h_flag_$script_name" \
        "'$script_path' -h" \
        0 \
        "Check if $script_name supports -h flag"
}

# Test script with no arguments (should show usage or work)
test_script_no_args() {
    local script_path="$1"
    local script_name="$2"
    local expected_exit="${3:-1}"

    run_test "no_args_$script_name" \
        "'$script_path'" \
        "$expected_exit" \
        "Check behavior when $script_name is called with no arguments"
}

# Test git-undo specifically
test_git_undo() {
    local script_path="$1"

    # Create a git repo and test
    run_test "git_undo_functional" \
        "git init . && \
         echo 'test' > test.txt && \
         git add test.txt && \
         git commit -m 'test commit' && \
         echo 'test2' > test2.txt && \
         git add test2.txt && \
         git commit -m 'second commit' && \
         '$script_path' && \
         git status --porcelain | grep -q 'A  test2.txt'" \
        0 \
        "Test git-undo functionality with a real git repository"
}

# Test dexec with Docker not available (should fail gracefully)
test_dexec_no_docker() {
    local script_path="$1"

    run_test "dexec_no_docker" \
        "PATH=/tmp '$script_path'" \
        1 \
        "Test dexec behavior when docker is not available"
}

# Main test execution
main() {
    local bin_dir="./bin"

    echo "========================================"
    echo "Shell Script Test Suite"
    echo "========================================"
    echo

    # Check if bin directory exists
    if [[ ! -d "$bin_dir" ]]; then
        log_error "bin directory not found. Please run from project root."
        exit 1
    fi

    # Test git-undo
    if [[ -f "$bin_dir/git-undo" ]]; then
        log_info "Testing git-undo..."
        test_script_exists "$bin_dir/git-undo" "git-undo"
        test_git_undo "$bin_dir/git-undo"
    else
        log_warning "git-undo not found, skipping tests"
    fi

    # Test dexec
    if [[ -f "$bin_dir/dexec" ]]; then
        log_info "Testing dexec..."
        test_script_exists "$bin_dir/dexec" "dexec"
        test_script_help "$bin_dir/dexec" "dexec"
        test_dexec_no_docker "$bin_dir/dexec"
    else
        log_warning "dexec not found, skipping tests"
    fi

    # Test other shell scripts
    local shell_scripts=(
        "git-cal"
        "git-url"
        "op"
        "tovim"
        "workOn"
        "palette"
        "battery"
        "cleanup"
        "gcb"
        "gcf"
        "icheckout"
    )

    for script in "${shell_scripts[@]}"; do
        if [[ -f "$bin_dir/$script" ]]; then
            log_info "Testing $script..."
            test_script_exists "$bin_dir/$script" "$script"

            # Check if script is actually a shell script
            if head -1 "$bin_dir/$script" | grep -q "^#!.*sh"; then
                test_script_no_args "$bin_dir/$script" "$script"
            else
                log_info "$script appears to be a binary, skipping shell-specific tests"
            fi
        else
            log_warning "$script not found, skipping tests"
        fi
    done

    # Summary
    echo
    echo "========================================"
    echo "Test Summary"
    echo "========================================"
    echo "Total tests: $TOTAL_TESTS"
    echo -e "Passed: ${GREEN}$PASSED_TESTS${NC}"
    echo -e "Failed: ${RED}$FAILED_TESTS${NC}"
    echo

    if [[ $FAILED_TESTS -eq 0 ]]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        echo
        echo "Failed tests:"
        for result in "${TEST_RESULTS[@]}"; do
            if [[ $result == FAIL:* ]]; then
                echo "  - ${result#FAIL: }"
            fi
        done
        exit 1
    fi
}

# Run tests
main "$@"
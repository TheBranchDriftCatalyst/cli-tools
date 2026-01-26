#!/usr/bin/env bats
# BATS tests for git-related scripts

setup() {
    export TEST_DIR="$(mktemp -d)"
    cd "$TEST_DIR"

    # Set up a test git repository
    git init .
    git config user.email "test@example.com"
    git config user.name "Test User"
}

teardown() {
    cd /
    rm -rf "$TEST_DIR"
}

@test "git-undo script exists and is executable" {
    [ -f "./bin/git-undo" ]
    [ -x "./bin/git-undo" ]
}

@test "git-undo has correct shebang" {
    head -1 ./bin/git-undo | grep -q "^#!.*bash"
}

@test "git-undo works with a basic commit" {
    # Create a commit
    echo "test content" > test.txt
    git add test.txt
    git commit -m "Initial commit"

    # Create another commit
    echo "second content" > test2.txt
    git add test2.txt
    git commit -m "Second commit"

    # Record the commit hash before undo
    local last_commit
    last_commit=$(git rev-parse HEAD)

    # Run git-undo
    run ./bin/git-undo

    # Check that the command succeeded
    [ "$status" -eq 0 ]

    # Check that HEAD has moved back
    local new_head
    new_head=$(git rev-parse HEAD)
    [ "$new_head" != "$last_commit" ]

    # Check that test2.txt is now staged but not committed
    git status --porcelain | grep -q "A  test2.txt"
}

@test "git-undo fails gracefully outside git repository" {
    cd /tmp
    run ./bin/git-undo
    [ "$status" -ne 0 ]
}

@test "git-url script exists and is executable" {
    [ -f "./bin/git-url" ]
    [ -x "./bin/git-url" ]
}

@test "git-cal script exists and is executable" {
    [ -f "./bin/git-cal" ]
    [ -x "./bin/git-cal" ]
}

@test "git scripts work in git repository context" {
    # Set up a proper git repo with remote
    git remote add origin https://github.com/test/test.git

    # Test git-url (if it exists and is a shell script)
    if [ -f "./bin/git-url" ] && head -1 ./bin/git-url | grep -q "^#!"; then
        run ./bin/git-url
        # Should not crash (exit code varies by implementation)
        [ "$status" -le 1 ]
    fi

    # Test git-cal (if it exists and is a shell script)
    if [ -f "./bin/git-cal" ] && head -1 ./bin/git-cal | grep -q "^#!"; then
        run ./bin/git-cal
        # Should not crash (exit code varies by implementation)
        [ "$status" -le 1 ]
    fi
}
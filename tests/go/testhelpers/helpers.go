package testhelpers

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestEnvironment provides a clean test environment
type TestEnvironment struct {
	TempDir     string
	OriginalDir string
	t           *testing.T
}

// NewTestEnvironment creates a new isolated test environment
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "cli-tools-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	env := &TestEnvironment{
		TempDir:     tempDir,
		OriginalDir: originalDir,
		t:           t,
	}

	t.Cleanup(env.Cleanup)

	return env
}

// Cleanup removes the test environment
func (e *TestEnvironment) Cleanup() {
	e.t.Helper()

	// Restore original directory
	if err := os.Chdir(e.OriginalDir); err != nil {
		e.t.Logf("Warning: failed to restore original directory: %v", err)
	}

	// Remove temp directory
	if err := os.RemoveAll(e.TempDir); err != nil {
		e.t.Logf("Warning: failed to remove temp directory: %v", err)
	}
}

// ChDir changes to the test directory
func (e *TestEnvironment) ChDir() {
	e.t.Helper()

	if err := os.Chdir(e.TempDir); err != nil {
		e.t.Fatalf("Failed to change to temp directory: %v", err)
	}
}

// CreateFile creates a file with content in the test directory
func (e *TestEnvironment) CreateFile(name, content string) string {
	e.t.Helper()

	path := filepath.Join(e.TempDir, name)

	// Create directory if needed
	if dir := filepath.Dir(path); dir != e.TempDir {
		if err := os.MkdirAll(dir, 0755); err != nil {
			e.t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.t.Fatalf("Failed to create file %s: %v", path, err)
	}

	return path
}

// CreateExecutableScript creates an executable script
func (e *TestEnvironment) CreateExecutableScript(name, content string) string {
	e.t.Helper()

	path := e.CreateFile(name, content)
	if err := os.Chmod(path, 0755); err != nil {
		e.t.Fatalf("Failed to make script executable: %v", err)
	}

	return path
}

// FileExists checks if a file exists in the test directory
func (e *TestEnvironment) FileExists(name string) bool {
	e.t.Helper()

	path := filepath.Join(e.TempDir, name)
	_, err := os.Stat(path)
	return err == nil
}

// FileContent reads file content from the test directory
func (e *TestEnvironment) FileContent(name string) string {
	e.t.Helper()

	path := filepath.Join(e.TempDir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		e.t.Fatalf("Failed to read file %s: %v", path, err)
	}

	return string(content)
}

// AssertFileExists asserts that a file exists
func (e *TestEnvironment) AssertFileExists(name string) {
	e.t.Helper()

	if !e.FileExists(name) {
		e.t.Errorf("File %s should exist but doesn't", name)
	}
}

// AssertFileNotExists asserts that a file does not exist
func (e *TestEnvironment) AssertFileNotExists(name string) {
	e.t.Helper()

	if e.FileExists(name) {
		e.t.Errorf("File %s should not exist but does", name)
	}
}

// AssertFileContent asserts file content matches expected
func (e *TestEnvironment) AssertFileContent(name, expected string) {
	e.t.Helper()

	actual := e.FileContent(name)
	if actual != expected {
		e.t.Errorf("File %s content mismatch:\nExpected: %q\nActual: %q", name, expected, actual)
	}
}

// GitTestRepo provides git repository testing utilities
type GitTestRepo struct {
	*TestEnvironment
}

// NewGitTestRepo creates a new git test repository
func NewGitTestRepo(t *testing.T) *GitTestRepo {
	t.Helper()

	env := NewTestEnvironment(t)
	env.ChDir()

	repo := &GitTestRepo{TestEnvironment: env}

	// Initialize git repo
	repo.RunCommand("git", "init", ".")
	repo.RunCommand("git", "config", "user.email", "test@example.com")
	repo.RunCommand("git", "config", "user.name", "Test User")

	return repo
}

// RunCommand runs a command and fails test if it fails
func (r *GitTestRepo) RunCommand(name string, args ...string) {
	r.t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = r.TempDir
	if err := cmd.Run(); err != nil {
		r.t.Fatalf("Command failed: %s %v: %v", name, args, err)
	}
}

// AddCommit adds a file and commits it
func (r *GitTestRepo) AddCommit(filename, content, message string) {
	r.t.Helper()

	r.CreateFile(filename, content)
	r.RunCommand("git", "add", filename)
	r.RunCommand("git", "commit", "-m", message)
}
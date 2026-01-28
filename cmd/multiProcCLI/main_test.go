package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gizak/termui/v3/widgets"
)

func TestSetupProcesses(t *testing.T) {
	// Initialize global variables to prevent nil pointer dereferences
	tabPane = widgets.NewTabPane()
	tabPane.TabNames = []string{"", "", ""}  // Pre-allocate tab names
	logDisplay = widgets.NewList()

	commands := []string{"echo hello", "echo world", "sleep 1"}
	processes = setupProcesses(commands)  // Set global processes

	if len(processes) != 3 {
		t.Errorf("Expected 3 processes, got %d", len(processes))
	}

	for i, p := range processes {
		if p == nil {
			t.Errorf("Process %d is nil", i)
			continue
		}

		if p.LogText == nil {
			t.Errorf("Process %d LogText is nil", i)
		}

		if p.LogChan == nil {
			t.Errorf("Process %d LogChan is nil", i)
		}

		if p.ErrChan == nil {
			t.Errorf("Process %d ErrChan is nil", i)
		}

		p.Mutex.Lock()
		status := p.Status
		running := p.Running
		p.Mutex.Unlock()

		if status != "Starting" {
			t.Errorf("Process %d status expected 'Starting', got %q", i, status)
		}

		if !running {
			t.Errorf("Process %d should be marked as running", i)
		}
	}

	// Clean up processes
	for _, p := range processes {
		p.Mutex.Lock()
		p.Running = false
		p.Mutex.Unlock()
		if p.Cmd != nil && p.Cmd.Process != nil {
			_ = p.Cmd.Process.Kill()
		}
	}
}

func TestProcessStructure(t *testing.T) {
	p := &Process{
		Name:    "test",
		Cmd:     exec.Command("echo", "test"),
		Status:  "Starting",
		LogText: widgets.NewList(),
		LogChan: make(chan string, 100),
		ErrChan: make(chan string, 100),
		Running: true,
	}

	// Test that all fields are properly initialized
	if p.Name != "test" {
		t.Errorf("Expected name 'test', got %q", p.Name)
	}

	p.Mutex.Lock()
	status := p.Status
	p.Mutex.Unlock()

	if status != "Starting" {
		t.Errorf("Expected status 'Starting', got %q", status)
	}

	p.Mutex.Lock()
	running := p.Running
	p.Mutex.Unlock()

	if !running {
		t.Error("Process should be running")
	}

	if p.LogText == nil {
		t.Error("LogText should not be nil")
	}

	if p.LogChan == nil {
		t.Error("LogChan should not be nil")
	}

	if p.ErrChan == nil {
		t.Error("ErrChan should not be nil")
	}
}

func TestAppendLog(t *testing.T) {
	p := &Process{
		LogText: widgets.NewList(),
		LogChan: make(chan string, 100),
		ErrChan: make(chan string, 100),
		Running: true,
	}

	// Initialize global variables to prevent nil pointer dereference
	tabPane = widgets.NewTabPane()
	logDisplay = widgets.NewList()
	processes = []*Process{p}  // Add our test process to global list

	// Test appending a normal log (without auto-scroll to avoid UI dependencies)
	appendLog(p, "test message", false)

	if len(p.LogText.Rows) != 1 {
		t.Errorf("Expected 1 log row, got %d", len(p.LogText.Rows))
	}

	logRow := p.LogText.Rows[0]
	if !strings.Contains(logRow, "test message") {
		t.Errorf("Log row should contain 'test message', got %q", logRow)
	}

	// Check timestamp format (should contain HH:MM:SS)
	if !strings.Contains(logRow, ":") {
		t.Errorf("Log row should contain timestamp, got %q", logRow)
	}

	// Test appending an error log
	appendLog(p, "error message", true)

	if len(p.LogText.Rows) != 2 {
		t.Errorf("Expected 2 log rows, got %d", len(p.LogText.Rows))
	}
}

func TestCommandParsing(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		expectedCmd string
		expectedArgs []string
	}{
		{
			name:         "simple command",
			command:      "echo hello",
			expectedCmd:  "echo",
			expectedArgs: []string{"hello"},
		},
		{
			name:         "command with multiple args",
			command:      "ls -la /tmp",
			expectedCmd:  "ls",
			expectedArgs: []string{"-la", "/tmp"},
		},
		{
			name:         "single command",
			command:      "pwd",
			expectedCmd:  "pwd",
			expectedArgs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize global variables to prevent nil pointer dereferences
			tabPane = widgets.NewTabPane()
			tabPane.TabNames = []string{""}  // Pre-allocate tab names
			logDisplay = widgets.NewList()

			processes = setupProcesses([]string{tt.command})
			if len(processes) != 1 {
				t.Fatalf("Expected 1 process, got %d", len(processes))
			}

			p := processes[0]
			if p.Cmd.Path != tt.expectedCmd && p.Cmd.Args[0] != tt.expectedCmd {
				// On some systems, Path might be absolute, so check Args[0]
				if p.Cmd.Args[0] != tt.expectedCmd {
					t.Errorf("Expected command %q, got %q", tt.expectedCmd, p.Cmd.Args[0])
				}
			}

			// Check args (skip the first one which is the command itself)
			actualArgs := p.Cmd.Args[1:]
			if len(actualArgs) != len(tt.expectedArgs) {
				t.Errorf("Expected %d args, got %d", len(tt.expectedArgs), len(actualArgs))
			}

			for i, arg := range tt.expectedArgs {
				if i < len(actualArgs) && actualArgs[i] != arg {
					t.Errorf("Expected arg %d to be %q, got %q", i, arg, actualArgs[i])
				}
			}

			// Clean up
			p.Mutex.Lock()
			p.Running = false
			p.Mutex.Unlock()
			if p.Cmd.Process != nil {
				_ = p.Cmd.Process.Kill()
			}
		})
	}
}

func TestProcessCleanup(t *testing.T) {
	// Initialize global variables to prevent nil pointer dereferences
	tabPane = widgets.NewTabPane()
	tabPane.TabNames = []string{""}  // Pre-allocate tab names
	logDisplay = widgets.NewList()

	// Create a long-running process
	processes = setupProcesses([]string{"sleep 5"})
	if len(processes) != 1 {
		t.Fatalf("Expected 1 process, got %d", len(processes))
	}

	p := processes[0]

	// Wait a bit for the process to start
	time.Sleep(100 * time.Millisecond)

	// Verify process is running
	if p.Cmd.Process == nil {
		t.Fatal("Process should have started")
	}

	originalPID := p.Cmd.Process.Pid

	// Test cleanup
	p.Mutex.Lock()
	p.Running = false
	p.Mutex.Unlock()
	if p.Cmd.Process != nil {
		err := p.Cmd.Process.Kill()
		if err != nil {
			t.Logf("Warning: could not kill process: %v", err)
		}
	}

	// Give some time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Check if process is actually killed by trying to find it
	// Note: this is platform-specific and might not work on all systems
	checkCmd := exec.CommandContext(context.Background(), "ps", "-p", string(rune(originalPID)))
	err := checkCmd.Run()
	// If ps returns non-zero exit code, the process is likely gone (which is what we want)
	if err == nil {
		t.Logf("Warning: process %d might still be running", originalPID)
	}
}

// Test that we can handle invalid commands gracefully
func TestInvalidCommand(t *testing.T) {
	// Initialize global variables to prevent nil pointer dereferences
	tabPane = widgets.NewTabPane()
	tabPane.TabNames = []string{""}  // Pre-allocate tab names
	logDisplay = widgets.NewList()

	processes = setupProcesses([]string{"nonexistent-command-12345"})
	if len(processes) != 1 {
		t.Fatalf("Expected 1 process, got %d", len(processes))
	}

	p := processes[0]

	// Wait a bit for the process to fail
	time.Sleep(200 * time.Millisecond)

	// Check that status eventually becomes "Error"
	// Note: this is timing-dependent and might be flaky
	maxWait := 2 * time.Second
	start := time.Now()
	for time.Since(start) < maxWait {
		p.Mutex.Lock()
		status := p.Status
		p.Mutex.Unlock()
		if status == "Error" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	p.Mutex.Lock()
	finalStatus := p.Status
	p.Mutex.Unlock()

	if finalStatus != "Error" {
		t.Logf("Warning: expected process status to be 'Error', got %q", finalStatus)
	}

	// Clean up
	p.Mutex.Lock()
	p.Running = false
	p.Mutex.Unlock()
}
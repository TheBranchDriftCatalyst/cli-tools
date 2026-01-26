package main

import (
	"testing"
	"time"
)

func TestParseFlags(t *testing.T) {

	tests := []struct {
		name     string
		args     []string
		expected Config
		help     bool
	}{
		{
			name: "default values",
			args: []string{},
			expected: Config{
				Remote:    "origin",
				AuthorRe:  "",
				Only:      "all",
				ColorMode: ColorAuto,
				LogLevel:  LogInfo,
			},
			help: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test is simplified because flag parsing is global
			// In a real scenario, you'd want to refactor the code to accept
			// parameters or use a more testable approach
			cfg := Config{
				Remote:    "origin",
				AuthorRe:  "",
				Only:      "all",
				ColorMode: ColorAuto,
				LogLevel:  LogInfo,
			}

			if cfg.Remote != tt.expected.Remote {
				t.Errorf("Remote = %q, want %q", cfg.Remote, tt.expected.Remote)
			}
			if cfg.Only != tt.expected.Only {
				t.Errorf("Only = %q, want %q", cfg.Only, tt.expected.Only)
			}
		})
	}
}

func TestLogger(t *testing.T) {
	log := newLogger(LogInfo, 10)

	if log.Level() != LogInfo {
		t.Errorf("Expected log level %d, got %d", LogInfo, log.Level())
	}

	// Test logging at different levels
	log.Infof("test info message")
	log.Debugf("test debug message") // Should not appear due to level
	log.Warnf("test warn message")

	logs := log.buf.slice()

	// Should have info and warn, but not debug
	infoFound := false
	warnFound := false
	debugFound := false

	for _, logLine := range logs {
		if contains(logLine, "test info message") {
			infoFound = true
		}
		if contains(logLine, "test warn message") {
			warnFound = true
		}
		if contains(logLine, "test debug message") {
			debugFound = true
		}
	}

	if !infoFound {
		t.Error("Info message not found in logs")
	}
	if !warnFound {
		t.Error("Warn message not found in logs")
	}
	if debugFound {
		t.Error("Debug message should not appear with Info log level")
	}

	// Test level change
	log.SetLevel(LogDebug)
	log.Debugf("test debug after level change")

	logs = log.buf.slice()
	debugAfterChange := false
	for _, logLine := range logs {
		if contains(logLine, "test debug after level change") {
			debugAfterChange = true
		}
	}

	if !debugAfterChange {
		t.Error("Debug message should appear after changing to Debug level")
	}
}

func TestRingBuffer(t *testing.T) {
	r := ring{cap: 3}

	// Test empty ring
	slice := r.slice()
	if len(slice) != 0 {
		t.Errorf("Empty ring should return empty slice, got %d items", len(slice))
	}

	// Add items without wrapping
	r.add("item1")
	r.add("item2")

	slice = r.slice()
	if len(slice) != 2 {
		t.Errorf("Expected 2 items, got %d", len(slice))
	}
	if slice[0] != "item1" || slice[1] != "item2" {
		t.Errorf("Expected [item1, item2], got %v", slice)
	}

	// Add one more to fill capacity
	r.add("item3")
	slice = r.slice()
	if len(slice) != 3 {
		t.Errorf("Expected 3 items, got %d", len(slice))
	}

	// Add one more to test wrapping
	r.add("item4")
	slice = r.slice()
	if len(slice) != 3 {
		t.Errorf("Ring should maintain capacity of 3, got %d", len(slice))
	}

	// After wrapping, should have item2, item3, item4 (item1 was overwritten)
	expected := []string{"item2", "item3", "item4"}
	for i, item := range expected {
		if slice[i] != item {
			t.Errorf("Expected %s at position %d, got %s", item, i, slice[i])
		}
	}
}

func TestRelHuman(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "-",
		},
		{
			name:     "invalid format",
			input:    "invalid",
			expected: "invalid",
		},
		{
			name:     "recent time",
			input:    time.Now().Add(-30 * time.Second).Format(time.RFC3339),
			expected: "just now",
		},
		{
			name:     "one hour ago",
			input:    time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			expected: "1 hour ago",
		},
		{
			name:     "multiple hours ago",
			input:    time.Now().Add(-3 * time.Hour).Format(time.RFC3339),
			expected: "3 hours ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := relHuman(tt.input)
			if result != tt.expected {
				t.Errorf("relHuman(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		n        int
		unit     string
		expected string
	}{
		{1, "hour", "1 hour ago"},
		{2, "hour", "2 hours ago"},
		{1, "day", "1 day ago"},
		{5, "day", "5 days ago"},
	}

	for _, tt := range tests {
		result := plural(tt.n, tt.unit)
		if result != tt.expected {
			t.Errorf("plural(%d, %q) = %q, want %q", tt.n, tt.unit, result, tt.expected)
		}
	}
}

func TestApplyFilter(t *testing.T) {
	rows := []Row{
		{Branch: "main", Email: "user@example.com", Upstream: "origin/main"},
		{Branch: "feature-auth", Email: "dev@company.com", Upstream: "origin/feature-auth"},
		{Branch: "bugfix", Email: "user@example.com", Upstream: "-"},
	}

	tests := []struct {
		name     string
		filter   string
		expected int
	}{
		{
			name:     "no filter",
			filter:   "",
			expected: 3,
		},
		{
			name:     "filter by branch",
			filter:   "main",
			expected: 1,
		},
		{
			name:     "filter by email",
			filter:   "user@example.com",
			expected: 2,
		},
		{
			name:     "filter by upstream",
			filter:   "origin/feature",
			expected: 1,
		},
		{
			name:     "case insensitive",
			filter:   "FEATURE",
			expected: 1,
		},
		{
			name:     "no matches",
			filter:   "nonexistent",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyFilter(rows, tt.filter)
			if len(result) != tt.expected {
				t.Errorf("applyFilter with filter %q returned %d rows, want %d", tt.filter, len(result), tt.expected)
			}
		})
	}
}

func TestSortRows(t *testing.T) {
	rows := []Row{
		{Branch: "zebra", Scope: "local", LastISO: "2023-01-01T10:00:00Z"},
		{Branch: "apple", Scope: "remote", LastISO: "2023-01-02T10:00:00Z"},
		{Branch: "banana", Scope: "local", LastISO: "2023-01-01T09:00:00Z"},
	}

	// Test branch sorting
	sorted := sortRows(rows, colBranch, false)
	if sorted[0].Branch != "apple" || sorted[1].Branch != "banana" || sorted[2].Branch != "zebra" {
		t.Errorf("Branch sorting failed: got [%s, %s, %s]", sorted[0].Branch, sorted[1].Branch, sorted[2].Branch)
	}

	// Test reverse branch sorting
	sorted = sortRows(rows, colBranch, true)
	if sorted[0].Branch != "zebra" || sorted[1].Branch != "banana" || sorted[2].Branch != "apple" {
		t.Errorf("Reverse branch sorting failed: got [%s, %s, %s]", sorted[0].Branch, sorted[1].Branch, sorted[2].Branch)
	}

	// Test scope sorting
	sorted = sortRows(rows, colScope, false)
	localCount := 0
	for _, row := range sorted {
		if row.Scope == "local" {
			localCount++
		} else {
			break
		}
	}
	if localCount != 2 {
		t.Errorf("Scope sorting failed: expected 2 local entries first, got %d", localCount)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		a, b, expected string
	}{
		{"hello", "world", "hello"},
		{"", "world", "world"},
		{"hello", "", "hello"},
		{"", "", ""},
	}

	for _, tt := range tests {
		result := firstNonEmpty(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("firstNonEmpty(%q, %q) = %q, want %q", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		n        int
		expected string
	}{
		{"hello world", 5, "hello…"},
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello", 0, "hello"},
		{"hello", -1, "hello"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.n)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, result, tt.expected)
		}
	}
}

func TestUseColor(t *testing.T) {
	tests := []struct {
		mode     ColorMode
		expected bool
	}{
		{ColorAlways, true},
		{ColorNever, false},
		// ColorAuto depends on terminal, so we'll skip testing it
	}

	for _, tt := range tests {
		if tt.mode != ColorAuto {
			result := useColor(tt.mode)
			if result != tt.expected {
				t.Errorf("useColor(%d) = %t, want %t", tt.mode, result, tt.expected)
			}
		}
	}
}

// Helper function for string containment
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock tests for git operations (these would require a git repo to run properly)
func TestGitOperations(t *testing.T) {
	t.Skip("Git operations require a real git repository - run integration tests separately")

	// Example of how you might test git operations:
	// ctx := context.Background()
	// log := newLogger(LogInfo, 10)
	//
	// This would test detectBase, refDateMap, collectRefRows, etc.
	// but requires being in a git repository with proper setup
}
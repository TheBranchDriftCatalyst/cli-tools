package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with tilde",
			input:    "~/test/file.txt",
			expected: "", // Will be filled with actual home dir
		},
		{
			name:     "absolute path",
			input:    "/usr/bin/test",
			expected: "/usr/bin/test",
		},
		{
			name:     "relative path",
			input:    "test/file.txt",
			expected: "test/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)

			if tt.name == "path with tilde" {
				// Check that ~ was expanded to actual home directory
				home, _ := os.UserHomeDir()
				expected := filepath.Join(home, "test/file.txt")
				if result != expected {
					t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, expected)
				}
			} else {
				if result != tt.expected {
					t.Errorf("expandPath(%q) = %q, want %q", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestUnstor(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "unstor_test")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file to be "restored"
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Create a manifest file
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	originalPath := filepath.Join(tmpDir, "original", "test.txt")

	// Create the original directory
	if err := os.MkdirAll(filepath.Dir(originalPath), 0755); err != nil {
		t.Fatalf("Error creating original directory: %v", err)
	}

	manifest := Manifest{
		Entries: []ManifestEntry{
			{
				OriginalPath: originalPath,
				NewPath:      testFile,
			},
		},
	}

	manifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		t.Fatalf("Error marshalling manifest: %v", err)
	}

	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("Error writing manifest file: %v", err)
	}

	// Change to the temp directory and run unstor
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error changing to temp directory: %v", err)
	}

	// Set up args and run main
	os.Args = []string{"unstor", "manifest.yaml"}
	main()

	// Check if file was moved back to original location
	if _, err := os.Stat(originalPath); os.IsNotExist(err) {
		t.Errorf("File was not restored to original location: %s", originalPath)
	}

	// Check if file was removed from new location
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Errorf("File still exists at new location: %s", testFile)
	}

	// Verify content
	content, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatalf("Error reading restored file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("File content mismatch. Expected %q, got %q", "test content", string(content))
	}
}

func TestUnstor_MissingFile(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "unstor_missing_test")
	if err != nil {
		t.Fatalf("Error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a manifest file with a non-existent file
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	originalPath := filepath.Join(tmpDir, "original", "test.txt")
	newPath := filepath.Join(tmpDir, "nonexistent.txt")

	manifest := Manifest{
		Entries: []ManifestEntry{
			{
				OriginalPath: originalPath,
				NewPath:      newPath,
			},
		},
	}

	manifestData, err := yaml.Marshal(&manifest)
	if err != nil {
		t.Fatalf("Error marshalling manifest: %v", err)
	}

	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		t.Fatalf("Error writing manifest file: %v", err)
	}

	// Change to the temp directory
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Error changing to temp directory: %v", err)
	}

	// Set up args and run main (should not panic, just skip)
	os.Args = []string{"unstor", "manifest.yaml"}

	// This should not panic or fail, just skip the missing file
	main()
}
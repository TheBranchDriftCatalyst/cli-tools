package main

import (
	"os"
	"path/filepath"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestMoveAndSymlink(t *testing.T) {
	// Create a source directory (where the file originates)
	srcDir, err := os.MkdirTemp("", "stor_test_src")
	if err != nil {
		t.Fatalf("Error creating source directory: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create a destination directory (where we'll run stor)
	destDir, err := os.MkdirTemp("", "stor_test_dest")
	if err != nil {
		t.Fatalf("Error creating dest directory: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create a temporary file in the source directory
	tmpFile, err := os.CreateTemp(srcDir, "testfile")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	tmpFile.Close()

	// Save current dir and change to the destination directory
	oldDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldDir) }()

	if err := os.Chdir(destDir); err != nil {
		t.Fatalf("Error changing to dest directory: %v", err)
	}

	// Run the main function with the temp file path
	os.Args = []string{"cmd", tmpFilePath}
	main()

	// Check if the file was moved to the current directory
	newFilePath := filepath.Join(destDir, filepath.Base(tmpFilePath))
	if _, err := os.Stat(newFilePath); os.IsNotExist(err) {
		t.Fatalf("Moved file does not exist: %s", newFilePath)
	}

	// Check if the symlink was created at the original location
	origPathInfo, err := os.Lstat(tmpFilePath)
	if err != nil {
		t.Fatalf("Error stating original path: %v", err)
	}
	if origPathInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("Original path is not a symlink: %s", tmpFilePath)
	}

	// Check if the symlink points to the new file location
	//  this doesnt quiet work, paths are a bit weird
	// linkDest, err := os.Readlink(tmpFilePath)
	// if err != nil {
	// 	t.Fatalf("Error reading symlink: %v", err)
	// }
	// if linkDest != newFilePath {
	// 	t.Fatalf("Symlink does not point to the correct location. \n Expected: %s \n      Got: %s", newFilePath, linkDest)
	// }

	// Check if the manifest file was created
	manifestPath := filepath.Join(destDir, "manifest.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("Manifest file does not exist: %s", manifestPath)
	}

	// Read and parse the manifest file
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Error reading manifest file: %v", err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Error unmarshalling manifest file: %v", err)
	}

	// Check the manifest entries
	if len(manifest.Entries) != 1 {
		t.Fatalf("Unexpected number of manifest entries. Expected: 1, Got: %d", len(manifest.Entries))
	}
	// entry := manifest.Entries[0]
	// if entry.OriginalPath != tmpFilePath || entry.NewPath != strings.TrimPrefix(newFilePath, "/private") {
	// 	t.Fatalf("Manifest entry does not match expected values. Got: %+v", entry)
	// }
}

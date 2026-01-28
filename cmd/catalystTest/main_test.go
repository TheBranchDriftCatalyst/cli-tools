package main

import (
	"os"
	"testing"
)

func TestMain(t *testing.T) {
	// Capture output for testing
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	// Run main function
	main()

	// Restore stdout
	w.Close()
	os.Stdout = old

	// Read captured output
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	output := string(buf[:n])
	expected := "Hello, World!\n"

	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}
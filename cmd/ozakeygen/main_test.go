package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOzakeygenSmoke(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "test.key")
	if err := run(outPath); err != nil {
		t.Fatalf("ozakeygen: %v", err)
	}
	// Verify the key file was created.
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("key file not created: %v", err)
	}
}

func TestOzakeygenStdout(t *testing.T) {
	// Empty outPath writes to stdout.
	if err := run(""); err != nil {
		t.Fatalf("ozakeygen stdout: %v", err)
	}
}

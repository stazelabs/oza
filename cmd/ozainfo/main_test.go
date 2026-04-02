package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stazelabs/oza/cmd/internal/testutil"
)

func TestOzainfoSmoke(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)

	cmd := exec.Command(os.Args[0], "-test.run=^$") // dummy; we call run() directly
	_ = cmd // not used; we test run() directly

	if err := run(path); err != nil {
		t.Fatalf("ozainfo run: %v", err)
	}
}

func TestOzainfoJSON(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	jsonOutput = true
	defer func() { jsonOutput = false }()
	if err := run(path); err != nil {
		t.Fatalf("ozainfo --json run: %v", err)
	}
}

func TestOzainfoMissingFile(t *testing.T) {
	if err := run("/nonexistent/path.oza"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

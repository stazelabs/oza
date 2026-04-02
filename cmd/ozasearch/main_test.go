package main

import (
	"testing"

	"github.com/stazelabs/oza/cmd/internal/testutil"
)

func TestOzasearchSmoke(t *testing.T) {
	path := testutil.BuildTestOZA(t, true)
	if err := run(path, "alpha", 10, false, false); err != nil {
		t.Fatalf("ozasearch: %v", err)
	}
}

func TestOzasearchJSON(t *testing.T) {
	path := testutil.BuildTestOZA(t, true)
	if err := run(path, "alpha", 10, true, false); err != nil {
		t.Fatalf("ozasearch --json: %v", err)
	}
}

func TestOzasearchTitleOnly(t *testing.T) {
	path := testutil.BuildTestOZA(t, true)
	if err := run(path, "main", 10, false, true); err != nil {
		t.Fatalf("ozasearch --title-only: %v", err)
	}
}

func TestOzasearchNoIndex(t *testing.T) {
	path := testutil.BuildTestOZA(t, false)
	if err := run(path, "alpha", 10, false, false); err == nil {
		t.Fatal("expected error for archive without search index")
	}
}

func TestOzasearchMissingFile(t *testing.T) {
	if err := run("/nonexistent/path.oza", "query", 10, false, false); err == nil {
		t.Fatal("expected error for missing file")
	}
}

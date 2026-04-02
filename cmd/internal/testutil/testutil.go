// Package testutil provides shared test helpers for CLI tool smoke tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stazelabs/oza/ozawrite"
)

// BuildTestOZA creates a small OZA archive in a temp file and returns its path.
// The archive contains a few HTML entries and metadata, suitable for basic
// smoke tests of CLI tools.
func BuildTestOZA(t *testing.T, search bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.oza")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: search,
	}
	w := ozawrite.NewWriter(f, opts)
	w.SetMetadata("title", "Test Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")
	w.SetMetadata("main_entry", "0")

	if _, err := w.AddEntry("index.html", "Main Page", "text/html",
		[]byte(`<html><body><h1>Main</h1><p>Welcome to the test archive.</p></body></html>`), true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddEntry("articles/alpha.html", "Alpha", "text/html",
		[]byte(`<html><body><h1>Alpha</h1><p>Alpha content.</p></body></html>`), true); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddEntry("style.css", "Style", "text/css",
		[]byte(`body { margin: 0; }`), false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddRedirect("old.html", "Old", 0); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

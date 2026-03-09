package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stazelabs/gozim/zim"

	"github.com/stazelabs/oza/oza"
)

const testZIM = "../../testdata/small.zim"

func zimAvailable(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(testZIM); os.IsNotExist(err) {
		t.Skip("testdata/small.zim not found; run 'make testdata' first")
	}
}

func TestConvert(t *testing.T) {
	zimAvailable(t)

	outPath := filepath.Join(t.TempDir(), "output.oza")

	// Convert.
	c, err := NewConverter(testZIM, outPath, ConvertOptions{
		ZstdLevel:   3, // fast for tests
		DictSamples: 100,
		ChunkSize:   512 * 1024,
		TrainDict:   true,
		BuildSearch: false,
	})
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	defer c.Close()

	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	c.stats.Print(os.Stdout)

	// Open the output OZA and verify.
	a, err := oza.Open(outPath)
	if err != nil {
		t.Fatalf("oza.Open: %v", err)
	}
	defer a.Close()

	if a.EntryCount() == 0 {
		t.Fatal("OZA has zero entries")
	}

	// Verify checksums.
	results, err := a.VerifyAll()
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("Checksum failed for %s", r.ID)
		}
	}
}

func TestConvertContentMatch(t *testing.T) {
	zimAvailable(t)

	outPath := filepath.Join(t.TempDir(), "output.oza")

	c, err := NewConverter(testZIM, outPath, ConvertOptions{
		ZstdLevel:   3,
		DictSamples: 100,
		ChunkSize:   512 * 1024,
		TrainDict:   false, // simpler for content matching
		BuildSearch: false,
	})
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	defer c.Close()

	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Open both archives.
	za, err := zim.Open(testZIM)
	if err != nil {
		t.Fatalf("zim.Open: %v", err)
	}
	defer za.Close()

	a, err := oza.Open(outPath)
	if err != nil {
		t.Fatalf("oza.Open: %v", err)
	}
	defer a.Close()

	// For each content entry in the ZIM, find it in the OZA and compare bytes.
	matched := 0
	for entry := range za.Entries() {
		ns := entry.Namespace()
		if entry.IsRedirect() {
			continue
		}
		ozaPath, cat := mapZIMPath(ns, entry.Path())
		if cat != categoryContent {
			continue
		}

		zimContent, err := entry.ReadContent()
		if err != nil {
			t.Errorf("ZIM ReadContent(%s): %v", ozaPath, err)
			continue
		}

		ozaEntry, err := a.EntryByPath(ozaPath)
		if err != nil {
			t.Errorf("OZA EntryByPath(%s): %v", ozaPath, err)
			continue
		}

		ozaContent, err := ozaEntry.ReadContent()
		if err != nil {
			t.Errorf("OZA ReadContent(%s): %v", ozaPath, err)
			continue
		}

		if len(zimContent) != len(ozaContent) {
			t.Errorf("Content size mismatch for %s: ZIM=%d OZA=%d", ozaPath, len(zimContent), len(ozaContent))
			continue
		}
		for j := range zimContent {
			if zimContent[j] != ozaContent[j] {
				t.Errorf("Content byte mismatch for %s at offset %d", ozaPath, j)
				break
			}
		}
		matched++
	}

	if matched == 0 {
		t.Fatal("No content entries matched")
	}
	t.Logf("Verified %d content entries byte-for-byte", matched)
}

func TestConvertRedirects(t *testing.T) {
	zimAvailable(t)

	outPath := filepath.Join(t.TempDir(), "output.oza")

	c, err := NewConverter(testZIM, outPath, ConvertOptions{
		ZstdLevel:   3,
		DictSamples: 100,
		ChunkSize:   512 * 1024,
		TrainDict:   false,
		BuildSearch: false,
	})
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	defer c.Close()

	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	a, err := oza.Open(outPath)
	if err != nil {
		t.Fatalf("oza.Open: %v", err)
	}
	defer a.Close()

	// Verify that all redirect entries resolve without error.
	resolved := 0
	for entry := range a.Entries() {
		if !entry.IsRedirect() {
			continue
		}
		_, err := entry.Resolve()
		if err != nil {
			t.Errorf("Redirect %s failed to resolve: %v", entry.Path(), err)
			continue
		}
		resolved++
	}
	t.Logf("Verified %d redirects resolve correctly", resolved)
}

func TestDryRun(t *testing.T) {
	zimAvailable(t)

	c, err := NewConverter(testZIM, "/dev/null", ConvertOptions{
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("NewConverter: %v", err)
	}
	defer c.Close()

	if err := c.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if c.stats.EntryTotal == 0 {
		t.Fatal("DryRun reported zero entries")
	}
	t.Logf("Dry run: %d entries", c.stats.EntryTotal)
}

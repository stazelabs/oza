package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/stazelabs/gozim/zim"

	"github.com/stazelabs/oza/oza"
)

// convertAndOpen runs the conversion pipeline and returns open ZIM and OZA handles.
// Both must be closed by the caller. Skips the test if testdata/small.zim is absent.
func convertAndOpen(t *testing.T) (*zim.Archive, *oza.Archive) {
	t.Helper()
	zimAvailable(t)

	outPath := filepath.Join(t.TempDir(), "fidelity.oza")
	c, err := NewConverter(testZIM, outPath, ConvertOptions{
		ZstdLevel:   3,
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

	za, err := zim.Open(testZIM)
	if err != nil {
		t.Fatalf("zim.Open: %v", err)
	}

	a, err := oza.Open(outPath)
	if err != nil {
		za.Close()
		t.Fatalf("oza.Open: %v", err)
	}
	return za, a
}

// TestFidelityContentMatch verifies that every non-redirect ZIM content entry
// round-trips byte-for-byte through the OZA archive (no-minify conversion).
func TestFidelityContentMatch(t *testing.T) {
	za, a := convertAndOpen(t)
	defer za.Close()
	defer a.Close()

	matched, failed := 0, 0
	for entry := range za.Entries() {
		if entry.IsRedirect() {
			continue
		}
		ozaPath, cat := mapZIMPath(entry.Namespace(), entry.Path())
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
			failed++
			continue
		}

		ozaContent, err := ozaEntry.ReadContent()
		if err != nil {
			t.Errorf("OZA ReadContent(%s): %v", ozaPath, err)
			failed++
			continue
		}

		if !bytes.Equal(zimContent, ozaContent) {
			t.Errorf("content mismatch at %s: ZIM=%d bytes OZA=%d bytes", ozaPath, len(zimContent), len(ozaContent))
			failed++
			continue
		}
		matched++
	}

	if matched == 0 {
		t.Fatal("no content entries matched")
	}
	if failed > 0 {
		t.Errorf("%d entries missing or mismatched; %d matched", failed, matched)
	}
	t.Logf("verified %d content entries byte-for-byte", matched)
}

// TestFidelityRedirectChains verifies that every ZIM redirect resolves to the
// same final content path in the OZA archive.
func TestFidelityRedirectChains(t *testing.T) {
	za, a := convertAndOpen(t)
	defer za.Close()
	defer a.Close()

	checked, failed := 0, 0
	for entry := range za.Entries() {
		if !entry.IsRedirect() {
			continue
		}
		srcPath, cat := mapZIMPath(entry.Namespace(), entry.Path())
		if cat != categoryContent {
			continue
		}

		// Resolve the ZIM chain to its final content entry.
		zimFinal, err := entry.Resolve()
		if err != nil {
			t.Errorf("ZIM Resolve(%s): %v", srcPath, err)
			failed++
			continue
		}
		wantPath, targetCat := mapZIMPath(zimFinal.Namespace(), zimFinal.Path())
		if targetCat != categoryContent {
			continue
		}

		// Find the corresponding OZA redirect entry.
		ozaEntry, err := a.EntryByPath(srcPath)
		if err != nil {
			t.Errorf("OZA EntryByPath(%s): %v", srcPath, err)
			failed++
			continue
		}
		if !ozaEntry.IsRedirect() {
			t.Errorf("OZA entry %s: expected redirect, got content entry", srcPath)
			failed++
			continue
		}

		// Resolve the OZA chain and compare final paths.
		ozaFinal, err := ozaEntry.Resolve()
		if err != nil {
			t.Errorf("OZA Resolve(%s): %v", srcPath, err)
			failed++
			continue
		}
		if ozaFinal.Path() != wantPath {
			t.Errorf("redirect %s: ZIM final=%s OZA final=%s", srcPath, wantPath, ozaFinal.Path())
			failed++
			continue
		}
		checked++
	}

	if failed > 0 {
		t.Errorf("%d redirect chains failed; %d verified", failed, checked)
	}
	t.Logf("verified %d redirect chains", checked)
}

// TestFidelityMetadata verifies that required metadata keys are present in the
// OZA archive and that ZIM metadata values are preserved after conversion.
func TestFidelityMetadata(t *testing.T) {
	za, a := convertAndOpen(t)
	defer za.Close()
	defer a.Close()

	// Required OZA keys must always be present and non-empty.
	for _, key := range []string{"title", "language", "creator", "date", "source"} {
		val, err := a.Metadata(key)
		if err != nil {
			t.Errorf("required OZA metadata key %q missing: %v", key, err)
			continue
		}
		if val == "" {
			t.Errorf("required OZA metadata key %q is empty", key)
		}
	}

	// For each ZIM M/ metadata entry, the value should be preserved in OZA
	// under the normalised (lowercase) key name.
	for entry := range za.EntriesByNamespace('M') {
		ozaKey := mapMetadataKey(entry.Path())
		zimVal, err := entry.ReadContent()
		if err != nil {
			continue
		}
		ozaVal, err := a.Metadata(ozaKey)
		if err != nil {
			// Optional keys may be absent; skip silently.
			continue
		}
		if ozaVal != string(zimVal) {
			t.Errorf("metadata %q: ZIM=%q OZA=%q", ozaKey, string(zimVal), ozaVal)
		}
	}
}

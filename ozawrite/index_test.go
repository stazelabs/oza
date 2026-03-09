package ozawrite_test

import (
	"os"
	"testing"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

func TestIndexV3RoundTrip(t *testing.T) {
	// Use the public Writer API to build an archive, then verify the path/title
	// indices produce correct results via the reader.
	paths := []string{
		"A/Albert_Einstein",
		"A/Alfred_Nobel",
		"A/Quantum_mechanics",
		"B/Biology",
		"B/Botany",
		"C/Chemistry",
	}
	titles := []string{
		"Albert Einstein",
		"Alfred Nobel",
		"Quantum mechanics",
		"Biology",
		"Botany",
		"Chemistry",
	}

	a := buildTestArchive(t, paths, titles)
	defer a.Close()

	// Verify all entries can be looked up by path.
	for i, path := range paths {
		e, err := a.EntryByPath(path)
		if err != nil {
			t.Errorf("EntryByPath(%q): %v", path, err)
			continue
		}
		if e.Path() != path {
			t.Errorf("entry %d: Path = %q, want %q", i, e.Path(), path)
		}
		if e.Title() != titles[i] {
			t.Errorf("entry %d: Title = %q, want %q", i, e.Title(), titles[i])
		}
	}

	// Verify all entries can be looked up by title.
	for _, title := range titles {
		e, err := a.EntryByTitle(title)
		if err != nil {
			t.Errorf("EntryByTitle(%q): %v", title, err)
			continue
		}
		if e.Title() != title {
			t.Errorf("EntryByTitle: got %q, want %q", e.Title(), title)
		}
	}

	// Verify EntryByID returns correct path and title.
	for i := range paths {
		e, err := a.EntryByID(uint32(i))
		if err != nil {
			t.Errorf("EntryByID(%d): %v", i, err)
			continue
		}
		if e.Path() != paths[i] {
			t.Errorf("EntryByID(%d): Path = %q, want %q", i, e.Path(), paths[i])
		}
		if e.Title() != titles[i] {
			t.Errorf("EntryByID(%d): Title = %q, want %q", i, e.Title(), titles[i])
		}
	}
}

func TestIndexV3EmptyKeys(t *testing.T) {
	// Edge case: entries with empty path or title.
	a := buildTestArchive(t, []string{"", "a.html"}, []string{"", "A"})
	defer a.Close()

	e, err := a.EntryByPath("")
	if err != nil {
		t.Fatalf("EntryByPath(empty): %v", err)
	}
	if e.Path() != "" {
		t.Errorf("Path = %q, want empty", e.Path())
	}
}

func TestIndexV3SingleEntry(t *testing.T) {
	a := buildTestArchive(t, []string{"only.html"}, []string{"Only"})
	defer a.Close()

	e, err := a.EntryByPath("only.html")
	if err != nil {
		t.Fatalf("EntryByPath: %v", err)
	}
	if e.Title() != "Only" {
		t.Errorf("Title = %q, want %q", e.Title(), "Only")
	}
}

func TestIndexV3ManyEntries(t *testing.T) {
	// Force multiple restart blocks (>64 entries).
	n := 200
	paths := make([]string, n)
	titles := make([]string, n)
	for i := 0; i < n; i++ {
		paths[i] = "A/" + string(rune('A'+i/26)) + string(rune('a'+i%26))
		titles[i] = "Title " + string(rune('A'+i/26)) + string(rune('a'+i%26))
	}

	a := buildTestArchive(t, paths, titles)
	defer a.Close()

	// Spot-check lookups across different restart blocks.
	for _, idx := range []int{0, 63, 64, 65, 127, 128, 199} {
		e, err := a.EntryByPath(paths[idx])
		if err != nil {
			t.Errorf("EntryByPath(%q): %v", paths[idx], err)
			continue
		}
		if e.Title() != titles[idx] {
			t.Errorf("entry %d: Title = %q, want %q", idx, e.Title(), titles[idx])
		}
	}
}

// buildTestArchive creates an in-memory archive with the given paths and titles.
func buildTestArchive(t *testing.T, paths, titles []string) *oza.Archive {
	t.Helper()
	if len(paths) != len(titles) {
		t.Fatalf("paths and titles must have same length")
	}

	f, err := createTempFile(t)
	if err != nil {
		t.Fatal(err)
	}

	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
	}
	w := ozawrite.NewWriter(f, opts)
	w.SetMetadata("title", "Test")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	for i := range paths {
		if _, err := w.AddEntry(paths[i], titles[i], "text/html", []byte("<html>"+titles[i]+"</html>"), false); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func createTempFile(t *testing.T) (*os.File, error) {
	return os.CreateTemp(t.TempDir(), "test*.oza")
}

package oza_test

import (
	"bytes"
	"os"
	"sync"
	"testing"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

// --- helpers ---

func newTestArchive(t *testing.T, fn func(w *ozawrite.Writer)) (*oza.Archive, func()) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test*.oza")
	if err != nil {
		t.Fatal(err)
	}
	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
	}
	w := ozawrite.NewWriter(f, opts)
	setRequiredMeta(w)
	fn(w)
	if err := w.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	return a, func() { a.Close() }
}

func setRequiredMeta(w *ozawrite.Writer) {
	w.SetMetadata("title", "Test Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")
}

// --- TestRoundTrip ---

func TestRoundTrip(t *testing.T) {
	type entry struct {
		path    string
		title   string
		mime    string
		content []byte
		front   bool
	}
	entries := []entry{
		{"index.html", "Index", "text/html", []byte("<html><body>Hello</body></html>"), true},
		{"style.css", "Style", "text/css", []byte("body { color: red; }"), false},
		{"script.js", "Script", "application/javascript", []byte("var x = 1;"), false},
		{"img.png", "Image", "image/png", []byte("\x89PNG\r\n\x1a\n"), false},
	}

	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		for _, e := range entries {
			if _, err := w.AddEntry(e.path, e.title, e.mime, e.content, e.front); err != nil {
				t.Fatal(err)
			}
		}
		// Redirect from "home.html" -> index.html (ID 0).
		if _, err := w.AddRedirect("home.html", "Home", 0); err != nil {
			t.Fatal(err)
		}
	})
	defer cleanup()

	// Verify entry count (content only) and redirect count.
	if got := a.EntryCount(); got != uint32(len(entries)) {
		t.Errorf("EntryCount = %d, want %d (content only)", got, len(entries))
	}
	if got := a.RedirectCount(); got != 1 {
		t.Errorf("RedirectCount = %d, want 1", got)
	}

	// Verify metadata round-trip.
	if title, err := a.Metadata("title"); err != nil || title != "Test Archive" {
		t.Errorf("Metadata(title) = %q, %v", title, err)
	}

	// Verify MIME types.
	mimes := a.MIMETypes()
	if len(mimes) < 3 {
		t.Fatalf("MIMETypes len = %d, want >= 3", len(mimes))
	}
	if mimes[0] != "text/html" {
		t.Errorf("MIME[0] = %q, want text/html", mimes[0])
	}
	if mimes[1] != "text/css" {
		t.Errorf("MIME[1] = %q, want text/css", mimes[1])
	}
	if mimes[2] != "application/javascript" {
		t.Errorf("MIME[2] = %q, want application/javascript", mimes[2])
	}

	// Verify each content entry by path.
	for i, e := range entries {
		got, err := a.EntryByPath(e.path)
		if err != nil {
			t.Errorf("EntryByPath(%q): %v", e.path, err)
			continue
		}
		if got.Path() != e.path {
			t.Errorf("entry %d: Path = %q, want %q", i, got.Path(), e.path)
		}
		if got.Title() != e.title {
			t.Errorf("entry %d: Title = %q, want %q", i, got.Title(), e.title)
		}
		if got.MIMEType() != e.mime {
			t.Errorf("entry %d: MIMEType = %q, want %q", i, got.MIMEType(), e.mime)
		}
		if got.IsFrontArticle() != e.front {
			t.Errorf("entry %d: IsFrontArticle = %v, want %v", i, got.IsFrontArticle(), e.front)
		}
		content, err := got.ReadContent()
		if err != nil {
			t.Errorf("entry %d: ReadContent: %v", i, err)
			continue
		}
		if !bytes.Equal(content, e.content) {
			t.Errorf("entry %d: content mismatch: got %d bytes, want %d", i, len(content), len(e.content))
		}
	}

	// Verify redirect resolves to index.html content.
	redir, err := a.EntryByPath("home.html")
	if err != nil {
		t.Fatalf("EntryByPath(home.html): %v", err)
	}
	if !redir.IsRedirect() {
		t.Error("home.html: expected redirect")
	}
	content, err := redir.ReadContent()
	if err != nil {
		t.Fatalf("redir.ReadContent: %v", err)
	}
	if !bytes.Equal(content, entries[0].content) {
		t.Error("redirect: content does not match target")
	}

	// Verify EntryByID matches EntryByPath.
	for i := range entries {
		byID, err := a.EntryByID(uint32(i))
		if err != nil {
			t.Errorf("EntryByID(%d): %v", i, err)
			continue
		}
		byPath, err := a.EntryByPath(entries[i].path)
		if err != nil {
			continue
		}
		if byID.Path() != byPath.Path() {
			t.Errorf("entry %d: EntryByID path %q != EntryByPath path %q", i, byID.Path(), byPath.Path())
		}
	}

	// Verify EntryByTitle.
	for _, e := range entries {
		got, err := a.EntryByTitle(e.title)
		if err != nil {
			t.Errorf("EntryByTitle(%q): %v", e.title, err)
			continue
		}
		if got.Title() != e.title {
			t.Errorf("EntryByTitle: got title %q, want %q", got.Title(), e.title)
		}
	}

	// Verify file checksum.
	if err := a.Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// --- TestRoundTripDedup ---

func TestRoundTripDedup(t *testing.T) {
	content := []byte("<html>identical content for deduplication test</html>")

	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		w.AddEntry("a.html", "A", "text/html", content, false)
		w.AddEntry("b.html", "B", "text/html", content, false)
		w.AddEntry("c.html", "C", "text/html", content, false)
	})
	defer cleanup()

	for _, path := range []string{"a.html", "b.html", "c.html"} {
		e, err := a.EntryByPath(path)
		if err != nil {
			t.Errorf("EntryByPath(%q): %v", path, err)
			continue
		}
		got, err := e.ReadContent()
		if err != nil {
			t.Errorf("%s: ReadContent: %v", path, err)
			continue
		}
		if !bytes.Equal(got, content) {
			t.Errorf("%s: content mismatch", path)
		}
	}
}

// --- TestRoundTripLargeChunks ---

func TestRoundTripLargeChunks(t *testing.T) {
	// Use a tiny chunk size to force multiple chunks.
	f, err := os.CreateTemp(t.TempDir(), "test*.oza")
	if err != nil {
		t.Fatal(err)
	}
	opts := ozawrite.WriterOptions{
		ZstdLevel:       3,
		ChunkTargetSize: 50, // very small: each entry gets its own chunk
		TrainDict:       false,
		BuildSearch:     false,
	}
	w := ozawrite.NewWriter(f, opts)
	setRequiredMeta(w)

	var contents [][]byte
	var paths []string
	for i := 0; i < 8; i++ {
		path := "page" + string(rune('0'+i)) + ".html"
		content := bytes.Repeat([]byte("x"), 60+i) // each is > ChunkTargetSize
		paths = append(paths, path)
		contents = append(contents, content)
		w.AddEntry(path, "Page", "text/html", content, false)
	}
	if err := w.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	for i, path := range paths {
		e, err := a.EntryByPath(path)
		if err != nil {
			t.Errorf("EntryByPath(%q): %v", path, err)
			continue
		}
		got, err := e.ReadContent()
		if err != nil {
			t.Errorf("%s: ReadContent: %v", path, err)
			continue
		}
		if !bytes.Equal(got, contents[i]) {
			t.Errorf("%s: content mismatch (got %d bytes, want %d)", path, len(got), len(contents[i]))
		}
	}
}

// --- TestVerify ---

func TestVerify(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test*.oza")
	if err != nil {
		t.Fatal(err)
	}
	opts := ozawrite.WriterOptions{ZstdLevel: 3, TrainDict: false}
	w := ozawrite.NewWriter(f, opts)
	setRequiredMeta(w)
	w.AddEntry("index.html", "Index", "text/html", []byte("<html>test</html>"), false)
	if err := w.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}

	// Read all bytes.
	f.Seek(0, 0)
	data, err := os.ReadFile(f.Name())
	f.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Write clean copy and verify it passes.
	cleanPath := f.Name() + ".clean"
	if err := os.WriteFile(cleanPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(cleanPath)

	a, err := oza.OpenWithOptions(cleanPath, oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Verify(); err != nil {
		t.Errorf("Verify on clean file: %v", err)
	}
	a.Close()

	// Corrupt one byte in the content area and verify detection.
	corrupted := make([]byte, len(data))
	copy(corrupted, data)
	corrupted[len(corrupted)/2] ^= 0xFF

	corruptPath := f.Name() + ".corrupt"
	if err := os.WriteFile(corruptPath, corrupted, 0600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(corruptPath)

	// Opening a corrupted file may succeed (header is intact) but Verify must fail.
	ac, err := oza.OpenWithOptions(corruptPath, oza.WithMmap(false))
	if err == nil {
		if verr := ac.Verify(); verr == nil {
			t.Error("Verify on corrupted file: expected error, got nil")
		}
		ac.Close()
	}
	// If Open itself fails due to corruption, that's also acceptable.
}

// --- TestCacheEviction ---

func TestCacheEviction(t *testing.T) {
	// Force many chunks and a tiny cache so eviction is exercised.
	f, err := os.CreateTemp(t.TempDir(), "test*.oza")
	if err != nil {
		t.Fatal(err)
	}
	opts := ozawrite.WriterOptions{
		ZstdLevel:       3,
		ChunkTargetSize: 10, // each entry in its own chunk
		TrainDict:       false,
		BuildSearch:     false,
	}
	w := ozawrite.NewWriter(f, opts)
	setRequiredMeta(w)

	var contents [][]byte
	var paths []string
	for i := 0; i < 10; i++ {
		path := "p" + string(rune('a'+i)) + ".html"
		content := []byte("<html>entry " + string(rune('a'+i)) + "</html>")
		paths = append(paths, path)
		contents = append(contents, content)
		w.AddEntry(path, "P", "text/html", content, false)
	}
	if err := w.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	// Open with cache size of 2 (much smaller than 10 chunks).
	a, err := oza.OpenWithOptions(f.Name(),
		oza.WithMmap(false),
		oza.WithCacheSize(2),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	// Read all entries multiple times to exercise eviction.
	for round := 0; round < 3; round++ {
		for i, path := range paths {
			e, err := a.EntryByPath(path)
			if err != nil {
				t.Errorf("round %d: EntryByPath(%q): %v", round, path, err)
				continue
			}
			got, err := e.ReadContent()
			if err != nil {
				t.Errorf("round %d: %s: ReadContent: %v", round, path, err)
				continue
			}
			if !bytes.Equal(got, contents[i]) {
				t.Errorf("round %d: %s: content mismatch", round, path)
			}
		}
	}
}

// --- TestIterators ---

func TestIterators(t *testing.T) {
	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		w.AddEntry("z.html", "Z", "text/html", []byte("<html>z</html>"), false)
		w.AddEntry("a.html", "A", "text/html", []byte("<html>a</html>"), true)
		w.AddEntry("m.html", "M", "text/html", []byte("<html>m</html>"), false)
	})
	defer cleanup()

	// Entries() should yield all 3 in ID order.
	count := 0
	for range a.Entries() {
		count++
	}
	if count != 3 {
		t.Errorf("Entries count = %d, want 3", count)
	}

	// EntriesByPath() should be sorted.
	var paths []string
	for e := range a.EntriesByPath() {
		paths = append(paths, e.Path())
	}
	for i := 1; i < len(paths); i++ {
		if paths[i] < paths[i-1] {
			t.Errorf("EntriesByPath not sorted at index %d: %q < %q", i, paths[i], paths[i-1])
		}
	}

	// FrontArticles() should yield only the one with the flag.
	frontCount := 0
	for e := range a.FrontArticles() {
		if !e.IsFrontArticle() {
			t.Errorf("FrontArticles yielded non-front entry %q", e.Path())
		}
		frontCount++
	}
	if frontCount != 1 {
		t.Errorf("FrontArticles count = %d, want 1", frontCount)
	}
}

func TestIteratorsErr(t *testing.T) {
	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		w.AddEntry("z.html", "Z", "text/html", []byte("<html>z</html>"), false)
		w.AddEntry("a.html", "A", "text/html", []byte("<html>a</html>"), true)
		w.AddEntry("m.html", "M", "text/html", []byte("<html>m</html>"), false)
	})
	defer cleanup()

	// EntriesErr should yield all 3 with no errors.
	count := 0
	for e, err := range a.EntriesErr() {
		if err != nil {
			t.Fatalf("EntriesErr: unexpected error at entry %d: %v", count, err)
		}
		if e.Path() == "" {
			t.Errorf("EntriesErr: entry %d has empty path", count)
		}
		count++
	}
	if count != 3 {
		t.Errorf("EntriesErr count = %d, want 3", count)
	}

	// EntriesByPathErr should be sorted.
	var paths []string
	for e, err := range a.EntriesByPathErr() {
		if err != nil {
			t.Fatalf("EntriesByPathErr: %v", err)
		}
		paths = append(paths, e.Path())
	}
	for i := 1; i < len(paths); i++ {
		if paths[i] < paths[i-1] {
			t.Errorf("EntriesByPathErr not sorted at index %d: %q < %q", i, paths[i], paths[i-1])
		}
	}

	// EntriesByTitleErr should be sorted.
	var titles []string
	for e, err := range a.EntriesByTitleErr() {
		if err != nil {
			t.Fatalf("EntriesByTitleErr: %v", err)
		}
		titles = append(titles, e.Title())
	}
	for i := 1; i < len(titles); i++ {
		if titles[i] < titles[i-1] {
			t.Errorf("EntriesByTitleErr not sorted at index %d: %q < %q", i, titles[i], titles[i-1])
		}
	}

	// FrontArticlesErr should yield only the one with the flag.
	frontCount := 0
	for e, err := range a.FrontArticlesErr() {
		if err != nil {
			t.Fatalf("FrontArticlesErr: %v", err)
		}
		if !e.IsFrontArticle() {
			t.Errorf("FrontArticlesErr yielded non-front entry %q", e.Path())
		}
		frontCount++
	}
	if frontCount != 1 {
		t.Errorf("FrontArticlesErr count = %d, want 1", frontCount)
	}
}

// --- TestEntriesByMIME ---

func TestEntriesByMIME(t *testing.T) {
	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		w.AddEntry("index.html", "Index", "text/html", []byte("<html>index</html>"), true)
		w.AddEntry("about.html", "About", "text/html", []byte("<html>about</html>"), true)
		w.AddEntry("style.css", "Style", "text/css", []byte("body{}"), false)
		w.AddEntry("app.js", "App", "application/javascript", []byte("var x=1;"), false)
		w.AddEntry("data.json", "Data", "application/json", []byte(`{"a":1}`), false)
	})
	defer cleanup()

	// EntryCountByMIME returns correct counts.
	if got := a.EntryCountByMIME("text/html"); got != 2 {
		t.Errorf("EntryCountByMIME(text/html) = %d, want 2", got)
	}
	if got := a.EntryCountByMIME("text/css"); got != 1 {
		t.Errorf("EntryCountByMIME(text/css) = %d, want 1", got)
	}
	if got := a.EntryCountByMIME("application/javascript"); got != 1 {
		t.Errorf("EntryCountByMIME(application/javascript) = %d, want 1", got)
	}
	if got := a.EntryCountByMIME("application/json"); got != 1 {
		t.Errorf("EntryCountByMIME(application/json) = %d, want 1", got)
	}

	// Unknown MIME type returns 0.
	if got := a.EntryCountByMIME("image/png"); got != 0 {
		t.Errorf("EntryCountByMIME(image/png) = %d, want 0", got)
	}

	// MIMEEntryCount by raw index.
	if got := a.MIMEEntryCount(oza.MIMEIndexHTML); got != 2 {
		t.Errorf("MIMEEntryCount(HTML) = %d, want 2", got)
	}
	if got := a.MIMEEntryCount(oza.MIMEIndexCSS); got != 1 {
		t.Errorf("MIMEEntryCount(CSS) = %d, want 1", got)
	}

	// EntriesByMIME yields correct entries in ID order.
	var htmlPaths []string
	for e := range a.EntriesByMIME("text/html") {
		htmlPaths = append(htmlPaths, e.Path())
	}
	if len(htmlPaths) != 2 || htmlPaths[0] != "index.html" || htmlPaths[1] != "about.html" {
		t.Errorf("EntriesByMIME(text/html) paths = %v, want [index.html about.html]", htmlPaths)
	}

	// EntriesByMIME for unknown type yields nothing.
	count := 0
	for range a.EntriesByMIME("image/png") {
		count++
	}
	if count != 0 {
		t.Errorf("EntriesByMIME(image/png) count = %d, want 0", count)
	}
}

func TestEntriesByMIMEErr(t *testing.T) {
	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		w.AddEntry("index.html", "Index", "text/html", []byte("<html>hi</html>"), true)
		w.AddEntry("style.css", "Style", "text/css", []byte("body{}"), false)
	})
	defer cleanup()

	count := 0
	for e, err := range a.EntriesByMIMEErr("text/html") {
		if err != nil {
			t.Fatalf("EntriesByMIMEErr: unexpected error: %v", err)
		}
		if e.MIMEType() != "text/html" {
			t.Errorf("EntriesByMIMEErr yielded entry with MIME %q, want text/html", e.MIMEType())
		}
		count++
	}
	if count != 1 {
		t.Errorf("EntriesByMIMEErr(text/html) count = %d, want 1", count)
	}

	// Unknown type yields nothing, no error.
	for _, err := range a.EntriesByMIMEErr("image/webp") {
		if err != nil {
			t.Fatalf("EntriesByMIMEErr(unknown): unexpected error: %v", err)
		}
		t.Fatal("EntriesByMIMEErr(unknown): should not yield any entries")
	}
}

// --- TestSearch ---

func TestSearch(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test*.oza")
	if err != nil {
		t.Fatal(err)
	}
	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: true,
	}
	w := ozawrite.NewWriter(f, opts)
	setRequiredMeta(w)

	// Front articles are indexed; non-front entries are not.
	w.AddEntry("quantum.html", "Quantum Mechanics", "text/html",
		[]byte("<html>quantum mechanics is the branch of physics</html>"), true)
	w.AddEntry("relativity.html", "Theory of Relativity", "text/html",
		[]byte("<html>general relativity describes gravity</html>"), true)
	w.AddEntry("style.css", "Style", "text/css",
		[]byte("body{color:red}"), false)

	if err := w.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if !a.HasSearch() {
		t.Fatal("archive should have search index")
	}
	if !a.HasTitleSearch() {
		t.Fatal("archive should have title search index")
	}
	if !a.HasBodySearch() {
		t.Fatal("archive should have body search index")
	}

	// "quantum" should match only the quantum entry.
	results, err := a.Search("quantum", oza.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search(quantum): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search(quantum): got %d results, want 1", len(results))
	}
	if results[0].Entry.Path() != "quantum.html" {
		t.Errorf("Search(quantum): got path %q, want quantum.html", results[0].Entry.Path())
	}

	// "gravity" should match only the relativity entry (body match only).
	results, err = a.Search("gravity", oza.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search(gravity): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search(gravity): got %d results, want 1", len(results))
	}
	if results[0].Entry.Path() != "relativity.html" {
		t.Errorf("Search(gravity): got path %q, want relativity.html", results[0].Entry.Path())
	}
	if results[0].TitleMatch {
		t.Error("Search(gravity): should not be a title match")
	}
	if !results[0].BodyMatch {
		t.Error("Search(gravity): should be a body match")
	}

	// Title-only search for "Quantum" should match.
	results, err = a.SearchTitles("Quantum", 10)
	if err != nil {
		t.Fatalf("SearchTitles(Quantum): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchTitles(Quantum): got %d results, want 1", len(results))
	}
	if !results[0].TitleMatch {
		t.Error("SearchTitles(Quantum): should be a title match")
	}

	// Title-only search for "gravity" (body content) should return no results.
	results, err = a.SearchTitles("gravity", 10)
	if err != nil {
		t.Fatalf("SearchTitles(gravity): %v", err)
	}
	if len(results) != 0 {
		t.Errorf("SearchTitles(gravity): got %d results, want 0", len(results))
	}

	// Query shorter than 3 bytes returns no results.
	results, err = a.Search("ab", oza.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search(ab): %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search(ab): got %d results, want 0", len(results))
	}

	// Title matches should rank before body-only matches.
	// "Quantum" appears in title of quantum.html and body of quantum.html.
	// "relativity" appears in title of relativity.html.
	results, err = a.Search("relativity", oza.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search(relativity): %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search(relativity): got %d results, want 1", len(results))
	}
	// "relativity" is in the title, so it should be both a title and body match.
	if !results[0].TitleMatch {
		t.Error("Search(relativity): expected title match")
	}

	// Archive with no search index returns an error.
	noSearchOpts := ozawrite.WriterOptions{ZstdLevel: 3, TrainDict: false, BuildSearch: false}
	f2, _ := os.CreateTemp(t.TempDir(), "test*.oza")
	w2 := ozawrite.NewWriter(f2, noSearchOpts)
	setRequiredMeta(w2)
	w2.AddEntry("x.html", "X", "text/html", []byte("<html>x</html>"), false)
	w2.Close()
	f2.Close()
	a2, _ := oza.OpenWithOptions(f2.Name(), oza.WithMmap(false))
	defer a2.Close()
	if a2.HasSearch() {
		t.Error("archive without search should have HasSearch=false")
	}
	if _, err := a2.Search("test", oza.SearchOptions{Limit: 10}); err == nil {
		t.Error("Search on archive without index should return error")
	}
}

// --- TestConcurrentReads ---

// TestConcurrentReads verifies the Archive concurrency contract: all read
// methods are safe to call from multiple goroutines simultaneously after Open
// returns. Run with -race to catch data races.
func TestConcurrentReads(t *testing.T) {
	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		for i := range 8 {
			path := "page" + string(rune('a'+i)) + ".html"
			title := "Page " + string(rune('A'+i))
			w.AddEntry(path, title, "text/html", []byte("<html>"+path+"</html>"), i%2 == 0)
		}
	})
	defer cleanup()

	paths := make([]string, 8)
	titles := make([]string, 8)
	for i := range 8 {
		paths[i] = "page" + string(rune('a'+i)) + ".html"
		titles[i] = "Page " + string(rune('A'+i))
	}

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(g int) {
			defer wg.Done()
			i := g % len(paths)

			if _, err := a.EntryByPath(paths[i]); err != nil {
				t.Errorf("g%d: EntryByPath: %v", g, err)
			}
			if _, err := a.EntryByTitle(titles[i]); err != nil {
				t.Errorf("g%d: EntryByTitle: %v", g, err)
			}
			if _, err := a.EntryByID(uint32(i)); err != nil {
				t.Errorf("g%d: EntryByID: %v", g, err)
			}
			for e := range a.Entries() {
				if _, err := e.ReadContent(); err != nil {
					t.Errorf("g%d: ReadContent: %v", g, err)
				}
				break // one is enough to exercise the path
			}
			_ = a.EntryCount()
			_ = a.MIMETypes()
			if _, err := a.Metadata("title"); err != nil {
				t.Errorf("g%d: Metadata: %v", g, err)
			}
		}(g)
	}
	wg.Wait()
}

// --- TestReservedFieldWarnings ---

// TestReservedFieldWarnings verifies that non-zero reserved bytes in the header
// or section descriptors produce advisory warnings via Archive.Warnings(), but
// do not prevent the archive from opening.
func TestReservedFieldWarnings(t *testing.T) {
	// Build a clean archive and verify it produces no warnings.
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "clean*.oza")
	if err != nil {
		t.Fatal(err)
	}
	clean := ozawrite.WriterOptions{ZstdLevel: 3, BuildSearch: false}
	w := ozawrite.NewWriter(f, clean)
	setRequiredMeta(w)
	w.AddEntry("index.html", "Index", "text/html", []byte("<html>ok</html>"), false)
	if err := w.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	raw, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}

	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	if got := a.Warnings(); len(got) != 0 {
		t.Errorf("clean archive: got %d warnings, want 0: %v", len(got), got)
	}
	a.Close()

	// Write a copy with a non-zero header reserved field (bytes [68:128]).
	modified := make([]byte, len(raw))
	copy(modified, raw)
	modified[68] = 0xFF

	f2, err := os.CreateTemp(dir, "hdrres*.oza")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f2.Write(modified); err != nil {
		f2.Close()
		t.Fatal(err)
	}
	f2.Close()

	a2, err := oza.OpenWithOptions(f2.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatalf("open with non-zero header reserved field: %v", err)
	}
	defer a2.Close()
	warns := a2.Warnings()
	if len(warns) == 0 {
		t.Error("archive with non-zero header reserved field: got no warnings, want at least one")
	}

	// Write a copy with a non-zero section descriptor reserved block ([33:36]).
	// SectionSize = 80; the section table starts at hdr.SectionTableOff.
	// Read that offset from the raw header (bytes [40:48] of the header = SectionTableOff).
	sectionTableOff := int(raw[40]) | int(raw[41])<<8 | int(raw[42])<<16 | int(raw[43])<<24 |
		int(raw[44])<<32 | int(raw[45])<<40 | int(raw[46])<<48 | int(raw[47])<<56
	modified2 := make([]byte, len(raw))
	copy(modified2, raw)
	if sectionTableOff+33 < len(modified2) {
		modified2[sectionTableOff+33] = 0x01 // reserved byte [33] of first section descriptor
	}

	f3, err := os.CreateTemp(dir, "secres*.oza")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f3.Write(modified2); err != nil {
		f3.Close()
		t.Fatal(err)
	}
	f3.Close()

	a3, err := oza.OpenWithOptions(f3.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatalf("open with non-zero section reserved field: %v", err)
	}
	defer a3.Close()
	warns3 := a3.Warnings()
	if len(warns3) == 0 {
		t.Error("archive with non-zero section reserved bytes: got no warnings, want at least one")
	}
}

// --- TestVerifyAll ---

func TestVerifyAll(t *testing.T) {
	a, cleanup := newTestArchive(t, func(w *ozawrite.Writer) {
		w.AddEntry("index.html", "Index", "text/html", []byte("<html>hello</html>"), false)
	})
	defer cleanup()

	results, err := a.VerifyAll()
	if err != nil {
		t.Fatalf("VerifyAll: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("VerifyAll: no results")
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("VerifyAll: tier=%s id=%s FAILED", r.Tier, r.ID)
		}
	}
}

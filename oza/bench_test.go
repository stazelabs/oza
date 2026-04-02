package oza_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

// Fixture paths populated by TestMain.
var (
	standardFixture string // 100 entries, no search
	searchFixture   string // 100 entries, search enabled
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "oza-bench-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "bench setup: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	standardFixture = filepath.Join(dir, "standard.oza")
	if err := buildFixture(standardFixture, 100, false); err != nil {
		fmt.Fprintf(os.Stderr, "bench setup standard: %v\n", err)
		os.Exit(1)
	}
	searchFixture = filepath.Join(dir, "search.oza")
	if err := buildFixture(searchFixture, 100, true); err != nil {
		fmt.Fprintf(os.Stderr, "bench setup search: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// --- helpers ---

func benchGenerateHTML(i int) []byte {
	return []byte(fmt.Sprintf(`<html><head><title>Article %d</title></head><body>
<h1>Article %d</h1>
<p>This is article number %d discussing quantum mechanics, relativity theory,
and computational complexity. The entry covers topics in science, history, and
technology. Each article is unique with identifier %d and explores different
aspects of knowledge including mathematics, philosophy, and engineering.</p>
<p>Additional content for article %d to ensure realistic entry sizes for
benchmarking purposes. Topics include algorithms, data structures, and
distributed systems architecture.</p>
</body></html>`, i, i, i, i, i))
}

func benchGeneratePath(i int) string {
	return fmt.Sprintf("articles/entry_%04d.html", i)
}

func benchGenerateTitle(i int) string {
	return fmt.Sprintf("Article %d", i)
}

func benchSetRequiredMeta(w *ozawrite.Writer) {
	w.SetMetadata("title", "Bench Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "bench")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")
}

func buildFixture(path string, n int, search bool) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: search,
	}
	w := ozawrite.NewWriter(f, opts)
	benchSetRequiredMeta(w)

	for i := 0; i < n; i++ {
		if _, err := w.AddEntry(benchGeneratePath(i), benchGenerateTitle(i), "text/html", benchGenerateHTML(i), true); err != nil {
			return err
		}
	}
	return w.Close()
}

func openFixture(b *testing.B, path string, opts ...oza.Option) *oza.Archive {
	b.Helper()
	a, err := oza.OpenWithOptions(path, opts...)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { a.Close() })
	return a
}

// --- benchmarks ---

func BenchmarkOpen(b *testing.B) {
	for b.Loop() {
		a, err := oza.OpenWithOptions(standardFixture, oza.WithMmap(false))
		if err != nil {
			b.Fatal(err)
		}
		a.Close()
	}
}

func BenchmarkEntryByPath(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	b.ResetTimer()
	for b.Loop() {
		if _, err := a.EntryByPath("articles/entry_0050.html"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEntryByID(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	b.ResetTimer()
	for b.Loop() {
		if _, err := a.EntryByID(50); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadContent(b *testing.B) {
	b.Run("Cached", func(b *testing.B) {
		a := openFixture(b, standardFixture, oza.WithMmap(false))
		e, err := a.EntryByID(50)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(e.Size()))
		b.ResetTimer()
		for b.Loop() {
			if _, err := e.ReadContent(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Uncached", func(b *testing.B) {
		a := openFixture(b, standardFixture, oza.WithMmap(false), oza.WithCacheSize(1))
		e, err := a.EntryByID(50)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(e.Size()))
		b.ResetTimer()
		for b.Loop() {
			if _, err := e.ReadContent(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkVerify(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	b.ResetTimer()
	for b.Loop() {
		if err := a.Verify(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyAll(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	b.ResetTimer()
	for b.Loop() {
		results, err := a.VerifyAll()
		if err != nil {
			b.Fatal(err)
		}
		for _, r := range results {
			if !r.OK {
				b.Fatalf("verification failed: %s %s", r.Tier, r.ID)
			}
		}
	}
}

func BenchmarkSearch(b *testing.B) {
	a := openFixture(b, searchFixture, oza.WithMmap(false))
	opts := oza.SearchOptions{Limit: 10}
	b.ResetTimer()
	for b.Loop() {
		if _, err := a.Search("quantum", opts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEntriesByMIME(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	b.ResetTimer()
	for b.Loop() {
		for range a.EntriesByMIME("text/html") {
		}
	}
}

func BenchmarkEntryCountByMIME(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	b.ResetTimer()
	for b.Loop() {
		a.EntryCountByMIME("text/html")
	}
}

// --- concurrent access benchmarks ---

func BenchmarkReadContentParallel(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	entries := make([]oza.Entry, 10)
	for i := range entries {
		e, err := a.EntryByID(uint32(i))
		if err != nil {
			b.Fatal(err)
		}
		entries[i] = e
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := entries[i%len(entries)].ReadContent(); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkEntryByPathParallel(b *testing.B) {
	a := openFixture(b, standardFixture, oza.WithMmap(false))
	paths := make([]string, 20)
	for i := range paths {
		paths[i] = benchGeneratePath(i)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := a.EntryByPath(paths[i%len(paths)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkCacheThrashing(b *testing.B) {
	// Small cache (4 slots) with diverse access to force evictions.
	a := openFixture(b, standardFixture, oza.WithMmap(false), oza.WithCacheSize(4))
	entries := make([]oza.Entry, 50)
	for i := range entries {
		e, err := a.EntryByID(uint32(i))
		if err != nil {
			b.Fatal(err)
		}
		entries[i] = e
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := entries[i%len(entries)].ReadContent(); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkSearchParallel(b *testing.B) {
	a := openFixture(b, searchFixture, oza.WithMmap(false))
	queries := []string{"quantum", "relativity", "article", "science", "complexity"}
	opts := oza.SearchOptions{Limit: 10}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := a.Search(queries[i%len(queries)], opts); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

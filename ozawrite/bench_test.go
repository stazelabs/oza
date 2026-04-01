package ozawrite

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

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

func benchRequiredMeta(w *Writer) {
	w.SetMetadata("title", "Bench Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "bench")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")
}

func makeChunkData(n int) []byte {
	var data []byte
	for i := 0; len(data) < n; i++ {
		data = append(data, benchGenerateHTML(i)...)
	}
	return data[:n]
}

func benchWrite(b *testing.B, n int) {
	b.Helper()
	contents := make([][]byte, n)
	paths := make([]string, n)
	titles := make([]string, n)
	for i := 0; i < n; i++ {
		contents[i] = benchGenerateHTML(i)
		paths[i] = benchGeneratePath(i)
		titles[i] = benchGenerateTitle(i)
	}
	dir := b.TempDir()
	b.ResetTimer()
	for b.Loop() {
		f, err := os.CreateTemp(dir, "bench*.oza")
		if err != nil {
			b.Fatal(err)
		}
		opts := WriterOptions{
			ZstdLevel:   3,
			TrainDict:   false,
			BuildSearch: false,
		}
		w := NewWriter(f, opts)
		benchRequiredMeta(w)
		for i := 0; i < n; i++ {
			if _, err := w.AddEntry(paths[i], titles[i], "text/html", contents[i], true); err != nil {
				f.Close()
				b.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			f.Close()
			b.Fatal(err)
		}
		f.Close()
		os.Remove(f.Name())
	}
}

// --- benchmarks ---

func BenchmarkWriteSmall(b *testing.B) {
	benchWrite(b, 100)
}

func BenchmarkWriteMedium(b *testing.B) {
	benchWrite(b, 10_000)
}

func BenchmarkCompressChunk(b *testing.B) {
	data := makeChunkData(64 * 1024)
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := compressZstd(data, 6, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTrainDictionary(b *testing.B) {
	// Generate enough samples to exceed the 128KB minimum history size.
	samples := make([][]byte, 500)
	for i := range samples {
		samples[i] = benchGenerateHTML(i)
	}
	b.ResetTimer()
	for b.Loop() {
		if _, err := trainDictionary(1, samples, 112*1024); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildTrigramIndex(b *testing.B) {
	const n = 1000
	texts := make([][]byte, n)
	for i := range texts {
		texts[i] = []byte(fmt.Sprintf(
			"article %d about quantum mechanics and computational complexity with topic %d covering science history technology algorithms data structures", i, i))
	}
	b.ResetTimer()
	for b.Loop() {
		tb := newTrigramBuilder()
		for i, t := range texts {
			tb.IndexEntry(uint32(i), t)
		}
		if _, err := tb.Build(0); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildTrigramIndexLarge(b *testing.B) {
	const n = 5000
	texts := make([][]byte, n)
	for i := range texts {
		texts[i] = benchGenerateHTML(i)
	}
	b.ResetTimer()
	for b.Loop() {
		tb := newTrigramBuilder()
		for i, t := range texts {
			tb.IndexEntry(uint32(i), t)
		}
		if _, err := tb.Build(0); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWriteWithDict benchmarks archive creation with dictionary training enabled.
func BenchmarkWriteWithDict(b *testing.B) {
	const n = 500
	contents := make([][]byte, n)
	paths := make([]string, n)
	titles := make([]string, n)
	for i := 0; i < n; i++ {
		contents[i] = benchGenerateHTML(i)
		paths[i] = benchGeneratePath(i)
		titles[i] = benchGenerateTitle(i)
	}
	dir := b.TempDir()
	b.ResetTimer()
	for b.Loop() {
		f, err := os.Create(filepath.Join(dir, "dict.oza"))
		if err != nil {
			b.Fatal(err)
		}
		opts := WriterOptions{
			ZstdLevel: 3,
			TrainDict: true,
		}
		w := NewWriter(f, opts)
		benchRequiredMeta(w)
		for i := 0; i < n; i++ {
			if _, err := w.AddEntry(paths[i], titles[i], "text/html", contents[i], true); err != nil {
				f.Close()
				b.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			f.Close()
			b.Fatal(err)
		}
		f.Close()
	}
}

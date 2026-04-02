package ozawrite

import (
	"os"
	"testing"

	"github.com/stazelabs/oza/oza"
)

// FuzzTrigramBuilder fuzzes the trigram index builder with arbitrary text input.
// The fuzzed output is fed to the reader's parser to verify no panics or
// corruption on the round trip.
func FuzzTrigramBuilder(f *testing.F) {
	f.Add([]byte("hello world"))
	f.Add([]byte("日本語 hello"))
	f.Add([]byte(""))
	f.Add([]byte("aaa"))
	f.Add([]byte("a b c d e f g"))

	f.Fuzz(func(t *testing.T, text []byte) {
		tb := newTrigramBuilder()
		tb.IndexEntry(0, text)
		tb.IndexEntry(1, text)
		data, err := tb.Build(0)
		if err != nil {
			return // Build can legitimately fail on some inputs
		}
		if len(data) < 16 {
			return
		}
		// Feed to the reader's parser to verify it doesn't panic.
		_, _ = oza.ParseTrigramIndex(data)
	})
}

// FuzzStringTableSerialize fuzzes the string table serializer by building a
// table from arbitrary token input and verifying the serialized output can be
// manually decoded without panics.
func FuzzStringTableSerialize(f *testing.F) {
	f.Add("hello/world/index.html")
	f.Add("A/ B/ C")
	f.Add("")
	f.Add("a")
	f.Add("alpha/beta/gamma/delta/epsilon.html")

	f.Fuzz(func(t *testing.T, input string) {
		tokens := tokenizePath(input)
		if len(tokens) == 0 {
			return
		}

		stb := newStringTableBuilder()
		// Add tokens multiple times to ensure they exceed minFreq.
		for i := 0; i < 5; i++ {
			stb.AddTokens(tokens)
		}
		table := stb.Build(1)
		data := table.Serialize()

		// Verify we can read it back without panics.
		off := 0
		for i := 0; i < table.Count(); i++ {
			if off+2 > len(data) {
				t.Fatalf("entry %d: data truncated at offset %d (len=%d)", i, off, len(data))
			}
			slen := int(data[off]) | int(data[off+1])<<8
			off += 2
			if off+slen > len(data) {
				t.Fatalf("entry %d: string truncated", i)
			}
			got := string(data[off : off+slen])
			if got != table.Entry(i) {
				t.Errorf("entry %d: got %q, want %q", i, got, table.Entry(i))
			}
			off += slen
		}
	})
}

// FuzzWriterRoundTrip fuzzes the writer by adding entries with arbitrary
// content and verifying the output can be opened and read by the reader.
func FuzzWriterRoundTrip(f *testing.F) {
	f.Add("index.html", "Index", "text/html", []byte("<html><body>Hello</body></html>"))
	f.Add("style.css", "Style", "text/css", []byte("body{margin:0}"))
	f.Add("data.json", "Data", "application/json", []byte(`{"key":"value"}`))
	f.Add("empty.txt", "Empty", "text/plain", []byte("content"))

	f.Fuzz(func(t *testing.T, path, title, mime string, content []byte) {
		if path == "" || title == "" || mime == "" || len(content) == 0 {
			return
		}
		// Filter out paths with null bytes which are invalid.
		for _, c := range path {
			if c == 0 {
				return
			}
		}

		tmpFile, err := os.CreateTemp(t.TempDir(), "fuzz*.oza")
		if err != nil {
			t.Fatal(err)
		}
		defer tmpFile.Close()

		w := NewWriter(tmpFile, WriterOptions{
			ZstdLevel:   3,
			TrainDict:   false,
			BuildSearch: false,
		})
		w.SetMetadata("title", "Fuzz Test")
		w.SetMetadata("language", "en")
		w.SetMetadata("creator", "fuzz")
		w.SetMetadata("date", "2026-01-01")
		w.SetMetadata("source", "https://example.com")

		if _, err := w.AddEntry(path, title, mime, content, false); err != nil {
			return // Invalid input is OK
		}
		if err := w.Close(); err != nil {
			return // Close can fail for invalid input combinations
		}
		tmpFile.Close()

		// Verify the archive can be opened and the entry read back.
		a, err := oza.OpenWithOptions(tmpFile.Name(), oza.WithMmap(false))
		if err != nil {
			t.Fatalf("failed to open written archive: %v", err)
		}
		defer a.Close()

		if a.EntryCount() == 0 {
			t.Fatal("archive has zero entries")
		}

		e, err := a.EntryByID(0)
		if err != nil {
			t.Fatalf("EntryByID(0): %v", err)
		}
		got, err := e.ReadContent()
		if err != nil {
			t.Fatalf("ReadContent: %v", err)
		}

		// Content may have been minified, so we only check non-empty content
		// produces non-empty output (minification can change bytes).
		if len(content) > 0 && len(got) == 0 {
			t.Error("non-empty input produced empty output")
		}
	})
}

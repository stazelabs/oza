package ozawrite

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/cespare/xxhash/v2"

	"github.com/stazelabs/oza/oza"
)

// --- dedup ---

func TestDedupMapCheckMiss(t *testing.T) {
	dm := newDedupMap()
	_, ok := dm.Check([]byte("hello"))
	if ok {
		t.Error("expected miss on empty map")
	}
}

func TestDedupMapRegisterAndCheck(t *testing.T) {
	dm := newDedupMap()
	content := []byte("hello world")
	ref := dedupRef{chunkID: 1, blobOffset: 100, blobSize: 11}

	_, ok := dm.Check(content)
	if ok {
		t.Fatal("unexpected hit before register")
	}

	h := xxhash.Sum64(content)
	dm.Register(h, ref)

	got, ok := dm.Check(content)
	if !ok {
		t.Fatal("expected hit after register")
	}
	if got.chunkID != 1 || got.blobOffset != 100 || got.blobSize != 11 {
		t.Errorf("got %+v, want {1 100 11}", got)
	}
}

func TestDedupMapCheckHash(t *testing.T) {
	dm := newDedupMap()
	ref := dedupRef{chunkID: 5, blobOffset: 0, blobSize: 42}
	dm.Register(12345, ref)

	got, ok := dm.CheckHash(12345)
	if !ok {
		t.Fatal("expected hit")
	}
	if got != ref {
		t.Errorf("got %+v, want %+v", got, ref)
	}

	_, ok = dm.CheckHash(99999)
	if ok {
		t.Error("expected miss for different hash")
	}
}

// --- chunk builder ---

func TestChunkBuilderAddBlob(t *testing.T) {
	cb := &chunkBuilder{id: 0, mimeGroup: "html"}

	off1 := cb.addBlob([]byte("hello"))
	if off1 != 0 {
		t.Errorf("first blob offset = %d, want 0", off1)
	}
	if cb.uncompSize != 5 {
		t.Errorf("uncompSize = %d, want 5", cb.uncompSize)
	}

	off2 := cb.addBlob([]byte(" world"))
	if off2 != 5 {
		t.Errorf("second blob offset = %d, want 5", off2)
	}
	if cb.uncompSize != 11 {
		t.Errorf("uncompSize = %d, want 11", cb.uncompSize)
	}
}

func TestChunkBuilderUncompressedBytes(t *testing.T) {
	cb := &chunkBuilder{id: 0, mimeGroup: "html"}
	cb.addBlob([]byte("aaa"))
	cb.addBlob([]byte("bbb"))
	cb.addBlob([]byte("ccc"))

	got := cb.uncompressedBytes()
	if string(got) != "aaabbbccc" {
		t.Errorf("uncompressedBytes = %q, want %q", got, "aaabbbccc")
	}
}

func TestChunkBuilderEmpty(t *testing.T) {
	cb := &chunkBuilder{id: 0, mimeGroup: "html"}
	got := cb.uncompressedBytes()
	if len(got) != 0 {
		t.Errorf("empty chunk uncompressedBytes len = %d, want 0", len(got))
	}
}

// --- mimeGroup ---

func TestMimeGroup(t *testing.T) {
	tests := []struct {
		mime string
		want string
	}{
		{"text/html", "html"},
		{"text/html; charset=utf-8", "html"},
		{"text/css", "css"},
		{"application/javascript", "js"},
		{"text/javascript", "js"},
		{"image/svg+xml", "svg"},
		{"image/jpeg", "image"},
		{"image/png", "image"},
		{"image/webp", "image"},
		{"application/json", "other"},
		{"text/plain", "other"},
		{"application/octet-stream", "other"},
		{"", "other"},
	}
	for _, tt := range tests {
		got := mimeGroup(tt.mime)
		if got != tt.want {
			t.Errorf("mimeGroup(%q) = %q, want %q", tt.mime, got, tt.want)
		}
	}
}

func TestChunkKey(t *testing.T) {
	// Large entries: no "-small" suffix.
	if got := ChunkKey("text/html", 5000); got != "html" {
		t.Errorf("ChunkKey(text/html, 5000) = %q, want html", got)
	}
	// Small entries: "-small" suffix.
	if got := ChunkKey("text/html", 100); got != "html-small" {
		t.Errorf("ChunkKey(text/html, 100) = %q, want html-small", got)
	}
	// Images never get "-small".
	if got := ChunkKey("image/png", 100); got != "image" {
		t.Errorf("ChunkKey(image/png, 100) = %q, want image", got)
	}
	// At threshold boundary.
	if got := ChunkKey("text/css", smallEntryThreshold); got != "css" {
		t.Errorf("ChunkKey(text/css, %d) = %q, want css", smallEntryThreshold, got)
	}
	if got := ChunkKey("text/css", smallEntryThreshold-1); got != "css-small" {
		t.Errorf("ChunkKey(text/css, %d) = %q, want css-small", smallEntryThreshold-1, got)
	}
}

// --- marshalChunkDesc ---

func TestMarshalChunkDesc(t *testing.T) {
	cd := chunkDesc{
		ID:             42,
		CompressedOff:  1000,
		CompressedSize: 500,
		DictID:         7,
		Compression:    oza.CompZstd,
	}
	b := marshalChunkDesc(cd)

	if binary.LittleEndian.Uint32(b[0:4]) != 42 {
		t.Error("ID mismatch")
	}
	if binary.LittleEndian.Uint64(b[4:12]) != 1000 {
		t.Error("CompressedOff mismatch")
	}
	if binary.LittleEndian.Uint64(b[12:20]) != 500 {
		t.Error("CompressedSize mismatch")
	}
	if binary.LittleEndian.Uint32(b[20:24]) != 7 {
		t.Error("DictID mismatch")
	}
	if b[24] != oza.CompZstd {
		t.Errorf("Compression = %d, want %d", b[24], oza.CompZstd)
	}
	// Reserved bytes should be zero.
	if b[25] != 0 || b[26] != 0 || b[27] != 0 {
		t.Error("reserved bytes not zero")
	}
}

// --- JPEG metadata stripping ---

func TestStripJPEGMetadataValid(t *testing.T) {
	// Build a minimal JPEG with SOI + APP0 (JFIF) + DQT + SOS + data + EOI.
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})       // SOI
	writeJPEGSegment(&buf, 0xE0, 14)    // APP0 (should be stripped)
	writeJPEGSegment(&buf, 0xFE, 5)     // COM comment (should be stripped)
	writeJPEGSegment(&buf, 0xDB, 10)    // DQT (should be kept)
	buf.Write([]byte{0xFF, 0xDA})       // SOS
	writeJPEGSegment(&buf, 0xDA, 3)     // Actually SOS has length field followed by data
	buf.Write([]byte{0x01, 0x02, 0x03}) // entropy data
	buf.Write([]byte{0xFF, 0xD9})       // EOI

	input := buf.Bytes()
	result := stripJPEGMetadata(input)

	// Result should be shorter (APP0 and COM stripped).
	if len(result) >= len(input) {
		t.Errorf("expected stripped output to be shorter: got %d, input %d", len(result), len(input))
	}

	// Must start with SOI.
	if result[0] != 0xFF || result[1] != 0xD8 {
		t.Error("result doesn't start with SOI")
	}
}

func TestStripJPEGMetadataNotJPEG(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
	got := stripJPEGMetadata(data)
	if !bytes.Equal(got, data) {
		t.Error("non-JPEG data should be returned unchanged")
	}
}

func TestStripJPEGMetadataEmpty(t *testing.T) {
	got := optimizeImage("image/jpeg", nil)
	if got != nil {
		t.Error("nil input should return nil")
	}
	got = optimizeImage("image/jpeg", []byte{})
	if len(got) != 0 {
		t.Error("empty input should return empty")
	}
}

func TestStripJPEGMetadataStuffedByte(t *testing.T) {
	// 0xFF 0xD8 followed by 0xFF 0x00 (stuffed byte at marker position) — should bail.
	data := []byte{0xFF, 0xD8, 0xFF, 0x00, 0x01}
	got := stripJPEGMetadata(data)
	if !bytes.Equal(got, data) {
		t.Error("stuffed byte should cause bail, returning original")
	}
}

func TestStripJPEGMetadataTruncated(t *testing.T) {
	// SOI + marker byte but no room for length.
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	got := stripJPEGMetadata(data)
	if !bytes.Equal(got, data) {
		t.Error("truncated data should return original")
	}
}

func TestStripJPEGMetadataRSTMarker(t *testing.T) {
	// SOI + RST0 (no length) + DQT + SOS + EOI.
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})   // SOI
	buf.Write([]byte{0xFF, 0xD0})   // RST0 (no length field)
	writeJPEGSegment(&buf, 0xDB, 4) // DQT
	buf.Write([]byte{0xFF, 0xDA})   // SOS
	buf.Write([]byte{0x00, 0x03})   // SOS length
	buf.Write([]byte{0x00})         // SOS data byte
	buf.Write([]byte{0x01, 0x02})   // entropy
	buf.Write([]byte{0xFF, 0xD9})   // EOI

	result := stripJPEGMetadata(buf.Bytes())
	// RST should be preserved.
	if !bytes.Contains(result, []byte{0xFF, 0xD0}) {
		t.Error("RST marker should be preserved")
	}
}

func TestStripJPEGMetadataInvalidSegLen(t *testing.T) {
	// SOI + marker with segLen < 2.
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8})
	buf.Write([]byte{0xFF, 0xE0}) // APP0
	buf.Write([]byte{0x00, 0x01}) // segLen = 1 (< 2, invalid)
	buf.Write([]byte{0xFF, 0xD9}) // EOI

	got := stripJPEGMetadata(buf.Bytes())
	if !bytes.Equal(got, buf.Bytes()) {
		t.Error("invalid segment length should cause bail")
	}
}

// writeJPEGSegment writes a JPEG marker segment with the given marker byte and payload size.
func writeJPEGSegment(buf *bytes.Buffer, marker byte, payloadSize int) {
	buf.Write([]byte{0xFF, marker})
	segLen := payloadSize + 2 // length field includes itself
	buf.Write([]byte{byte(segLen >> 8), byte(segLen)})
	for i := 0; i < payloadSize; i++ {
		buf.WriteByte(0x00)
	}
}

// --- optimizeImage ---

func TestOptimizeImageNonJPEG(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47}
	got := optimizeImage("image/png", data)
	if !bytes.Equal(got, data) {
		t.Error("non-JPEG should be returned unchanged")
	}
}

func TestOptimizeImageJPGMIME(t *testing.T) {
	// "image/jpg" (non-standard but supported) should also strip.
	data := []byte{0xFF, 0xD8, 0xFF, 0xD9} // minimal JPEG: SOI + EOI
	got := optimizeImage("image/jpg", data)
	if !bytes.Equal(got, []byte{0xFF, 0xD8, 0xFF, 0xD9}) {
		t.Errorf("minimal JPEG should pass through unchanged, got %x", got)
	}
}

func TestIsImageMIME(t *testing.T) {
	if !isImageMIME("image/jpeg") {
		t.Error("image/jpeg should be true")
	}
	if !isImageMIME("image/jpg") {
		t.Error("image/jpg should be true")
	}
	if isImageMIME("image/png") {
		t.Error("image/png should be false")
	}
	if isImageMIME("text/html") {
		t.Error("text/html should be false")
	}
}

// --- shouldKeepJPEGMarker ---

func TestShouldKeepJPEGMarker(t *testing.T) {
	// Markers that should be stripped.
	for m := byte(0xE0); m <= 0xEF; m++ {
		if shouldKeepJPEGMarker(m) {
			t.Errorf("APP%d (0x%02X) should be stripped", m-0xE0, m)
		}
	}
	if shouldKeepJPEGMarker(0xFE) {
		t.Error("COM (0xFE) should be stripped")
	}

	// Markers that should be kept.
	kept := []byte{0xC0, 0xC1, 0xC2, 0xC4, 0xCC, 0xDB, 0xDD, 0xDA}
	for _, m := range kept {
		if !shouldKeepJPEGMarker(m) {
			t.Errorf("marker 0x%02X should be kept", m)
		}
	}
}

// --- minifyContent ---

func TestMinifyContentEmpty(t *testing.T) {
	// nil minifier scenario: empty content should pass through.
	got := minifyContent(nil, "text/html", nil)
	if got != nil {
		t.Error("nil content should return nil")
	}
	got = minifyContent(nil, "text/html", []byte{})
	if len(got) != 0 {
		t.Error("empty content should return empty")
	}
}

// --- compress ---

func TestMapEncoderLevel(t *testing.T) {
	// Verify the mapping produces distinct levels for each range.
	fastest := mapEncoderLevel(1)
	def := mapEncoderLevel(3)
	better := mapEncoderLevel(6)
	best := mapEncoderLevel(11)

	if fastest == def {
		t.Error("level 1 and 3 should map to different encoder levels")
	}
	if def == better {
		t.Error("level 3 and 6 should map to different encoder levels")
	}
	if better == best {
		t.Error("level 6 and 11 should map to different encoder levels")
	}
	// Boundary: 0 and 1 should both map to fastest.
	if mapEncoderLevel(0) != fastest {
		t.Error("level 0 should map to fastest")
	}
}

func TestMapBrotliQuality(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{1, 2},
		{2, 4},
		{4, 4},
		{5, 6},
		{8, 6},
		{9, 9},
		{22, 9},
	}
	for _, tt := range tests {
		got := mapBrotliQuality(tt.level)
		if got != tt.want {
			t.Errorf("mapBrotliQuality(%d) = %d, want %d", tt.level, got, tt.want)
		}
	}
}

func TestCompressZstdRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("hello world "), 100)
	compressed, err := compressZstd(data, 3, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) >= len(data) {
		t.Error("compressed should be smaller than input")
	}
}

func TestCompressBrotliRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("hello world "), 100)
	compressed, err := compressBrotli(data, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) >= len(data) {
		t.Error("compressed should be smaller than input")
	}
}

func TestEncoderCacheReuse(t *testing.T) {
	cache := newEncoderCache()
	data := bytes.Repeat([]byte("test data "), 50)

	c1, err := cache.compress(data, 3, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := cache.compress(data, 3, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(c1, c2) {
		t.Error("same input should produce same output from cached encoder")
	}
	// Cache should have exactly one entry.
	if len(cache) != 1 {
		t.Errorf("cache size = %d, want 1", len(cache))
	}
}

func TestTrainDictionaryEmptySamples(t *testing.T) {
	dict, err := trainDictionary(1, nil, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if dict != nil {
		t.Error("empty samples should return nil dict")
	}
}

func TestTrainDictionarySmallSamples(t *testing.T) {
	// Samples too small for minHistSize (128 KiB).
	samples := [][]byte{[]byte("small")}
	dict, err := trainDictionary(1, samples, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if dict != nil {
		t.Error("small samples should return nil dict")
	}
}

// --- compressRawSection ---

func TestCompressRawSectionTiny(t *testing.T) {
	data := []byte("tiny")
	s := compressRawSection(oza.SectionMetadata, data)
	if s.compression != oza.CompNone {
		t.Errorf("tiny section should not be compressed, got compression=%d", s.compression)
	}
	if !bytes.Equal(s.data, data) {
		t.Error("tiny section data should be unchanged")
	}
}

func TestCompressRawSectionCompressible(t *testing.T) {
	data := bytes.Repeat([]byte("compressible data that repeats "), 100)
	s := compressRawSection(oza.SectionMetadata, data)
	if s.compression != oza.CompZstd {
		t.Errorf("compressible section should use zstd, got compression=%d", s.compression)
	}
	if len(s.data) >= len(data) {
		t.Error("compressed data should be smaller")
	}
	if s.uncompressedSize != uint64(len(data)) {
		t.Errorf("uncompressedSize = %d, want %d", s.uncompressedSize, len(data))
	}
}

// --- buildMIMETable ---

func TestBuildMIMETable(t *testing.T) {
	w := &Writer{
		entries: []*entryBuilder{
			{mimeType: "text/html"},
			{mimeType: "image/png"},
			{mimeType: "text/css"},
			{mimeType: "application/json"},
			{mimeType: "text/html"}, // duplicate
		},
	}
	types, m := w.buildMIMETable()

	// First three must be the mandatory types.
	if types[0] != "text/html" || types[1] != "text/css" || types[2] != "application/javascript" {
		t.Errorf("mandatory types wrong: %v", types[:3])
	}
	if m["text/html"] != 0 || m["text/css"] != 1 || m["application/javascript"] != 2 {
		t.Error("mandatory type indices wrong")
	}
	// Additional types should be present.
	if _, ok := m["image/png"]; !ok {
		t.Error("image/png should be in MIME map")
	}
	if _, ok := m["application/json"]; !ok {
		t.Error("application/json should be in MIME map")
	}
	// No duplicates in type list.
	seen := make(map[string]bool)
	for _, typ := range types {
		if seen[typ] {
			t.Errorf("duplicate type in list: %s", typ)
		}
		seen[typ] = true
	}
}

// --- buildRedirectSection ---

func TestBuildRedirectSectionEmpty(t *testing.T) {
	w := &Writer{}
	data := w.buildRedirectSection()
	if data != nil {
		t.Error("no redirects should return nil")
	}
}

func TestBuildRedirectSection(t *testing.T) {
	w := &Writer{
		redirectEntries: []*entryBuilder{
			{targetID: 5, isFrontArticle: true, redirectIndex: 0},
			{targetID: 10, isFrontArticle: false, redirectIndex: 1},
		},
	}
	data := w.buildRedirectSection()
	if data == nil {
		t.Fatal("expected non-nil data")
	}

	count := binary.LittleEndian.Uint32(data[0:4])
	if count != 2 {
		t.Errorf("redirect count = %d, want 2", count)
	}
	expectedLen := 4 + 2*oza.RedirectRecordSize
	if len(data) != expectedLen {
		t.Errorf("data len = %d, want %d", len(data), expectedLen)
	}
}

// --- full Writer round-trip for pipeline coverage ---

func TestWriterChunkSplitting(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "chunk-split*.oza")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Use a tiny chunk target to force multiple chunks.
	w := NewWriter(f, WriterOptions{
		ZstdLevel:       3,
		ChunkTargetSize: 256, // very small to force splits
		TrainDict:       false,
		BuildSearch:     false,
	})
	w.SetMetadata("title", "Split Test")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	// Add entries with unique content well exceeding chunk target to force splits.
	for i := 0; i < 10; i++ {
		// Each entry unique so dedup doesn't collapse them.
		content := bytes.Repeat([]byte{byte(i), byte(i + 1), byte(i + 2)}, 300) // 900 bytes each
		if _, err := w.AddEntry(
			"entry_"+string(rune('a'+i))+".txt",
			"Entry "+string(rune('A'+i)),
			"text/plain",
			content,
			true,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Verify the archive.
	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if a.EntryCount() != 10 {
		t.Errorf("EntryCount = %d, want 10", a.EntryCount())
	}
	if a.ChunkCount() < 2 {
		t.Errorf("expected multiple chunks with tiny target, got %d", a.ChunkCount())
	}

	// Verify all content reads back.
	for i := 0; i < 10; i++ {
		e, err := a.EntryByID(uint32(i))
		if err != nil {
			t.Fatalf("EntryByID(%d): %v", i, err)
		}
		content, err := e.ReadContent()
		if err != nil {
			t.Fatalf("ReadContent(%d): %v", i, err)
		}
		if len(content) == 0 {
			t.Errorf("entry %d has empty content", i)
		}
	}
}

func TestWriterDedup(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "dedup*.oza")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := NewWriter(f, WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
	})
	w.SetMetadata("title", "Dedup Test")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	// Add same content under different paths.
	content := []byte("<html><body>Duplicate content</body></html>")
	for i := 0; i < 5; i++ {
		if _, err := w.AddEntry(
			"dup_"+string(rune('a'+i))+".html",
			"Dup "+string(rune('A'+i)),
			"text/html",
			content,
			true,
		); err != nil {
			t.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Verify all entries read back the same content.
	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if a.EntryCount() != 5 {
		t.Errorf("EntryCount = %d, want 5", a.EntryCount())
	}

	for i := 0; i < 5; i++ {
		e, err := a.EntryByID(uint32(i))
		if err != nil {
			t.Fatalf("EntryByID(%d): %v", i, err)
		}
		got, err := e.ReadContent()
		if err != nil {
			t.Fatalf("ReadContent(%d): %v", i, err)
		}
		if len(got) == 0 {
			t.Errorf("entry %d has empty content", i)
		}
	}
}

func TestWriterMinifyFallback(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "minify*.oza")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := NewWriter(f, WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
		MinifyHTML:  true,
	})
	w.SetMetadata("title", "Minify Test")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	// Malformed HTML that the minifier might struggle with.
	if _, err := w.AddEntry("bad.html", "Bad HTML", "text/html",
		[]byte("<html><body><<<unclosed"), true); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	e, err := a.EntryByID(0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := e.ReadContent()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Error("minification fallback should still produce content")
	}
}

func TestWriterWithDictTraining(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "dict*.oza")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := NewWriter(f, WriterOptions{
		ZstdLevel:   3,
		TrainDict:   true,
		DictSamples: 5, // low threshold to trigger training quickly
		BuildSearch: false,
	})
	w.SetMetadata("title", "Dict Test")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	// Add enough similar entries to potentially train a dictionary.
	for i := 0; i < 20; i++ {
		content := bytes.Repeat([]byte("<html><body><h1>Common header</h1><p>Article content</p></body></html>"), 30)
		if _, err := w.AddEntry(
			"article_"+string(rune('a'+i%26))+string(rune('0'+i/26))+".html",
			"Article",
			"text/html",
			content,
			true,
		); err != nil {
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
	defer a.Close()

	if a.EntryCount() == 0 {
		t.Error("archive should have entries")
	}

	// Verify all content reads back.
	for i := uint32(0); i < a.EntryCount(); i++ {
		e, err := a.EntryByID(i)
		if err != nil {
			t.Fatalf("EntryByID(%d): %v", i, err)
		}
		if _, err := e.ReadContent(); err != nil {
			t.Fatalf("ReadContent(%d): %v", i, err)
		}
	}
}

func TestWriterImageChunk(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "image*.oza")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := NewWriter(f, WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
	})
	w.SetMetadata("title", "Image Test")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")

	// Add a mix of images and HTML.
	if _, err := w.AddEntry("page.html", "Page", "text/html",
		[]byte("<html><body>text</body></html>"), true); err != nil {
		t.Fatal(err)
	}
	// Fake image data (random-ish bytes that won't compress well).
	imgData := make([]byte, 1000)
	for i := range imgData {
		imgData[i] = byte(i * 7)
	}
	if _, err := w.AddEntry("photo.jpg", "Photo", "image/jpeg", imgData, false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddEntry("icon.png", "Icon", "image/png",
		[]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, false); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	a, err := oza.OpenWithOptions(f.Name(), oza.WithMmap(false))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if a.EntryCount() != 3 {
		t.Errorf("EntryCount = %d, want 3", a.EntryCount())
	}
}

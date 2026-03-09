package ozawrite_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"testing"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

// newTestWriter creates a temporary file, returns the writer and a cleanup func.
func newTestWriter(t *testing.T) (*ozawrite.Writer, *os.File, func()) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "test*.oza")
	if err != nil {
		t.Fatal(err)
	}
	opts := ozawrite.WriterOptions{
		ZstdLevel:   3,  // fast for tests
		TrainDict:   false,
		BuildSearch: false,
	}
	w := ozawrite.NewWriter(f, opts)
	return w, f, func() { f.Close() }
}

func requiredMeta(w *ozawrite.Writer) {
	w.SetMetadata("title", "Test Archive")
	w.SetMetadata("language", "en")
	w.SetMetadata("creator", "test")
	w.SetMetadata("date", "2026-01-01")
	w.SetMetadata("source", "https://example.com")
}

// readFile returns all bytes written to f.
func readFile(t *testing.T, f *os.File) []byte {
	t.Helper()
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// --- TestWriteMinimal ---

func TestWriteMinimal(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()

	requiredMeta(w)
	content := []byte("<html><body>Hello</body></html>")
	id, err := w.AddEntry("index.html", "Index", "text/html", content, true)
	if err != nil {
		t.Fatal(err)
	}
	if id != 0 {
		t.Fatalf("first entry id = %d, want 0", id)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, f)
	if len(data) < oza.HeaderSize {
		t.Fatalf("output too short: %d bytes", len(data))
	}

	// Parse header.
	hdr, err := oza.ParseHeader(data[:oza.HeaderSize])
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}
	if hdr.EntryCount != 1 {
		t.Errorf("EntryCount = %d, want 1", hdr.EntryCount)
	}
	if hdr.SectionCount == 0 {
		t.Error("SectionCount = 0")
	}

	// Parse section table.
	tableOff := hdr.SectionTableOff
	tableEnd := tableOff + uint64(hdr.SectionCount)*oza.SectionSize
	if uint64(len(data)) < tableEnd {
		t.Fatalf("file too short for section table")
	}
	sections, err := oza.ParseSectionTable(data[tableOff:tableEnd], hdr.SectionCount)
	if err != nil {
		t.Fatalf("ParseSectionTable: %v", err)
	}

	// Verify we can find and parse the MIME table section.
	var mimeSection *oza.SectionDesc
	for i := range sections {
		if sections[i].Type == oza.SectionMIMETable {
			mimeSection = &sections[i]
			break
		}
	}
	if mimeSection == nil {
		t.Fatal("MIME table section not found")
	}
	mimeEnd := mimeSection.Offset + mimeSection.CompressedSize
	if uint64(len(data)) < mimeEnd {
		t.Fatal("file too short for MIME section")
	}
	mimeTypes, err := oza.ParseMIMETable(data[mimeSection.Offset:mimeEnd])
	if err != nil {
		t.Fatalf("ParseMIMETable: %v", err)
	}
	if len(mimeTypes) < 3 {
		t.Errorf("MIME table has %d entries, want >= 3", len(mimeTypes))
	}

	// Verify file checksum (last 32 bytes).
	if uint64(len(data)) < hdr.ChecksumOff+32 {
		t.Fatal("file too short for checksum")
	}
	body := data[:hdr.ChecksumOff]
	wantHash := sha256.Sum256(body)
	gotHash := data[hdr.ChecksumOff : hdr.ChecksumOff+32]
	if !bytes.Equal(wantHash[:], gotHash) {
		t.Errorf("file checksum mismatch")
	}
}

// --- TestWriteWithRedirect ---

func TestWriteWithRedirect(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()

	requiredMeta(w)
	contentID, _ := w.AddEntry("page.html", "Page", "text/html", []byte("<html>page</html>"), false)
	redirID, err := w.AddRedirect("redir.html", "Redirect", contentID)
	if err != nil {
		t.Fatal(err)
	}
	// Redirect ID should have bit 31 set.
	if !oza.IsRedirectID(redirID) {
		t.Fatalf("redirect ID 0x%08x should have bit 31 set", redirID)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, f)
	hdr, _ := oza.ParseHeader(data[:oza.HeaderSize])
	// EntryCount should only count content entries.
	if hdr.EntryCount != 1 {
		t.Fatalf("EntryCount = %d, want 1 (content only)", hdr.EntryCount)
	}

	// Find redirect table section.
	tableEnd := hdr.SectionTableOff + uint64(hdr.SectionCount)*oza.SectionSize
	sections, _ := oza.ParseSectionTable(data[hdr.SectionTableOff:tableEnd], hdr.SectionCount)
	var redirectSection *oza.SectionDesc
	for i := range sections {
		if sections[i].Type == oza.SectionRedirectTab {
			redirectSection = &sections[i]
			break
		}
	}
	if redirectSection == nil {
		t.Fatal("redirect table section not found")
	}

	// Parse the redirect section (it may be compressed).
	redirectData := data[redirectSection.Offset : redirectSection.Offset+redirectSection.CompressedSize]
	// For small data the section won't be compressed.
	if len(redirectData) < 4+oza.RedirectRecordSize {
		t.Fatalf("redirect section too short: %d bytes", len(redirectData))
	}
	count := binary.LittleEndian.Uint32(redirectData[0:4])
	if count != 1 {
		t.Fatalf("redirect count = %d, want 1", count)
	}
	rr, err := oza.ParseRedirectRecord(redirectData[4:])
	if err != nil {
		t.Fatalf("ParseRedirectRecord: %v", err)
	}
	if rr.TargetID != contentID {
		t.Errorf("redirect target = %d, want %d", rr.TargetID, contentID)
	}
}

// --- TestWriteDeduplication ---

func TestWriteDeduplication(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()

	requiredMeta(w)
	content := []byte("<html>same content</html>")
	id0, _ := w.AddEntry("a.html", "A", "text/html", content, false)
	id1, _ := w.AddEntry("b.html", "B", "text/html", content, false)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, f)
	hdr, _ := oza.ParseHeader(data[:oza.HeaderSize])

	tableEnd := hdr.SectionTableOff + uint64(hdr.SectionCount)*oza.SectionSize
	sections, _ := oza.ParseSectionTable(data[hdr.SectionTableOff:tableEnd], hdr.SectionCount)

	var entrySection *oza.SectionDesc
	for i := range sections {
		if sections[i].Type == oza.SectionEntryTable {
			entrySection = &sections[i]
			break
		}
	}
	entryData := data[entrySection.Offset : entrySection.Offset+entrySection.CompressedSize]

	// Parse variable-length entry table: header + offset table + records.
	count := binary.LittleEndian.Uint32(entryData[0:4])
	recordDataOff := binary.LittleEndian.Uint32(entryData[4:8])
	parseEntry := func(id uint32) oza.EntryRecord {
		t.Helper()
		if id >= count {
			t.Fatalf("entry ID %d out of range (count=%d)", id, count)
		}
		off := binary.LittleEndian.Uint32(entryData[oza.EntryTableHeaderSize+id*4:])
		rec, _, err := oza.ParseVarEntryRecord(entryData[recordDataOff+off:])
		if err != nil {
			t.Fatalf("ParseVarEntryRecord(id=%d): %v", id, err)
		}
		return rec
	}
	rec0 := parseEntry(id0)
	rec1 := parseEntry(id1)

	// Both entries must point to the same chunk/blob.
	if rec0.ChunkID != rec1.ChunkID {
		t.Errorf("dedup: different chunk IDs (%d vs %d)", rec0.ChunkID, rec1.ChunkID)
	}
	if rec0.BlobOffset != rec1.BlobOffset {
		t.Errorf("dedup: different blob offsets (%d vs %d)", rec0.BlobOffset, rec1.BlobOffset)
	}
}

// --- TestWriteCompression ---

func TestWriteCompression(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()

	requiredMeta(w)
	html := bytes.Repeat([]byte("Hello World "), 200) // highly compressible

	// Minimal valid JPEG header bytes (not actually a valid image, but MIME type matters).
	jpeg := append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, bytes.Repeat([]byte{0x00}, 100)...)

	w.AddEntry("page.html", "Page", "text/html", html, false)
	w.AddEntry("img.jpg", "Img", "image/jpeg", jpeg, false)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, f)
	hdr, _ := oza.ParseHeader(data[:oza.HeaderSize])
	tableEnd := hdr.SectionTableOff + uint64(hdr.SectionCount)*oza.SectionSize
	sections, _ := oza.ParseSectionTable(data[hdr.SectionTableOff:tableEnd], hdr.SectionCount)

	var contentSection *oza.SectionDesc
	for i := range sections {
		if sections[i].Type == oza.SectionContent {
			contentSection = &sections[i]
			break
		}
	}
	if contentSection == nil {
		t.Fatal("content section not found")
	}

	contentData := data[contentSection.Offset : contentSection.Offset+contentSection.CompressedSize]
	chunkCount := binary.LittleEndian.Uint32(contentData[0:4])
	if chunkCount == 0 {
		t.Fatal("no chunks in content section")
	}

	// Read first chunk descriptor and verify HTML chunk is compressed.
	// We can't easily identify which chunk is which without full decompression,
	// but we can verify that at least one chunk has compression != CompNone.
	hasCompressed := false
	for i := range int(chunkCount) {
		off := 4 + i*oza.ChunkDescSize
		comp := contentData[off+24] // Compression byte at position 24 in ChunkDesc
		if comp != oza.CompNone {
			hasCompressed = true
			break
		}
	}
	if !hasCompressed {
		t.Error("expected at least one compressed chunk for HTML content")
	}
}

// --- TestWriteMetadata ---

func TestWriteMetadata(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()

	requiredMeta(w)
	w.SetMetadata("description", "A test archive")
	w.AddEntry("index.html", "Index", "text/html", []byte("<html/>"), false)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, f)
	hdr, _ := oza.ParseHeader(data[:oza.HeaderSize])
	tableEnd := hdr.SectionTableOff + uint64(hdr.SectionCount)*oza.SectionSize
	sections, _ := oza.ParseSectionTable(data[hdr.SectionTableOff:tableEnd], hdr.SectionCount)

	var metaSection *oza.SectionDesc
	for i := range sections {
		if sections[i].Type == oza.SectionMetadata {
			metaSection = &sections[i]
			break
		}
	}
	if metaSection == nil {
		t.Fatal("metadata section not found")
	}
	metaData := data[metaSection.Offset : metaSection.Offset+metaSection.CompressedSize]
	m, err := oza.ParseMetadata(metaData)
	if err != nil {
		t.Fatalf("ParseMetadata: %v", err)
	}
	for _, key := range oza.RequiredMetadataKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("required metadata key %q missing", key)
		}
	}
	if string(m["description"]) != "A test archive" {
		t.Errorf("description = %q, want %q", m["description"], "A test archive")
	}
}

// --- TestWriteMIMEConvention ---

func TestWriteMIMEConvention(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()

	requiredMeta(w)
	// Add CSS and JS before HTML to verify index 0/1/2 is always enforced.
	w.AddEntry("style.css", "Style", "text/css", []byte("body{}"), false)
	w.AddEntry("script.js", "Script", "application/javascript", []byte("var x=1;"), false)
	w.AddEntry("index.html", "Index", "text/html", []byte("<html/>"), false)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, f)
	hdr, _ := oza.ParseHeader(data[:oza.HeaderSize])
	tableEnd := hdr.SectionTableOff + uint64(hdr.SectionCount)*oza.SectionSize
	sections, _ := oza.ParseSectionTable(data[hdr.SectionTableOff:tableEnd], hdr.SectionCount)

	var mimeSection *oza.SectionDesc
	for i := range sections {
		if sections[i].Type == oza.SectionMIMETable {
			mimeSection = &sections[i]
			break
		}
	}
	mimeData := data[mimeSection.Offset : mimeSection.Offset+mimeSection.CompressedSize]
	types, err := oza.ParseMIMETable(mimeData)
	if err != nil {
		t.Fatalf("ParseMIMETable: %v", err)
	}
	if types[0] != "text/html" {
		t.Errorf("MIME[0] = %q, want text/html", types[0])
	}
	if types[1] != "text/css" {
		t.Errorf("MIME[1] = %q, want text/css", types[1])
	}
	if types[2] != "application/javascript" {
		t.Errorf("MIME[2] = %q, want application/javascript", types[2])
	}
}

// --- TestWriteChecksums ---

func TestWriteChecksums(t *testing.T) {
	w, f, cleanup := newTestWriter(t)
	defer cleanup()

	requiredMeta(w)
	w.AddEntry("index.html", "Index", "text/html", []byte("<html>test</html>"), false)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := readFile(t, f)
	hdr, _ := oza.ParseHeader(data[:oza.HeaderSize])

	// File-level checksum.
	body := data[:hdr.ChecksumOff]
	wantHash := sha256.Sum256(body)
	gotHash := [32]byte{}
	copy(gotHash[:], data[hdr.ChecksumOff:hdr.ChecksumOff+32])
	if wantHash != gotHash {
		t.Error("file-level SHA-256 checksum mismatch")
	}

	// Section-level checksums.
	tableEnd := hdr.SectionTableOff + uint64(hdr.SectionCount)*oza.SectionSize
	sections, _ := oza.ParseSectionTable(data[hdr.SectionTableOff:tableEnd], hdr.SectionCount)
	for _, s := range sections {
		sdata := data[s.Offset : s.Offset+s.CompressedSize]
		got := sha256.Sum256(sdata)
		if got != s.SHA256 {
			t.Errorf("section type 0x%04x: SHA-256 mismatch", s.Type)
		}
	}
}

// --- TestWriteMissingMetadata ---

func TestWriteMissingMetadata(t *testing.T) {
	w, _, cleanup := newTestWriter(t)
	defer cleanup()

	// Do NOT set required metadata — Close should return an error.
	w.AddEntry("index.html", "Index", "text/html", []byte("<html/>"), false)
	if err := w.Close(); err == nil {
		t.Error("expected error for missing required metadata, got nil")
	}
}


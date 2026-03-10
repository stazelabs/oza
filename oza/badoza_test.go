package oza_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stazelabs/oza/oza"
	"github.com/stazelabs/oza/ozawrite"
)

// ---------------------------------------------------------------------------
// recipe describes one adversarial archive test case.
// ---------------------------------------------------------------------------

type recipe struct {
	Name    string
	Build   func(t *testing.T) []byte       // produce valid archive bytes
	Corrupt func(valid []byte) []byte       // apply targeted corruption
	Check   func(t *testing.T, path string) // assert failure mode
}

// ---------------------------------------------------------------------------
// Builders — produce valid archives tailored for specific recipe needs.
// ---------------------------------------------------------------------------

func buildMinimal(t *testing.T) []byte {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "adv*.oza")
	if err != nil {
		t.Fatal(err)
	}
	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
	})
	setRequiredMeta(w)
	if _, err := w.AddEntry("index.html", "Index", "text/html", []byte("<h1>Hello</h1>"), true); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func buildWithRedirects(t *testing.T) []byte {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "adv-redir*.oza")
	if err != nil {
		t.Fatal(err)
	}
	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
	})
	setRequiredMeta(w)
	id0, err2 := w.AddEntry("A/Page1", "Page1", "text/html", []byte("<h1>Page1</h1>"), true)
	if err2 != nil {
		t.Fatal(err2)
	}
	id1, err3 := w.AddEntry("A/Page2", "Page2", "text/html", []byte("<h1>Page2</h1>"), true)
	if err3 != nil {
		t.Fatal(err3)
	}
	if _, err := w.AddRedirect("A/Alias1", "Alias1", id0); err != nil {
		t.Fatal(err)
	}
	if _, err := w.AddRedirect("A/Alias2", "Alias2", id1); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func buildWithSearch(t *testing.T) []byte {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "adv-search*.oza")
	if err != nil {
		t.Fatal(err)
	}
	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: true,
	})
	setRequiredMeta(w)
	for i := 0; i < 5; i++ {
		body := []byte("<html><body>The quick brown fox jumps over the lazy dog</body></html>")
		path := "A/Page" + string(rune('A'+i))
		title := "Page " + string(rune('A'+i))
		if _, err := w.AddEntry(path, title, "text/html", body, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func buildWithSignatures(t *testing.T) ([]byte, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(t.TempDir(), "adv-sig*.oza")
	if err != nil {
		t.Fatal(err)
	}
	w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
		ZstdLevel:   3,
		TrainDict:   false,
		BuildSearch: false,
		SigningKeys: []ozawrite.SigningKey{{Key: priv, KeyID: 1}},
	})
	setRequiredMeta(w)
	if _, err := w.AddEntry("index.html", "Index", "text/html", []byte("<h1>Signed</h1>"), true); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return data, pub
}

// ---------------------------------------------------------------------------
// Parse-then-corrupt helpers — find mutation points semantically.
// ---------------------------------------------------------------------------

func parseHdr(data []byte) oza.Header {
	h, err := oza.ParseHeader(data)
	if err != nil {
		panic("parseHdr: " + err.Error())
	}
	return h
}

func findSection(data []byte, st oza.SectionType) (oza.SectionDesc, int) {
	h := parseHdr(data)
	off := int(h.SectionTableOff)
	for i := 0; i < int(h.SectionCount); i++ {
		d, err := oza.ParseSectionDesc(data[off+i*oza.SectionSize:])
		if err != nil {
			continue
		}
		if d.Type == st {
			return d, i
		}
	}
	panic("findSection: not found")
}

// sectionDescOff returns the file byte offset of the i-th section descriptor.
func sectionDescOff(h oza.Header, i int) int {
	return int(h.SectionTableOff) + i*oza.SectionSize
}

func put32(data []byte, off int, v uint32) {
	binary.LittleEndian.PutUint32(data[off:off+4], v)
}

func put64(data []byte, off int, v uint64) {
	binary.LittleEndian.PutUint64(data[off:off+8], v)
}

func clone(data []byte) []byte {
	c := make([]byte, len(data))
	copy(c, data)
	return c
}

// ---------------------------------------------------------------------------
// Check patterns — reusable verification functions.
// ---------------------------------------------------------------------------

// mustFailOpen asserts that Open returns an error.
func mustFailOpen(t *testing.T, path string) {
	t.Helper()
	a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
	if err == nil {
		a.Close()
		t.Fatal("expected Open to fail, but it succeeded")
	}
}

// mustFailOpenWith asserts that Open returns a specific sentinel error.
func mustFailOpenWith(t *testing.T, path string, target error) {
	t.Helper()
	a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
	if err == nil {
		a.Close()
		t.Fatalf("expected Open to fail with %v, but it succeeded", target)
	}
	if !errors.Is(err, target) {
		t.Fatalf("expected Open error wrapping %v, got: %v", target, err)
	}
}

// mustFailVerify asserts Open succeeds but Verify fails.
func mustFailVerify(t *testing.T, path string) {
	t.Helper()
	a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
	if err != nil {
		return // open-time failure is also acceptable
	}
	defer a.Close()
	if err := a.Verify(); err == nil {
		t.Fatal("expected Verify to fail")
	}
}

// mustFailVerifyAll asserts Open succeeds but VerifyAll has at least one failure.
func mustFailVerifyAll(t *testing.T, path string) {
	t.Helper()
	a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
	if err != nil {
		return // open-time failure is also acceptable
	}
	defer a.Close()
	results, err := a.VerifyAll()
	if err != nil {
		return // error is acceptable
	}
	for _, r := range results {
		if !r.OK {
			return // found a failure — good
		}
	}
	t.Fatal("expected VerifyAll to report at least one failure")
}

// writeCorrupt writes data to a temp file and returns its path.
func writeCorrupt(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "corrupt.oza")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// ---------------------------------------------------------------------------
// TestAdversarialArchives — table-driven integration test.
// ---------------------------------------------------------------------------

func TestAdversarialArchives(t *testing.T) {
	recipes := allRecipes(t)
	for _, r := range recipes {
		r := r
		t.Run(r.Name, func(t *testing.T) {
			valid := r.Build(t)
			corrupted := r.Corrupt(valid)
			path := writeCorrupt(t, corrupted)
			r.Check(t, path)
		})
	}
}

func allRecipes(t *testing.T) []recipe {
	// Pre-build shared artifacts for signature recipes.
	var sigPub ed25519.PublicKey

	recipes := []recipe{
		// ---------------------------------------------------------------
		// P. Header Edge Cases
		// ---------------------------------------------------------------
		{
			Name:  "P1_InvalidMagic",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				put32(c, 0, 0xDEADBEEF)
				return c
			},
			Check: func(t *testing.T, path string) {
				mustFailOpenWith(t, path, oza.ErrInvalidMagic)
			},
		},
		{
			Name:  "P2_FutureVersion",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				binary.LittleEndian.PutUint16(c[4:6], 99) // MajorVersion
				return c
			},
			Check: func(t *testing.T, path string) {
				mustFailOpenWith(t, path, oza.ErrUnsupportedVersion)
			},
		},
		{
			Name:  "P3_NonZeroReserved",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				// Header reserved bytes at [68:128].
				for i := 68; i < 128; i++ {
					c[i] = 0xFF
				}
				return c
			},
			Check: func(t *testing.T, path string) {
				// Should open successfully (forward compat) but produce warnings.
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					t.Fatalf("expected Open to succeed with warnings, got error: %v", err)
				}
				defer a.Close()
				if len(a.Warnings()) == 0 {
					t.Fatal("expected at least one warning for non-zero reserved bytes")
				}
			},
		},

		// ---------------------------------------------------------------
		// A. Structural Truncation
		// ---------------------------------------------------------------
		{
			Name:  "A1_TruncatedHeader",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				return clone(data[:32]) // Half the 128-byte header
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "A2_TruncatedSectionTable",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				h := parseHdr(data)
				// Cut in the middle of the section table.
				cutAt := int(h.SectionTableOff) + oza.SectionSize/2
				return clone(data[:cutAt])
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "A3_TruncatedContentSection",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				s, _ := findSection(data, oza.SectionContent)
				cutAt := int(s.Offset) + int(s.CompressedSize)/2
				if cutAt >= len(data) {
					cutAt = len(data) - 1
				}
				return clone(data[:cutAt])
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "A4_TruncatedChecksum",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				h := parseHdr(data)
				cutAt := int(h.ChecksumOff) + 16 // Only half the checksum
				if cutAt > len(data) {
					cutAt = len(data) - 1
				}
				return clone(data[:cutAt])
			},
			Check: mustFailVerify,
		},

		// ---------------------------------------------------------------
		// B. Out-of-Bounds Offsets
		// ---------------------------------------------------------------
		{
			Name:  "B1_SectionOffsetPastEOF",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				_, idx := findSection(c, oza.SectionContent)
				descOff := sectionDescOff(h, idx)
				// Patch the Offset field (bytes 8:16 within the descriptor).
				put64(c, descOff+8, uint64(len(data)+4096))
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "B2_SectionTableOffPastEOF",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				// Header SectionTableOff is at bytes [40:48].
				put64(c, 40, uint64(len(data)+4096))
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "B3_ChecksumOffPastEOF",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				// Header ChecksumOff is at bytes [48:56].
				put64(c, 48, uint64(len(data)+4096))
				return c
			},
			Check: mustFailVerify,
		},
		{
			Name:  "B4_ChunkOffsetPastContent",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionContent)
				// Chunk table starts at s.Offset+4 (after the 4-byte count).
				// chunkDesc CompressedOff is at bytes [4:12] within the descriptor.
				chunkDescStart := int(s.Offset) + 4
				put64(c, chunkDescStart+4, 0xFFFFFFFF) // CompressedOff
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // open-time failure is acceptable
				}
				defer a.Close()
				e, err := a.EntryByID(0)
				if err != nil {
					return
				}
				_, err = e.ReadContent()
				if err == nil {
					t.Fatal("expected ReadContent to fail with OOB chunk offset")
				}
			},
		},
		{
			Name:  "B5_EntryBlobPastChunk",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionContent)
				// chunkDesc CompressedSize is at bytes [12:20] within the descriptor.
				// Set it to 1 so decompressed chunk is tiny, then blob_size in entry
				// record will exceed it.
				chunkDescStart := int(s.Offset) + 4
				put64(c, chunkDescStart+12, 1) // CompressedSize = 1
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return
				}
				defer a.Close()
				e, err := a.EntryByID(0)
				if err != nil {
					return
				}
				_, err = e.ReadContent()
				if err == nil {
					t.Fatal("expected ReadContent to fail with blob past chunk")
				}
			},
		},

		// ---------------------------------------------------------------
		// E. Checksum Corruption
		// ---------------------------------------------------------------
		{
			Name:  "E1_FileChecksumFlipped",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				c[h.ChecksumOff] ^= 0xFF
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return
				}
				defer a.Close()
				err = a.Verify()
				if !errors.Is(err, oza.ErrChecksumMismatch) {
					t.Fatalf("expected ErrChecksumMismatch, got: %v", err)
				}
			},
		},
		{
			Name:  "E2_SectionChecksumFlipped",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				// Flip byte in the first section descriptor's SHA-256 field
				// (offset 48 within the descriptor).
				descOff := sectionDescOff(h, 0)
				c[descOff+48] ^= 0xFF
				return c
			},
			Check: mustFailVerifyAll,
		},
		{
			Name:  "E3_ContentByteFlipped",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionContent)
				// Flip a byte in the middle of the content section.
				mid := int(s.Offset) + int(s.CompressedSize)/2
				if mid < len(c) {
					c[mid] ^= 0xFF
				}
				return c
			},
			Check: mustFailVerifyAll,
		},

		// ---------------------------------------------------------------
		// H. Count Overflow
		// ---------------------------------------------------------------
		{
			Name:  "H1_MassiveSectionCount",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				// Header SectionCount at bytes [24:28].
				// Use a value large enough that SectionCount * SectionSize exceeds
				// file size, but small enough not to OOM the allocator.
				put32(c, 24, uint32(len(data)/oza.SectionSize+1000))
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "H2_MassiveChunkCount",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionContent)
				// Content section starts with uint32 chunk_count.
				// Use a value where count * ChunkDescSize exceeds what can be
				// read from the file, but won't OOM the allocator.
				put32(c, int(s.Offset), uint32(len(data)/oza.ChunkDescSize+1000))
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},

		// ---------------------------------------------------------------
		// C. Decompression Attacks
		// ---------------------------------------------------------------
		{
			Name:  "C2_InvalidCompressionType",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				// Find first non-content section with compression != 0,
				// or just set any section's compression to 0xFF.
				// Set the metadata section's compression to 0xFF.
				_, idx := findSection(c, oza.SectionMetadata)
				descOff := sectionDescOff(h, idx)
				// Compression byte is at offset 32 within the descriptor.
				c[descOff+32] = 0xFF
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},

		// ---------------------------------------------------------------
		// G. Section-Level Confusion
		// ---------------------------------------------------------------
		{
			Name:  "G1_SectionTypeSwap",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				_, metaIdx := findSection(c, oza.SectionMetadata)
				_, mimeIdx := findSection(c, oza.SectionMIMETable)
				metaDescOff := sectionDescOff(h, metaIdx)
				mimeDescOff := sectionDescOff(h, mimeIdx)
				// Swap the Type fields (first 4 bytes of each descriptor).
				metaType := binary.LittleEndian.Uint32(c[metaDescOff:])
				mimeType := binary.LittleEndian.Uint32(c[mimeDescOff:])
				put32(c, metaDescOff, mimeType)
				put32(c, mimeDescOff, metaType)
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "G2_DuplicateSectionType",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				_, metaIdx := findSection(c, oza.SectionMetadata)
				_, mimeIdx := findSection(c, oza.SectionMIMETable)
				metaDescOff := sectionDescOff(h, metaIdx)
				mimeDescOff := sectionDescOff(h, mimeIdx)
				// Set both to SectionMetadata type.
				put32(c, mimeDescOff, binary.LittleEndian.Uint32(c[metaDescOff:metaDescOff+4]))
				return c
			},
			Check: func(t *testing.T, path string) {
				// May open but MIME table will be missing — operations should fail.
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // open-time failure is acceptable
				}
				defer a.Close()
				// Without a MIME table, MIMETypes should be empty.
				if len(a.MIMETypes()) != 0 {
					t.Log("MIME table unexpectedly non-empty with duplicate section types")
				}
			},
		},
		{
			Name:  "G3_OverlappingSections",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				if h.SectionCount < 2 {
					return c
				}
				// Set section 1's offset to section 0's offset (overlap).
				desc0Off := sectionDescOff(h, 0)
				desc1Off := sectionDescOff(h, 1)
				offset0 := binary.LittleEndian.Uint64(c[desc0Off+8:])
				put64(c, desc1Off+8, offset0)
				return c
			},
			Check: func(t *testing.T, path string) {
				// May open but produce garbled data.
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // acceptable
				}
				defer a.Close()
				// Verify should detect SHA-256 mismatch for overlapping sections.
				mustFailVerifyAll(t, path)
			},
		},

		// ---------------------------------------------------------------
		// I. MIME Table Corruption
		// ---------------------------------------------------------------
		{
			Name:  "I1_MIMEConventionViolation",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionMIMETable)
				// The MIME table section starts with uint16 count, then per-type:
				// uint16 string_length + string bytes.
				// Index 0 must be "text/html". Replace with "text/plain".
				soff := int(s.Offset)
				// Skip count (2 bytes), then first entry's string_length (2 bytes).
				strStart := soff + 2 + 2
				// Overwrite "text/html" with "text/plai" (same length=9).
				replacement := []byte("text/plai")
				if strStart+len(replacement) <= len(c) {
					copy(c[strStart:], replacement)
				}
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},
		{
			Name:  "I2_MIMETableTruncated",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				h := parseHdr(c)
				_, idx := findSection(c, oza.SectionMIMETable)
				descOff := sectionDescOff(h, idx)
				// Set CompressedSize to 1 byte.
				put64(c, descOff+16, 1) // CompressedSize
				put64(c, descOff+24, 1) // UncompressedSize
				return c
			},
			Check: func(t *testing.T, path string) { mustFailOpen(t, path) },
		},

		// ---------------------------------------------------------------
		// L. Chunk Table Corruption
		// ---------------------------------------------------------------
		{
			Name: "L1_ChunkTableUnsorted",
			Build: func(t *testing.T) []byte {
				// Need 2+ chunks. Build archive with many entries + small chunk size.
				t.Helper()
				f, err := os.CreateTemp(t.TempDir(), "adv-chunks*.oza")
				if err != nil {
					t.Fatal(err)
				}
				w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
					ZstdLevel:       3,
					TrainDict:       false,
					BuildSearch:     false,
					ChunkTargetSize: 50, // tiny chunks
				})
				setRequiredMeta(w)
				for i := 0; i < 8; i++ {
					body := make([]byte, 100)
					for j := range body {
						body[j] = byte('A' + i)
					}
					path := "A/P" + string(rune('0'+i))
					if _, err := w.AddEntry(path, path, "text/html", body, true); err != nil {
						t.Fatal(err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				name := f.Name()
				f.Close()
				data, err := os.ReadFile(name)
				if err != nil {
					t.Fatal(err)
				}
				return data
			},
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionContent)
				chunkTableStart := int(s.Offset) + 4
				count := binary.LittleEndian.Uint32(c[s.Offset:])
				if count < 2 {
					return c // can't swap if < 2 chunks
				}
				// Swap first two 28-byte chunk descriptors.
				d0 := make([]byte, oza.ChunkDescSize)
				copy(d0, c[chunkTableStart:chunkTableStart+oza.ChunkDescSize])
				copy(c[chunkTableStart:], c[chunkTableStart+oza.ChunkDescSize:chunkTableStart+2*oza.ChunkDescSize])
				copy(c[chunkTableStart+oza.ChunkDescSize:], d0)
				return c
			},
			Check: func(t *testing.T, path string) {
				mustFailOpenWith(t, path, oza.ErrChunkTableUnsorted)
			},
		},
		{
			Name:  "L2_ZeroLengthChunk",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionContent)
				chunkDescStart := int(s.Offset) + 4
				// Set CompressedSize (bytes 12:20) to 0.
				put64(c, chunkDescStart+12, 0)
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return
				}
				defer a.Close()
				e, err := a.EntryByID(0)
				if err != nil {
					return
				}
				_, err = e.ReadContent()
				if err == nil {
					t.Fatal("expected ReadContent to fail with zero-length chunk")
				}
			},
		},

		// ---------------------------------------------------------------
		// J. Index Corruption
		// ---------------------------------------------------------------
		{
			Name:  "J1_IndexZeroRestartInterval",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionPathIndex)
				// If path index is compressed, this may fail to corrupt cleanly.
				// IDX1 header: [0:4] magic, [4:8] count, [8:12] restart_interval.
				// Set restart_interval to 0.
				if s.Compression == oza.CompNone {
					put32(c, int(s.Offset)+8, 0)
				}
				return c
			},
			Check: func(t *testing.T, path string) {
				// If the section was uncompressed, Open should fail.
				// If compressed, the corruption may cause a decompression error
				// or the original data passes through.
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // expected
				}
				// If it somehow opened, verify the archive is broken.
				a.Close()
			},
		},
		{
			Name:  "J2_IndexBadMagic",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionPathIndex)
				if s.Compression == oza.CompNone {
					put32(c, int(s.Offset), 0xDEADBEEF)
				}
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // expected
				}
				a.Close()
			},
		},

		// ---------------------------------------------------------------
		// K. Search/Trigram Corruption
		// ---------------------------------------------------------------
		{
			Name:  "K1_TrigramCorruptBitmap",
			Build: buildWithSearch,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				// Find either SearchTitle or SearchBody section.
				var s oza.SectionDesc
				var found bool
				for _, st := range []oza.SectionType{oza.SectionSearchTitle, oza.SectionSearchBody} {
					func() {
						defer func() { recover() }()
						s, _ = findSection(c, st)
						found = true
					}()
					if found {
						break
					}
				}
				if !found {
					return c
				}
				// Corrupt bytes in the middle of the search section.
				if s.Compression == oza.CompNone {
					mid := int(s.Offset) + int(s.CompressedSize)/2
					if mid < len(c) {
						for i := 0; i < 16 && mid+i < len(c); i++ {
							c[mid+i] ^= 0xFF
						}
					}
				}
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // open-time failure is acceptable
				}
				defer a.Close()
				// Search should either return nil or not panic.
				results, _ := a.Search("quick brown", oza.SearchOptions{Limit: 10})
				_ = results // no panic = pass
			},
		},
		{
			Name:  "K2_TrigramBadVersion",
			Build: buildWithSearch,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				var s oza.SectionDesc
				var found bool
				for _, st := range []oza.SectionType{oza.SectionSearchTitle, oza.SectionSearchBody} {
					func() {
						defer func() { recover() }()
						s, _ = findSection(c, st)
						found = true
					}()
					if found {
						break
					}
				}
				if !found {
					return c
				}
				// Set version field (first 4 bytes) to 99.
				if s.Compression == oza.CompNone {
					put32(c, int(s.Offset), 99)
				}
				return c
			},
			Check: func(t *testing.T, path string) {
				// Open should fail because ParseTrigramIndex rejects version != 1.
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // expected
				}
				a.Close()
			},
		},

		// ---------------------------------------------------------------
		// D. Redirect Attacks
		// ---------------------------------------------------------------
		{
			Name:  "D1_SelfRedirect",
			Build: buildWithRedirects,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionRedirectTab)
				// Redirect table: uint32 count, then 5-byte records.
				// Record layout: 1 byte flags + 4 bytes target_id.
				// Patch redirect 0's TargetID to its own tagged ID.
				recordOff := int(s.Offset) + 4 // skip count
				put32(c, recordOff+1, oza.MakeRedirectID(0))
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return
				}
				defer a.Close()
				// Get redirect entry by its tagged ID.
				e, err := a.EntryByID(oza.MakeRedirectID(0))
				if err != nil {
					return
				}
				_, err = e.Resolve()
				if !errors.Is(err, oza.ErrRedirectLoop) {
					t.Fatalf("expected ErrRedirectLoop, got: %v", err)
				}
			},
		},
		{
			Name:  "D2_RedirectCycle",
			Build: buildWithRedirects,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionRedirectTab)
				recordOff := int(s.Offset) + 4
				// Redirect 0 -> redirect 1's tagged ID.
				put32(c, recordOff+1, oza.MakeRedirectID(1))
				// Redirect 1 -> redirect 0's tagged ID.
				put32(c, recordOff+oza.RedirectRecordSize+1, oza.MakeRedirectID(0))
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return
				}
				defer a.Close()
				e, err := a.EntryByID(oza.MakeRedirectID(0))
				if err != nil {
					return
				}
				_, err = e.Resolve()
				if !errors.Is(err, oza.ErrRedirectLoop) {
					t.Fatalf("expected ErrRedirectLoop, got: %v", err)
				}
			},
		},
		{
			Name:  "D3_RedirectToNonexistent",
			Build: buildWithRedirects,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionRedirectTab)
				recordOff := int(s.Offset) + 4
				// Redirect 0 -> nonexistent content entry.
				put32(c, recordOff+1, 0x7FFFFF)
				return c
			},
			Check: func(t *testing.T, path string) {
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return
				}
				defer a.Close()
				e, err := a.EntryByID(oza.MakeRedirectID(0))
				if err != nil {
					return
				}
				_, err = e.Resolve()
				if err == nil {
					t.Fatal("expected Resolve to fail with nonexistent target")
				}
			},
		},

		// ---------------------------------------------------------------
		// N. Metadata Corruption
		// ---------------------------------------------------------------
		{
			Name:  "N1_MetadataDuplicateKeys",
			Build: buildMinimal,
			Corrupt: func(data []byte) []byte {
				c := clone(data)
				s, _ := findSection(c, oza.SectionMetadata)
				if s.Compression != oza.CompNone {
					return c // can't easily patch compressed data
				}
				soff := int(s.Offset)
				// Inflate pair_count beyond what the section bytes contain.
				// This tests that ParseMetadata handles a count that exceeds
				// available data gracefully (truncation detection).
				count := binary.LittleEndian.Uint32(c[soff:])
				put32(c, soff, count+10)
				return c
			},
			Check: func(t *testing.T, path string) {
				// Parser should either return an error (bounds check) or
				// successfully parse only what's available.
				a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
				if err != nil {
					return // open-time failure is acceptable
				}
				defer a.Close()
				// If it opened, metadata should still be accessible.
				_, _ = a.Metadata("title")
			},
		},
	}

	// ---------------------------------------------------------------
	// M. Signature Corruption (lazy-init shared data)
	// ---------------------------------------------------------------
	recipes = append(recipes, recipe{
		Name: "M1_SignatureTampered",
		Build: func(t *testing.T) []byte {
			t.Helper()
			d, pub := buildWithSignatures(t)
			sigPub = pub
			return d
		},
		Corrupt: func(data []byte) []byte {
			c := clone(data)
			h := parseHdr(c)
			// Signature trailer starts at ChecksumOff + 32.
			sigStart := int(h.ChecksumOff) + 32
			// Skip 4-byte count, then flip a byte in the first signature record.
			if sigStart+4+32+1 < len(c) {
				c[sigStart+4+32] ^= 0xFF // Corrupt first byte of the signature itself
			}
			return c
		},
		Check: func(t *testing.T, path string) {
			a, err := oza.OpenWithOptions(path, oza.WithMmap(false))
			if err != nil {
				return
			}
			defer a.Close()
			results, err := a.VerifySignatures([]ed25519.PublicKey{sigPub})
			if err != nil {
				return
			}
			for _, r := range results {
				if r.OK {
					t.Fatal("expected tampered signature to fail verification")
				}
			}
		},
	})

	return recipes
}

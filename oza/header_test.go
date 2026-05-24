package oza

import (
	"testing"
)

func makeTestHeader() Header {
	var uuid [16]byte
	for i := range uuid {
		uuid[i] = byte(i + 1)
	}
	return Header{
		Magic:             Magic,
		MajorVersion:      MajorVersion,
		MinorVersion:      MinorVersion,
		UUID:              uuid,
		SectionCount:      7,
		EntryCount:        1000,
		ContentSize:       0xDEADBEEF,
		SectionTableOff:   128,
		ChecksumOff:       0xCAFEBABE,
		Flags:             FlagHasSearch | FlagHasChrome,
		RedirectCount:     42,
		FrontArticleCount: 500,
	}
}

func TestParseHeader(t *testing.T) {
	h := makeTestHeader()
	b := h.Marshal()

	got, err := ParseHeader(b[:])
	if err != nil {
		t.Fatalf("ParseHeader: %v", err)
	}

	if got.Magic != h.Magic {
		t.Errorf("Magic: got 0x%08X, want 0x%08X", got.Magic, h.Magic)
	}
	if got.MajorVersion != h.MajorVersion {
		t.Errorf("MajorVersion: got %d, want %d", got.MajorVersion, h.MajorVersion)
	}
	if got.MinorVersion != h.MinorVersion {
		t.Errorf("MinorVersion: got %d, want %d", got.MinorVersion, h.MinorVersion)
	}
	if got.UUID != h.UUID {
		t.Errorf("UUID: got %v, want %v", got.UUID, h.UUID)
	}
	if got.SectionCount != h.SectionCount {
		t.Errorf("SectionCount: got %d, want %d", got.SectionCount, h.SectionCount)
	}
	if got.EntryCount != h.EntryCount {
		t.Errorf("EntryCount: got %d, want %d", got.EntryCount, h.EntryCount)
	}
	if got.ContentSize != h.ContentSize {
		t.Errorf("ContentSize: got %d, want %d", got.ContentSize, h.ContentSize)
	}
	if got.SectionTableOff != h.SectionTableOff {
		t.Errorf("SectionTableOff: got %d, want %d", got.SectionTableOff, h.SectionTableOff)
	}
	if got.ChecksumOff != h.ChecksumOff {
		t.Errorf("ChecksumOff: got %d, want %d", got.ChecksumOff, h.ChecksumOff)
	}
	if got.Flags != h.Flags {
		t.Errorf("Flags: got %d, want %d", got.Flags, h.Flags)
	}
	if got.RedirectCount != h.RedirectCount {
		t.Errorf("RedirectCount: got %d, want %d", got.RedirectCount, h.RedirectCount)
	}
	if got.FrontArticleCount != h.FrontArticleCount {
		t.Errorf("FrontArticleCount: got %d, want %d", got.FrontArticleCount, h.FrontArticleCount)
	}
	if !got.HasSearch() {
		t.Error("HasSearch() should be true")
	}
	if !got.HasChrome() {
		t.Error("HasChrome() should be true")
	}
	if got.HasSignatures() {
		t.Error("HasSignatures() should be false")
	}
}

func TestParseHeaderInvalidMagic(t *testing.T) {
	h := makeTestHeader()
	b := h.Marshal()
	b[0] = 0xFF // corrupt magic

	_, err := ParseHeader(b[:])
	if err != ErrInvalidMagic {
		t.Errorf("expected ErrInvalidMagic, got %v", err)
	}
}

func TestParseHeaderShortData(t *testing.T) {
	data := make([]byte, 32) // too short
	_, err := ParseHeader(data)
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestParseHeaderUnsupportedVersion(t *testing.T) {
	h := makeTestHeader()
	h.MajorVersion = 99
	b := h.Marshal()

	_, err := ParseHeader(b[:])
	if err != ErrUnsupportedVersion {
		t.Errorf("expected ErrUnsupportedVersion, got %v", err)
	}
}

// TestParseHeader_RejectsVersionZero pins the SPEC §3.2 contract that
// version 1 is the only currently-defined major. MajorVersion=0 is not a
// valid in-flight pre-v1 marker; readers MUST reject it just like a future
// major version.
func TestParseHeader_RejectsVersionZero(t *testing.T) {
	h := makeTestHeader()
	h.MajorVersion = 0
	b := h.Marshal()

	_, err := ParseHeader(b[:])
	if err != ErrUnsupportedVersion {
		t.Errorf("expected ErrUnsupportedVersion for MajorVersion=0, got %v", err)
	}
}

// TestParseHeader_AcceptsFutureMinor pins the forward-compatibility contract
// for minor versions: a v1.X reader MUST accept v1.Y headers (Y > X). Major
// version is the breaking-change axis; minor is additive.
func TestParseHeader_AcceptsFutureMinor(t *testing.T) {
	h := makeTestHeader()
	h.MinorVersion = MinorVersion + 5 // pretend we're a future minor
	b := h.Marshal()

	got, err := ParseHeader(b[:])
	if err != nil {
		t.Fatalf("unexpected error parsing future minor version: %v", err)
	}
	if got.MinorVersion != MinorVersion+5 {
		t.Errorf("MinorVersion = %d, want %d", got.MinorVersion, MinorVersion+5)
	}
}

// TestParseHeader_IgnoresUnknownFlags pins the forward-compatibility contract
// for the flags field: SPEC §3.2 says "readers MUST ignore unknown bits". A
// header with bit 7 set (currently undefined) must parse cleanly and the
// known accessors must still report the right values.
func TestParseHeader_IgnoresUnknownFlags(t *testing.T) {
	h := makeTestHeader()
	const unknownBit uint32 = 1 << 7
	h.Flags = FlagHasSearch | unknownBit
	b := h.Marshal()

	got, err := ParseHeader(b[:])
	if err != nil {
		t.Fatalf("unexpected error parsing header with unknown flag bit: %v", err)
	}
	if !got.HasSearch() {
		t.Error("HasSearch() should be true")
	}
	if got.HasChrome() {
		t.Error("HasChrome() should be false (bit 1 not set)")
	}
	if got.HasSignatures() {
		t.Error("HasSignatures() should be false (bit 2 not set)")
	}
	if got.Flags&unknownBit == 0 {
		t.Error("unknown flag bit was not preserved in parsed header")
	}
}

func FuzzParseHeader(f *testing.F) {
	h := makeTestHeader()
	b := h.Marshal()
	f.Add(b[:])

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic
		ParseHeader(data)
	})
}

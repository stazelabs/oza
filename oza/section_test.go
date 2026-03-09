package oza

import (
	"testing"
)

func makeTestSectionDesc() SectionDesc {
	var sha [32]byte
	for i := range sha {
		sha[i] = byte(i)
	}
	return SectionDesc{
		Type:             SectionContent,
		Flags:            0x03,
		Offset:           0x1000,
		CompressedSize:   512,
		UncompressedSize: 2048,
		Compression:      CompZstd,
		DictID:           42,
		SHA256:           sha,
	}
}

func TestParseSectionDescRoundTrip(t *testing.T) {
	s := makeTestSectionDesc()
	b := s.Marshal()

	got, err := ParseSectionDesc(b[:])
	if err != nil {
		t.Fatalf("ParseSectionDesc: %v", err)
	}

	if got.Type != s.Type {
		t.Errorf("Type: got %d, want %d", got.Type, s.Type)
	}
	if got.Flags != s.Flags {
		t.Errorf("Flags: got %d, want %d", got.Flags, s.Flags)
	}
	if got.Offset != s.Offset {
		t.Errorf("Offset: got %d, want %d", got.Offset, s.Offset)
	}
	if got.CompressedSize != s.CompressedSize {
		t.Errorf("CompressedSize: got %d, want %d", got.CompressedSize, s.CompressedSize)
	}
	if got.UncompressedSize != s.UncompressedSize {
		t.Errorf("UncompressedSize: got %d, want %d", got.UncompressedSize, s.UncompressedSize)
	}
	if got.Compression != s.Compression {
		t.Errorf("Compression: got %d, want %d", got.Compression, s.Compression)
	}
	if got.DictID != s.DictID {
		t.Errorf("DictID: got %d, want %d", got.DictID, s.DictID)
	}
	if got.SHA256 != s.SHA256 {
		t.Errorf("SHA256 mismatch")
	}
}

func TestParseSectionDescShortData(t *testing.T) {
	data := make([]byte, 40) // too short
	_, err := ParseSectionDesc(data)
	if err == nil {
		t.Error("expected error for short data")
	}
}

func TestParseSectionDescUnknownType(t *testing.T) {
	s := makeTestSectionDesc()
	s.Type = SectionType(0xFFFF) // unknown type
	b := s.Marshal()

	got, err := ParseSectionDesc(b[:])
	if err != nil {
		t.Errorf("unknown section type should parse without error: %v", err)
	}
	if got.Type != SectionType(0xFFFF) {
		t.Errorf("Type: got %d, want %d", got.Type, SectionType(0xFFFF))
	}
}

func TestParseSectionTable(t *testing.T) {
	s1 := makeTestSectionDesc()
	s2 := makeTestSectionDesc()
	s2.Type = SectionMetadata
	s2.Offset = 0x2000

	b1 := s1.Marshal()
	b2 := s2.Marshal()
	data := append(b1[:], b2[:]...)

	descs, err := ParseSectionTable(data, 2)
	if err != nil {
		t.Fatalf("ParseSectionTable: %v", err)
	}
	if len(descs) != 2 {
		t.Fatalf("got %d sections, want 2", len(descs))
	}
	if descs[0].Type != s1.Type {
		t.Errorf("section 0 type: got %d, want %d", descs[0].Type, s1.Type)
	}
	if descs[1].Type != s2.Type {
		t.Errorf("section 1 type: got %d, want %d", descs[1].Type, s2.Type)
	}
}

func FuzzParseSectionDesc(f *testing.F) {
	s := makeTestSectionDesc()
	b := s.Marshal()
	f.Add(b[:])

	f.Fuzz(func(t *testing.T, data []byte) {
		ParseSectionDesc(data)
	})
}

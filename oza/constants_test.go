package oza

import "testing"

func TestMagicBytes(t *testing.T) {
	// Magic = 0x01415A4F stored little-endian produces on-disk bytes "OZA\x01"
	// (same convention as ZIM: 0x044D495A LE -> "ZIM\x04")
	const want = uint32(0x01415A4F)
	if Magic != want {
		t.Errorf("Magic = 0x%08X, want 0x%08X", Magic, want)
	}
}

func TestMagicString(t *testing.T) {
	// Verify on-disk little-endian layout spells "OZA\x01"
	m := uint32(Magic)
	b := [4]byte{byte(m), byte(m >> 8), byte(m >> 16), byte(m >> 24)}
	if b[0] != 'O' || b[1] != 'Z' || b[2] != 'A' || b[3] != 0x01 {
		t.Errorf("Magic on-disk bytes = %v, want [O Z A 0x01]", b)
	}
}

func TestSizes(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"HeaderSize", HeaderSize, 64},
		{"SectionSize", SectionSize, 80},
		{"EntryTableHeaderSize", EntryTableHeaderSize, 8},
		{"ChunkDescSize", ChunkDescSize, 28},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestVersions(t *testing.T) {
	if MajorVersion != 1 {
		t.Errorf("MajorVersion = %d, want 1", MajorVersion)
	}
	if MinorVersion != 0 {
		t.Errorf("MinorVersion = %d, want 0", MinorVersion)
	}
}

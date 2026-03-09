package oza

import (
	"encoding/binary"
	"fmt"
)

// SectionDesc is the fixed 80-byte section descriptor.
//
// Binary layout (all integers little-endian except SHA256 which is raw bytes):
//
//	[0:4]   Type             SectionType (uint32)
//	[4:8]   Flags            uint32
//	[8:16]  Offset           uint64
//	[16:24] CompressedSize   uint64
//	[24:32] UncompressedSize uint64
//	[32]    Compression      uint8
//	[33:36] reserved         (3 bytes)
//	[36:40] DictID           uint32
//	[40:48] reserved         (8 bytes)
//	[48:80] SHA256           [32]byte
type SectionDesc struct {
	Type             SectionType
	Flags            uint32
	Offset           uint64
	CompressedSize   uint64
	UncompressedSize uint64
	Compression      uint8
	DictID           uint32
	SHA256           [32]byte
}

// ParseSectionDesc parses a single 80-byte section descriptor from data.
func ParseSectionDesc(data []byte) (SectionDesc, error) {
	if len(data) < SectionSize {
		return SectionDesc{}, fmt.Errorf("oza: section descriptor too short: %d bytes, need %d", len(data), SectionSize)
	}

	var s SectionDesc
	s.Type = SectionType(binary.LittleEndian.Uint32(data[0:4]))
	s.Flags = binary.LittleEndian.Uint32(data[4:8])
	s.Offset = binary.LittleEndian.Uint64(data[8:16])
	s.CompressedSize = binary.LittleEndian.Uint64(data[16:24])
	s.UncompressedSize = binary.LittleEndian.Uint64(data[24:32])
	s.Compression = data[32]
	// [33:36] reserved
	s.DictID = binary.LittleEndian.Uint32(data[36:40])
	// [40:48] reserved
	copy(s.SHA256[:], data[48:80])

	return s, nil
}

// Marshal serializes s to a fixed 80-byte array.
func (s SectionDesc) Marshal() [SectionSize]byte {
	var b [SectionSize]byte
	binary.LittleEndian.PutUint32(b[0:4], uint32(s.Type))
	binary.LittleEndian.PutUint32(b[4:8], s.Flags)
	binary.LittleEndian.PutUint64(b[8:16], s.Offset)
	binary.LittleEndian.PutUint64(b[16:24], s.CompressedSize)
	binary.LittleEndian.PutUint64(b[24:32], s.UncompressedSize)
	b[32] = s.Compression
	// [33:36] reserved, left zero
	binary.LittleEndian.PutUint32(b[36:40], s.DictID)
	// [40:48] reserved, left zero
	copy(b[48:80], s.SHA256[:])
	return b
}

// ParseSectionTable parses count section descriptors from data.
func ParseSectionTable(data []byte, count uint32) ([]SectionDesc, error) {
	need := int(count) * SectionSize
	if len(data) < need {
		return nil, fmt.Errorf("oza: section table too short: %d bytes for %d sections (need %d)", len(data), count, need)
	}

	descs := make([]SectionDesc, count)
	for i := range descs {
		off := i * SectionSize
		d, err := ParseSectionDesc(data[off : off+SectionSize])
		if err != nil {
			return nil, fmt.Errorf("oza: section %d: %w", i, err)
		}
		descs[i] = d
	}
	return descs, nil
}

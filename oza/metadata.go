package oza

import (
	"encoding/binary"
	"fmt"
)

// RequiredMetadataKeys lists the metadata keys that must be present in every OZA archive.
var RequiredMetadataKeys = []string{"title", "language", "creator", "date", "source"}

// ParseMetadata parses key-value metadata pairs from data.
//
// Format: uint32 pair_count, per pair: uint16 key_len + key + uint32 val_len + value.
func ParseMetadata(data []byte) (map[string][]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("oza: metadata too short")
	}

	count := int(binary.LittleEndian.Uint32(data[0:4]))
	m := make(map[string][]byte)
	off := 4

	for i := range count {
		// Read key
		if off+2 > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (key length)", i)
		}
		klen := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2

		if off+klen > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (key bytes)", i)
		}
		key := string(data[off : off+klen])
		off += klen

		// Read value
		if off+4 > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (value length)", i)
		}
		vlen := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4

		if off+vlen > len(data) {
			return nil, fmt.Errorf("oza: metadata truncated at pair %d (value bytes)", i)
		}
		val := make([]byte, vlen)
		copy(val, data[off:off+vlen])
		off += vlen

		m[key] = val
	}

	return m, nil
}

// MarshalMetadata serializes key-value metadata pairs.
func MarshalMetadata(pairs map[string][]byte) ([]byte, error) {
	// Calculate size: 4 (count) + sum of (2 + len(key) + 4 + len(val)) per pair.
	size := 4
	for k, v := range pairs {
		size += 2 + len(k) + 4 + len(v)
	}

	buf := make([]byte, size)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(len(pairs)))
	off := 4

	for k, v := range pairs {
		binary.LittleEndian.PutUint16(buf[off:off+2], uint16(len(k)))
		off += 2
		copy(buf[off:], k)
		off += len(k)

		binary.LittleEndian.PutUint32(buf[off:off+4], uint32(len(v)))
		off += 4
		copy(buf[off:], v)
		off += len(v)
	}

	return buf, nil
}

// ValidateMetadata checks that all required metadata keys are present.
func ValidateMetadata(m map[string][]byte) error {
	for _, key := range RequiredMetadataKeys {
		if _, ok := m[key]; !ok {
			return fmt.Errorf("%w: %q", ErrMissingMetadata, key)
		}
	}
	return nil
}

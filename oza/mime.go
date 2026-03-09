package oza

import (
	"encoding/binary"
	"fmt"
)

// Standard MIME type index conventions.
const (
	MIMEIndexHTML = 0
	MIMEIndexCSS  = 1
	MIMEIndexJS   = 2
)

// Standard MIME types that must occupy the first three indices.
var standardMIMETypes = [3]string{
	"text/html",
	"text/css",
	"application/javascript",
}

// ParseMIMETable parses a MIME type table from data.
//
// Format: uint16 count, then per type: uint16 len + len bytes.
func ParseMIMETable(data []byte) ([]string, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("oza: MIME table too short")
	}

	count := int(binary.LittleEndian.Uint16(data[0:2]))
	types := make([]string, 0, count)
	off := 2

	for i := range count {
		if off+2 > len(data) {
			return nil, fmt.Errorf("oza: MIME table truncated at entry %d", i)
		}
		slen := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2
		if off+slen > len(data) {
			return nil, fmt.Errorf("oza: MIME table entry %d truncated (need %d bytes)", i, slen)
		}
		types = append(types, string(data[off:off+slen]))
		off += slen
	}

	// Validate index 0/1/2 convention if at least 3 entries present.
	if len(types) >= 3 {
		for i, want := range standardMIMETypes {
			if types[i] != want {
				return nil, fmt.Errorf("oza: MIME index %d must be %q, got %q", i, want, types[i])
			}
		}
	}

	return types, nil
}

// MarshalMIMETable serializes a slice of MIME type strings.
//
// Enforces that if types has at least 3 entries, indices 0, 1, 2 are
// text/html, text/css, application/javascript respectively.
func MarshalMIMETable(types []string) ([]byte, error) {
	if len(types) >= 3 {
		for i, want := range standardMIMETypes {
			if types[i] != want {
				return nil, fmt.Errorf("oza: MIME index %d must be %q, got %q", i, want, types[i])
			}
		}
	}

	// Calculate size: 2 (count) + sum of (2 + len(s)) per entry.
	size := 2
	for _, t := range types {
		size += 2 + len(t)
	}

	buf := make([]byte, size)
	binary.LittleEndian.PutUint16(buf[0:2], uint16(len(types)))
	off := 2
	for _, t := range types {
		binary.LittleEndian.PutUint16(buf[off:off+2], uint16(len(t)))
		off += 2
		copy(buf[off:], t)
		off += len(t)
	}

	return buf, nil
}

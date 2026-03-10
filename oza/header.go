package oza

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Header is the fixed 128-byte OZA file header.
type Header struct {
	Magic             uint32
	MajorVersion      uint16
	MinorVersion      uint16
	UUID              [16]byte
	SectionCount      uint32
	EntryCount        uint32
	ContentSize       uint64
	SectionTableOff   uint64
	ChecksumOff       uint64
	Flags             uint32
	RedirectCount     uint32
	FrontArticleCount uint32
	Reserved          [60]byte
}

// ParseHeader parses a 128-byte OZA header from data.
func ParseHeader(data []byte) (Header, error) {
	if len(data) < HeaderSize {
		return Header{}, fmt.Errorf("oza: header too short: %d bytes, need %d", len(data), HeaderSize)
	}

	var h Header
	h.Magic = binary.LittleEndian.Uint32(data[0:4])
	if h.Magic != Magic {
		return Header{}, ErrInvalidMagic
	}

	h.MajorVersion = binary.LittleEndian.Uint16(data[4:6])
	if h.MajorVersion > MajorVersion {
		return Header{}, ErrUnsupportedVersion
	}

	h.MinorVersion = binary.LittleEndian.Uint16(data[6:8])
	copy(h.UUID[:], data[8:24])
	h.SectionCount = binary.LittleEndian.Uint32(data[24:28])
	h.EntryCount = binary.LittleEndian.Uint32(data[28:32])
	h.ContentSize = binary.LittleEndian.Uint64(data[32:40])
	h.SectionTableOff = binary.LittleEndian.Uint64(data[40:48])
	h.ChecksumOff = binary.LittleEndian.Uint64(data[48:56])
	h.Flags = binary.LittleEndian.Uint32(data[56:60])
	h.RedirectCount = binary.LittleEndian.Uint32(data[60:64])
	h.FrontArticleCount = binary.LittleEndian.Uint32(data[64:68])
	copy(h.Reserved[:], data[68:128])

	return h, nil
}

// Marshal serializes h to a fixed 128-byte array.
func (h Header) Marshal() [HeaderSize]byte {
	var b [HeaderSize]byte
	binary.LittleEndian.PutUint32(b[0:4], h.Magic)
	binary.LittleEndian.PutUint16(b[4:6], h.MajorVersion)
	binary.LittleEndian.PutUint16(b[6:8], h.MinorVersion)
	copy(b[8:24], h.UUID[:])
	binary.LittleEndian.PutUint32(b[24:28], h.SectionCount)
	binary.LittleEndian.PutUint32(b[28:32], h.EntryCount)
	binary.LittleEndian.PutUint64(b[32:40], h.ContentSize)
	binary.LittleEndian.PutUint64(b[40:48], h.SectionTableOff)
	binary.LittleEndian.PutUint64(b[48:56], h.ChecksumOff)
	binary.LittleEndian.PutUint32(b[56:60], h.Flags)
	binary.LittleEndian.PutUint32(b[60:64], h.RedirectCount)
	binary.LittleEndian.PutUint32(b[64:68], h.FrontArticleCount)
	// Bytes 68-127 are reserved and remain zero from the var declaration.
	return b
}

// HasSearch reports whether the archive contains a search index.
func (h Header) HasSearch() bool { return h.Flags&FlagHasSearch != 0 }

// HasChrome reports whether the archive contains a chrome section.
func (h Header) HasChrome() bool { return h.Flags&FlagHasChrome != 0 }

// HasSignatures reports whether the archive contains signatures.
func (h Header) HasSignatures() bool { return h.Flags&FlagHasSignatures != 0 }

// ReadHeader reads and parses a Header from r.
func ReadHeader(r io.Reader) (Header, error) {
	var buf [HeaderSize]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return Header{}, fmt.Errorf("oza: reading header: %w", err)
	}
	return ParseHeader(buf[:])
}

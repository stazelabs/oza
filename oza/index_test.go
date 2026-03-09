package oza

import (
	"encoding/binary"
	"testing"
)

// makeMinimalIndex builds a minimal valid IDX1 index section with a single
// record whose key is composed of a single literal-only tuple.
func makeMinimalIndex(key string, entryID uint32) []byte {
	// String table: empty (no shared strings needed for literal-only tuples).
	stringTableCount := uint32(0)
	stringTableSize := uint32(0)

	restartInterval := uint32(64)
	restartCount := uint32(1)

	// Header: 24 bytes + restart offsets (4 bytes each).
	headerSize := 24 + int(restartCount)*4

	// String table follows restart offsets.
	tableStart := headerSize

	// Record starts after string table.
	recordStart := tableStart + int(stringTableSize)

	// Record format (restart entry): uint32 entryID, uint8 tokenCount, then tuples.
	// Tuple: uint16 tableIdx (0xFFFF = no table), uint16 literalLen, [literalLen] bytes.
	tokenCount := 1
	tupleSize := 4 + len(key)
	recordSize := 5 + tupleSize // 4 (entryID) + 1 (tokenCount) + tuple

	totalSize := recordStart + recordSize
	buf := make([]byte, totalSize)

	// IDX1 header
	binary.LittleEndian.PutUint32(buf[0:4], IndexV1Magic)
	binary.LittleEndian.PutUint32(buf[4:8], 1) // count
	binary.LittleEndian.PutUint32(buf[8:12], restartInterval)
	binary.LittleEndian.PutUint32(buf[12:16], restartCount)
	binary.LittleEndian.PutUint32(buf[16:20], stringTableCount)
	binary.LittleEndian.PutUint32(buf[20:24], stringTableSize)

	// Restart offset: points to recordStart.
	binary.LittleEndian.PutUint32(buf[24:28], uint32(recordStart))

	// Record
	off := recordStart
	binary.LittleEndian.PutUint32(buf[off:], entryID)
	buf[off+4] = byte(tokenCount)
	off += 5

	// Tuple: tableIdx=0xFFFF (literal only), literalLen, literal bytes.
	binary.LittleEndian.PutUint16(buf[off:], 0xFFFF)
	binary.LittleEndian.PutUint16(buf[off+2:], uint16(len(key)))
	off += 4
	copy(buf[off:], key)

	return buf
}

func FuzzParseIndex(f *testing.F) {
	// Seed: minimal valid index with one entry.
	seed := makeMinimalIndex("A/Hello_World", 0)
	f.Add(seed)

	// Seed: empty key.
	f.Add(makeMinimalIndex("", 42))

	f.Fuzz(func(t *testing.T, data []byte) {
		idx, err := ParseIndex(data)
		if err != nil {
			return
		}
		// Exercise record access and iteration; must not panic.
		n := idx.Count()
		if n > 0 {
			idx.Record(0)
			if n > 1 {
				idx.Record(n - 1)
			}
		}
		idx.ForEach(func(entryID uint32, key string) {})
		idx.Search("test")
	})
}

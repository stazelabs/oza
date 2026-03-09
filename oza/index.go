package oza

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// Index is a parsed path or title index section using front-coded blocks
// with a shared string table (IDX1 format).
//
// Wire format (IDX1):
//
//	uint32   magic (0x49445831 = "IDX1")
//	uint32   count
//	uint32   restart_interval
//	uint32   restart_count
//	uint32   string_table_count
//	uint32   string_table_size
//	uint32[restart_count]  restart_offsets (byte offset from section start)
//	STRING TABLE: per entry uint16 len, [len] bytes
//	RECORDS: front-coded with token-encoded keys
type Index struct {
	data            []byte
	count           uint32
	restartInterval uint32
	restartCount    uint32
	restartOffsets  []uint32
	stringTable     []string
}

// ParseIndex parses an index section (path or title).
func ParseIndex(data []byte) (*Index, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("oza: index section too short")
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != IndexV1Magic {
		return nil, fmt.Errorf("oza: index section bad magic 0x%08x, expected 0x%08x", magic, IndexV1Magic)
	}

	count := binary.LittleEndian.Uint32(data[4:8])
	interval := binary.LittleEndian.Uint32(data[8:12])
	if count > 0 && interval == 0 {
		return nil, fmt.Errorf("oza: index restart_interval is zero with non-empty index")
	}
	restartCount := binary.LittleEndian.Uint32(data[12:16])
	tableCount := binary.LittleEndian.Uint32(data[16:20])
	tableSize := binary.LittleEndian.Uint32(data[20:24])

	offsetsEnd := 24 + int(restartCount)*4
	if len(data) < offsetsEnd {
		return nil, fmt.Errorf("oza: index too short for %d restart offsets", restartCount)
	}
	restartOffsets := make([]uint32, restartCount)
	for i := range restartOffsets {
		restartOffsets[i] = binary.LittleEndian.Uint32(data[24+i*4:])
	}

	tableStart := offsetsEnd
	tableEnd := tableStart + int(tableSize)
	if len(data) < tableEnd {
		return nil, fmt.Errorf("oza: index too short for string table")
	}
	stringTable, err := parseStringTable(data[tableStart:tableEnd], int(tableCount))
	if err != nil {
		return nil, err
	}

	return &Index{
		data:            data,
		count:           count,
		restartInterval: interval,
		restartCount:    restartCount,
		restartOffsets:  restartOffsets,
		stringTable:     stringTable,
	}, nil
}

// parseStringTable reads tableCount length-prefixed strings from data.
func parseStringTable(data []byte, count int) ([]string, error) {
	if count > len(data)/2 {
		return nil, fmt.Errorf("oza: string table count %d too large for %d-byte table", count, len(data))
	}
	table := make([]string, count)
	off := 0
	for i := 0; i < count; i++ {
		if off+2 > len(data) {
			return nil, fmt.Errorf("oza: string table entry %d truncated", i)
		}
		slen := int(binary.LittleEndian.Uint16(data[off:]))
		off += 2
		if off+slen > len(data) {
			return nil, fmt.Errorf("oza: string table entry %d data truncated", i)
		}
		table[i] = string(data[off : off+slen])
		off += slen
	}
	return table, nil
}

// Count returns the number of records in the index.
func (idx *Index) Count() int { return int(idx.count) }

// Record returns the entryID and key string for the i-th record (0-based, in sorted order).
func (idx *Index) Record(i int) (entryID uint32, key string, err error) {
	if i < 0 || i >= int(idx.count) {
		return 0, "", fmt.Errorf("oza: index record %d out of range [0,%d)", i, idx.count)
	}

	blockIdx := i / int(idx.restartInterval)
	blockStart := i - (i % int(idx.restartInterval))

	if blockIdx >= int(idx.restartCount) {
		return 0, "", fmt.Errorf("oza: index block %d out of range", blockIdx)
	}

	off := int(idx.restartOffsets[blockIdx])
	prevKey := ""
	var sb strings.Builder

	for j := blockStart; j <= i; j++ {
		if off+5 > len(idx.data) {
			return 0, "", fmt.Errorf("oza: index record %d: offset out of bounds", j)
		}

		entryID = binary.LittleEndian.Uint32(idx.data[off:])

		if j == blockStart {
			tokenCount := int(idx.data[off+4])
			off += 5
			var err error
			key, off, err = idx.decodeTuples(&sb, off, tokenCount)
			if err != nil {
				return 0, "", fmt.Errorf("oza: index restart record %d: %w", j, err)
			}
		} else {
			if off+7 > len(idx.data) {
				return 0, "", fmt.Errorf("oza: index record %d header truncated", j)
			}
			prefixLen := int(binary.LittleEndian.Uint16(idx.data[off+4:]))
			tokenCount := int(idx.data[off+6])
			off += 7
			if prefixLen > len(prevKey) {
				return 0, "", fmt.Errorf("oza: index record %d prefixLen %d exceeds prev key len %d", j, prefixLen, len(prevKey))
			}
			suffix, newOff, err := idx.decodeTuples(&sb, off, tokenCount)
			if err != nil {
				return 0, "", fmt.Errorf("oza: index record %d: %w", j, err)
			}
			off = newOff
			key = prevKey[:prefixLen] + suffix
		}

		prevKey = key
	}

	return entryID, key, nil
}

// ForEachErr iterates all records sequentially, calling fn for each.
// It returns the first error encountered during decoding or returned by fn.
// This is O(N) — much faster than calling Record(i) in a loop.
func (idx *Index) ForEachErr(fn func(entryID uint32, key string) error) error {
	var sb strings.Builder
	for b := 0; b < int(idx.restartCount); b++ {
		off := int(idx.restartOffsets[b])
		prevKey := ""

		blockStart := b * int(idx.restartInterval)
		blockEnd := blockStart + int(idx.restartInterval)
		if blockEnd > int(idx.count) {
			blockEnd = int(idx.count)
		}

		for j := blockStart; j < blockEnd; j++ {
			if off+5 > len(idx.data) {
				return fmt.Errorf("oza: index record %d: offset out of bounds", j)
			}

			entryID := binary.LittleEndian.Uint32(idx.data[off:])
			var key string

			if j == blockStart {
				tokenCount := int(idx.data[off+4])
				off += 5
				var err error
				key, off, err = idx.decodeTuples(&sb, off, tokenCount)
				if err != nil {
					return fmt.Errorf("oza: index restart record %d: %w", j, err)
				}
			} else {
				if off+7 > len(idx.data) {
					return fmt.Errorf("oza: index record %d header truncated", j)
				}
				prefixLen := int(binary.LittleEndian.Uint16(idx.data[off+4:]))
				tokenCount := int(idx.data[off+6])
				off += 7
				if prefixLen > len(prevKey) {
					return fmt.Errorf("oza: index record %d: prefixLen %d exceeds prev key len %d", j, prefixLen, len(prevKey))
				}
				suffix, newOff, err := idx.decodeTuples(&sb, off, tokenCount)
				if err != nil {
					return fmt.Errorf("oza: index record %d: %w", j, err)
				}
				off = newOff
				key = prevKey[:prefixLen] + suffix
			}

			prevKey = key
			if err := fn(entryID, key); err != nil {
				return err
			}
		}
	}
	return nil
}

// ForEach iterates all records sequentially, calling fn for each.
// Decode errors cause iteration to stop silently; use ForEachErr if error
// propagation is required.
// This is O(N) — much faster than calling Record(i) in a loop which is O(N * restartInterval/2).
func (idx *Index) ForEach(fn func(entryID uint32, key string)) {
	var sb strings.Builder
	for b := 0; b < int(idx.restartCount); b++ {
		off := int(idx.restartOffsets[b])
		prevKey := ""

		blockStart := b * int(idx.restartInterval)
		blockEnd := blockStart + int(idx.restartInterval)
		if blockEnd > int(idx.count) {
			blockEnd = int(idx.count)
		}

		for j := blockStart; j < blockEnd; j++ {
			if off+5 > len(idx.data) {
				return
			}

			entryID := binary.LittleEndian.Uint32(idx.data[off:])
			var key string

			if j == blockStart {
				tokenCount := int(idx.data[off+4])
				off += 5
				var err error
				key, off, err = idx.decodeTuples(&sb, off, tokenCount)
				if err != nil {
					return
				}
			} else {
				if off+7 > len(idx.data) {
					return
				}
				prefixLen := int(binary.LittleEndian.Uint16(idx.data[off+4:]))
				tokenCount := int(idx.data[off+6])
				off += 7
				if prefixLen > len(prevKey) {
					return
				}
				suffix, newOff, err := idx.decodeTuples(&sb, off, tokenCount)
				if err != nil {
					return
				}
				off = newOff
				key = prevKey[:prefixLen] + suffix
			}

			prevKey = key
			fn(entryID, key)
		}
	}
}

// decodeTuples reads tokenCount (tableIdx, literalLen, literal) tuples and
// reconstructs the string they represent. The caller provides a reusable
// strings.Builder to avoid per-call allocations.
func (idx *Index) decodeTuples(sb *strings.Builder, off, tokenCount int) (string, int, error) {
	sb.Reset()
	for t := 0; t < tokenCount; t++ {
		if off+4 > len(idx.data) {
			return "", 0, fmt.Errorf("tuple %d truncated", t)
		}
		tableIdx := binary.LittleEndian.Uint16(idx.data[off:])
		literalLen := int(binary.LittleEndian.Uint16(idx.data[off+2:]))
		off += 4
		if tableIdx != 0xFFFF {
			if int(tableIdx) >= len(idx.stringTable) {
				return "", 0, fmt.Errorf("tuple %d: table index %d out of range", t, tableIdx)
			}
			sb.WriteString(idx.stringTable[tableIdx])
		}
		if literalLen > 0 {
			if off+literalLen > len(idx.data) {
				return "", 0, fmt.Errorf("tuple %d literal truncated", t)
			}
			sb.Write(idx.data[off : off+literalLen])
			off += literalLen
		}
	}
	return sb.String(), off, nil
}

// Search returns the entryID for the record whose key exactly matches the given
// key. Returns ErrNotFound if the key is not present.
func (idx *Index) Search(key string) (uint32, error) {
	n := int(idx.restartCount)
	if n == 0 {
		return 0, ErrNotFound
	}

	var sb strings.Builder

	// Binary search on restart keys to find the right block.
	blockIdx := sort.Search(n, func(i int) bool {
		_, k, err := idx.readRestartKey(&sb, i)
		if err != nil {
			return false
		}
		return k >= key
	})

	// If the restart key at blockIdx exactly matches, check it.
	if blockIdx < n {
		id, k, err := idx.readRestartKey(&sb, blockIdx)
		if err == nil && k == key {
			return id, nil
		}
		if k > key {
			blockIdx--
		}
	} else {
		blockIdx = n - 1
	}
	if blockIdx < 0 {
		return 0, ErrNotFound
	}

	// Linear scan within the block.
	blockStart := blockIdx * int(idx.restartInterval)
	blockEnd := blockStart + int(idx.restartInterval)
	if blockEnd > int(idx.count) {
		blockEnd = int(idx.count)
	}

	off := int(idx.restartOffsets[blockIdx])
	prevKey := ""

	for j := blockStart; j < blockEnd; j++ {
		if off+5 > len(idx.data) {
			break
		}

		entryID := binary.LittleEndian.Uint32(idx.data[off:])
		var k string

		if j == blockStart {
			tokenCount := int(idx.data[off+4])
			off += 5
			var err error
			k, off, err = idx.decodeTuples(&sb, off, tokenCount)
			if err != nil {
				break
			}
		} else {
			if off+7 > len(idx.data) {
				break
			}
			prefixLen := int(binary.LittleEndian.Uint16(idx.data[off+4:]))
			tokenCount := int(idx.data[off+6])
			off += 7
			if prefixLen > len(prevKey) {
				break
			}
			suffix, newOff, err := idx.decodeTuples(&sb, off, tokenCount)
			if err != nil {
				break
			}
			off = newOff
			k = prevKey[:prefixLen] + suffix
		}

		prevKey = k
		if k == key {
			return entryID, nil
		}
		if k > key {
			break
		}
	}

	return 0, ErrNotFound
}

// readRestartKey reads the full key at restart block i.
func (idx *Index) readRestartKey(sb *strings.Builder, i int) (uint32, string, error) {
	if i < 0 || i >= int(idx.restartCount) {
		return 0, "", fmt.Errorf("oza: restart block %d out of range", i)
	}
	off := int(idx.restartOffsets[i])
	if off+5 > len(idx.data) {
		return 0, "", fmt.Errorf("oza: restart block offset out of bounds")
	}
	entryID := binary.LittleEndian.Uint32(idx.data[off:])
	tokenCount := int(idx.data[off+4])
	off += 5
	key, _, err := idx.decodeTuples(sb, off, tokenCount)
	if err != nil {
		return 0, "", fmt.Errorf("oza: restart block: %w", err)
	}
	return entryID, key, nil
}

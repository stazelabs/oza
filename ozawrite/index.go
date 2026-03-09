package ozawrite

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/stazelabs/oza/oza"
)

// pathRecord associates an OZA entry ID with its path string.
type pathRecord struct {
	entryID uint32
	path    string
}

// titleRecord associates an OZA entry ID with its title string.
type titleRecord struct {
	entryID uint32
	title   string
}

// buildPathIndex serializes a path index section from records.
func buildPathIndex(records []pathRecord) []byte {
	sort.Slice(records, func(i, j int) bool { return records[i].path < records[j].path })

	keys := make([]string, len(records))
	ids := make([]uint32, len(records))
	for i, r := range records {
		keys[i] = r.path
		ids[i] = r.entryID
	}
	return serializeIndex(ids, keys, tokenizePath)
}

// buildTitleIndex serializes a title index section from records.
//
// Same format as path index but sorted by title.
func buildTitleIndex(records []titleRecord) []byte {
	sort.Slice(records, func(i, j int) bool { return records[i].title < records[j].title })

	keys := make([]string, len(records))
	ids := make([]uint32, len(records))
	for i, r := range records {
		keys[i] = r.title
		ids[i] = r.entryID
	}
	return serializeIndex(ids, keys, tokenizeTitle)
}

const restartInterval = 64

// serializeIndex encodes a front-coded index with a shared string table (IDX3).
func serializeIndex(ids []uint32, keys []string, tokenize func(string) []string) []byte {
	n := len(ids)
	if n == 0 {
		buf := make([]byte, 24)
		binary.LittleEndian.PutUint32(buf[0:4], oza.IndexV3Magic)
		return buf
	}

	// 1. Build string table from all keys.
	stb := newStringTableBuilder()
	allTokens := make([][]string, n)
	for i, key := range keys {
		tokens := tokenize(key)
		allTokens[i] = tokens
		stb.AddTokens(tokens)
	}
	table := stb.Build(3) // minimum frequency 3
	tableBytes := table.Serialize()

	restartCount := (n + restartInterval - 1) / restartInterval

	// Header: magic(4) + count(4) + interval(4) + restartCount(4) +
	//         tableCount(4) + tableSize(4) + offsets(4*restartCount)
	headerSize := 24 + 4*restartCount
	tableStart := headerSize
	recordStart := tableStart + len(tableBytes)

	// 2. Encode records.
	var recordBuf bytes.Buffer
	restartOffsets := make([]uint32, restartCount)
	prevKey := ""

	for i := 0; i < n; i++ {
		isRestart := i%restartInterval == 0
		if isRestart {
			restartOffsets[i/restartInterval] = uint32(recordStart + recordBuf.Len())
			prevKey = ""
		}

		key := keys[i]

		if isRestart {
			// Restart record: entryID(4) + tokenCount(1) + tuples
			var hdr [5]byte
			binary.LittleEndian.PutUint32(hdr[0:4], ids[i])
			hdr[4] = uint8(len(allTokens[i]))
			recordBuf.Write(hdr[:])
			writeTuples(&recordBuf, allTokens[i], table)
		} else {
			// Non-restart: entryID(4) + prefixLen(2) + tokenCount(1) + tuples
			prefixLen := sharedPrefix(prevKey, key)
			suffix := key[prefixLen:]
			suffixTokens := tokenize(suffix)
			var hdr [7]byte
			binary.LittleEndian.PutUint32(hdr[0:4], ids[i])
			binary.LittleEndian.PutUint16(hdr[4:6], uint16(prefixLen))
			hdr[6] = uint8(len(suffixTokens))
			recordBuf.Write(hdr[:])
			writeTuples(&recordBuf, suffixTokens, table)
		}

		prevKey = key
	}

	// 3. Assemble final buffer.
	totalSize := recordStart + recordBuf.Len()
	buf := make([]byte, totalSize)

	binary.LittleEndian.PutUint32(buf[0:4], oza.IndexV3Magic)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(n))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(restartInterval))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(restartCount))
	binary.LittleEndian.PutUint32(buf[16:20], uint32(table.Count()))
	binary.LittleEndian.PutUint32(buf[20:24], uint32(len(tableBytes)))
	for i, off := range restartOffsets {
		binary.LittleEndian.PutUint32(buf[24+i*4:], off)
	}
	copy(buf[tableStart:], tableBytes)
	copy(buf[recordStart:], recordBuf.Bytes())

	return buf
}

// sharedPrefix returns the length of the common prefix between a and b.
func sharedPrefix(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

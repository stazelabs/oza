package oza

import (
	"encoding/binary"
	"fmt"
	"sync"
)

// chunkDesc is the parsed 28-byte chunk descriptor from the CONTENT section.
//
// Wire layout (little-endian, matches ozawrite.chunkDesc):
//
//	[0:4]   ID             uint32
//	[4:12]  CompressedOff  uint64  (offset from start of chunk data area)
//	[12:20] CompressedSize uint64
//	[20:24] DictID         uint32  (0 if not dict-compressed)
//	[24]    Compression    uint8
//	[25:28] reserved
type chunkDesc struct {
	ID             uint32
	CompressedOff  uint64
	CompressedSize uint64
	DictID         uint32
	Compression    uint8
}

func parseChunkDesc(data []byte) (chunkDesc, error) {
	if len(data) < ChunkDescSize {
		return chunkDesc{}, fmt.Errorf("oza: chunk descriptor too short: %d bytes, need %d", len(data), ChunkDescSize)
	}
	return chunkDesc{
		ID:             binary.LittleEndian.Uint32(data[0:4]),
		CompressedOff:  binary.LittleEndian.Uint64(data[4:12]),
		CompressedSize: binary.LittleEndian.Uint64(data[12:20]),
		DictID:         binary.LittleEndian.Uint32(data[20:24]),
		Compression:    data[24],
	}, nil
}

// decompressedChunk holds the raw decompressed bytes for one chunk.
type decompressedChunk struct {
	data []byte
}

// chunkCache is a FIFO-eviction cache of decompressed chunks.
type chunkCache struct {
	mu      sync.Mutex
	m       map[uint32]*decompressedChunk
	order   []uint32
	maxSize int
}

func newChunkCache(maxSize int) *chunkCache {
	if maxSize <= 0 {
		maxSize = 8
	}
	return &chunkCache{
		m:       make(map[uint32]*decompressedChunk),
		maxSize: maxSize,
	}
}

func (c *chunkCache) get(id uint32) (*decompressedChunk, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch, ok := c.m[id]
	return ch, ok
}

func (c *chunkCache) put(id uint32, ch *decompressedChunk) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.m[id]; ok {
		return // already cached by a concurrent reader
	}
	if len(c.m) >= c.maxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.m, oldest)
	}
	c.m[id] = ch
	c.order = append(c.order, id)
}

// readChunk decompresses the chunk with the given ID, using the in-memory cache.
func (a *Archive) readChunk(chunkID uint32) (*decompressedChunk, error) {
	if ch, ok := a.cache.get(chunkID); ok {
		return ch, nil
	}
	if int(chunkID) >= len(a.chunkDescs) {
		return nil, fmt.Errorf("oza: chunk ID %d out of range (have %d chunks)", chunkID, len(a.chunkDescs))
	}
	desc := a.chunkDescs[chunkID]
	compData := make([]byte, desc.CompressedSize)
	fileOff := a.chunkDataOff + int64(desc.CompressedOff)
	if _, err := a.r.ReadAt(compData, fileOff); err != nil {
		return nil, fmt.Errorf("oza: reading chunk %d: %w", chunkID, err)
	}
	raw, err := decompressChunk(compData, desc.Compression, desc.DictID, a.dicts)
	if err != nil {
		return nil, fmt.Errorf("oza: decompressing chunk %d: %w", chunkID, err)
	}
	ch := &decompressedChunk{data: raw}
	a.cache.put(chunkID, ch)
	return ch, nil
}

// readBlob extracts a blob from the given chunk at the given byte offset and size.
func (a *Archive) readBlob(chunkID, blobOffset, blobSize uint32) ([]byte, error) {
	ch, err := a.readChunk(chunkID)
	if err != nil {
		return nil, err
	}
	start := int(blobOffset)
	end := start + int(blobSize)
	if end > len(ch.data) {
		return nil, fmt.Errorf("oza: blob [%d:%d] exceeds chunk size %d", start, end, len(ch.data))
	}
	out := make([]byte, blobSize)
	copy(out, ch.data[start:end])
	return out, nil
}

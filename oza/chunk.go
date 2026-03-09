package oza

import (
	"container/list"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
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

// cacheEntry is the value stored in each list element of the LRU cache.
type cacheEntry struct {
	id    uint32
	chunk *decompressedChunk
}

// chunkCache is an LRU-eviction cache of decompressed chunks.
// The list front is the most recently used entry; the back is evicted first.
type chunkCache struct {
	mu      sync.Mutex
	m       map[uint32]*list.Element // chunk ID → list element
	lru     *list.List               // front = MRU, back = LRU
	maxSize int
	hits    int64 // accessed via sync/atomic
	misses  int64 // accessed via sync/atomic
}

func newChunkCache(maxSize int) *chunkCache {
	if maxSize <= 0 {
		maxSize = 8
	}
	return &chunkCache{
		m:       make(map[uint32]*list.Element),
		lru:     list.New(),
		maxSize: maxSize,
	}
}

func (c *chunkCache) get(id uint32) (*decompressedChunk, bool) {
	c.mu.Lock()
	el, ok := c.m[id]
	if ok {
		c.lru.MoveToFront(el)
	}
	c.mu.Unlock()
	if ok {
		atomic.AddInt64(&c.hits, 1)
		return el.Value.(*cacheEntry).chunk, true
	}
	atomic.AddInt64(&c.misses, 1)
	return nil, false
}

// stats returns the current fill, capacity, and lifetime hit/miss counts.
func (c *chunkCache) stats() (current, capacity int, hits, misses int64) {
	c.mu.Lock()
	current = c.lru.Len()
	c.mu.Unlock()
	return current, c.maxSize,
		atomic.LoadInt64(&c.hits),
		atomic.LoadInt64(&c.misses)
}

func (c *chunkCache) put(id uint32, ch *decompressedChunk) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.m[id]; ok {
		return // already cached by a concurrent reader
	}
	if c.lru.Len() >= c.maxSize {
		// Evict the least recently used entry (back of list).
		if lru := c.lru.Back(); lru != nil {
			c.lru.Remove(lru)
			delete(c.m, lru.Value.(*cacheEntry).id)
		}
	}
	el := c.lru.PushFront(&cacheEntry{id: id, chunk: ch})
	c.m[id] = el
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
	if a.maxDecompressedSize > 0 && int64(len(raw)) > a.maxDecompressedSize {
		return nil, fmt.Errorf("oza: chunk %d decompressed to %d bytes: %w", chunkID, len(raw), ErrDecompressedTooLarge)
	}
	ch := &decompressedChunk{data: raw}
	a.cache.put(chunkID, ch)
	return ch, nil
}

// readBlob extracts a blob from the given chunk at the given byte offset and size.
func (a *Archive) readBlob(chunkID, blobOffset, blobSize uint32) ([]byte, error) {
	if a.maxBlobSize > 0 && int64(blobSize) > a.maxBlobSize {
		return nil, fmt.Errorf("oza: blob size %d exceeds limit %d: %w", blobSize, a.maxBlobSize, ErrBlobTooLarge)
	}
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

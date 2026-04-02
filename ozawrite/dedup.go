package ozawrite

import "github.com/cespare/xxhash/v2"

// dedupRef identifies where a blob already lives in a written chunk.
type dedupRef struct {
	chunkID    uint32
	blobOffset uint32
	blobSize   uint32
}

// dedupMap tracks content hashes seen so far to avoid writing duplicate blobs.
// Uses xxhash (64-bit) for fast lookups. Collisions are theoretically possible
// but vanishingly unlikely at archive scale (< 2^32 entries).
type dedupMap struct {
	m map[uint64]dedupRef
}

func newDedupMap() *dedupMap {
	return &dedupMap{m: make(map[uint64]dedupRef)}
}

// Check returns the existing ref if content with this hash was already registered.
func (d *dedupMap) Check(content []byte) (dedupRef, bool) {
	h := xxhash.Sum64(content)
	ref, ok := d.m[h]
	return ref, ok
}

// CheckHash is like Check but accepts a pre-computed hash.
func (d *dedupMap) CheckHash(h uint64) (dedupRef, bool) {
	ref, ok := d.m[h]
	return ref, ok
}

// Register records hash -> ref for future deduplication.
func (d *dedupMap) Register(hash uint64, ref dedupRef) {
	d.m[hash] = ref
}

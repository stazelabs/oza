package ozawrite

import "crypto/sha256"

// dedupRef identifies where a blob already lives in a written chunk.
type dedupRef struct {
	chunkID    uint32
	blobOffset uint32
	blobSize   uint32
}

// dedupMap tracks content hashes seen so far to avoid writing duplicate blobs.
type dedupMap struct {
	m map[[32]byte]dedupRef
}

func newDedupMap() *dedupMap {
	return &dedupMap{m: make(map[[32]byte]dedupRef)}
}

// Check returns the existing ref if content with this hash was already registered.
func (d *dedupMap) Check(content []byte) (dedupRef, bool) {
	h := sha256.Sum256(content)
	ref, ok := d.m[h]
	return ref, ok
}

// CheckHash is like Check but accepts a pre-computed hash.
func (d *dedupMap) CheckHash(h [32]byte) (dedupRef, bool) {
	ref, ok := d.m[h]
	return ref, ok
}

// Register records hash -> ref for future deduplication.
func (d *dedupMap) Register(hash [32]byte, ref dedupRef) {
	d.m[hash] = ref
}

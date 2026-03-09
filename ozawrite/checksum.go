package ozawrite

import (
	"crypto/sha256"
	"encoding/binary"
)

// computeSectionSHA256 returns the SHA-256 of data.
func computeSectionSHA256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// truncateHash returns the first 8 bytes of a SHA-256 as a little-endian uint64.
// Used for EntryRecord.ContentHash.
func truncateHash(full [32]byte) uint64 {
	return binary.LittleEndian.Uint64(full[:8])
}

package ozawrite

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
)

// computeSectionSHA256 returns the SHA-256 of data.
func computeSectionSHA256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// computeFileSHA256 seeks r to the beginning and hashes n bytes.
// It is used to hash everything written before the checksum trailer.
func computeFileSHA256(r io.ReadSeeker, n int64) ([32]byte, error) {
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return [32]byte{}, err
	}
	h := sha256.New()
	if _, err := io.CopyN(h, r, n); err != nil {
		return [32]byte{}, err
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

// truncateHash returns the first 8 bytes of a SHA-256 as a little-endian uint64.
// Used for EntryRecord.ContentHash.
func truncateHash(full [32]byte) uint64 {
	return binary.LittleEndian.Uint64(full[:8])
}

package ozawrite

import "crypto/sha256"

// computeSectionSHA256 returns the SHA-256 of data.
func computeSectionSHA256(data []byte) [32]byte {
	return sha256.Sum256(data)
}

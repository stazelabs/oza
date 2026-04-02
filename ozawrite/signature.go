package ozawrite

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"

	"github.com/stazelabs/oza/oza"
)

// SigningKey pairs an Ed25519 private key with a caller-defined numeric key ID.
// The key ID is a label only; OZA does not define a PKI.
type SigningKey struct {
	Key   ed25519.PrivateKey
	KeyID uint32
}

// buildSignatureTrailer signs fileHash with each key and returns the raw trailer
// bytes to be appended after the 32-byte file checksum.
//
// Trailer layout:
//
//	4 bytes: signature_count (little-endian)
//	Per signature (128 bytes):
//	  32 bytes: Ed25519 public key
//	  64 bytes: signature of fileHash
//	   4 bytes: key_id (little-endian)
//	  28 bytes: reserved (zero)
func buildSignatureTrailer(fileHash [32]byte, keys []SigningKey) ([]byte, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("ozawrite: buildSignatureTrailer called with no keys")
	}

	out := make([]byte, 4+len(keys)*oza.SignatureRecordSize)
	binary.LittleEndian.PutUint32(out[0:4], uint32(len(keys)))

	for i, sk := range keys {
		if len(sk.Key) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("ozawrite: signing key %d: invalid private key size %d", i, len(sk.Key))
		}
		sig := ed25519.Sign(sk.Key, fileHash[:])

		off := 4 + i*oza.SignatureRecordSize
		rec := out[off : off+oza.SignatureRecordSize]

		pubKey := sk.Key.Public().(ed25519.PublicKey)
		copy(rec[0:32], pubKey)
		copy(rec[32:96], sig)
		binary.LittleEndian.PutUint32(rec[96:100], sk.KeyID)
		// rec[100:128] reserved — already zero
	}

	return out, nil
}

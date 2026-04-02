package oza

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"math"
)

// SignatureRecordSize is the fixed size in bytes of one Ed25519 signature
// record in the SIGNATURES trailer (32 pubkey + 64 sig + 4 keyID + 28 reserved).
const SignatureRecordSize = 128

// SignatureRecord holds one Ed25519 signature record from the SIGNATURES trailer.
type SignatureRecord struct {
	PublicKey [32]byte
	Signature [64]byte
	KeyID     uint32
}

// SignatureVerifyResult reports whether a signature verified against a trusted key.
type SignatureVerifyResult struct {
	KeyID     uint32
	PublicKey [32]byte
	OK        bool
}

// HasSignatures reports whether the archive contains a SIGNATURES trailer.
func (a *Archive) HasSignatures() bool { return a.hdr.HasSignatures() }

// ReadSignatures reads the SIGNATURES trailer from a.
// The trailer starts at ChecksumOff+32 (immediately after the 32-byte file checksum).
// Returns nil, nil if the archive does not have the has_signatures flag set.
// Returns an error if the flag is set but the trailer is missing or malformed.
func (a *Archive) ReadSignatures() ([]SignatureRecord, error) {
	if !a.hdr.HasSignatures() {
		return nil, nil
	}

	trailerOff := int64(a.hdr.ChecksumOff) + 32

	// Read the 4-byte count.
	var countBuf [4]byte
	if _, err := a.r.ReadAt(countBuf[:], trailerOff); err != nil {
		return nil, fmt.Errorf("oza: reading signature count: %w", err)
	}
	count := binary.LittleEndian.Uint32(countBuf[:])

	if count == 0 {
		return nil, nil
	}

	// Guard against integer overflow on 32-bit platforms.
	if uint64(count) > uint64(math.MaxInt)/uint64(SignatureRecordSize) {
		return nil, fmt.Errorf("oza: signature count %d exceeds platform limit", count)
	}

	// Read all records.
	data := make([]byte, int(count)*SignatureRecordSize)
	if _, err := a.r.ReadAt(data, trailerOff+4); err != nil {
		return nil, fmt.Errorf("oza: reading signature records: %w", err)
	}

	recs := make([]SignatureRecord, count)
	for i := range recs {
		off := i * SignatureRecordSize
		copy(recs[i].PublicKey[:], data[off:off+32])
		copy(recs[i].Signature[:], data[off+32:off+96])
		recs[i].KeyID = binary.LittleEndian.Uint32(data[off+96 : off+100])
	}
	return recs, nil
}

// VerifySignatures checks Ed25519 signatures in the SIGNATURES trailer against
// trustedKeys. It reads the stored file SHA-256 from ChecksumOff (fast path —
// no full file re-read required).
//
// Only records whose public key matches a trusted key are checked; unrecognised
// keys are skipped. For each trusted key that has a matching record,
// SignatureVerifyResult.OK is true iff the signature is valid.
//
// Returns an empty slice (not an error) if the archive has no SIGNATURES trailer
// or if none of the trusted keys appear in it.
func (a *Archive) VerifySignatures(trustedKeys []ed25519.PublicKey) ([]SignatureVerifyResult, error) {
	recs, err := a.ReadSignatures()
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}

	// Read the stored file SHA-256 (the signed payload).
	var fileHash [32]byte
	if _, err := a.r.ReadAt(fileHash[:], int64(a.hdr.ChecksumOff)); err != nil {
		return nil, fmt.Errorf("oza: reading file checksum for signature verification: %w", err)
	}

	var results []SignatureVerifyResult
	for _, trusted := range trustedKeys {
		if len(trusted) != ed25519.PublicKeySize {
			continue
		}
		for _, rec := range recs {
			if [32]byte(trusted) != rec.PublicKey {
				continue
			}
			ok := ed25519.Verify(trusted, fileHash[:], rec.Signature[:])
			results = append(results, SignatureVerifyResult{
				KeyID:     rec.KeyID,
				PublicKey: rec.PublicKey,
				OK:        ok,
			})
			break // matched this trusted key; move to the next
		}
	}
	return results, nil
}

package oza

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

// VerifyResult records the outcome of one integrity check.
type VerifyResult struct {
	Tier     string // "file", "section", or "entry"
	ID       string // human-readable identifier
	Expected [32]byte
	Computed [32]byte
	OK       bool
}

// Verify checks the file-level SHA-256 checksum.
// The checksum covers all bytes from offset 0 to ChecksumOff, and the 32-byte
// hash is stored immediately after.
func (a *Archive) Verify() error {
	size := a.r.Size()
	if size < int64(a.hdr.ChecksumOff)+32 {
		return fmt.Errorf("oza: file too short to contain checksum")
	}

	// Hash all bytes before the checksum (streaming to avoid a large allocation).
	h := sha256.New()
	sr := io.NewSectionReader(a.r, 0, int64(a.hdr.ChecksumOff))
	if _, err := io.Copy(h, sr); err != nil {
		return fmt.Errorf("oza: reading file body for checksum: %w", err)
	}
	var computed [32]byte
	h.Sum(computed[:0])

	// Read the stored checksum.
	var stored [32]byte
	if _, err := a.r.ReadAt(stored[:], int64(a.hdr.ChecksumOff)); err != nil {
		return fmt.Errorf("oza: reading file checksum: %w", err)
	}

	if computed != stored {
		return ErrChecksumMismatch
	}
	return nil
}

// VerifySection reads a specific section and verifies its SHA-256 against the
// value stored in the section descriptor.
func (a *Archive) VerifySection(st SectionType) error {
	for _, s := range a.sections {
		if s.Type != st {
			continue
		}
		data := make([]byte, s.CompressedSize)
		if s.CompressedSize > 0 {
			if _, err := a.r.ReadAt(data, int64(s.Offset)); err != nil {
				return fmt.Errorf("oza: reading section 0x%04x for verification: %w", st, err)
			}
		}
		computed := sha256.Sum256(data)
		if computed != s.SHA256 {
			return ErrChecksumMismatch
		}
		return nil
	}
	return fmt.Errorf("oza: section type 0x%04x not found", st)
}

// VerifyAll runs all three integrity tiers and returns one VerifyResult per check.
// File-level and section-level checks use SHA-256. Entry-level checks verify the
// truncated content hash stored in each content EntryRecord.
func (a *Archive) VerifyAll() ([]VerifyResult, error) {
	var results []VerifyResult

	// Tier 1: file-level checksum.
	{
		size := a.r.Size()
		var result VerifyResult
		result.Tier = "file"
		result.ID = "file"

		if size < int64(a.hdr.ChecksumOff)+32 {
			result.OK = false
			results = append(results, result)
		} else {
			h := sha256.New()
			sr := io.NewSectionReader(a.r, 0, int64(a.hdr.ChecksumOff))
			if _, err := io.Copy(h, sr); err == nil {
				h.Sum(result.Computed[:0])
				a.r.ReadAt(result.Expected[:], int64(a.hdr.ChecksumOff)) //nolint:errcheck
				result.OK = result.Computed == result.Expected
			}
			results = append(results, result)
		}
	}

	// Tier 2: section-level SHA-256.
	for _, s := range a.sections {
		var result VerifyResult
		result.Tier = "section"
		result.ID = fmt.Sprintf("section-0x%04x", uint32(s.Type))
		result.Expected = s.SHA256

		data := make([]byte, s.CompressedSize)
		if s.CompressedSize > 0 {
			if _, err := a.r.ReadAt(data, int64(s.Offset)); err != nil {
				result.OK = false
				results = append(results, result)
				continue
			}
		}
		result.Computed = sha256.Sum256(data)
		result.OK = result.Computed == result.Expected
		results = append(results, result)
	}

	// Tier 3: entry-level content hash (truncated SHA-256).
	for i := uint32(0); i < a.EntryCount(); i++ {
		rec, err := a.contentEntryRecord(i)
		if err != nil {
			break
		}
		if rec.IsRedirect() {
			continue
		}
		blob, err := a.readBlob(rec.ChunkID, rec.BlobOffset, rec.BlobSize)
		if err != nil {
			var r VerifyResult
			r.Tier = "entry"
			r.ID = fmt.Sprintf("entry-%d", i)
			r.OK = false
			results = append(results, r)
			continue
		}
		full := sha256.Sum256(blob)
		computed := binary.LittleEndian.Uint64(full[:8])
		var r VerifyResult
		r.Tier = "entry"
		r.ID = fmt.Sprintf("entry-%d", i)
		// Store truncated hashes as big-endian in the Expected/Computed fields for display.
		binary.LittleEndian.PutUint64(r.Expected[:8], rec.ContentHash)
		binary.LittleEndian.PutUint64(r.Computed[:8], computed)
		r.OK = computed == rec.ContentHash
		results = append(results, r)
	}

	return results, nil
}

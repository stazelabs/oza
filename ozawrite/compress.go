package ozawrite

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/klauspost/compress/zstd"

	"github.com/stazelabs/oza/oza"
)

// mapEncoderLevel maps an integer zstd level (1-22) to a klauspost EncoderLevel.
// klauspost supports: SpeedFastest(1), SpeedDefault(3), SpeedBetterCompression(6),
// SpeedBestCompression(11). We map to the closest supported level.
func mapEncoderLevel(level int) zstd.EncoderLevel {
	switch {
	case level <= 1:
		return zstd.SpeedFastest
	case level <= 4:
		return zstd.SpeedDefault
	case level <= 8:
		return zstd.SpeedBetterCompression
	default:
		return zstd.SpeedBestCompression
	}
}

// sectionEncoderCache is a package-level encoder cache used by compressZstd
// for section compression (metadata, indexes, etc.). Chunk compression uses
// per-worker caches instead.
var sectionEncoderCache = newEncoderCache()

// compressZstd compresses data at the given level. If dict is non-nil the
// encoder uses it (CompZstdDict); otherwise plain zstd (CompZstd) is used.
// Reuses encoders from sectionEncoderCache to avoid allocating several MB per call.
func compressZstd(data []byte, level int, dict []byte) ([]byte, error) {
	var dictID uint32
	if len(dict) > 0 {
		// Use a synthetic dictID to differentiate cached encoders by dict content.
		// Section compression always uses dictID=0 (no dict) in practice, but this
		// keeps the function general.
		dictID = 1
	}
	return sectionEncoderCache.compress(data, level, dict, dictID)
}

// encoderCacheKey identifies a unique encoder configuration.
type encoderCacheKey struct {
	level  zstd.EncoderLevel
	dictID uint32 // 0 = no dict
}

// encoderCache reuses zstd encoders across chunks with the same (level, dictID).
// Encoders are initialized with zstd.NewWriter(nil, ...) so Reset(w) can be
// called before each use without reallocating internal state.
type encoderCache map[encoderCacheKey]*zstd.Encoder

func newEncoderCache() encoderCache {
	return make(encoderCache)
}

// compress compresses data, reusing a cached encoder for (level, dictID) if available.
func (c encoderCache) compress(data []byte, level int, dict []byte, dictID uint32) ([]byte, error) {
	key := encoderCacheKey{mapEncoderLevel(level), dictID}
	enc, ok := c[key]
	if !ok {
		opts := []zstd.EOption{
			zstd.WithEncoderLevel(mapEncoderLevel(level)),
			zstd.WithEncoderConcurrency(1),
			zstd.WithWindowSize(8 << 20),
			zstd.WithAllLitEntropyCompression(true),
		}
		if len(dict) > 0 {
			opts = append(opts, zstd.WithEncoderDict(dict))
		}
		var err error
		enc, err = zstd.NewWriter(nil, opts...)
		if err != nil {
			return nil, err
		}
		c[key] = enc
	}

	var buf bytes.Buffer
	buf.Grow(len(data) / 2)
	enc.Reset(&buf)
	if _, err := enc.Write(data); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// trainDictionary trains a Zstd dictionary from samples and returns the raw
// dictionary bytes. Returns nil if samples is empty or if the trained
// dictionary fails round-trip validation (a known issue with BuildDict).
func trainDictionary(id uint32, samples [][]byte, maxSize int) (dict []byte, err error) {
	if len(samples) == 0 {
		return nil, nil
	}
	// BuildDict requires a non-empty History (the shared content prefix loaded
	// into the decoder before decompression). Concatenate samples up to maxSize.
	var hist []byte
	for _, s := range samples {
		hist = append(hist, s...)
		if len(hist) >= maxSize {
			break
		}
	}
	if len(hist) > maxSize {
		hist = hist[:maxSize]
	}
	// zstd's bestFastEncoder slides its window by (len(hist) - maxMatchOff).
	// maxMatchOff is 131074 bytes; if hist is shorter the offset goes negative
	// and causes a slice-bounds panic. Require at least 128 KiB of history.
	const minHistSize = 128 * 1024
	if len(hist) < minHistSize {
		return nil, nil
	}

	// BuildDict can panic on certain inputs (e.g. "can only encode up to 64K
	// sequences"). Recover gracefully and skip the dictionary.
	defer func() {
		if r := recover(); r != nil {
			dict = nil
			err = nil
		}
	}()

	dict, err = zstd.BuildDict(zstd.BuildDictOptions{
		ID:       id,
		Contents: samples,
		History:  hist,
	})
	if err != nil {
		return nil, err
	}

	// Validate the dictionary with a compress→decompress round-trip on a few
	// samples. BuildDict can produce dictionaries with invalid offsets or that
	// silently produce corrupt compressed output.
	if err := validateDict(dict, samples); err != nil {
		return nil, nil // discard the dict; caller will use plain zstd
	}
	return dict, nil
}

// validateDict compresses and decompresses up to 5 samples with the given
// dictionary, returning an error if any round-trip fails.
func validateDict(dict []byte, samples [][]byte) error {
	enc, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(1),
		zstd.WithEncoderDict(dict),
	)
	if err != nil {
		return err
	}
	dec, err := zstd.NewReader(nil,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderDicts(dict),
	)
	if err != nil {
		enc.Close()
		return err
	}
	defer dec.Close()

	errMismatch := errors.New("round-trip mismatch")
	limit := 5
	if len(samples) < limit {
		limit = len(samples)
	}
	for i := 0; i < limit; i++ {
		var buf bytes.Buffer
		enc.Reset(&buf)
		if _, err := enc.Write(samples[i]); err != nil {
			return err
		}
		if err := enc.Close(); err != nil {
			return err
		}
		got, err := dec.DecodeAll(buf.Bytes(), nil)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, samples[i]) {
			return errMismatch
		}
	}
	return nil
}

// compressJob is sent from the main goroutine to compression workers.
type compressJob struct {
	chunkID   uint32
	mimeGroup string
	raw       []byte // uncompressed chunk data; owned by this job
	dict      []byte // dictionary bytes; shared read-only
	dictID    uint32
	level     int
}

// compressResult is sent from workers to the writer goroutine.
type compressResult struct {
	chunkID     uint32
	compressed  []byte
	compression uint8
	dictID      uint32
	err         error
}

// compressionWorker reads jobs from w.compressIn, compresses each chunk using
// its own encoderCache, and sends results to w.compressOut.
func (w *Writer) compressionWorker() {
	cache := newEncoderCache()
	for job := range w.compressIn {
		var res compressResult
		res.chunkID = job.chunkID
		res.dictID = job.dictID
		if job.mimeGroup == "image" {
			res.compressed = job.raw
			res.compression = oza.CompNone
		} else {
			cd, err := cache.compress(job.raw, job.level, job.dict, job.dictID)
			if err != nil {
				res.err = fmt.Errorf("compressing chunk %d: %w", job.chunkID, err)
			} else {
				res.compressed = cd
				if job.dictID > 0 {
					res.compression = oza.CompZstdDict
				} else {
					res.compression = oza.CompZstd
				}
			}
		}
		w.compressOut <- res
	}
}

// chunkWriterLoop reads compressed results from w.compressOut, reorders them
// by chunk ID, and writes them sequentially to the temp file. It sends the
// first error (or nil) to w.writerDone when finished.
func (w *Writer) chunkWriterLoop() {
	pending := make(map[uint32]compressResult)
	var nextExpected uint32
	var firstErr error

	for res := range w.compressOut {
		if res.err != nil && firstErr == nil {
			firstErr = res.err
			// Continue draining to unblock workers.
			continue
		}
		if firstErr != nil {
			continue
		}
		pending[res.chunkID] = res

		// Drain all consecutive ready chunks.
		for {
			r, ok := pending[nextExpected]
			if !ok {
				break
			}
			delete(pending, nextExpected)

			if w.chunkTmp == nil {
				f, err := os.CreateTemp("", "ozawrite-chunks-*")
				if err != nil {
					firstErr = fmt.Errorf("creating chunk temp file: %w", err)
					break
				}
				w.chunkTmp = f
			}

			if _, err := w.chunkTmp.Write(r.compressed); err != nil {
				firstErr = fmt.Errorf("writing chunk %d to temp file: %w", r.chunkID, err)
				break
			}

			w.chunkDescs = append(w.chunkDescs, chunkDesc{
				ID:             r.chunkID,
				CompressedOff:  w.chunkOff,
				CompressedSize: uint64(len(r.compressed)),
				DictID:         r.dictID,
				Compression:    r.compression,
			})
			w.chunkOff += uint64(len(r.compressed))

			if w.opts.Progress != nil {
				w.opts.Progress("compress", len(w.chunkDescs), 0)
			}

			nextExpected++
		}
	}

	w.writerDone <- firstErr
}

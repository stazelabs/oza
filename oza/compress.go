package oza

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// defaultMaxDecoderMemory is the hard cap enforced during decompression itself,
// preventing a crafted archive from allocating unbounded memory before the
// post-decompression size check fires. Matches the default maxDecompressedSize.
const defaultMaxDecoderMemory = 1 << 30 // 1 GiB

var defaultDecoderPool = &sync.Pool{
	New: func() any {
		d, err := zstd.NewReader(nil,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderMaxMemory(defaultMaxDecoderMemory),
		)
		if err != nil {
			panic("oza: zstd.NewReader: " + err.Error())
		}
		return d
	},
}

var (
	dictPoolsMu sync.Mutex
	dictPools   = map[uint32]*sync.Pool{}
)

// decompressSection decompresses section data based on the descriptor.
func decompressSection(data []byte, desc SectionDesc, dicts map[uint32][]byte) ([]byte, error) {
	return decompressBytes(data, desc.Compression, desc.DictID, dicts)
}

// decompressChunk decompresses chunk data using the given compression type and dict.
func decompressChunk(data []byte, compression uint8, dictID uint32, dicts map[uint32][]byte) ([]byte, error) {
	return decompressBytes(data, compression, dictID, dicts)
}

func decompressBytes(data []byte, compression uint8, dictID uint32, dicts map[uint32][]byte) ([]byte, error) {
	switch compression {
	case CompNone:
		out := make([]byte, len(data))
		copy(out, data)
		return out, nil
	case CompZstd:
		return decodeZstd(data)
	case CompZstdDict:
		raw, ok := dicts[dictID]
		if !ok {
			return nil, fmt.Errorf("oza: dict %d not found", dictID)
		}
		return decodeZstdDict(data, dictID, raw)
	case CompBrotli:
		return decodeBrotli(data)
	default:
		return nil, fmt.Errorf("oza: unknown compression type %d", compression)
	}
}

func decodeZstd(data []byte) ([]byte, error) {
	d := defaultDecoderPool.Get().(*zstd.Decoder)
	out, err := d.DecodeAll(data, nil)
	defaultDecoderPool.Put(d)
	if err != nil {
		return nil, fmt.Errorf("oza: zstd decompress: %w", err)
	}
	return out, nil
}

func decodeZstdDict(data []byte, dictID uint32, raw []byte) ([]byte, error) {
	dictPoolsMu.Lock()
	pool, ok := dictPools[dictID]
	if !ok {
		capturedRaw := raw
		capturedID := dictID
		pool = &sync.Pool{
			New: func() any {
				d, err := zstd.NewReader(nil,
					zstd.WithDecoderConcurrency(1),
					zstd.WithDecoderMaxMemory(defaultMaxDecoderMemory),
					zstd.WithDecoderDicts(capturedRaw),
				)
				if err != nil {
					panic(fmt.Sprintf("oza: zstd dict decoder %d: %v", capturedID, err))
				}
				return d
			},
		}
		dictPools[dictID] = pool
	}
	dictPoolsMu.Unlock()

	d := pool.Get().(*zstd.Decoder)
	out, err := d.DecodeAll(data, nil)
	pool.Put(d)
	if err != nil {
		return nil, fmt.Errorf("oza: zstd dict %d decompress: %w", dictID, err)
	}
	return out, nil
}

func decodeBrotli(data []byte) ([]byte, error) {
	out, err := io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
	if err != nil {
		return nil, fmt.Errorf("oza: brotli decompress: %w", err)
	}
	return out, nil
}

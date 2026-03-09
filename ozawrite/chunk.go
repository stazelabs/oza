package ozawrite

import (
	"encoding/binary"
	"strings"

	"github.com/stazelabs/oza/oza"
)

// chunkDesc is the 28-byte on-disk descriptor for one compressed chunk within
// the CONTENT section. Layout (little-endian):
//
//	[0:4]   ID             uint32
//	[4:12]  CompressedOff  uint64  (byte offset from start of chunk data area)
//	[12:20] CompressedSize uint64
//	[20:24] DictID         uint32  (0 if not dict-compressed)
//	[24]    Compression    uint8   (oza.CompNone / CompZstd / CompZstdDict)
//	[25:28] reserved       3 bytes
type chunkDesc struct {
	ID            uint32
	CompressedOff uint64
	CompressedSize uint64
	DictID        uint32
	Compression   uint8
}

func marshalChunkDesc(c chunkDesc) [oza.ChunkDescSize]byte {
	var b [oza.ChunkDescSize]byte
	binary.LittleEndian.PutUint32(b[0:4], c.ID)
	binary.LittleEndian.PutUint64(b[4:12], c.CompressedOff)
	binary.LittleEndian.PutUint64(b[12:20], c.CompressedSize)
	binary.LittleEndian.PutUint32(b[20:24], c.DictID)
	b[24] = c.Compression
	// [25:28] reserved, zero
	return b
}

// chunkBuilder accumulates blobs that will be compressed together.
type chunkBuilder struct {
	id         uint32
	mimeGroup  string // "html", "css", "js", "image", "other"
	blobs      [][]byte
	uncompSize int
}

// addBlob appends a blob and returns its byte offset within the uncompressed chunk.
func (c *chunkBuilder) addBlob(data []byte) uint32 {
	offset := uint32(c.uncompSize)
	c.blobs = append(c.blobs, data)
	c.uncompSize += len(data)
	return offset
}

// uncompressedBytes returns the concatenated uncompressed chunk data.
func (c *chunkBuilder) uncompressedBytes() []byte {
	total := 0
	for _, b := range c.blobs {
		total += len(b)
	}
	out := make([]byte, 0, total)
	for _, b := range c.blobs {
		out = append(out, b...)
	}
	return out
}

// smallEntryThreshold is the content size below which entries are routed to a
// separate "small" chunk bucket with a specialised Zstd dictionary. Short
// entries benefit 2-3x from a dictionary trained on similar-sized content.
const smallEntryThreshold = 4096

// ChunkKey returns the chunk grouping key for an entry, combining its MIME
// group with a size bucket. Non-image entries smaller than smallEntryThreshold
// get a "-small" suffix so they are chunked and dictionary-trained separately.
func ChunkKey(mimeType string, contentLen int) string {
	g := mimeGroup(mimeType)
	if g != "image" && contentLen < smallEntryThreshold {
		return g + "-small"
	}
	return g
}

// mimeGroup classifies a MIME type into a broad grouping for chunk co-location.
func mimeGroup(mimeType string) string {
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = strings.TrimSpace(mimeType[:i])
	}
	switch {
	case mimeType == "text/html":
		return "html"
	case mimeType == "text/css":
		return "css"
	case mimeType == "application/javascript", mimeType == "text/javascript":
		return "js"
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	default:
		return "other"
	}
}

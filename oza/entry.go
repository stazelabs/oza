package oza

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Entry flag bits.
const (
	EntryFlagFrontArticle uint8 = 1 << 0
)

// EntryRecord is a content entry record.
//
// Variable-length binary layout:
//
//	uint8   type_and_flags  (bits 0-3 = EntryType, bits 4-7 = flags)
//	uvarint mime_index
//	uvarint chunk_id
//	uvarint blob_offset
//	uvarint blob_size
//	uint64  content_hash    (fixed 8 bytes, little-endian)
//
// ID is implicit (index in the offset table) and is set by the caller.
// RedirectTarget is only used for redirect entries (stored in SectionRedirectTab).
type EntryRecord struct {
	ID             uint32
	Type           EntryType
	Flags          uint8
	MIMEIndex      uint16
	ChunkID        uint32
	BlobOffset     uint32
	BlobSize       uint32
	RedirectTarget uint32
	ContentHash    uint64
}

// ParseVarEntryRecord parses a variable-length entry record from data.
// Returns the parsed record and the number of bytes consumed.
func ParseVarEntryRecord(data []byte) (EntryRecord, int, error) {
	if len(data) < 2 { // minimum: 1 byte type_and_flags + at least 1 more
		return EntryRecord{}, 0, fmt.Errorf("oza: var entry record too short: %d bytes", len(data))
	}

	var e EntryRecord
	pos := 0

	// type_and_flags: bits 0-3 = type, bits 4-7 = flags
	tf := data[pos]
	e.Type = EntryType(tf & 0x0F)
	e.Flags = tf >> 4
	pos++

	v, n := binary.Uvarint(data[pos:])
	if n <= 0 {
		return EntryRecord{}, 0, fmt.Errorf("oza: bad uvarint for mime_index")
	}
	e.MIMEIndex = uint16(v)
	pos += n

	v, n = binary.Uvarint(data[pos:])
	if n <= 0 {
		return EntryRecord{}, 0, fmt.Errorf("oza: bad uvarint for chunk_id")
	}
	e.ChunkID = uint32(v)
	pos += n

	v, n = binary.Uvarint(data[pos:])
	if n <= 0 {
		return EntryRecord{}, 0, fmt.Errorf("oza: bad uvarint for blob_offset")
	}
	e.BlobOffset = uint32(v)
	pos += n

	v, n = binary.Uvarint(data[pos:])
	if n <= 0 {
		return EntryRecord{}, 0, fmt.Errorf("oza: bad uvarint for blob_size")
	}
	e.BlobSize = uint32(v)
	pos += n

	if pos+8 > len(data) {
		return EntryRecord{}, 0, fmt.Errorf("oza: var entry record too short for content_hash")
	}
	e.ContentHash = binary.LittleEndian.Uint64(data[pos : pos+8])
	pos += 8

	return e, pos, nil
}

// AppendVarEntryRecord appends a variable-length entry record to buf.
func AppendVarEntryRecord(buf []byte, e EntryRecord) []byte {
	// type_and_flags
	buf = append(buf, byte(e.Type&0x0F)|(e.Flags<<4))

	// uvarint fields
	var tmp [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(tmp[:], uint64(e.MIMEIndex))
	buf = append(buf, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(e.ChunkID))
	buf = append(buf, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(e.BlobOffset))
	buf = append(buf, tmp[:n]...)
	n = binary.PutUvarint(tmp[:], uint64(e.BlobSize))
	buf = append(buf, tmp[:n]...)

	// fixed uint64 content_hash
	var h [8]byte
	binary.LittleEndian.PutUint64(h[:], e.ContentHash)
	buf = append(buf, h[:]...)

	return buf
}

// IsRedirect reports whether this entry is a redirect.
func (e EntryRecord) IsRedirect() bool { return e.Type == EntryRedirect }

// RedirectRecord is the compact 5-byte redirect record stored in SectionRedirectTab.
//
// Binary layout (little-endian):
//
//	[0]     Flags     uint8   (bit 0 = front_article)
//	[1:5]   TargetID  uint32  (content entry ID, bit 31 always clear)
type RedirectRecord struct {
	Flags    uint8
	TargetID uint32
}

// ParseRedirectRecord parses a single 5-byte redirect record from data.
func ParseRedirectRecord(data []byte) (RedirectRecord, error) {
	if len(data) < RedirectRecordSize {
		return RedirectRecord{}, fmt.Errorf("oza: redirect record too short: %d bytes, need %d", len(data), RedirectRecordSize)
	}
	return RedirectRecord{
		Flags:    data[0],
		TargetID: binary.LittleEndian.Uint32(data[1:5]),
	}, nil
}

// Marshal serializes r to a fixed 5-byte array.
func (r RedirectRecord) Marshal() [RedirectRecordSize]byte {
	var b [RedirectRecordSize]byte
	b[0] = r.Flags
	binary.LittleEndian.PutUint32(b[1:5], r.TargetID)
	return b
}

// IsFrontArticle reports whether this redirect is a front article.
func (r RedirectRecord) IsFrontArticle() bool { return r.Flags&EntryFlagFrontArticle != 0 }

// IsFrontArticle reports whether this entry is a front article.
func (e EntryRecord) IsFrontArticle() bool { return e.Flags&EntryFlagFrontArticle != 0 }

// --- High-level Entry type ---

// Entry is a fully resolved entry with path, title, and a back-pointer to the
// archive it belongs to. It is the primary type callers interact with.
type Entry struct {
	archive *Archive
	record  EntryRecord
	path    string
	title   string
}

// ID returns the entry's numeric ID within the archive.
func (e Entry) ID() uint32 { return e.record.ID }

// Path returns the entry's URL path within the archive.
func (e Entry) Path() string { return e.path }

// Title returns the entry's display title.
func (e Entry) Title() string { return e.title }

// MIMEIndex returns the raw MIME type index for this entry. The index can be
// compared against the package-level constants (MIMEIndexHTML, MIMEIndexCSS,
// MIMEIndexJS) for fast type checks without string allocation. Returns 0 for
// redirect entries (the index has no meaning for redirects; check IsRedirect
// first if that distinction matters).
func (e Entry) MIMEIndex() uint { return uint(e.record.MIMEIndex) }

// MIMEType returns the MIME type string for this entry.
// Returns an empty string for redirect entries. MIMEIndex is validated against
// the table at entry-record parse time, so an out-of-range index is never
// reachable here on a well-formed Entry.
func (e Entry) MIMEType() string {
	if e.record.IsRedirect() {
		return ""
	}
	if int(e.record.MIMEIndex) >= len(e.archive.mimeTypes) {
		return "" // unreachable for entries returned by Archive methods
	}
	return e.archive.mimeTypes[e.record.MIMEIndex]
}

// IsRedirect reports whether this entry is a redirect.
func (e Entry) IsRedirect() bool { return e.record.IsRedirect() }

// IsFrontArticle reports whether this entry is a front article.
func (e Entry) IsFrontArticle() bool { return e.record.IsFrontArticle() }

// Size returns the uncompressed blob size in bytes. Returns 0 for redirects.
func (e Entry) Size() uint32 { return e.record.BlobSize }

// Resolve follows the redirect chain and returns the final content entry.
// Returns ErrRedirectLoop if a cycle is detected.
func (e Entry) Resolve() (Entry, error) {
	visited := map[uint32]bool{e.record.ID: true}
	cur := e
	for cur.record.IsRedirect() {
		target := cur.record.RedirectTarget
		if visited[target] {
			return Entry{}, ErrRedirectLoop
		}
		visited[target] = true
		next, err := cur.archive.EntryByID(target)
		if err != nil {
			return Entry{}, err
		}
		cur = next
	}
	return cur, nil
}

// ReadContent reads and returns the full content of this entry, resolving
// redirects automatically.
func (e Entry) ReadContent() ([]byte, error) {
	content, err := e.Resolve()
	if err != nil {
		return nil, err
	}
	return e.archive.readBlob(content.record.ChunkID, content.record.BlobOffset, content.record.BlobSize)
}

// ContentReader returns an io.Reader over the entry's content, resolving
// redirects automatically.
func (e Entry) ContentReader() (io.Reader, error) {
	data, err := e.ReadContent()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// WriteTo writes the entry's content directly to w, resolving redirects
// automatically. It uses the zero-copy path, avoiding an allocation for the
// blob data. The returned int64 is the number of bytes written.
func (e Entry) WriteTo(w io.Writer) (int64, error) {
	content, err := e.Resolve()
	if err != nil {
		return 0, err
	}
	data, err := e.archive.readBlobSlice(content.record.ChunkID, content.record.BlobOffset, content.record.BlobSize)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// ReadContentSlice returns a zero-copy sub-slice of cached chunk data for this
// entry's content, resolving redirects automatically. The returned slice must
// not be modified and is only valid while the underlying chunk remains in cache.
// Use ReadContent instead if the data will be retained or mutated.
func (e Entry) ReadContentSlice() ([]byte, error) {
	content, err := e.Resolve()
	if err != nil {
		return nil, err
	}
	return e.archive.readBlobSlice(content.record.ChunkID, content.record.BlobOffset, content.record.BlobSize)
}

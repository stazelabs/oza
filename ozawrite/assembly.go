package ozawrite

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/stazelabs/oza/oza"
)

// cleanupTemp removes the chunk temp file and trigram temp files.
func (w *Writer) cleanupTemp() {
	if w.chunkTmp != nil {
		name := w.chunkTmp.Name()
		w.chunkTmp.Close()
		os.Remove(name) //nolint:errcheck // best-effort cleanup
		w.chunkTmp = nil
	}
	w.titleTB = nil
	w.bodyTB = nil
}

// contentSectionSize returns the total byte size of the CONTENT section.
func (w *Writer) contentSectionSize() uint64 {
	n := len(w.chunkDescs)
	tableBytes := uint64(4 + n*oza.ChunkDescSize)
	return tableBytes + w.chunkOff
}

// writeContentSection streams the CONTENT section directly to dst, computing
// its SHA-256 as it goes. This avoids materializing the entire section in memory.
func (w *Writer) writeContentSection(dst io.Writer) ([32]byte, error) {
	h := sha256.New()
	mw := io.MultiWriter(dst, h)

	n := len(w.chunkDescs)

	// Write chunk table header.
	var countBuf [4]byte
	binary.LittleEndian.PutUint32(countBuf[:], uint32(n))
	if _, err := mw.Write(countBuf[:]); err != nil {
		return [32]byte{}, err
	}
	for _, cd := range w.chunkDescs {
		b := marshalChunkDesc(cd)
		if _, err := mw.Write(b[:]); err != nil {
			return [32]byte{}, err
		}
	}

	// Stream compressed chunk data from temp file.
	if w.chunkTmp != nil && w.chunkOff > 0 {
		if _, err := w.chunkTmp.Seek(0, io.SeekStart); err != nil {
			return [32]byte{}, fmt.Errorf("seeking chunk temp file: %w", err)
		}
		if _, err := io.Copy(mw, w.chunkTmp); err != nil {
			return [32]byte{}, fmt.Errorf("streaming chunk data: %w", err)
		}
	}

	var sha [32]byte
	copy(sha[:], h.Sum(nil))
	return sha, nil
}

// buildMIMETable constructs the MIME type list (enforcing index 0/1/2) and a
// lookup map from type string to index.
func (w *Writer) buildMIMETable() ([]string, map[string]uint16) {
	// Start with the three mandatory types.
	types := []string{"text/html", "text/css", "application/javascript"}
	m := map[string]uint16{
		"text/html":              0,
		"text/css":               1,
		"application/javascript": 2,
	}
	for _, e := range w.entries {
		mt := e.mimeType
		if _, ok := m[mt]; !ok {
			m[mt] = uint16(len(types))
			types = append(types, mt)
		}
	}
	return types, m
}

// buildEntryTable serialises content entries as variable-length records with an
// offset table for O(1) random access.
//
// Layout: uint32 entry_count, uint32 record_data_offset,
//
//	uint32[N] offsets, variable-length records.
func (w *Writer) buildEntryTable(mimeMap map[string]uint16) []byte {
	count := len(w.entries)
	records := make([]byte, 0, count*16) // estimated avg ~15 bytes/record
	offsets := make([]uint32, count)

	for i, e := range w.entries {
		offsets[i] = uint32(len(records))
		var rec oza.EntryRecord
		rec.Type = oza.EntryContent
		rec.MIMEIndex = mimeMap[e.mimeType]
		rec.ChunkID = e.chunkID
		rec.BlobOffset = e.blobOffset
		rec.BlobSize = e.blobSize
		rec.ContentHash = truncateHash(e.contentHash)
		if e.isFrontArticle {
			rec.Flags |= oza.EntryFlagFrontArticle
		}
		records = oza.AppendVarEntryRecord(records, rec)
	}

	headerSize := oza.EntryTableHeaderSize + count*4
	buf := make([]byte, headerSize+len(records))
	binary.LittleEndian.PutUint32(buf[0:4], uint32(count))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(headerSize))
	for i, off := range offsets {
		binary.LittleEndian.PutUint32(buf[oza.EntryTableHeaderSize+i*4:], off)
	}
	copy(buf[headerSize:], records)
	return buf
}

// buildRedirectSection serialises redirect entries into the compact format.
// Wire format: uint32 count + 5-byte records (flags, target_id).
func (w *Writer) buildRedirectSection() []byte {
	if len(w.redirectEntries) == 0 {
		return nil
	}
	data := make([]byte, 4+len(w.redirectEntries)*oza.RedirectRecordSize)
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(w.redirectEntries)))
	for i, e := range w.redirectEntries {
		var rr oza.RedirectRecord
		rr.TargetID = e.targetID
		if e.isFrontArticle {
			rr.Flags |= oza.EntryFlagFrontArticle
		}
		b := rr.Marshal()
		copy(data[4+i*oza.RedirectRecordSize:], b[:])
	}
	return data
}

// buildIndexSections builds the raw path and title index section bytes.
// Both content and redirect entries are included; redirect entries use
// tagged IDs (bit 31 set).
func (w *Writer) buildIndexSections() (pathIdx []byte, titleIdx []byte) {
	total := len(w.entries) + len(w.redirectEntries)
	paths := make([]pathRecord, 0, total)
	titles := make([]titleRecord, 0, total)
	for _, e := range w.entries {
		paths = append(paths, pathRecord{entryID: e.id, path: e.path})
		titles = append(titles, titleRecord{entryID: e.id, title: e.title})
	}
	for _, e := range w.redirectEntries {
		taggedID := oza.MakeRedirectID(e.redirectIndex)
		paths = append(paths, pathRecord{entryID: taggedID, path: e.path})
		titles = append(titles, titleRecord{entryID: taggedID, title: e.title})
	}
	return buildPathIndex(paths), buildTitleIndex(titles)
}

// buildDictSections creates one rawSection per trained dictionary.
// Dict section data: [0:4] dictID uint32, [4:] raw dict bytes.
func buildDictSections(dicts map[string][]byte, dictIDs map[string]uint32) []rawSection {
	var out []rawSection
	for group, d := range dicts {
		id := dictIDs[group]
		data := make([]byte, 4+len(d))
		binary.LittleEndian.PutUint32(data[0:4], id)
		copy(data[4:], d)
		out = append(out, newRawSection(oza.SectionZstdDict, data))
	}
	return out
}

// newRawSection creates an uncompressed rawSection.
func newRawSection(typ oza.SectionType, data []byte) rawSection {
	return rawSection{
		typ:              typ,
		data:             data,
		uncompressedSize: uint64(len(data)),
		compression:      oza.CompNone,
	}
}

// compressRawSection compresses data with Zstd. If compression doesn't reduce
// size (or the section is tiny), it falls back to uncompressed.
func compressRawSection(typ oza.SectionType, data []byte) rawSection {
	const minCompressSize = 256 // don't bother compressing tiny sections
	if len(data) < minCompressSize {
		return newRawSection(typ, data)
	}
	compressed, err := compressZstd(data, 19, nil)
	if err != nil || len(compressed) >= len(data) {
		return newRawSection(typ, data)
	}
	return rawSection{
		typ:              typ,
		data:             compressed,
		uncompressedSize: uint64(len(data)),
		compression:      oza.CompZstd,
	}
}

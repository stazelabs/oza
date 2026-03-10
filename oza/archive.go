package oza

import (
	"encoding/binary"
	"fmt"
)

// Archive provides read access to an OZA archive file.
//
// Concurrency: an Archive is safe for concurrent use by multiple goroutines
// after Open or OpenWithOptions returns. All internal state is populated
// during opening and never modified afterward; the chunk cache is
// independently protected by its own mutex. Close must not be called
// concurrently with any other method.
type Archive struct {
	r        reader
	hdr      Header
	sections []SectionDesc

	mimeTypes []string
	metadata  map[string][]byte

	entryOffsets []uint32 // offset table for variable-length entry records
	entryRecords []byte   // raw variable-length entry record data

	redirectData  []byte // raw redirect table section bytes
	redirectCount uint32 // number of redirect records

	pathIdx        *Index
	titleIdx       *Index
	titleSearchIdx *TrigramIndex // SectionSearchTitle (0x000C)
	bodySearchIdx  *TrigramIndex // SectionSearchBody (0x000D)

	// Reverse maps built on open for O(1) EntryByID path/title resolution.
	// Keys use tagged IDs: bit 31 set for redirects.
	idToPath  map[uint32]string
	idToTitle map[uint32]string

	dicts map[uint32][]byte // dictID -> raw dict bytes

	chunkDescs   []chunkDesc // chunk table from CONTENT section
	chunkDataOff int64       // file offset of chunk data area

	cache                *chunkCache
	maxDecompressedSize  int64 // decompression bomb limit; 0 = disabled
	maxBlobSize          int64 // per-blob size limit; 0 = disabled
	maxMetadataValueSize int64 // per-metadata-value size limit; 0 = disabled

	warnings []string // non-fatal advisory messages from open
}

// Option configures Archive opening behaviour.
type Option func(*options)

type options struct {
	cacheSize            int
	useMmap              bool
	verifyOnOpen         bool
	maxDecompressedSize  int64
	maxBlobSize          int64
	maxMetadataValueSize int64
}

// WithCacheSize sets the number of decompressed chunks to keep in memory.
func WithCacheSize(n int) Option { return func(o *options) { o.cacheSize = n } }

// WithMmap enables or disables memory-mapped I/O (default: enabled).
func WithMmap(enabled bool) Option { return func(o *options) { o.useMmap = enabled } }

// WithVerifyOnOpen verifies all section SHA-256 checksums when opening.
func WithVerifyOnOpen() Option { return func(o *options) { o.verifyOnOpen = true } }

// WithMaxDecompressedSize sets the maximum allowed decompressed size for any
// single chunk or section. Archives containing chunks larger than this limit
// will fail with ErrDecompressedTooLarge. Defaults to 1 GiB.
func WithMaxDecompressedSize(n int64) Option {
	return func(o *options) { o.maxDecompressedSize = n }
}

// WithMaxBlobSize sets the maximum allowed size for a single blob (entry content).
// Entries whose blob size exceeds this limit will fail with ErrBlobTooLarge.
// Defaults to 256 MiB.
func WithMaxBlobSize(n int64) Option {
	return func(o *options) { o.maxBlobSize = n }
}

// WithMaxMetadataValueSize sets the maximum allowed size for a single metadata
// value. Values exceeding this limit will fail with ErrMetadataValueTooLarge.
// Defaults to 16 MiB.
func WithMaxMetadataValueSize(n int64) Option {
	return func(o *options) { o.maxMetadataValueSize = n }
}

// Open opens an OZA archive at path with default options.
func Open(path string) (*Archive, error) {
	return OpenWithOptions(path)
}

// OpenWithOptions opens an OZA archive at path with the supplied options.
func OpenWithOptions(path string, opts ...Option) (*Archive, error) {
	o := &options{
		cacheSize:            8,
		useMmap:              true,
		maxDecompressedSize:  1 << 30,   // 1 GiB default
		maxBlobSize:          256 << 20, // 256 MiB default
		maxMetadataValueSize: 16 << 20,  // 16 MiB default
	}
	for _, fn := range opts {
		fn(o)
	}

	r, err := openReader(path, o.useMmap)
	if err != nil {
		return nil, err
	}

	a := &Archive{
		r:                    r,
		dicts:                make(map[uint32][]byte),
		cache:                newChunkCache(o.cacheSize),
		maxDecompressedSize:  o.maxDecompressedSize,
		maxBlobSize:          o.maxBlobSize,
		maxMetadataValueSize: o.maxMetadataValueSize,
	}
	if err := a.load(o.verifyOnOpen); err != nil {
		r.Close()
		return nil, err
	}
	return a, nil
}

// Close releases all resources held by the archive.
func (a *Archive) Close() error { return a.r.Close() }

// Warnings returns non-fatal advisory messages generated while opening the
// archive. A non-empty slice typically indicates the file was written by a
// newer version of the format that uses reserved fields this reader ignores.
func (a *Archive) Warnings() []string { return a.warnings }

// load reads and parses all sections.
func (a *Archive) load(verify bool) error {
	// 1. Header.
	var hdrBuf [HeaderSize]byte
	if _, err := a.r.ReadAt(hdrBuf[:], 0); err != nil {
		return fmt.Errorf("oza: reading header: %w", err)
	}
	hdr, err := ParseHeader(hdrBuf[:])
	if err != nil {
		return err
	}
	a.hdr = hdr

	// 1b. Warn on non-zero header reserved bytes (offsets 68-127).
	{
		var nonZero bool
		for _, b := range hdrBuf[68:128] {
			if b != 0 {
				nonZero = true
				break
			}
		}
		if nonZero {
			a.warnings = append(a.warnings,
				"oza: header reserved bytes [68:128] are non-zero; file may have been written by a newer version",
			)
		}
	}

	// 2. Section table.
	tableSize := int64(hdr.SectionCount) * SectionSize
	tableBuf := make([]byte, tableSize)
	if tableSize > 0 {
		if _, err := a.r.ReadAt(tableBuf, int64(hdr.SectionTableOff)); err != nil {
			return fmt.Errorf("oza: reading section table: %w", err)
		}
	}
	sections, err := ParseSectionTable(tableBuf, hdr.SectionCount)
	if err != nil {
		return err
	}
	a.sections = sections

	// 2b. Warn on non-zero section descriptor reserved bytes.
	// SectionDesc layout: [33:36] reserved (3 bytes), [40:48] reserved (8 bytes).
	for i := 0; i < int(hdr.SectionCount); i++ {
		off := i * SectionSize
		stype := binary.LittleEndian.Uint32(tableBuf[off : off+4])
		if tableBuf[off+33]|tableBuf[off+34]|tableBuf[off+35] != 0 {
			a.warnings = append(a.warnings, fmt.Sprintf(
				"oza: section %d (type 0x%04x) reserved bytes [33:36] are non-zero; file may have been written by a newer version",
				i, stype,
			))
		}
		r2 := tableBuf[off+40] | tableBuf[off+41] | tableBuf[off+42] | tableBuf[off+43] |
			tableBuf[off+44] | tableBuf[off+45] | tableBuf[off+46] | tableBuf[off+47]
		if r2 != 0 {
			a.warnings = append(a.warnings, fmt.Sprintf(
				"oza: section %d (type 0x%04x) reserved bytes [40:48] are non-zero; file may have been written by a newer version",
				i, stype,
			))
		}
	}

	// 3. Load each section.
	for _, s := range sections {
		if err := a.loadSection(s); err != nil {
			return err
		}
	}

	// 4. Build reverse maps from index data.
	if err := a.buildReverseMaps(); err != nil {
		return err
	}

	if verify {
		results, err := a.VerifyAll()
		if err != nil {
			return err
		}
		for _, r := range results {
			if !r.OK {
				return fmt.Errorf("oza: verification failed for %s: checksum mismatch", r.ID)
			}
		}
	}
	return nil
}

// loadSection reads and parses one section.
func (a *Archive) loadSection(s SectionDesc) error {
	switch s.Type {
	case SectionContent:
		return a.loadContentSection(s)
	default:
		data, err := a.readSectionData(s)
		if err != nil {
			return fmt.Errorf("oza: reading section 0x%04x: %w", s.Type, err)
		}
		return a.parseSection(s.Type, data)
	}
}

// readSectionData reads and decompresses the bytes of a section.
func (a *Archive) readSectionData(s SectionDesc) ([]byte, error) {
	buf := make([]byte, s.CompressedSize)
	if s.CompressedSize > 0 {
		if _, err := a.r.ReadAt(buf, int64(s.Offset)); err != nil {
			return nil, err
		}
	}
	if s.Compression == CompNone {
		return buf, nil
	}
	out, err := decompressSection(buf, s, a.dicts)
	if err != nil {
		return nil, err
	}
	if a.maxDecompressedSize > 0 && int64(len(out)) > a.maxDecompressedSize {
		return nil, fmt.Errorf("oza: section 0x%04x decompressed to %d bytes: %w", s.Type, len(out), ErrDecompressedTooLarge)
	}
	return out, nil
}

// parseSection dispatches parsed data to the appropriate field.
func (a *Archive) parseSection(t SectionType, data []byte) error {
	switch t {
	case SectionMIMETable:
		types, err := ParseMIMETable(data)
		if err != nil {
			return err
		}
		a.mimeTypes = types

	case SectionMetadata:
		meta, err := a.parseMetadata(data)
		if err != nil {
			return err
		}
		a.metadata = meta

	case SectionEntryTable:
		if err := a.parseEntryTable(data); err != nil {
			return err
		}

	case SectionPathIndex:
		idx, err := ParseIndex(data)
		if err != nil {
			return fmt.Errorf("oza: parsing path index: %w", err)
		}
		a.pathIdx = idx

	case SectionTitleIndex:
		idx, err := ParseIndex(data)
		if err != nil {
			return fmt.Errorf("oza: parsing title index: %w", err)
		}
		a.titleIdx = idx

	case SectionZstdDict:
		if len(data) < 4 {
			return fmt.Errorf("oza: zstd dict section too short")
		}
		dictID := binary.LittleEndian.Uint32(data[0:4])
		a.dicts[dictID] = data[4:]

	case SectionSearchTitle:
		idx, err := ParseTrigramIndex(data)
		if err != nil {
			return fmt.Errorf("oza: parsing title search index: %w", err)
		}
		a.titleSearchIdx = idx

	case SectionSearchBody:
		idx, err := ParseTrigramIndex(data)
		if err != nil {
			return fmt.Errorf("oza: parsing body search index: %w", err)
		}
		a.bodySearchIdx = idx

	case SectionRedirectTab:
		if len(data) < 4 {
			return fmt.Errorf("oza: redirect table section too short")
		}
		a.redirectCount = binary.LittleEndian.Uint32(data[0:4])
		a.redirectData = data
	}
	// Unknown section types are silently ignored (extensibility).
	return nil
}

// loadContentSection reads the chunk table from the CONTENT section without
// loading all compressed chunk data into memory.
func (a *Archive) loadContentSection(s SectionDesc) error {
	// Read count.
	var countBuf [4]byte
	if _, err := a.r.ReadAt(countBuf[:], int64(s.Offset)); err != nil {
		return fmt.Errorf("oza: reading content section count: %w", err)
	}
	count := binary.LittleEndian.Uint32(countBuf[:])

	// Read chunk table.
	tableSize := int(count) * ChunkDescSize
	tableBuf := make([]byte, tableSize)
	if tableSize > 0 {
		if _, err := a.r.ReadAt(tableBuf, int64(s.Offset)+4); err != nil {
			return fmt.Errorf("oza: reading chunk table: %w", err)
		}
	}
	a.chunkDescs = make([]chunkDesc, count)
	for i := range a.chunkDescs {
		off := i * ChunkDescSize
		desc, err := parseChunkDesc(tableBuf[off:])
		if err != nil {
			return fmt.Errorf("oza: chunk descriptor %d: %w", i, err)
		}
		if desc.ID != uint32(i) {
			return fmt.Errorf("oza: chunk descriptor %d has ID %d: %w", i, desc.ID, ErrChunkTableUnsorted)
		}
		a.chunkDescs[i] = desc
	}
	// Chunk data area starts immediately after the chunk table within the section.
	a.chunkDataOff = int64(s.Offset) + 4 + int64(tableSize)
	return nil
}

// buildReverseMaps populates idToPath and idToTitle from the loaded indices.
// Uses ForEachErr for O(N) sequential iteration instead of Record(i) per entry.
// A string interner deduplicates identical strings (common with redirects).
func (a *Archive) buildReverseMaps() error {
	si := newStringInterner()
	a.idToPath = make(map[uint32]string)
	a.idToTitle = make(map[uint32]string)

	if a.pathIdx != nil {
		if err := a.pathIdx.ForEachErr(func(id uint32, key string) error {
			a.idToPath[id] = si.Intern(key)
			return nil
		}); err != nil {
			return fmt.Errorf("oza: building path index: %w", err)
		}
	}
	if a.titleIdx != nil {
		if err := a.titleIdx.ForEachErr(func(id uint32, key string) error {
			a.idToTitle[id] = si.Intern(key)
			return nil
		}); err != nil {
			return fmt.Errorf("oza: building title index: %w", err)
		}
	}
	return nil
}

// parseMetadata wraps ParseMetadata and enforces the per-value size limit.
func (a *Archive) parseMetadata(data []byte) (map[string][]byte, error) {
	meta, err := ParseMetadata(data)
	if err != nil {
		return nil, err
	}
	if a.maxMetadataValueSize > 0 {
		for k, v := range meta {
			if int64(len(v)) > a.maxMetadataValueSize {
				return nil, fmt.Errorf("oza: metadata key %q value is %d bytes: %w", k, len(v), ErrMetadataValueTooLarge)
			}
		}
	}
	return meta, nil
}

// parseEntryTable parses the variable-length entry table section.
// Layout: uint32 entry_count, uint32 record_data_offset, uint32[N] offsets, record data.
func (a *Archive) parseEntryTable(data []byte) error {
	if len(data) < EntryTableHeaderSize {
		return fmt.Errorf("oza: entry table too short: %d bytes", len(data))
	}
	count := binary.LittleEndian.Uint32(data[0:4])
	recordDataOff := binary.LittleEndian.Uint32(data[4:8])

	offsetTableEnd := EntryTableHeaderSize + int(count)*4
	if offsetTableEnd > len(data) || int(recordDataOff) > len(data) {
		return fmt.Errorf("oza: entry table truncated: count=%d, len=%d", count, len(data))
	}

	a.entryOffsets = make([]uint32, count)
	for i := range a.entryOffsets {
		a.entryOffsets[i] = binary.LittleEndian.Uint32(data[EntryTableHeaderSize+i*4:])
	}
	a.entryRecords = data[recordDataOff:]
	return nil
}

// contentEntryRecord parses the content entry record for the given ID.
func (a *Archive) contentEntryRecord(id uint32) (EntryRecord, error) {
	if int(id) >= len(a.entryOffsets) {
		return EntryRecord{}, fmt.Errorf("oza: entry ID %d out of range (count=%d)", id, len(a.entryOffsets))
	}
	off := a.entryOffsets[id]
	if int(off) >= len(a.entryRecords) {
		return EntryRecord{}, fmt.Errorf("oza: entry offset %d out of range", off)
	}
	rec, _, err := ParseVarEntryRecord(a.entryRecords[off:])
	if err != nil {
		return EntryRecord{}, err
	}
	if int(rec.MIMEIndex) >= len(a.mimeTypes) {
		return EntryRecord{}, fmt.Errorf("oza: entry %d: mime_index %d out of range (table size %d): %w",
			id, rec.MIMEIndex, len(a.mimeTypes), ErrInvalidEntry)
	}
	rec.ID = id
	return rec, nil
}

// --- Public accessors ---

// EntryCount returns the number of content entries in the archive.
func (a *Archive) EntryCount() uint32 { return uint32(len(a.entryOffsets)) }

// RedirectCount returns the number of redirect entries in the archive.
func (a *Archive) RedirectCount() uint32 { return a.redirectCount }

// UUID returns the archive's unique identifier.
func (a *Archive) UUID() [16]byte { return a.hdr.UUID }

// MIMETypes returns the MIME type table.
func (a *Archive) MIMETypes() []string { return a.mimeTypes }

// Metadata returns the value of a metadata key, or ErrNotFound if absent.
func (a *Archive) Metadata(key string) (string, error) {
	v, ok := a.metadata[key]
	if !ok {
		return "", ErrNotFound
	}
	return string(v), nil
}

// EntryByID returns the entry with the given ID. O(1).
// IDs with bit 31 set are redirect IDs; bits 0-30 index into the redirect table.
func (a *Archive) EntryByID(id uint32) (Entry, error) {
	if IsRedirectID(id) {
		return a.redirectEntryByIndex(id)
	}
	rec, err := a.contentEntryRecord(id)
	if err != nil {
		return Entry{}, err
	}
	return Entry{
		archive: a,
		record:  rec,
		path:    a.idToPath[id],
		title:   a.idToTitle[id],
	}, nil
}

// redirectEntryByIndex builds an Entry from a tagged redirect ID.
func (a *Archive) redirectEntryByIndex(taggedID uint32) (Entry, error) {
	idx := RedirectIndex(taggedID)
	if idx >= a.redirectCount {
		return Entry{}, fmt.Errorf("oza: redirect index %d out of range (count=%d)", idx, a.redirectCount)
	}
	off := 4 + int(idx)*RedirectRecordSize // skip 4-byte count header
	if off+RedirectRecordSize > len(a.redirectData) {
		return Entry{}, fmt.Errorf("oza: redirect record %d out of range", idx)
	}
	rr, err := ParseRedirectRecord(a.redirectData[off:])
	if err != nil {
		return Entry{}, err
	}
	// Build an EntryRecord compatible with the existing Entry API.
	rec := EntryRecord{
		ID:             taggedID,
		Type:           EntryRedirect,
		Flags:          rr.Flags,
		RedirectTarget: rr.TargetID,
	}
	return Entry{
		archive: a,
		record:  rec,
		path:    a.idToPath[taggedID],
		title:   a.idToTitle[taggedID],
	}, nil
}

// EntryByPath looks up an entry by its exact path using binary search.
func (a *Archive) EntryByPath(path string) (Entry, error) {
	if a.pathIdx == nil {
		return Entry{}, fmt.Errorf("oza: no path index")
	}
	id, err := a.pathIdx.Search(path)
	if err != nil {
		return Entry{}, err
	}
	if IsRedirectID(id) {
		e, err := a.redirectEntryByIndex(id)
		if err != nil {
			return Entry{}, err
		}
		e.path = path
		return e, nil
	}
	rec, err := a.contentEntryRecord(id)
	if err != nil {
		return Entry{}, err
	}
	return Entry{archive: a, record: rec, path: path, title: a.idToTitle[id]}, nil
}

// EntryByTitle looks up an entry by its exact title using binary search.
func (a *Archive) EntryByTitle(title string) (Entry, error) {
	if a.titleIdx == nil {
		return Entry{}, fmt.Errorf("oza: no title index")
	}
	id, err := a.titleIdx.Search(title)
	if err != nil {
		return Entry{}, err
	}
	if IsRedirectID(id) {
		e, err := a.redirectEntryByIndex(id)
		if err != nil {
			return Entry{}, err
		}
		e.title = title
		return e, nil
	}
	rec, err := a.contentEntryRecord(id)
	if err != nil {
		return Entry{}, err
	}
	return Entry{archive: a, record: rec, path: a.idToPath[id], title: title}, nil
}

// MainEntry returns the entry pointed to by the "main_entry" metadata key.
func (a *Archive) MainEntry() (Entry, error) {
	path, err := a.Metadata("main_entry")
	if err != nil {
		return Entry{}, fmt.Errorf("oza: main_entry metadata not set")
	}
	return a.EntryByPath(path)
}

// FileHeader returns the archive's parsed file header.
func (a *Archive) FileHeader() Header { return a.hdr }

// Sections returns the archive's section descriptor table.
func (a *Archive) Sections() []SectionDesc { return a.sections }

// HasSearch reports whether the archive contains any trigram search index.
func (a *Archive) HasSearch() bool {
	return a.titleSearchIdx != nil || a.bodySearchIdx != nil
}

// HasTitleSearch reports whether the archive contains a title trigram search index.
func (a *Archive) HasTitleSearch() bool { return a.titleSearchIdx != nil }

// ChunkCount returns the number of chunks in the CONTENT section.
func (a *Archive) ChunkCount() int { return len(a.chunkDescs) }

// CacheStats returns the chunk cache's current fill, capacity, and lifetime hit/miss counts.
func (a *Archive) CacheStats() (current, capacity int, hits, misses int64) {
	return a.cache.stats()
}

// ForEachTitleKey calls fn for each title string in title-sorted order.
// It is O(N) — significantly faster than iterating via EntriesByTitle, which
// uses Record(i) and re-decodes from a restart block on every call.
func (a *Archive) ForEachTitleKey(fn func(title string)) {
	if a.titleIdx == nil {
		return
	}
	a.titleIdx.ForEach(func(_ uint32, title string) { fn(title) })
}

// ForEachEntryRecord calls fn for each content entry record in ID order.
// Unlike Entries or FrontArticles, it skips the idToPath/idToTitle map lookups,
// making it more efficient when only the record fields (e.g. flags) are needed.
func (a *Archive) ForEachEntryRecord(fn func(id uint32, rec EntryRecord)) {
	for i := uint32(0); i < uint32(len(a.entryOffsets)); i++ {
		off := a.entryOffsets[i]
		if int(off) >= len(a.entryRecords) {
			continue
		}
		rec, _, err := ParseVarEntryRecord(a.entryRecords[off:])
		if err != nil {
			continue
		}
		rec.ID = i
		fn(i, rec)
	}
}

// HasBodySearch reports whether the archive contains a body trigram search index.
func (a *Archive) HasBodySearch() bool { return a.bodySearchIdx != nil }

// TitleCount returns the number of records in the title index (content + redirect
// entries combined). Returns 0 if the archive has no title index.
func (a *Archive) TitleCount() int {
	if a.titleIdx == nil {
		return 0
	}
	return a.titleIdx.Count()
}

// BrowseTitles returns up to limit entries from the title index in alphabetical
// order, starting at the given offset. Intended for paginated browsing; O(limit)
// per call. Returns nil if the archive has no title index or offset is out of range.
func (a *Archive) BrowseTitles(offset, limit int) []Entry {
	if a.titleIdx == nil || offset < 0 || limit <= 0 {
		return nil
	}
	total := a.titleIdx.Count()
	if offset >= total {
		return nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	result := make([]Entry, 0, end-offset)
	for i := offset; i < end; i++ {
		id, title, err := a.titleIdx.Record(i)
		if err != nil {
			break
		}
		if e, ok := a.entryFromIndex(id, title, false); ok {
			result = append(result, e)
		}
	}
	return result
}

// TitleSearchDocCount returns the number of distinct entry IDs in the title search
// index (SectionSearchTitle) and whether the index is present.
func (a *Archive) TitleSearchDocCount() (uint32, bool) {
	if a.titleSearchIdx == nil {
		return 0, false
	}
	return a.titleSearchIdx.DocCount(), true
}

// BodySearchDocCount returns the number of distinct entry IDs in the body search
// index (SectionSearchBody) and whether the index is present.
func (a *Archive) BodySearchDocCount() (uint32, bool) {
	if a.bodySearchIdx == nil {
		return 0, false
	}
	return a.bodySearchIdx.DocCount(), true
}

// SearchOptions controls search behavior.
type SearchOptions struct {
	Limit     int  // max results; 0 = default 20
	TitleOnly bool // search only the title index
}

// SearchResult wraps an Entry with ranking metadata.
type SearchResult struct {
	Entry      Entry
	TitleMatch bool // true if found in title search index
	BodyMatch  bool // true if found in body search index
}

// Search returns ranked results. Title matches sort before body-only matches.
// Within each tier, results are sorted by entry ID.
func (a *Archive) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	if !a.HasSearch() {
		return nil, fmt.Errorf("oza: archive has no search index")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}

	return a.searchTwoTier(query, limit, opts.TitleOnly)
}

// searchTwoTier implements ranked search across title and body indices.
func (a *Archive) searchTwoTier(query string, limit int, titleOnly bool) ([]SearchResult, error) {
	titleIDs := map[uint32]bool{}
	if a.titleSearchIdx != nil {
		for _, id := range a.titleSearchIdx.Search(query, 0) {
			titleIDs[id] = true
		}
	}

	bodyIDs := map[uint32]bool{}
	if !titleOnly && a.bodySearchIdx != nil {
		for _, id := range a.bodySearchIdx.Search(query, 0) {
			bodyIDs[id] = true
		}
	}

	// Collect all unique IDs, marking which index matched.
	type match struct {
		id    uint32
		title bool
		body  bool
	}
	seen := map[uint32]*match{}
	// Title matches first (preserves ID order from the sorted posting intersection).
	for id := range titleIDs {
		seen[id] = &match{id: id, title: true}
	}
	for id := range bodyIDs {
		if m, ok := seen[id]; ok {
			m.body = true
		} else {
			seen[id] = &match{id: id, body: true}
		}
	}

	// Separate into title-match and body-only groups, each sorted by ID.
	var titleMatches, bodyOnlyMatches []match
	for _, m := range seen {
		if m.title {
			titleMatches = append(titleMatches, *m)
		} else {
			bodyOnlyMatches = append(bodyOnlyMatches, *m)
		}
	}
	sortByID := func(s []match) {
		for i := 1; i < len(s); i++ {
			for j := i; j > 0 && s[j].id < s[j-1].id; j-- {
				s[j], s[j-1] = s[j-1], s[j]
			}
		}
	}
	sortByID(titleMatches)
	sortByID(bodyOnlyMatches)

	// Merge: title matches first, then body-only.
	merged := make([]match, 0, len(titleMatches)+len(bodyOnlyMatches))
	merged = append(merged, titleMatches...)
	merged = append(merged, bodyOnlyMatches...)

	if len(merged) > limit {
		merged = merged[:limit]
	}

	results := make([]SearchResult, 0, len(merged))
	for _, m := range merged {
		e, err := a.EntryByID(m.id)
		if err != nil {
			continue
		}
		results = append(results, SearchResult{
			Entry:      e,
			TitleMatch: m.title,
			BodyMatch:  m.body,
		})
	}
	return results, nil
}

// SearchTitles is a convenience for title-only search (autocomplete).
func (a *Archive) SearchTitles(query string, limit int) ([]SearchResult, error) {
	return a.Search(query, SearchOptions{Limit: limit, TitleOnly: true})
}

// AllMetadata returns a copy of all metadata key-value pairs.
func (a *Archive) AllMetadata() map[string][]byte {
	m := make(map[string][]byte, len(a.metadata))
	for k, v := range a.metadata {
		m[k] = v
	}
	return m
}

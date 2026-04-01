// Package stats computes comprehensive archive statistics for ozainfo and ozaserve.
package stats

import (
	"fmt"
	"math"
	"sort"

	"github.com/stazelabs/oza/oza"
)

// ArchiveStats is the top-level statistics structure returned by Collect.
type ArchiveStats struct {
	FileSize       int64             `json:"file_size"`
	Header         HeaderStats       `json:"header"`
	Sections       []SectionStats    `json:"sections"`
	SectionSummary SectionSummary    `json:"section_summary"`
	MIMETypes      []string          `json:"mime_types"`
	MIMECensus     []MIMECensusEntry `json:"mime_census"`
	EntryStats     EntryStats        `json:"entry_stats"`
	ChunkStats     ChunkStats        `json:"chunk_stats"`
	SearchStats    SearchStats       `json:"search_stats"`
	Metadata       map[string]string `json:"metadata"`
}

// HeaderStats contains header-level archive information.
type HeaderStats struct {
	Magic             uint32   `json:"magic"`
	MajorVersion      uint16   `json:"major_version"`
	MinorVersion      uint16   `json:"minor_version"`
	UUID              string   `json:"uuid"`
	SectionCount      uint32   `json:"section_count"`
	EntryCount        uint32   `json:"entry_count"`
	RedirectCount     uint32   `json:"redirect_count"`
	FrontArticleCount uint32   `json:"front_article_count"`
	ContentSize       uint64   `json:"content_size"`
	Flags             uint32   `json:"flags"`
	FlagNames         []string `json:"flag_names,omitempty"`
}

// SectionStats describes a single archive section.
type SectionStats struct {
	Index            int    `json:"index"`
	Type             string `json:"type"`
	CompressedSize   uint64 `json:"compressed_size"`
	UncompressedSize uint64 `json:"uncompressed_size"`
	Compression      string `json:"compression"`
	SHA256           string `json:"sha256"`
}

// SectionSummary aggregates compressed/uncompressed totals across all sections.
type SectionSummary struct {
	TotalCompressed   int64   `json:"total_compressed"`
	TotalUncompressed int64   `json:"total_uncompressed"`
	Ratio             float64 `json:"ratio"` // compressed / uncompressed
}

// MIMECensusEntry holds per-MIME-type entry count and size statistics.
type MIMECensusEntry struct {
	MIMEType   string  `json:"mime_type"`
	Count      int     `json:"count"`
	TotalBytes int64   `json:"total_bytes"`
	AvgBytes   float64 `json:"avg_bytes"`
	MinBytes   uint32  `json:"min_bytes"`
	MaxBytes   uint32  `json:"max_bytes"`
}

// EntryStats holds aggregate entry-level statistics.
type EntryStats struct {
	ContentEntries uint32 `json:"content_entries"`
	Redirects      uint32 `json:"redirects"`
	FrontArticles  uint32 `json:"front_articles"`
	MetadataRefs   uint32 `json:"metadata_refs"`
	TotalBlobBytes int64  `json:"total_blob_bytes"`
}

// ChunkStats holds chunk-level statistics.
type ChunkStats struct {
	ChunkCount         int     `json:"chunk_count"`
	AvgEntriesPerChunk float64 `json:"avg_entries_per_chunk"`
	MinEntriesPerChunk int     `json:"min_entries_per_chunk"`
	MaxEntriesPerChunk int     `json:"max_entries_per_chunk"`
}

// SearchStats holds search index availability and doc counts.
type SearchStats struct {
	HasTitleSearch bool   `json:"has_title_search"`
	HasBodySearch  bool   `json:"has_body_search"`
	TitleDocCount  uint32 `json:"title_doc_count,omitempty"`
	BodyDocCount   uint32 `json:"body_doc_count,omitempty"`
}

// mimeCensusAccum accumulates per-MIME-type statistics during iteration.
type mimeCensusAccum struct {
	count      int
	totalBytes int64
	minBytes   uint32
	maxBytes   uint32
}

// Collect computes comprehensive statistics for an open archive.
// fileSize should be the on-disk file size (from os.Stat); pass 0 if unavailable.
func Collect(a *oza.Archive, fileSize int64) ArchiveStats {
	var s ArchiveStats
	s.FileSize = fileSize

	// Header.
	s.Header = collectHeader(a)

	// Sections.
	s.Sections, s.SectionSummary = collectSections(a)

	// MIME types.
	s.MIMETypes = a.MIMETypes()

	// Single-pass entry iteration for MIME census, chunk stats, and entry stats.
	s.MIMECensus, s.EntryStats, s.ChunkStats = collectEntryStats(a, s.MIMETypes)

	// Search stats.
	s.SearchStats = collectSearchStats(a)

	// Metadata.
	s.Metadata = collectMetadata(a)

	return s
}

func collectHeader(a *oza.Archive) HeaderStats {
	hdr := a.FileHeader()
	h := HeaderStats{
		Magic:             hdr.Magic,
		MajorVersion:      hdr.MajorVersion,
		MinorVersion:      hdr.MinorVersion,
		UUID:              formatUUID(hdr.UUID),
		SectionCount:      hdr.SectionCount,
		EntryCount:        hdr.EntryCount,
		RedirectCount:     a.RedirectCount(),
		FrontArticleCount: hdr.FrontArticleCount,
		ContentSize:       hdr.ContentSize,
		Flags:             hdr.Flags,
	}
	if hdr.HasSearch() {
		h.FlagNames = append(h.FlagNames, "has-search")
	}
	if hdr.HasChrome() {
		h.FlagNames = append(h.FlagNames, "has-chrome")
	}
	if hdr.HasSignatures() {
		h.FlagNames = append(h.FlagNames, "has-signatures")
	}
	return h
}

func collectSections(a *oza.Archive) ([]SectionStats, SectionSummary) {
	sections := a.Sections()
	out := make([]SectionStats, len(sections))
	var summary SectionSummary
	for i, sec := range sections {
		out[i] = SectionStats{
			Index:            i,
			Type:             sec.Type.String(),
			CompressedSize:   sec.CompressedSize,
			UncompressedSize: sec.UncompressedSize,
			Compression:      oza.CompressionName(sec.Compression),
			SHA256:           fmt.Sprintf("%x", sec.SHA256),
		}
		summary.TotalCompressed += int64(sec.CompressedSize)
		if sec.UncompressedSize > 0 {
			summary.TotalUncompressed += int64(sec.UncompressedSize)
		}
	}
	if summary.TotalUncompressed > 0 {
		summary.Ratio = float64(summary.TotalCompressed) / float64(summary.TotalUncompressed)
	}
	return out, summary
}

func collectEntryStats(a *oza.Archive, mimeTypes []string) ([]MIMECensusEntry, EntryStats, ChunkStats) {
	mimeAccum := make(map[uint16]*mimeCensusAccum)
	chunkEntries := make(map[uint32]int)
	var es EntryStats

	a.ForEachEntryRecord(func(_ uint32, rec oza.EntryRecord) {
		switch rec.Type {
		case oza.EntryContent:
			es.ContentEntries++
			es.TotalBlobBytes += int64(rec.BlobSize)

			acc, ok := mimeAccum[rec.MIMEIndex]
			if !ok {
				acc = &mimeCensusAccum{minBytes: math.MaxUint32}
				mimeAccum[rec.MIMEIndex] = acc
			}
			acc.count++
			acc.totalBytes += int64(rec.BlobSize)
			if rec.BlobSize < acc.minBytes {
				acc.minBytes = rec.BlobSize
			}
			if rec.BlobSize > acc.maxBytes {
				acc.maxBytes = rec.BlobSize
			}

			chunkEntries[rec.ChunkID]++
		case oza.EntryRedirect:
			es.Redirects++
		case oza.EntryMetadataRef:
			es.MetadataRefs++
		}
		if rec.IsFrontArticle() {
			es.FrontArticles++
		}
	})

	// Build MIME census, sorted by count descending.
	census := make([]MIMECensusEntry, 0, len(mimeAccum))
	for idx, acc := range mimeAccum {
		name := fmt.Sprintf("unknown(%d)", idx)
		if int(idx) < len(mimeTypes) {
			name = mimeTypes[idx]
		}
		census = append(census, MIMECensusEntry{
			MIMEType:   name,
			Count:      acc.count,
			TotalBytes: acc.totalBytes,
			AvgBytes:   float64(acc.totalBytes) / float64(acc.count),
			MinBytes:   acc.minBytes,
			MaxBytes:   acc.maxBytes,
		})
	}
	sort.Slice(census, func(i, j int) bool { return census[i].Count > census[j].Count })

	// Build chunk stats.
	cs := ChunkStats{ChunkCount: a.ChunkCount()}
	if len(chunkEntries) > 0 {
		cs.MinEntriesPerChunk = math.MaxInt
		var totalEntries int
		for _, n := range chunkEntries {
			totalEntries += n
			if n < cs.MinEntriesPerChunk {
				cs.MinEntriesPerChunk = n
			}
			if n > cs.MaxEntriesPerChunk {
				cs.MaxEntriesPerChunk = n
			}
		}
		cs.AvgEntriesPerChunk = float64(totalEntries) / float64(len(chunkEntries))
	}

	return census, es, cs
}

func collectSearchStats(a *oza.Archive) SearchStats {
	ss := SearchStats{
		HasTitleSearch: a.HasTitleSearch(),
		HasBodySearch:  a.HasBodySearch(),
	}
	if count, ok := a.TitleSearchDocCount(); ok {
		ss.TitleDocCount = count
	}
	if count, ok := a.BodySearchDocCount(); ok {
		ss.BodyDocCount = count
	}
	return ss
}

func collectMetadata(a *oza.Archive) map[string]string {
	raw := a.AllMetadata()
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if isBinary(v) {
			out[k] = fmt.Sprintf("<binary %d bytes>", len(v))
		} else {
			out[k] = string(v)
		}
	}
	return out
}

func formatUUID(uuid [16]byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

func isBinary(b []byte) bool {
	for _, c := range b {
		if (c < 0x20 && c != '\t' && c != '\n' && c != '\r') || c > 0x7e {
			return true
		}
	}
	return false
}

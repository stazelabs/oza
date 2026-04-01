package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/stazelabs/gozim/zim"
)

// ZIMStats holds statistics collected from a ZIM archive.
type ZIMStats struct {
	FileSize          int64                `json:"file_size"`
	EntryTotal        int                  `json:"entry_total"`
	ContentCount      int                  `json:"content_count"`
	RedirectCount     int                  `json:"redirect_count"`
	SkippedCount      int                  `json:"skipped_count"`
	MetadataCount     int                  `json:"metadata_count"`
	MIMECensus        []ZIMMIMECensusEntry `json:"mime_census"`
	Metadata          map[string]string    `json:"metadata"`
	TotalContentBytes int64                `json:"total_content_bytes,omitempty"` // only with --deep
}

// ZIMMIMECensusEntry holds per-MIME-type statistics for a ZIM archive.
type ZIMMIMECensusEntry struct {
	MIMEType   string  `json:"mime_type"`
	Count      int     `json:"count"`
	TotalBytes int64   `json:"total_bytes,omitempty"` // only with --deep
	AvgBytes   float64 `json:"avg_bytes,omitempty"`   // only with --deep
}

// mimeCensusAccum accumulates per-MIME statistics during iteration.
type zimMIMEAccum struct {
	count      int
	totalBytes int64
}

// collectZIM scans a ZIM archive and returns statistics.
// If deep is true, all content is read to compute byte-size statistics (slow).
func collectZIM(path string, deep bool) (ZIMStats, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return ZIMStats{}, fmt.Errorf("stat %s: %w", path, err)
	}

	za, err := zim.OpenWithOptions(path, zim.WithCacheSize(8))
	if err != nil {
		return ZIMStats{}, fmt.Errorf("open ZIM: %w", err)
	}
	defer za.Close()

	var s ZIMStats
	s.FileSize = fi.Size()
	s.Metadata = make(map[string]string)

	// Pass 1: collect metadata from M/ namespace.
	for entry := range za.EntriesByNamespace('M') {
		s.EntryTotal++
		s.MetadataCount++
		key := strings.ToLower(entry.Path())
		content, err := entry.ReadContent()
		if err != nil {
			continue
		}
		if !isBinaryBytes(content) {
			s.Metadata[key] = string(content)
		}
	}

	// Pass 2: classify and count all entries.
	mimeAccum := make(map[string]*zimMIMEAccum)

	// For deep mode, we collect entries to sort by cluster order before reading.
	type contentRef struct {
		entry    zim.Entry
		mimeType string
	}
	var contentRefs []contentRef

	for entry := range za.Entries() {
		ns := entry.Namespace()
		if ns == 'M' {
			continue // already counted
		}
		s.EntryTotal++

		cat := classifyNamespace(ns, entry.Path())
		switch cat {
		case catSkip:
			s.SkippedCount++
			continue
		case catContent:
			// process below
		}

		if entry.IsRedirect() {
			s.RedirectCount++
			continue
		}

		s.ContentCount++
		mime := entry.MIMEType()

		acc, ok := mimeAccum[mime]
		if !ok {
			acc = &zimMIMEAccum{}
			mimeAccum[mime] = acc
		}
		acc.count++

		if deep {
			contentRefs = append(contentRefs, contentRef{entry: entry, mimeType: mime})
		}
	}

	// Deep mode: read all content to get byte sizes.
	if deep && len(contentRefs) > 0 {
		// Sort by cluster order for sequential I/O.
		sort.Slice(contentRefs, func(i, j int) bool {
			ci := contentRefs[i].entry.ClusterNum()
			cj := contentRefs[j].entry.ClusterNum()
			if ci != cj {
				return ci < cj
			}
			return contentRefs[i].entry.BlobNum() < contentRefs[j].entry.BlobNum()
		})

		for _, ref := range contentRefs {
			content, err := ref.entry.ReadContentCopy()
			if err != nil {
				continue
			}
			n := int64(len(content))
			s.TotalContentBytes += n
			mimeAccum[ref.mimeType].totalBytes += n
		}
	}

	// Build sorted MIME census.
	s.MIMECensus = make([]ZIMMIMECensusEntry, 0, len(mimeAccum))
	for mime, acc := range mimeAccum {
		entry := ZIMMIMECensusEntry{
			MIMEType: mime,
			Count:    acc.count,
		}
		if deep && acc.count > 0 {
			entry.TotalBytes = acc.totalBytes
			entry.AvgBytes = float64(acc.totalBytes) / float64(acc.count)
		}
		s.MIMECensus = append(s.MIMECensus, entry)
	}
	sort.Slice(s.MIMECensus, func(i, j int) bool {
		return s.MIMECensus[i].Count > s.MIMECensus[j].Count
	})

	return s, nil
}

type entryCategory int

const (
	catContent entryCategory = iota
	catSkip
)

// classifyNamespace maps a ZIM namespace and path to a category.
func classifyNamespace(ns byte, path string) entryCategory {
	switch ns {
	case 'C':
		if strings.HasPrefix(path, "_mw_/") {
			return catSkip // chrome
		}
		return catContent
	case '-':
		return catContent
	case 'W':
		return catContent
	case 'M', 'X':
		return catSkip
	default:
		return catContent
	}
}

func isBinaryBytes(b []byte) bool {
	for _, c := range b {
		if (c < 0x20 && c != '\t' && c != '\n' && c != '\r') || c > 0x7e {
			return true
		}
	}
	return false
}

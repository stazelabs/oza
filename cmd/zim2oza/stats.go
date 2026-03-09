package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Stats holds conversion statistics.
type Stats struct {
	InputSize     int64
	OutputSize    int64
	EntryTotal    int
	EntryContent  int
	EntryRedirect int

	BytesRead   int64 // total bytes read from ZIM content entries
	CacheHits   int64 // gozim cluster cache hits
	CacheMisses int64 // gozim cluster cache misses

	TimeScan        time.Duration // scan + classify all ZIM entries
	TimeRead        time.Duration // time inside ReadContent calls
	TimeTransform   time.Duration // minify + image optimize (in AddEntry)
	TimeDedup       time.Duration // SHA-256 + dedup lookup (in AddEntry)
	TimeSearchIndex time.Duration // trigram search index build (in AddEntry + Close)
	TimeChunkBuild  time.Duration // chunk building + flush (in AddEntry)
	TimeDictTrain   time.Duration // Zstd dictionary training
	TimeCompress    time.Duration // chunk compression
	TimeAssemble    time.Duration // serialization + disk write
	TimeClose       time.Duration // total writer Close() time
	TimeTotal       time.Duration // wall clock for entire conversion
}

// Print writes a formatted statistics table to w.
func (s *Stats) Print(w io.Writer) {
	fmt.Fprintln(w, "=== Conversion Statistics ===")
	fmt.Fprintf(w, "  Input size:      %s\n", formatBytes(s.InputSize))
	fmt.Fprintf(w, "  Output size:     %s\n", formatBytes(s.OutputSize))
	if s.InputSize > 0 && s.OutputSize > 0 {
		ratio := float64(s.OutputSize) / float64(s.InputSize)
		fmt.Fprintf(w, "  Size ratio:      %.2f\n", ratio)
	}
	fmt.Fprintf(w, "  Entries total:   %d\n", s.EntryTotal)
	fmt.Fprintf(w, "  Content entries: %d\n", s.EntryContent)
	fmt.Fprintf(w, "  Redirects:       %d\n", s.EntryRedirect)
	fmt.Fprintf(w, "  Bytes read:      %s\n", formatBytes(s.BytesRead))
	if s.TimeRead > 0 && s.EntryContent > 0 {
		rate := float64(s.EntryContent) / s.TimeRead.Seconds()
		fmt.Fprintf(w, "  Read rate:       %.0f entries/sec\n", rate)
	}

	fmt.Fprintln(w, "--- Timings ---")
	fmt.Fprintf(w, "  Scan:            %s\n", s.TimeScan.Round(time.Millisecond))
	fmt.Fprintf(w, "  Read content:    %s\n", s.TimeRead.Round(time.Millisecond))
	fmt.Fprintf(w, "  Transform:       %s\n", s.TimeTransform.Round(time.Millisecond))
	fmt.Fprintf(w, "  Dedup (SHA-256): %s\n", s.TimeDedup.Round(time.Millisecond))
	fmt.Fprintf(w, "  Search index:    %s\n", s.TimeSearchIndex.Round(time.Millisecond))
	fmt.Fprintf(w, "  Chunk build:     %s\n", s.TimeChunkBuild.Round(time.Millisecond))
	fmt.Fprintf(w, "  Dict train:      %s\n", s.TimeDictTrain.Round(time.Millisecond))
	fmt.Fprintf(w, "  Compress:        %s\n", s.TimeCompress.Round(time.Millisecond))
	fmt.Fprintf(w, "  Assemble+write:  %s\n", s.TimeAssemble.Round(time.Millisecond))
	fmt.Fprintf(w, "  Close() total:   %s\n", s.TimeClose.Round(time.Millisecond))
	fmt.Fprintf(w, "  Total:           %s\n", s.TimeTotal.Round(time.Millisecond))

	if s.CacheHits+s.CacheMisses > 0 {
		total := s.CacheHits + s.CacheMisses
		hitPct := float64(s.CacheHits) / float64(total) * 100
		fmt.Fprintf(w, "--- Cluster Cache ---\n")
		fmt.Fprintf(w, "  Hits:            %d (%.1f%%)\n", s.CacheHits, hitPct)
		fmt.Fprintf(w, "  Misses:          %d (%.1f%%)\n", s.CacheMisses, 100-hitPct)
	}
}

// PrintJSON writes statistics as a JSON object to w.
func (s *Stats) PrintJSON(w io.Writer) error {
	obj := struct {
		InputSize      int64   `json:"input_size_bytes"`
		OutputSize     int64   `json:"output_size_bytes"`
		SizeRatio      float64 `json:"size_ratio,omitempty"`
		EntryTotal     int     `json:"entry_total"`
		EntryContent   int     `json:"entry_content"`
		EntryRedirect  int     `json:"entry_redirect"`
		BytesRead      int64   `json:"bytes_read"`
		CacheHits      int64   `json:"cache_hits"`
		CacheMisses    int64   `json:"cache_misses"`
		TimeScanMs     int64   `json:"time_scan_ms"`
		TimeReadMs     int64   `json:"time_read_ms"`
		TimeTransMs    int64   `json:"time_transform_ms"`
		TimeDedupMs    int64   `json:"time_dedup_ms"`
		TimeSearchMs   int64   `json:"time_search_index_ms"`
		TimeChunkMs    int64   `json:"time_chunk_build_ms"`
		TimeDictMs     int64   `json:"time_dict_train_ms"`
		TimeCompMs     int64   `json:"time_compress_ms"`
		TimeAssemMs    int64   `json:"time_assemble_ms"`
		TimeCloseMs    int64   `json:"time_close_ms"`
		TimeTotalMs    int64   `json:"time_total_ms"`
	}{
		InputSize:      s.InputSize,
		OutputSize:     s.OutputSize,
		EntryTotal:     s.EntryTotal,
		EntryContent:   s.EntryContent,
		EntryRedirect:  s.EntryRedirect,
		BytesRead:      s.BytesRead,
		CacheHits:      s.CacheHits,
		CacheMisses:    s.CacheMisses,
		TimeScanMs:     s.TimeScan.Milliseconds(),
		TimeReadMs:     s.TimeRead.Milliseconds(),
		TimeTransMs:    s.TimeTransform.Milliseconds(),
		TimeDedupMs:    s.TimeDedup.Milliseconds(),
		TimeSearchMs:   s.TimeSearchIndex.Milliseconds(),
		TimeChunkMs:    s.TimeChunkBuild.Milliseconds(),
		TimeDictMs:     s.TimeDictTrain.Milliseconds(),
		TimeCompMs:     s.TimeCompress.Milliseconds(),
		TimeAssemMs:    s.TimeAssemble.Milliseconds(),
		TimeCloseMs:    s.TimeClose.Milliseconds(),
		TimeTotalMs:    s.TimeTotal.Milliseconds(),
	}
	if s.InputSize > 0 && s.OutputSize > 0 {
		obj.SizeRatio = float64(s.OutputSize) / float64(s.InputSize)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(obj)
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.2f GiB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.2f MiB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.2f KiB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

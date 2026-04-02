package main

import (
	"fmt"
	"io"
	"time"

	"github.com/stazelabs/oza/ozawrite"
)

// Stats holds conversion statistics.
type Stats struct {
	InputSize     int64
	OutputSize    int64
	EntryTotal    int
	EntryContent  int
	EntryRedirect int

	// Transcode stats (populated from ozawrite.TranscodeTools.Stats).
	TranscodeGIFCount   int
	TranscodeGIFSaved   int64
	TranscodeGIFSkipped int
	TranscodePNGCount   int
	TranscodePNGSaved   int64
	TranscodePNGSkipped int
	TranscodeJPEGCount  int
	TranscodeJPEGSaved  int64
	TranscodeJPEGSkip   int
	TranscodeAVIFCount  int
	TranscodeAVIFSaved  int64
	TranscodeAVIFSkip   int

	TimeTransform   time.Duration
	TimeDedup       time.Duration
	TimeSearchIndex time.Duration
	TimeChunkBuild  time.Duration
	TimeDictTrain   time.Duration
	TimeCompress    time.Duration
	TimeAssemble    time.Duration
	TimeTotal       time.Duration
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
	if s.EntryRedirect > 0 {
		fmt.Fprintf(w, "  Redirects:       %d\n", s.EntryRedirect)
	}

	fmt.Fprintln(w, "--- Timings ---")
	fmt.Fprintf(w, "  Transform:       %s\n", s.TimeTransform.Round(time.Millisecond))
	fmt.Fprintf(w, "  Dedup:           %s\n", s.TimeDedup.Round(time.Millisecond))
	fmt.Fprintf(w, "  Search index:    %s\n", s.TimeSearchIndex.Round(time.Millisecond))
	fmt.Fprintf(w, "  Chunk build:     %s\n", s.TimeChunkBuild.Round(time.Millisecond))
	fmt.Fprintf(w, "  Dict train:      %s\n", s.TimeDictTrain.Round(time.Millisecond))
	fmt.Fprintf(w, "  Compress:        %s\n", s.TimeCompress.Round(time.Millisecond))
	fmt.Fprintf(w, "  Assemble+write:  %s\n", s.TimeAssemble.Round(time.Millisecond))
	fmt.Fprintf(w, "  Total:           %s\n", s.TimeTotal.Round(time.Millisecond))

	hasTranscode := s.TranscodeGIFCount+s.TranscodeGIFSkipped+
		s.TranscodePNGCount+s.TranscodePNGSkipped+
		s.TranscodeJPEGCount+s.TranscodeJPEGSkip+
		s.TranscodeAVIFCount+s.TranscodeAVIFSkip > 0
	if hasTranscode {
		fmt.Fprintln(w, "--- Transcode ---")
		if s.TranscodeGIFCount+s.TranscodeGIFSkipped > 0 {
			fmt.Fprintf(w, "  GIF→WebP:        %d transcoded, %d skipped, %s saved\n",
				s.TranscodeGIFCount, s.TranscodeGIFSkipped, formatBytes(s.TranscodeGIFSaved))
		}
		if s.TranscodePNGCount+s.TranscodePNGSkipped > 0 {
			fmt.Fprintf(w, "  PNG→WebP:        %d transcoded, %d skipped, %s saved\n",
				s.TranscodePNGCount, s.TranscodePNGSkipped, formatBytes(s.TranscodePNGSaved))
		}
		if s.TranscodeJPEGCount+s.TranscodeJPEGSkip > 0 {
			fmt.Fprintf(w, "  JPEG→WebP:       %d transcoded, %d skipped, %s saved\n",
				s.TranscodeJPEGCount, s.TranscodeJPEGSkip, formatBytes(s.TranscodeJPEGSaved))
		}
		if s.TranscodeAVIFCount+s.TranscodeAVIFSkip > 0 {
			fmt.Fprintf(w, "  →AVIF:           %d transcoded, %d skipped, %s saved\n",
				s.TranscodeAVIFCount, s.TranscodeAVIFSkip, formatBytes(s.TranscodeAVIFSaved))
		}
	}
}

// populateTranscodeStats copies transcode statistics from TranscodeTools into Stats.
func populateTranscodeStats(s *Stats, tools *ozawrite.TranscodeTools) {
	if tools == nil {
		return
	}
	ts := tools.Stats()
	s.TranscodeGIFCount = ts.GIFCount
	s.TranscodeGIFSaved = ts.GIFSaved
	s.TranscodeGIFSkipped = ts.GIFSkipped
	s.TranscodePNGCount = ts.PNGCount
	s.TranscodePNGSaved = ts.PNGSaved
	s.TranscodePNGSkipped = ts.PNGSkipped
	s.TranscodeJPEGCount = ts.JPEGCount
	s.TranscodeJPEGSaved = ts.JPEGSaved
	s.TranscodeJPEGSkip = ts.JPEGSkipped
	s.TranscodeAVIFCount = ts.AVIFCount
	s.TranscodeAVIFSaved = ts.AVIFSaved
	s.TranscodeAVIFSkip = ts.AVIFSkipped
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

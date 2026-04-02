package main

import (
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
	fmt.Fprintf(w, "  Entries:         %d content\n", s.EntryContent)
	fmt.Fprintln(w, "--- Timings ---")
	fmt.Fprintf(w, "  Transform:       %s\n", s.TimeTransform)
	fmt.Fprintf(w, "  Dedup:           %s\n", s.TimeDedup)
	fmt.Fprintf(w, "  Search index:    %s\n", s.TimeSearchIndex)
	fmt.Fprintf(w, "  Chunk build:     %s\n", s.TimeChunkBuild)
	fmt.Fprintf(w, "  Dict training:   %s\n", s.TimeDictTrain)
	fmt.Fprintf(w, "  Compression:     %s\n", s.TimeCompress)
	fmt.Fprintf(w, "  Assembly:        %s\n", s.TimeAssemble)
	fmt.Fprintf(w, "  Total:           %s\n", s.TimeTotal)
}

func formatBytes(b int64) string {
	const (
		kB = 1024
		mB = kB * 1024
		gB = mB * 1024
	)
	switch {
	case b >= gB:
		return fmt.Sprintf("%.2f GiB", float64(b)/float64(gB))
	case b >= mB:
		return fmt.Sprintf("%.2f MiB", float64(b)/float64(mB))
	case b >= kB:
		return fmt.Sprintf("%.2f KiB", float64(b)/float64(kB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

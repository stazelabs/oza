package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/cmd/internal/classify"
	"github.com/stazelabs/oza/cmd/internal/stats"
	"github.com/stazelabs/oza/oza"
)

var (
	jsonOutput   bool
	classifyFlag bool
)

func main() {
	root := &cobra.Command{
		Use:   "ozainfo <archive.oza>",
		Short: "Dump metadata and section table of an OZA archive",
		Long: `王座 ozainfo — display archive metadata, section table, and statistics.

Shows header info, metadata key-value pairs, section sizes and compression
ratios, entry counts, chunk stats, and search index status. Use --json
for machine-readable output or --classify for content profile analysis.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args[0])
		},
	}
	root.Flags().BoolVar(&jsonOutput, "json", false, "output statistics as JSON")
	root.Flags().BoolVar(&classifyFlag, "classify", false, "classify archive content profile and show recommendations")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(path string) error {
	// Get file size before opening.
	var fileSize int64
	if fi, err := os.Stat(path); err == nil {
		fileSize = fi.Size()
	}

	a, err := oza.Open(path)
	if err != nil {
		return err
	}
	defer a.Close()

	st := stats.Collect(a, fileSize)

	if jsonOutput {
		out := struct {
			stats.ArchiveStats
			Classification *classify.Result `json:"classification,omitempty"`
		}{ArchiveStats: st}
		if classifyFlag {
			cr := classify.Classify(classify.ExtractFromOZA(st))
			out.Classification = &cr
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	printText(path, a, st)
	if classifyFlag {
		printClassification(st)
	}
	return nil
}

func printText(path string, a *oza.Archive, st stats.ArchiveStats) {
	hdr := a.FileHeader()

	// --- Existing output (unchanged) ---
	fmt.Printf("File:          %s\n", path)
	fmt.Printf("Magic:         0x%08X\n", hdr.Magic)
	fmt.Printf("Version:       %d.%d\n", hdr.MajorVersion, hdr.MinorVersion)
	fmt.Printf("UUID:          %s\n", st.Header.UUID)
	fmt.Printf("Sections:      %d\n", hdr.SectionCount)
	fmt.Printf("Entries:       %d\n", hdr.EntryCount)
	fmt.Printf("Redirects:     %d\n", a.RedirectCount())
	fmt.Printf("Content size:  %d bytes\n", hdr.ContentSize)
	fmt.Printf("Flags:         0x%08X%s\n", hdr.Flags, formatFlags(hdr))
	fmt.Println()

	// Section table.
	sections := a.Sections()
	fmt.Printf("Section Table (%d sections):\n", len(sections))
	fmt.Printf("  %-3s  %-15s  %-12s  %-12s  %-12s  %-10s  %s\n",
		"#", "Type", "Offset", "Comp.Size", "Uncomp.Size", "Compr.", "SHA-256")
	for i, s := range sections {
		sha := fmt.Sprintf("%x", s.SHA256)
		if len(sha) > 16 {
			sha = sha[:16] + "..."
		}
		fmt.Printf("  %-3d  %-15s  0x%010x  %-12d  %-12d  %-10s  %s\n",
			i, s.Type.String(), s.Offset,
			s.CompressedSize, s.UncompressedSize,
			oza.CompressionName(s.Compression), sha)
	}
	fmt.Println()

	// MIME types.
	mimeTypes := a.MIMETypes()
	fmt.Printf("MIME Types (%d):\n", len(mimeTypes))
	for i, mt := range mimeTypes {
		fmt.Printf("  %3d  %s\n", i, mt)
	}
	fmt.Println()

	// Metadata.
	meta := a.AllMetadata()
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("Metadata (%d keys):\n", len(keys))
	for _, k := range keys {
		raw := meta[k]
		var v string
		if isBinary(raw) {
			v = fmt.Sprintf("<binary %d bytes>", len(raw))
		} else {
			v = string(raw)
			if len(v) > 80 {
				v = v[:80] + "..."
			}
		}
		fmt.Printf("  %-20s = %s\n", k, v)
	}
	fmt.Println()

	// --- Extended statistics ---

	// Entry Statistics.
	fmt.Println("Entry Statistics:")
	fmt.Printf("  Content entries:  %d\n", st.EntryStats.ContentEntries)
	fmt.Printf("  Redirects:        %d\n", st.EntryStats.Redirects)
	fmt.Printf("  Front articles:   %d\n", st.EntryStats.FrontArticles)
	fmt.Printf("  Metadata refs:    %d\n", st.EntryStats.MetadataRefs)
	fmt.Printf("  Total blob bytes: %s\n", formatBytes(st.EntryStats.TotalBlobBytes))
	fmt.Println()

	// MIME Census.
	if len(st.MIMECensus) > 0 {
		fmt.Printf("MIME Census (%d types, by entry count):\n", len(st.MIMECensus))
		fmt.Printf("  %-35s  %10s  %14s  %10s  %10s  %10s\n",
			"Type", "Count", "Total Bytes", "Avg", "Min", "Max")
		limit := len(st.MIMECensus)
		if limit > 20 {
			limit = 20
		}
		for _, c := range st.MIMECensus[:limit] {
			fmt.Printf("  %-35s  %10d  %14d  %10.0f  %10d  %10d\n",
				c.MIMEType, c.Count, c.TotalBytes, c.AvgBytes, c.MinBytes, c.MaxBytes)
		}
		if len(st.MIMECensus) > 20 {
			fmt.Printf("  ... and %d more types\n", len(st.MIMECensus)-20)
		}
		fmt.Println()
	}

	// Chunk Statistics.
	fmt.Println("Chunk Statistics:")
	fmt.Printf("  Chunks:           %d\n", st.ChunkStats.ChunkCount)
	if st.ChunkStats.ChunkCount > 0 {
		fmt.Printf("  Entries/chunk:    avg %.1f, min %d, max %d\n",
			st.ChunkStats.AvgEntriesPerChunk,
			st.ChunkStats.MinEntriesPerChunk,
			st.ChunkStats.MaxEntriesPerChunk)
	}
	fmt.Println()

	// Search Index.
	fmt.Println("Search Index:")
	fmt.Printf("  Title search:     %s\n", searchLine(st.SearchStats.HasTitleSearch, st.SearchStats.TitleDocCount))
	fmt.Printf("  Body search:      %s\n", searchLine(st.SearchStats.HasBodySearch, st.SearchStats.BodyDocCount))
	fmt.Println()

	// Size Summary.
	fmt.Println("Size Summary:")
	fmt.Printf("  Compressed:       %s\n", formatBytes(st.SectionSummary.TotalCompressed))
	fmt.Printf("  Uncompressed:     %s\n", formatBytes(st.SectionSummary.TotalUncompressed))
	if st.SectionSummary.TotalUncompressed > 0 {
		fmt.Printf("  Ratio:            %.1f%%\n", st.SectionSummary.Ratio*100)
	}
	if st.FileSize > 0 {
		fmt.Printf("  File size:        %s\n", formatBytes(st.FileSize))
	}
}

func searchLine(has bool, docCount uint32) string {
	if !has {
		return "no"
	}
	if docCount > 0 {
		return fmt.Sprintf("yes (%d docs)", docCount)
	}
	return "yes"
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB (%d bytes)", float64(b)/float64(1<<30), b)
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB (%d bytes)", float64(b)/float64(1<<20), b)
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB (%d bytes)", float64(b)/float64(1<<10), b)
	default:
		return fmt.Sprintf("%d bytes", b)
	}
}

// isBinary returns true if b contains bytes outside printable ASCII + common whitespace.
func isBinary(b []byte) bool {
	for _, c := range b {
		if (c < 0x20 && c != '\t' && c != '\n' && c != '\r') || c > 0x7e {
			return true
		}
	}
	return false
}

func printClassification(st stats.ArchiveStats) {
	r := classify.Classify(classify.ExtractFromOZA(st))
	fmt.Println("Content Classification:")
	fmt.Printf("  Profile:          %s (confidence %.0f%%)\n", r.Profile, r.Confidence*100)
	fmt.Println()
	fmt.Println("  Features:")
	f := r.Features
	fmt.Printf("    Text bytes:     %.1f%%\n", f.TextBytesRatio*100)
	fmt.Printf("    HTML bytes:     %.1f%%\n", f.HTMLBytesRatio*100)
	fmt.Printf("    Image bytes:    %.1f%%\n", f.ImageBytesRatio*100)
	if f.PDFBytesRatio > 0 {
		fmt.Printf("    PDF bytes:      %.1f%%\n", f.PDFBytesRatio*100)
	}
	if f.VideoBytesRatio > 0 {
		fmt.Printf("    Video bytes:    %.1f%%\n", f.VideoBytesRatio*100)
	}
	fmt.Printf("    Redirect density: %.1f%%\n", f.RedirectDensity*100)
	fmt.Printf("    Avg entry size: %s\n", formatBytes(int64(f.AvgEntryBytes)))
	fmt.Printf("    Small entries:  %.1f%%\n", f.SmallEntryRatio*100)
	fmt.Printf("    MIME types:     %d\n", f.MIMETypeCount)
	if f.SourceHint != "" {
		fmt.Printf("    Source hint:    %s\n", f.SourceHint)
	}
	fmt.Println()
	fmt.Println("  Recommended conversion parameters:")
	rec := r.Recommendations
	fmt.Printf("    Chunk size:     %s\n", formatBytes(int64(rec.ChunkSize)))
	fmt.Printf("    Zstd level:     %d\n", rec.ZstdLevel)
	fmt.Printf("    Dict samples:   %d\n", rec.DictSamples)
	fmt.Printf("    Minify:         %v\n", rec.Minify)
	fmt.Printf("    Optimize imgs:  %v\n", rec.OptimizeImages)
	fmt.Printf("    Search prune:   %.1f\n", rec.SearchPruneFreq)
	fmt.Printf("    Notes:          %s\n", rec.Notes)
}

func formatFlags(hdr oza.Header) string {
	var parts []string
	if hdr.HasSearch() {
		parts = append(parts, "has-search")
	}
	if hdr.HasChrome() {
		parts = append(parts, "has-chrome")
	}
	if hdr.HasSignatures() {
		parts = append(parts, "has-signatures")
	}
	if len(parts) == 0 {
		return ""
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

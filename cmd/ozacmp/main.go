package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stazelabs/oza/cmd/internal/stats"
	"github.com/stazelabs/oza/oza"
)

var (
	format string
	deep   bool
)

func main() {
	root := &cobra.Command{
		Use:   "ozacmp <source.zim> <converted.oza>",
		Short: "Compare a ZIM file and its OZA conversion side-by-side",
		Long: `王座 ozacmp — side-by-side comparison of a ZIM file and its OZA conversion.

Reports file sizes, compression ratios, entry counts, MIME census,
section breakdown, size budget, chunk statistics, search index presence,
and metadata parity between the two formats.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(args[0], args[1])
		},
	}
	root.Flags().StringVar(&format, "format", "text", "output format: text, json, md")
	root.Flags().BoolVar(&deep, "deep", false, "read all ZIM content to compute per-entry byte sizes (slow)")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// --- Data types ---

// Comparison holds the full comparison result.
type Comparison struct {
	ZIM                ZIMStats           `json:"zim"`
	OZA                stats.ArchiveStats `json:"oza"`
	SizeRatio          float64            `json:"size_ratio"`
	SizeDeltaPct       float64            `json:"size_delta_pct"`
	SizeBudget         SizeBudget         `json:"size_budget"`
	ConversionSettings []ConvSetting      `json:"conversion_settings"`
	MetadataMatch      []MetaCmp          `json:"metadata_match"`
	MIMECountMatch     []MIMECountCmp     `json:"mime_count_match"`
}

// ConvSetting holds a conversion setting key-value pair from OZA metadata.
type ConvSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SizeBudget breaks down where the OZA file's bytes go.
type SizeBudget struct {
	Entries       []SizeBudgetEntry `json:"entries"`
	ContentBytes  int64             `json:"content_bytes"`
	OverheadBytes int64             `json:"overhead_bytes"`
	ContentPct    float64           `json:"content_pct"`
	OverheadPct   float64           `json:"overhead_pct"`
}

// SizeBudgetEntry is one row in the size budget.
type SizeBudgetEntry struct {
	Section   string  `json:"section"`
	Size      int64   `json:"size"`
	PctOfFile float64 `json:"pct_of_file"`
	Category  string  `json:"category"` // "content" or "overhead"
}

// MetaCmp compares a single metadata key between ZIM and OZA.
type MetaCmp struct {
	Key      string `json:"key"`
	ZIMValue string `json:"zim_value,omitempty"`
	OZAValue string `json:"oza_value,omitempty"`
	Match    bool   `json:"match"`
}

// MIMECountCmp compares entry counts for a single MIME type.
type MIMECountCmp struct {
	MIMEType string `json:"mime_type"`
	ZIMCount int    `json:"zim_count"`
	OZACount int    `json:"oza_count"`
	Delta    int    `json:"delta"`
}

// --- Core logic ---

func run(zimPath, ozaPath string) error {
	fmt.Fprintf(os.Stderr, "Scanning ZIM: %s\n", zimPath)
	zs, err := collectZIM(zimPath, deep)
	if err != nil {
		return fmt.Errorf("ZIM: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Scanning OZA: %s\n", ozaPath)
	var ozaFileSize int64
	if fi, err := os.Stat(ozaPath); err == nil {
		ozaFileSize = fi.Size()
	}
	a, err := oza.Open(ozaPath)
	if err != nil {
		return fmt.Errorf("OZA: %w", err)
	}
	defer a.Close()
	ozaStats := stats.Collect(a, ozaFileSize)

	cmp := buildComparison(zs, ozaStats)

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cmp)
	case "md":
		printMarkdown(zimPath, ozaPath, cmp)
	case "text", "":
		printText(zimPath, ozaPath, cmp)
	default:
		return fmt.Errorf("unknown format %q (use text, json, or md)", format)
	}
	return nil
}

func buildComparison(zs ZIMStats, os stats.ArchiveStats) Comparison {
	var cmp Comparison
	cmp.ZIM = zs
	cmp.OZA = os

	if zs.FileSize > 0 {
		cmp.SizeRatio = float64(os.FileSize) / float64(zs.FileSize)
		cmp.SizeDeltaPct = (cmp.SizeRatio - 1.0) * 100
	}

	cmp.SizeBudget = buildSizeBudget(os.Sections, os.FileSize)
	cmp.ConversionSettings = extractConversionSettings(os.Metadata)
	cmp.MetadataMatch = compareMetadata(zs.Metadata, os.Metadata)
	cmp.MIMECountMatch = compareMIMECounts(zs.MIMECensus, os.MIMECensus)

	return cmp
}

func buildSizeBudget(sections []stats.SectionStats, fileSize int64) SizeBudget {
	var budget SizeBudget
	for _, sec := range sections {
		size := int64(sec.CompressedSize)
		cat := "overhead"
		if sec.Type == "CONTENT" {
			cat = "content"
		}
		var pct float64
		if fileSize > 0 {
			pct = float64(size) / float64(fileSize) * 100
		}
		budget.Entries = append(budget.Entries, SizeBudgetEntry{
			Section:   sec.Type,
			Size:      size,
			PctOfFile: pct,
			Category:  cat,
		})
		if cat == "content" {
			budget.ContentBytes += size
		} else {
			budget.OverheadBytes += size
		}
	}
	// Sort by size descending.
	sort.Slice(budget.Entries, func(i, j int) bool {
		return budget.Entries[i].Size > budget.Entries[j].Size
	})
	total := budget.ContentBytes + budget.OverheadBytes
	if total > 0 {
		budget.ContentPct = float64(budget.ContentBytes) / float64(total) * 100
		budget.OverheadPct = float64(budget.OverheadBytes) / float64(total) * 100
	}
	return budget
}

// conversionKeys are OZA metadata keys that describe conversion settings.
var conversionKeys = []struct{ key, label string }{
	{"converter", "Converter"},
	{"converter_version", "Version"},
	{"converter_flags", "Flags"},
	{"chunk_target_size", "Chunk Target Size"},
}

func extractConversionSettings(ozaMeta map[string]string) []ConvSetting {
	var result []ConvSetting
	for _, ck := range conversionKeys {
		if v, ok := ozaMeta[ck.key]; ok {
			result = append(result, ConvSetting{Key: ck.label, Value: v})
		}
	}
	return result
}

func compareMetadata(zimMeta, ozaMeta map[string]string) []MetaCmp {
	allKeys := make(map[string]struct{})
	for k := range zimMeta {
		allKeys[k] = struct{}{}
	}
	for k := range ozaMeta {
		allKeys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	var result []MetaCmp
	for _, k := range sorted {
		if strings.HasPrefix(k, "illustration_") || k == "converter" || k == "converter_flags" || k == "converter_version" || k == "chunk_target_size" {
			continue
		}
		zv := zimMeta[k]
		ov := ozaMeta[k]
		result = append(result, MetaCmp{Key: k, ZIMValue: zv, OZAValue: ov, Match: zv == ov})
	}
	return result
}

func compareMIMECounts(zimCensus []ZIMMIMECensusEntry, ozaCensus []stats.MIMECensusEntry) []MIMECountCmp {
	zimMap := make(map[string]int)
	for _, e := range zimCensus {
		zimMap[e.MIMEType] = e.Count
	}
	ozaMap := make(map[string]int)
	for _, e := range ozaCensus {
		ozaMap[e.MIMEType] = e.Count
	}

	allTypes := make(map[string]struct{})
	for t := range zimMap {
		allTypes[t] = struct{}{}
	}
	for t := range ozaMap {
		allTypes[t] = struct{}{}
	}

	result := make([]MIMECountCmp, 0, len(allTypes))
	for t := range allTypes {
		zc := zimMap[t]
		oc := ozaMap[t]
		result = append(result, MIMECountCmp{MIMEType: t, ZIMCount: zc, OZACount: oc, Delta: oc - zc})
	}
	sort.Slice(result, func(i, j int) bool {
		mi := max(result[i].ZIMCount, result[i].OZACount)
		mj := max(result[j].ZIMCount, result[j].OZACount)
		return mi > mj
	})
	return result
}

// --- Bar chart helper ---

const barWidth = 18

func bar(pct float64, width int) string {
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
}

// --- Text output ---

func printText(zimPath, ozaPath string, cmp Comparison) {
	fmt.Println("=== File Comparison ===")
	fmt.Printf("  ZIM file:     %-30s  %s\n", zimPath, fmtBytes(cmp.ZIM.FileSize))
	fmt.Printf("  OZA file:     %-30s  %s\n", ozaPath, fmtBytes(cmp.OZA.FileSize))
	if cmp.ZIM.FileSize > 0 {
		dir := "smaller"
		pct := -cmp.SizeDeltaPct
		if cmp.SizeDeltaPct > 0 {
			dir = "larger"
			pct = cmp.SizeDeltaPct
		}
		fmt.Printf("  Size ratio:   %.2f (OZA is %.1f%% %s)\n", cmp.SizeRatio, pct, dir)
	}
	fmt.Println()

	// Conversion settings.
	if len(cmp.ConversionSettings) > 0 {
		fmt.Println("=== Conversion Settings ===")
		for _, s := range cmp.ConversionSettings {
			fmt.Printf("  %-20s  %s\n", s.Key+":", s.Value)
		}
		fmt.Println()
	}

	// Entry counts.
	fmt.Println("=== Entry Counts ===")
	fmt.Printf("  %-20s  %10s  %10s  %10s\n", "", "ZIM", "OZA", "Delta")
	printCountRow("Content entries", cmp.ZIM.ContentCount, int(cmp.OZA.EntryStats.ContentEntries))
	printCountRow("Redirects", cmp.ZIM.RedirectCount, int(cmp.OZA.Header.RedirectCount))
	fmt.Printf("  %-20s  %10s  %10d  %10s\n", "Front articles", "\u2014", cmp.OZA.EntryStats.FrontArticles, "\u2014")
	fmt.Printf("  %-20s  %10d  %10s  %10s\n", "Skipped", cmp.ZIM.SkippedCount, "\u2014", "\u2014")
	fmt.Printf("  %-20s  %10d  %10s  %10s\n", "Metadata keys", cmp.ZIM.MetadataCount, "\u2014", "\u2014")
	fmt.Println()

	// MIME census.
	fmt.Println("=== MIME Census (by entry count) ===")
	fmt.Printf("  %-35s  %10s  %10s  %10s\n", "Type", "ZIM Count", "OZA Count", "Delta")
	for _, m := range cmp.MIMECountMatch {
		fmt.Printf("  %-35s  %10d  %10d  %10s\n", m.MIMEType, m.ZIMCount, m.OZACount, deltaStr(m.Delta))
	}
	fmt.Println()

	// OZA section breakdown.
	fmt.Println("=== OZA Section Breakdown ===")
	fmt.Printf("  %-15s  %14s  %14s  %8s\n", "Section", "Compressed", "Uncompressed", "Ratio")
	for _, sec := range cmp.OZA.Sections {
		ratio := ""
		if sec.UncompressedSize > 0 {
			ratio = fmt.Sprintf("%.1f%%", float64(sec.CompressedSize)/float64(sec.UncompressedSize)*100)
		}
		fmt.Printf("  %-15s  %14d  %14d  %8s\n",
			sec.Type, sec.CompressedSize, sec.UncompressedSize, ratio)
	}
	if cmp.OZA.SectionSummary.TotalUncompressed > 0 {
		fmt.Printf("  %-15s  %14d  %14d  %8.1f%%\n", "TOTAL",
			cmp.OZA.SectionSummary.TotalCompressed, cmp.OZA.SectionSummary.TotalUncompressed,
			cmp.OZA.SectionSummary.Ratio*100)
	}
	fmt.Println()

	// Size budget.
	printTextSizeBudget(cmp.SizeBudget)

	// Chunk stats.
	fmt.Println("=== OZA Chunk Statistics ===")
	fmt.Printf("  Chunks:           %d\n", cmp.OZA.ChunkStats.ChunkCount)
	if cmp.OZA.ChunkStats.ChunkCount > 0 {
		fmt.Printf("  Entries/chunk:    avg %.1f, min %d, max %d\n",
			cmp.OZA.ChunkStats.AvgEntriesPerChunk,
			cmp.OZA.ChunkStats.MinEntriesPerChunk,
			cmp.OZA.ChunkStats.MaxEntriesPerChunk)
	}
	fmt.Println()

	// Search index.
	fmt.Println("=== Search Index ===")
	fmt.Printf("  Title search:     %s\n", searchLine(cmp.OZA.SearchStats.HasTitleSearch, cmp.OZA.SearchStats.TitleDocCount))
	fmt.Printf("  Body search:      %s\n", searchLine(cmp.OZA.SearchStats.HasBodySearch, cmp.OZA.SearchStats.BodyDocCount))
	fmt.Println()

	// Metadata comparison.
	if len(cmp.MetadataMatch) > 0 {
		fmt.Println("=== Metadata Comparison ===")
		fmt.Printf("  %-20s  %-30s  %-30s  %s\n", "Key", "ZIM Value", "OZA Value", "Match")
		for _, m := range cmp.MetadataMatch {
			match := "yes"
			if !m.Match {
				match = "MISMATCH"
			}
			fmt.Printf("  %-20s  %-30s  %-30s  %s\n", m.Key, truncate(m.ZIMValue, 28), truncate(m.OZAValue, 28), match)
		}
	}
}

func printTextSizeBudget(budget SizeBudget) {
	fmt.Println("=== Size Budget ===")
	fmt.Printf("  %-15s  %12s  %9s  %-9s  %s\n", "Section", "Size", "% of File", "Category", "")
	for _, e := range budget.Entries {
		fmt.Printf("  %-15s  %12s  %8.1f%%  %-9s  %s\n",
			e.Section, fmtBytesShort(e.Size), e.PctOfFile, e.Category, bar(e.PctOfFile, barWidth))
	}
	fmt.Printf("  %s\n", strings.Repeat("\u2500", 72))
	fmt.Printf("  %-15s  %12s  %8.1f%%  %9s  %s\n",
		"Content total", fmtBytesShort(budget.ContentBytes), budget.ContentPct, "", bar(budget.ContentPct, barWidth))
	fmt.Printf("  %-15s  %12s  %8.1f%%  %9s  %s\n",
		"Overhead total", fmtBytesShort(budget.OverheadBytes), budget.OverheadPct, "", bar(budget.OverheadPct, barWidth))
	fmt.Println()
}

// --- Markdown output ---

func printMarkdown(zimPath, ozaPath string, cmp Comparison) {
	fmt.Println("# ZIM \u2194 OZA Comparison Report")
	fmt.Println()

	// File comparison.
	fmt.Println("## File Comparison")
	fmt.Println()
	fmt.Println("| | File | Size |")
	fmt.Println("|---|---|---:|")
	fmt.Printf("| ZIM | `%s` | %s |\n", zimPath, fmtBytesShort(cmp.ZIM.FileSize))
	fmt.Printf("| OZA | `%s` | %s |\n", ozaPath, fmtBytesShort(cmp.OZA.FileSize))
	if cmp.ZIM.FileSize > 0 {
		dir := "smaller"
		pct := -cmp.SizeDeltaPct
		if cmp.SizeDeltaPct > 0 {
			dir = "larger"
			pct = cmp.SizeDeltaPct
		}
		fmt.Printf("\n**Size ratio: %.2f** \u2014 OZA is **%.1f%% %s**\n", cmp.SizeRatio, pct, dir)
	}
	fmt.Println()

	// Conversion settings.
	if len(cmp.ConversionSettings) > 0 {
		fmt.Println("## Conversion Settings")
		fmt.Println()
		fmt.Println("| Setting | Value |")
		fmt.Println("|---|---|")
		for _, s := range cmp.ConversionSettings {
			fmt.Printf("| %s | `%s` |\n", s.Key, s.Value)
		}
		fmt.Println()
	}

	// Size budget.
	fmt.Println("## Size Budget")
	fmt.Println()
	fmt.Println("| Section | Size | % of File | Category | |")
	fmt.Println("|---|---:|---:|---|---|")
	for _, e := range cmp.SizeBudget.Entries {
		fmt.Printf("| %s | %s | %.1f%% | %s | `%s` |\n",
			e.Section, fmtBytesShort(e.Size), e.PctOfFile, e.Category, bar(e.PctOfFile, barWidth))
	}
	fmt.Println()
	fmt.Printf("| **Content total** | **%s** | **%.1f%%** | | `%s` |\n",
		fmtBytesShort(cmp.SizeBudget.ContentBytes), cmp.SizeBudget.ContentPct, bar(cmp.SizeBudget.ContentPct, barWidth))
	fmt.Printf("| **Overhead total** | **%s** | **%.1f%%** | | `%s` |\n",
		fmtBytesShort(cmp.SizeBudget.OverheadBytes), cmp.SizeBudget.OverheadPct, bar(cmp.SizeBudget.OverheadPct, barWidth))
	fmt.Println()

	// Entry counts.
	fmt.Println("## Entry Counts")
	fmt.Println()
	fmt.Println("| | ZIM | OZA | Delta |")
	fmt.Println("|---|---:|---:|---:|")
	fmt.Printf("| Content entries | %d | %d | %s |\n", cmp.ZIM.ContentCount, cmp.OZA.EntryStats.ContentEntries, deltaStr(int(cmp.OZA.EntryStats.ContentEntries)-cmp.ZIM.ContentCount))
	fmt.Printf("| Redirects | %d | %d | %s |\n", cmp.ZIM.RedirectCount, cmp.OZA.Header.RedirectCount, deltaStr(int(cmp.OZA.Header.RedirectCount)-cmp.ZIM.RedirectCount))
	fmt.Printf("| Front articles | \u2014 | %d | \u2014 |\n", cmp.OZA.EntryStats.FrontArticles)
	fmt.Printf("| Skipped | %d | \u2014 | \u2014 |\n", cmp.ZIM.SkippedCount)
	fmt.Printf("| Metadata keys | %d | \u2014 | \u2014 |\n", cmp.ZIM.MetadataCount)
	fmt.Println()

	// MIME census.
	fmt.Println("## MIME Census")
	fmt.Println()
	fmt.Println("| Type | ZIM Count | OZA Count | Delta |")
	fmt.Println("|---|---:|---:|---:|")
	for _, m := range cmp.MIMECountMatch {
		d := deltaStr(m.Delta)
		if m.Delta != 0 {
			d = "**" + d + "**"
		}
		fmt.Printf("| `%s` | %d | %d | %s |\n", m.MIMEType, m.ZIMCount, m.OZACount, d)
	}
	fmt.Println()

	// OZA section breakdown.
	fmt.Println("## OZA Section Breakdown")
	fmt.Println()
	fmt.Println("| Section | Compressed | Uncompressed | Ratio | |")
	fmt.Println("|---|---:|---:|---:|---|")
	for _, sec := range cmp.OZA.Sections {
		ratio := "\u2014"
		ratioVal := 0.0
		if sec.UncompressedSize > 0 {
			ratioVal = float64(sec.CompressedSize) / float64(sec.UncompressedSize) * 100
			ratio = fmt.Sprintf("%.1f%%", ratioVal)
		}
		fmt.Printf("| %s | %s | %s | %s | `%s` |\n",
			sec.Type, fmtBytesShort(int64(sec.CompressedSize)), fmtBytesShort(int64(sec.UncompressedSize)),
			ratio, bar(ratioVal, barWidth))
	}
	if cmp.OZA.SectionSummary.TotalUncompressed > 0 {
		r := cmp.OZA.SectionSummary.Ratio * 100
		fmt.Printf("| **TOTAL** | **%s** | **%s** | **%.1f%%** | `%s` |\n",
			fmtBytesShort(cmp.OZA.SectionSummary.TotalCompressed),
			fmtBytesShort(cmp.OZA.SectionSummary.TotalUncompressed),
			r, bar(r, barWidth))
	}
	fmt.Println()

	// Chunk stats.
	fmt.Println("## Chunk Statistics")
	fmt.Println()
	fmt.Printf("- **Chunks:** %d\n", cmp.OZA.ChunkStats.ChunkCount)
	if cmp.OZA.ChunkStats.ChunkCount > 0 {
		fmt.Printf("- **Entries/chunk:** avg %.1f, min %d, max %d\n",
			cmp.OZA.ChunkStats.AvgEntriesPerChunk,
			cmp.OZA.ChunkStats.MinEntriesPerChunk,
			cmp.OZA.ChunkStats.MaxEntriesPerChunk)
	}
	fmt.Println()

	// Search index.
	fmt.Println("## Search Index")
	fmt.Println()
	fmt.Printf("- **Title search:** %s\n", searchLine(cmp.OZA.SearchStats.HasTitleSearch, cmp.OZA.SearchStats.TitleDocCount))
	fmt.Printf("- **Body search:** %s\n", searchLine(cmp.OZA.SearchStats.HasBodySearch, cmp.OZA.SearchStats.BodyDocCount))
	fmt.Println()

	// Metadata comparison.
	if len(cmp.MetadataMatch) > 0 {
		fmt.Println("## Metadata Comparison")
		fmt.Println()
		fmt.Println("| Key | ZIM Value | OZA Value | Match |")
		fmt.Println("|---|---|---|---|")
		for _, m := range cmp.MetadataMatch {
			match := "\u2705"
			if !m.Match {
				match = "\u274C **MISMATCH**"
			}
			fmt.Printf("| `%s` | %s | %s | %s |\n", m.Key, mdEscape(truncate(m.ZIMValue, 40)), mdEscape(truncate(m.OZAValue, 40)), match)
		}
	}
}

// --- Helpers ---

func printCountRow(label string, zimCount, ozaCount int) {
	fmt.Printf("  %-20s  %10d  %10d  %10s\n", label, zimCount, ozaCount, deltaStr(ozaCount-zimCount))
}

func deltaStr(d int) string {
	if d == 0 {
		return "0"
	}
	return fmt.Sprintf("%+d", d)
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

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func mdEscape(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}

func fmtBytes(b int64) string {
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

func fmtBytesShort(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

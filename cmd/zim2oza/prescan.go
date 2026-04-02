package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/stazelabs/gozim/zim"

	"github.com/stazelabs/oza/cmd/internal/classify"
)

// prescanZIM does a lightweight scan of a ZIM file to collect metadata and
// entry/redirect counts for content profile classification. No content is read.
func prescanZIM(path string) (classify.ZIMQuickStats, error) {
	za, err := zim.OpenWithOptions(path, zim.WithCacheSize(2))
	if err != nil {
		return classify.ZIMQuickStats{}, fmt.Errorf("opening ZIM for prescan: %w", err)
	}
	defer za.Close()

	var s classify.ZIMQuickStats
	s.Metadata = make(map[string]string)
	s.MIMECounts = make(map[string]int)

	// Collect metadata from M/ namespace.
	for entry := range za.EntriesByNamespace('M') {
		key := strings.ToLower(entry.Path())
		content, err := entry.ReadContent()
		if err != nil {
			continue
		}
		if !isBinaryContent(content) {
			s.Metadata[key] = string(content)
		}
	}

	// Count content and redirect entries.
	for entry := range za.Entries() {
		ns := entry.Namespace()
		if ns == 'M' || ns == 'X' {
			continue
		}
		if ns == 'C' && strings.HasPrefix(entry.Path(), "_mw_/") {
			continue
		}
		if entry.IsRedirect() {
			s.RedirectCount++
		} else {
			s.ContentCount++
			s.MIMECounts[entry.MIMEType()]++
		}
	}

	return s, nil
}

func isBinaryContent(b []byte) bool {
	for _, c := range b {
		if (c < 0x20 && c != '\t' && c != '\n' && c != '\r') || c > 0x7e {
			return true
		}
	}
	return false
}

// applyAutoRecs applies classified recommendations to the conversion options,
// but only for parameters the user did not explicitly set. Returns the detected
// profile name for logging.
func applyAutoRecs(opts *ConvertOptions, recs classify.Recs, changed func(string) bool) {
	if !changed("zstd-level") {
		opts.ZstdLevel = recs.ZstdLevel
	}
	if !changed("dict-samples") {
		opts.DictSamples = recs.DictSamples
	}
	if !changed("chunk-size") {
		opts.ChunkSize = recs.ChunkSize
	}
	if !changed("minify") {
		opts.Minify = recs.Minify
	}
	if !changed("no-optimize-images") {
		opts.OptimizeImages = recs.OptimizeImages
	}
	if !changed("search-prune") {
		opts.SearchPruneFreq = recs.SearchPruneFreq
	}
}

// printAutoProfile logs the auto-detected profile and applied parameters.
func printAutoProfile(profile classify.Profile, confidence float64, recs classify.Recs) {
	fmt.Fprintf(os.Stderr, "Auto-detected profile: %s (confidence %.0f%%)\n", profile, confidence*100)
	fmt.Fprintf(os.Stderr, "  chunk-size=%d zstd-level=%d dict-samples=%d minify=%v optimize-images=%v search-prune=%.1f\n",
		recs.ChunkSize, recs.ZstdLevel, recs.DictSamples, recs.Minify, recs.OptimizeImages, recs.SearchPruneFreq)
	fmt.Fprintf(os.Stderr, "  %s\n", recs.Notes)
}

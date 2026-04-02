package classify

import (
	"strings"

	"github.com/stazelabs/oza/cmd/internal/stats"
)

// Features holds the computed feature vector for classification.
type Features struct {
	// Byte-fraction features (0.0 - 1.0). Set to -1 when unavailable.
	TextBytesRatio  float64 `json:"text_bytes_ratio"`
	HTMLBytesRatio  float64 `json:"html_bytes_ratio"`
	ImageBytesRatio float64 `json:"image_bytes_ratio"`
	PDFBytesRatio   float64 `json:"pdf_bytes_ratio"`
	VideoBytesRatio float64 `json:"video_bytes_ratio"`

	// Entry-count features.
	RedirectDensity float64 `json:"redirect_density"`
	AvgEntryBytes   float64 `json:"avg_entry_bytes"`
	SmallEntryRatio float64 `json:"small_entry_ratio"` // approx: MIME types with avg < 4KB
	EntryCount      uint32  `json:"entry_count"`

	// Structural features.
	MIMETypeCount    int     `json:"mime_type_count"`
	CompressionRatio float64 `json:"compression_ratio"`

	// Metadata-derived hint (soft signal).
	SourceHint string `json:"source_hint,omitempty"`
}

// Result is the classification output.
type Result struct {
	Profile         Profile  `json:"profile"`
	Confidence      float64  `json:"confidence"`
	Features        Features `json:"features"`
	Recommendations Recs     `json:"recommendations"`
}

// ExtractFromOZA computes a feature vector from OZA archive stats.
func ExtractFromOZA(s stats.ArchiveStats) Features {
	var f Features

	totalBytes := s.EntryStats.TotalBlobBytes
	if totalBytes > 0 {
		var textBytes, htmlBytes, imageBytes, pdfBytes, videoBytes int64
		var smallEntries int
		for _, m := range s.MIMECensus {
			mime := strings.ToLower(m.MIMEType)
			switch {
			case mime == "text/html":
				htmlBytes += m.TotalBytes
				textBytes += m.TotalBytes
			case strings.HasPrefix(mime, "text/"):
				textBytes += m.TotalBytes
			case strings.HasPrefix(mime, "image/"):
				imageBytes += m.TotalBytes
			case mime == "application/pdf":
				pdfBytes += m.TotalBytes
			case strings.HasPrefix(mime, "video/"):
				videoBytes += m.TotalBytes
			}
			if m.AvgBytes < 4096 {
				smallEntries += m.Count
			}
		}
		f.TextBytesRatio = float64(textBytes) / float64(totalBytes)
		f.HTMLBytesRatio = float64(htmlBytes) / float64(totalBytes)
		f.ImageBytesRatio = float64(imageBytes) / float64(totalBytes)
		f.PDFBytesRatio = float64(pdfBytes) / float64(totalBytes)
		f.VideoBytesRatio = float64(videoBytes) / float64(totalBytes)
		if s.EntryStats.ContentEntries > 0 {
			f.SmallEntryRatio = float64(smallEntries) / float64(s.EntryStats.ContentEntries)
		}
	} else {
		f.TextBytesRatio = -1
		f.HTMLBytesRatio = -1
		f.ImageBytesRatio = -1
		f.PDFBytesRatio = -1
		f.VideoBytesRatio = -1
	}

	total := uint64(s.EntryStats.ContentEntries) + uint64(s.EntryStats.Redirects)
	if total > 0 {
		f.RedirectDensity = float64(s.EntryStats.Redirects) / float64(total)
	}
	if s.EntryStats.ContentEntries > 0 {
		f.AvgEntryBytes = float64(totalBytes) / float64(s.EntryStats.ContentEntries)
	}
	f.EntryCount = s.EntryStats.ContentEntries
	f.MIMETypeCount = len(s.MIMECensus)
	f.CompressionRatio = s.SectionSummary.Ratio

	f.SourceHint = extractSourceHint(s.Metadata)
	return f
}

// ZIMQuickStats holds the minimal data needed to classify a ZIM archive
// without reading content (no byte-level features).
type ZIMQuickStats struct {
	ContentCount  int
	RedirectCount int
	MIMECounts    map[string]int // MIME type -> entry count
	Metadata      map[string]string
}

// ExtractFromZIMQuick computes features from a quick ZIM scan (no byte-level data).
// Byte-fraction features are set to -1.
func ExtractFromZIMQuick(s ZIMQuickStats) Features {
	var f Features
	f.TextBytesRatio = -1
	f.HTMLBytesRatio = -1
	f.ImageBytesRatio = -1
	f.PDFBytesRatio = -1
	f.VideoBytesRatio = -1

	total := uint64(s.ContentCount) + uint64(s.RedirectCount)
	if total > 0 {
		f.RedirectDensity = float64(s.RedirectCount) / float64(total)
	}
	f.EntryCount = uint32(s.ContentCount)
	f.MIMETypeCount = len(s.MIMECounts)

	// Approximate SmallEntryRatio: not available without byte data.
	f.SmallEntryRatio = -1

	f.SourceHint = extractSourceHint(s.Metadata)
	return f
}

// knownSources maps keywords found in archive metadata to normalized source hints.
var knownSources = []struct {
	keyword string
	hint    string
}{
	{"wiktionary", "wiktionary"},
	{"wikiquote", "wikiquote"},
	{"gutenberg", "gutenberg"},
	{"wikisource", "wikisource"},
	{"stackexchange", "stackexchange"},
	{"stack_exchange", "stackexchange"},
	{"stackoverflow", "stackexchange"},
	{"devdocs", "devdocs"},
	{"mankier", "devdocs"},
	{"openwrt", "devdocs"},
	{"zimit", "zimit"},
	{"ted.com", "ted"},
	{"vikidia", "vikidia"},
	{"wikipedia", "wikipedia"},
}

func extractSourceHint(metadata map[string]string) string {
	// Check name, source, title, and scraper fields.
	for _, key := range []string{"name", "source", "title", "scraper"} {
		val := strings.ToLower(metadata[key])
		if val == "" {
			continue
		}
		for _, ks := range knownSources {
			if strings.Contains(val, ks.keyword) {
				return ks.hint
			}
		}
	}
	return ""
}

// hasByteFeatures returns true if byte-level features are available.
func hasByteFeatures(f Features) bool {
	return f.TextBytesRatio >= 0
}

// Classify assigns a content profile to the given feature vector.
func Classify(f Features) Result {
	p, conf := classifyProfile(f)
	return Result{
		Profile:         p,
		Confidence:      conf,
		Features:        f,
		Recommendations: RecommendOptions(p),
	}
}

func classifyProfile(f Features) (Profile, float64) {
	hasByte := hasByteFeatures(f)

	// Without byte features, use count-based + metadata heuristics only.
	if !hasByte {
		return classifyWithoutBytes(f)
	}

	// 1. PDF-container: PDF bytes dominate.
	if f.PDFBytesRatio > 0.30 {
		return ProfilePDFContainer, clamp(0.6 + f.PDFBytesRatio)
	}

	// 2. Media-heavy: image+video bytes dominate.
	mediaRatio := f.ImageBytesRatio + f.VideoBytesRatio
	if mediaRatio > 0.60 {
		return ProfileMediaHeavy, clamp(0.5 + mediaRatio/2)
	}

	// 3. Dictionary: high redirects + many small entries + text dominant.
	if f.RedirectDensity > 0.35 && f.SmallEntryRatio > 0.50 && f.TextBytesRatio > 0.70 {
		return ProfileDictionary, clamp(0.5 + f.RedirectDensity/2 + f.SmallEntryRatio/4)
	}

	// 4. QA-Forum by source hint (checked early since SE archives can resemble books).
	if f.SourceHint == "stackexchange" {
		return ProfileQAForum, 0.90
	}

	// 5. Books: few large entries, low redirects, text dominant.
	if f.AvgEntryBytes > 50000 && f.RedirectDensity < 0.15 && f.TextBytesRatio > 0.50 {
		return ProfileBooks, clamp(0.6 + min64(f.AvgEntryBytes/200000, 0.3))
	}

	// 6. Docs: text dominant, very low image ratio, moderate entry sizes.
	if f.TextBytesRatio > 0.80 && f.ImageBytesRatio < 0.10 && f.AvgEntryBytes < 50000 {
		if isDocsSource(f.SourceHint) || f.RedirectDensity < 0.05 {
			return ProfileDocs, clamp(0.6 + f.TextBytesRatio/4)
		}
	}

	// 7. QA-Forum by structural signature.
	if f.TextBytesRatio > 0.50 && f.ImageBytesRatio > 0.10 && f.ImageBytesRatio < 0.50 &&
		f.MIMETypeCount > 5 && f.RedirectDensity < 0.10 {
		return ProfileQAForum, 0.65
	}

	// 8. Encyclopedia: text dominant with images, moderate redirects.
	if f.HTMLBytesRatio > 0.30 && f.RedirectDensity > 0.05 {
		return ProfileEncyclopedia, clamp(0.6 + f.HTMLBytesRatio/4)
	}

	// 9. Fallback.
	return ProfileMixedScrape, 0.50
}

// classifyWithoutBytes uses only count-based and metadata features.
// Confidence is capped at 0.6 since byte-level features are absent.
func classifyWithoutBytes(f Features) (Profile, float64) {
	const maxConf = 0.60

	// Dictionary: high redirect density is a strong signal even without bytes.
	if f.RedirectDensity > 0.35 {
		if f.SourceHint == "wiktionary" || f.SourceHint == "wikiquote" {
			return ProfileDictionary, maxConf
		}
		return ProfileDictionary, 0.50
	}

	// Source-hint-based classification.
	switch f.SourceHint {
	case "gutenberg", "wikisource":
		return ProfileBooks, 0.50
	case "stackexchange":
		return ProfileQAForum, 0.55
	case "devdocs":
		return ProfileDocs, 0.55
	case "ted":
		return ProfileMediaHeavy, 0.50
	case "zimit":
		return ProfileMixedScrape, 0.50
	case "wikipedia", "vikidia":
		return ProfileEncyclopedia, 0.55
	}

	// No strong signals.
	return ProfileMixedScrape, 0.40
}

func isDocsSource(hint string) bool {
	return hint == "devdocs"
}

func clamp(v float64) float64 {
	if v > 1.0 {
		return 1.0
	}
	if v < 0.0 {
		return 0.0
	}
	return v
}

func min64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

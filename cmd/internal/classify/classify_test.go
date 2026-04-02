package classify

import (
	"testing"

	"github.com/stazelabs/oza/cmd/internal/stats"
)

// makeStats builds a minimal ArchiveStats with the given MIME census and entry counts.
func makeStats(census []stats.MIMECensusEntry, contentEntries, redirects uint32, metadata map[string]string) stats.ArchiveStats {
	var totalBytes int64
	for _, c := range census {
		totalBytes += c.TotalBytes
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	return stats.ArchiveStats{
		MIMECensus: census,
		EntryStats: stats.EntryStats{
			ContentEntries: contentEntries,
			Redirects:      redirects,
			TotalBlobBytes: totalBytes,
		},
		SectionSummary: stats.SectionSummary{
			Ratio: 0.3,
		},
		Metadata: metadata,
	}
}

func mime(name string, count int, totalBytes int64) stats.MIMECensusEntry {
	avg := float64(0)
	if count > 0 {
		avg = float64(totalBytes) / float64(count)
	}
	return stats.MIMECensusEntry{
		MIMEType:   name,
		Count:      count,
		TotalBytes: totalBytes,
		AvgBytes:   avg,
		MinBytes:   uint32(avg / 2),
		MaxBytes:   uint32(avg * 2),
	}
}

func TestClassifyEncyclopedia(t *testing.T) {
	// Wikipedia-like: text/html dominant, moderate images, ~20% redirects.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 500, 50_000_000), // 50MB HTML
		mime("image/png", 200, 15_000_000), // 15MB images
		mime("text/css", 10, 500_000),      // 0.5MB CSS
		mime("application/javascript", 5, 200_000),
	}, 715, 180, nil)

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileEncyclopedia {
		t.Errorf("expected encyclopedia, got %s", r.Profile)
	}
	if r.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5, got %f", r.Confidence)
	}
}

func TestClassifyDictionary(t *testing.T) {
	// Wiktionary-like: many small entries, high redirect ratio, text dominant.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 10000, 20_000_000), // 2KB avg
		mime("text/css", 5, 100_000),
	}, 10005, 15000, map[string]string{"name": "wiktionary_he"})

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileDictionary {
		t.Errorf("expected dictionary, got %s", r.Profile)
	}
	if r.Confidence < 0.7 {
		t.Errorf("expected confidence >= 0.7, got %f", r.Confidence)
	}
}

func TestClassifyBooks(t *testing.T) {
	// Gutenberg-like: few large entries, low redirects, text dominant.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 20, 5_000_000), // 250KB avg
		mime("text/css", 2, 50_000),
	}, 22, 2, map[string]string{"name": "gutenberg_ar"})

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileBooks {
		t.Errorf("expected books, got %s", r.Profile)
	}
}

func TestClassifyQAForum(t *testing.T) {
	// StackExchange-like: moderate text+images, many MIME types, low redirects.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 300, 20_000_000),
		mime("image/png", 100, 8_000_000),
		mime("image/jpeg", 50, 5_000_000),
		mime("text/css", 10, 500_000),
		mime("application/javascript", 8, 300_000),
		mime("image/svg+xml", 20, 200_000),
	}, 488, 20, map[string]string{"name": "stackexchange_community"})

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileQAForum {
		t.Errorf("expected qa-forum, got %s", r.Profile)
	}
}

func TestClassifyDocs(t *testing.T) {
	// DevDocs-like: text dominant, no images, small entries, low redirects.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 200, 3_000_000), // 15KB avg
		mime("text/css", 5, 100_000),
		mime("application/javascript", 3, 50_000),
	}, 208, 5, map[string]string{"name": "devdocs_go"})

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileDocs {
		t.Errorf("expected docs, got %s", r.Profile)
	}
}

func TestClassifyMediaHeavy(t *testing.T) {
	// xkcd-like: image bytes dominate.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 100, 2_000_000),
		mime("image/png", 100, 30_000_000),
		mime("image/jpeg", 50, 20_000_000),
	}, 250, 10, nil)

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileMediaHeavy {
		t.Errorf("expected media-heavy, got %s", r.Profile)
	}
}

func TestClassifyPDFContainer(t *testing.T) {
	// Wikisource PDF variant: PDF bytes dominate.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 50, 1_000_000),
		mime("application/pdf", 30, 40_000_000),
	}, 80, 5, nil)

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfilePDFContainer {
		t.Errorf("expected pdf-container, got %s", r.Profile)
	}
}

func TestClassifyMixedScrape(t *testing.T) {
	// Zimit scrape: no dominant pattern.
	s := makeStats([]stats.MIMECensusEntry{
		mime("text/html", 50, 2_000_000),
		mime("application/javascript", 30, 3_000_000),
		mime("text/css", 20, 1_000_000),
		mime("image/png", 10, 1_000_000),
	}, 110, 2, map[string]string{"scraper": "zimit"})

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileMixedScrape {
		t.Errorf("expected mixed-scrape, got %s", r.Profile)
	}
}

func TestClassifyWithoutBytesWiktionary(t *testing.T) {
	// No byte-level features but source hint + high redirect density.
	s := stats.ArchiveStats{
		EntryStats: stats.EntryStats{
			ContentEntries: 5000,
			Redirects:      8000,
			TotalBlobBytes: 0,
		},
		Metadata: map[string]string{"name": "wiktionary_he"},
	}

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileDictionary {
		t.Errorf("expected dictionary, got %s", r.Profile)
	}
	if r.Confidence > 0.60 {
		t.Errorf("expected confidence <= 0.60 without byte features, got %f", r.Confidence)
	}
}

func TestClassifyWithoutBytesWikipedia(t *testing.T) {
	s := stats.ArchiveStats{
		EntryStats: stats.EntryStats{
			ContentEntries: 10000,
			Redirects:      2000,
			TotalBlobBytes: 0,
		},
		Metadata: map[string]string{"name": "wikipedia_en_top100"},
	}

	r := Classify(ExtractFromOZA(s))
	if r.Profile != ProfileEncyclopedia {
		t.Errorf("expected encyclopedia, got %s", r.Profile)
	}
	if r.Confidence > 0.60 {
		t.Errorf("expected confidence <= 0.60 without byte features, got %f", r.Confidence)
	}
}

func TestRecommendOptionsAllProfiles(t *testing.T) {
	for _, p := range AllProfiles() {
		r := RecommendOptions(p)
		if r.ChunkSize == 0 {
			t.Errorf("profile %s: ChunkSize is 0", p)
		}
		if r.ZstdLevel == 0 {
			t.Errorf("profile %s: ZstdLevel is 0", p)
		}
		if r.Notes == "" {
			t.Errorf("profile %s: Notes is empty", p)
		}
	}
}

func TestExtractSourceHints(t *testing.T) {
	tests := []struct {
		meta map[string]string
		want string
	}{
		{map[string]string{"name": "wiktionary_he"}, "wiktionary"},
		{map[string]string{"name": "wikipedia_en_all"}, "wikipedia"},
		{map[string]string{"source": "stackexchange.com"}, "stackexchange"},
		{map[string]string{"title": "Gutenberg Library"}, "gutenberg"},
		{map[string]string{"scraper": "zimit v2.0"}, "zimit"},
		{map[string]string{"name": "some_unknown_archive"}, ""},
		{map[string]string{}, ""},
	}
	for _, tt := range tests {
		got := extractSourceHint(tt.meta)
		if got != tt.want {
			t.Errorf("extractSourceHint(%v) = %q, want %q", tt.meta, got, tt.want)
		}
	}
}

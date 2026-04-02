// Package classify provides coarse-grained content profile classification
// for OZA and ZIM archives to inform conversion and compression strategies.
package classify

// Profile is a content profile label.
type Profile string

const (
	ProfileEncyclopedia Profile = "encyclopedia"
	ProfileDictionary   Profile = "dictionary"
	ProfileBooks        Profile = "books"
	ProfileQAForum      Profile = "qa-forum"
	ProfileDocs         Profile = "docs"
	ProfileMediaHeavy   Profile = "media-heavy"
	ProfilePDFContainer Profile = "pdf-container"
	ProfileMixedScrape  Profile = "mixed-scrape"
)

// AllProfiles returns all defined profiles in decision-rule order.
func AllProfiles() []Profile {
	return []Profile{
		ProfilePDFContainer,
		ProfileMediaHeavy,
		ProfileDictionary,
		ProfileBooks,
		ProfileDocs,
		ProfileQAForum,
		ProfileEncyclopedia,
		ProfileMixedScrape,
	}
}

// Recs holds suggested conversion parameters for a content profile.
type Recs struct {
	ChunkSize       int     `json:"chunk_size"`
	ZstdLevel       int     `json:"zstd_level"`
	DictSamples     int     `json:"dict_samples"`
	Minify          bool    `json:"minify"`
	OptimizeImages  bool    `json:"optimize_images"`
	SearchPruneFreq float64 `json:"search_prune_freq"`
	Notes           string  `json:"notes"`
}

// recommendations maps each profile to its recommended conversion parameters.
var recommendations = map[Profile]Recs{
	ProfileEncyclopedia: {
		ChunkSize: 4 << 20, ZstdLevel: 6, DictSamples: 2000,
		Minify: true, OptimizeImages: true, SearchPruneFreq: 0.5,
		Notes: "balanced defaults for text+image articles",
	},
	ProfileDictionary: {
		ChunkSize: 1 << 20, ZstdLevel: 9, DictSamples: 4000,
		Minify: true, OptimizeImages: false, SearchPruneFreq: 0.3,
		Notes: "smaller chunks for tiny entries; higher compression for repetitive patterns; lower prune threshold",
	},
	ProfileBooks: {
		ChunkSize: 8 << 20, ZstdLevel: 6, DictSamples: 1000,
		Minify: true, OptimizeImages: false, SearchPruneFreq: 0.7,
		Notes: "larger chunks for big entries; fewer dict samples (dissimilar content); higher prune (long documents)",
	},
	ProfileQAForum: {
		ChunkSize: 4 << 20, ZstdLevel: 6, DictSamples: 3000,
		Minify: true, OptimizeImages: true, SearchPruneFreq: 0.4,
		Notes: "more dict samples for repetitive Q&A templates",
	},
	ProfileDocs: {
		ChunkSize: 2 << 20, ZstdLevel: 6, DictSamples: 3000,
		Minify: true, OptimizeImages: false, SearchPruneFreq: 0.5,
		Notes: "smaller chunks for small-medium entries; more dict samples for similar structure",
	},
	ProfileMediaHeavy: {
		ChunkSize: 8 << 20, ZstdLevel: 3, DictSamples: 500,
		Minify: false, OptimizeImages: true, SearchPruneFreq: 0.5,
		Notes: "low zstd level (images are incompressible); image optimization is the real win",
	},
	ProfilePDFContainer: {
		ChunkSize: 4 << 20, ZstdLevel: 6, DictSamples: 1000,
		Minify: false, OptimizeImages: false, SearchPruneFreq: 0.5,
		Notes: "standard defaults; real opportunity is PDF text extraction (future)",
	},
	ProfileMixedScrape: {
		ChunkSize: 4 << 20, ZstdLevel: 6, DictSamples: 2000,
		Minify: true, OptimizeImages: true, SearchPruneFreq: 0.5,
		Notes: "conservative defaults for unknown content",
	},
}

// RecommendOptions returns the suggested conversion parameters for a profile.
func RecommendOptions(p Profile) Recs {
	if r, ok := recommendations[p]; ok {
		return r
	}
	return recommendations[ProfileMixedScrape]
}

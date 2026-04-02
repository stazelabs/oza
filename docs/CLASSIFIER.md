# CLASSIFIER.md -- Archive Content Classifier

## Motivation

OZA archives vary widely in content profile -- Wikipedia articles, Wiktionary dictionaries, Gutenberg books, StackExchange Q&A, TED media, scraped sites. Today, conversion uses fixed defaults for chunk size, compression level, dictionary training, etc. A coarse-grained classifier that detects the archive's content profile enables per-profile strategy defaults, informs a future `ozaclean` tool, and gives users insight into what they're working with.

## Taxonomy: 8 Content Profiles

| Profile | Key Signals | Archetype Sources |
|---|---|---|
| `encyclopedia` | Text+images, 10-30% redirects, moderate avg entry size | Wikipedia, Vikidia, World Factbook |
| `dictionary` | Very many small entries, 40%+ redirects, text dominant | Wiktionary, Wikiquote |
| `books` | Few large entries (avg >50KB), low redirects, text dominant | Gutenberg, Wikisource |
| `qa-forum` | Structured HTML, moderate images, low redirects, many MIME types | StackExchange sites |
| `docs` | Text dominant, <10% images, low redirects | DevDocs, man pages, OpenWrt wiki |
| `media-heavy` | Image+video bytes >60% of total | xkcd, TED talks, photo collections |
| `pdf-container` | PDF bytes >30% of total | Wikisource PDFs, RACHEL |
| `mixed-scrape` | No dominant pattern (fallback) | Zimit-scraped sites |

**Why 8?** Each profile maps to at least one concrete, distinct conversion strategy parameter change. Fewer would conflate genuinely different optimization opportunities (e.g., dictionary has very different chunking needs than books). More would create buckets that map to identical strategies.

## Feature Vector

All features are computable from `stats.ArchiveStats` (for OZA) or ZIM stats with `--deep` (for ZIM). No content reads beyond what `stats.Collect()` already does.

### Byte-fraction features (0.0 - 1.0)

| Feature | Derivation |
|---|---|
| `TextBytesRatio` | Sum of `text/*` MIME bytes / total blob bytes |
| `HTMLBytesRatio` | `text/html` bytes / total blob bytes |
| `ImageBytesRatio` | Sum of `image/*` MIME bytes / total blob bytes |
| `PDFBytesRatio` | `application/pdf` bytes / total blob bytes |
| `VideoBytesRatio` | Sum of `video/*` bytes / total blob bytes |

### Entry-count features

| Feature | Derivation |
|---|---|
| `RedirectDensity` | redirects / (content entries + redirects) |
| `AvgEntryBytes` | total blob bytes / content entries |
| `SmallEntryRatio` | Entries in MIME types with avg < 4KB / total entries (approximation from MIMECensus) |
| `EntryCount` | Absolute content entry count |

### Structural features

| Feature | Derivation |
|---|---|
| `MIMETypeCount` | Number of distinct MIME types |
| `CompressionRatio` | From SectionSummary (compressed / uncompressed) |

### Metadata-derived hints (soft signals)

| Feature | Derivation |
|---|---|
| `SourceHint` | Parsed from metadata `name` or `source` for known keywords: wiktionary, gutenberg, stackexchange, devdocs, zimit, wikisource, ted, vikidia |

## Decision Logic

Rule-based classifier with ordered checks. First match wins. Deliberately not ML -- the feature space is small and the profiles are well-separated by construction.

```
1. PDFBytesRatio > 0.30                                              -> pdf-container
2. ImageBytesRatio + VideoBytesRatio > 0.60                          -> media-heavy
3. RedirectDensity > 0.35 AND SmallEntryRatio > 0.50
   AND TextBytesRatio > 0.70                                         -> dictionary
4. AvgEntryBytes > 50000 AND RedirectDensity < 0.15
   AND TextBytesRatio > 0.50                                         -> books
5. TextBytesRatio > 0.80 AND ImageBytesRatio < 0.10
   AND AvgEntryBytes < 50000
   AND (SourceHint in [devdocs, mankier, openwrt] OR
        RedirectDensity < 0.05)                                       -> docs
6. SourceHint in [stackexchange, stackoverflow]
   OR (TextBytesRatio > 0.50 AND ImageBytesRatio in [0.10, 0.50]
       AND MIMETypeCount > 5 AND RedirectDensity < 0.10)             -> qa-forum
7. HTMLBytesRatio > 0.30 AND RedirectDensity > 0.05                  -> encyclopedia
8. (fallback)                                                         -> mixed-scrape
```

### Confidence scoring

Each classification emits a confidence (0.0 - 1.0) based on how strongly the features match the profile. Confidence is capped at 0.6 when byte-level features are unavailable (e.g., ZIM without deep scan).

## Strategy Recommendations Per Profile

Each profile maps to concrete conversion parameter overrides:

| Profile | ChunkSize | ZstdLevel | DictSamples | Minify | OptImages | SearchPrune | Rationale |
|---|---|---|---|---|---|---|---|
| `encyclopedia` | 4 MB | 6 | 2000 | yes | yes | 0.5 | Balanced defaults work well |
| `dictionary` | 1 MB | 9 | 4000 | yes | no | 0.3 | Smaller chunks (tiny entries); higher compression (repetitive patterns); lower prune (many short entries) |
| `books` | 8 MB | 6 | 1000 | yes | no | 0.7 | Larger chunks (big entries benefit from long-range context); fewer dict samples (dissimilar entries) |
| `qa-forum` | 4 MB | 6 | 3000 | yes | yes | 0.4 | More dict samples (repetitive templates) |
| `docs` | 2 MB | 6 | 3000 | yes | no | 0.5 | Smaller chunks (small-medium entries); more dict samples (similar structure) |
| `media-heavy` | 8 MB | 3 | 500 | no | yes | 0.5 | Don't waste CPU on incompressible images; image optimization is the real win |
| `pdf-container` | 4 MB | 6 | 1000 | no | no | 0.5 | Standard; real opportunity is PDF text extraction (future) |
| `mixed-scrape` | 4 MB | 6 | 2000 | yes | yes | 0.5 | Conservative defaults for unknown content |

### Future transformation opportunities per profile

| Profile | Opportunity |
|---|---|
| `dictionary` | Deduplicate boilerplate HTML wrappers; shared CSS extraction |
| `books` | Chapter-splitting for multi-part entries; table-of-contents generation |
| `media-heavy` | Lossy image re-encoding (WebP/AVIF); thumbnail generation |
| `pdf-container` | PDF-to-markdown extraction; OCR for scanned documents |
| `mixed-scrape` | Web-app artifact removal; CSS/JS deduplication |

## Implementation Plan

### Package location

`cmd/internal/classify/` -- this is a tool concern (informs conversion strategy), not a library concern. The `oza` and `ozawrite` packages should not depend on it.

### New files

| File | Contents |
|---|---|
| `cmd/internal/classify/profiles.go` | `Profile` type constants, recommendation table |
| `cmd/internal/classify/classify.go` | `Features`, `Result`, `Recs` types; `ExtractFromOZA()`, `Classify()`, `RecommendOptions()` |
| `cmd/internal/classify/classify_test.go` | Synthetic feature vectors for each profile + boundary cases |

### Integration points

**`ozainfo --classify`**: After `stats.Collect()`, runs classification and appends the result to text/JSON output. Adds ~15 lines to `cmd/ozainfo/main.go`.

**`zim2oza --auto`**: Before conversion, classifies using the scan phase data and applies recommended `WriterOptions`. Explicit flags still override auto-detected values. Prints detected profile and overrides in `--verbose` mode.

### NOT creating

A separate `ozaclassify` CLI tool. Classification is a ~20-line function call, not a standalone workflow. Embedding in `ozainfo` (inspection) and `zim2oza` (action) covers both use cases.

### ZIM classification without deep scan

For ZIM files, byte-level MIME statistics require reading all content (`--deep`). Without deep mode:
- All `*BytesRatio` features are set to -1 (unknown)
- Classification relies on count-based and metadata heuristics (`RedirectDensity`, `EntryCount`, `MIMETypeCount`, `SourceHint`)
- Confidence is capped at 0.6
- For `zim2oza --auto`, the scan phase already reads all content anyway, so byte-level features are available at no extra cost

## Verification

1. `cd cmd && go test ./internal/classify/...` -- unit tests pass
2. `make test` -- no regressions
3. `make lint` -- clean
4. `bin/ozainfo --classify --json testdata/*.oza` -- profiles match expected archetypes
5. `bin/zim2oza --auto --dry-run --verbose testdata/ray_charles.zim /dev/null` -- shows detected profile and applied overrides

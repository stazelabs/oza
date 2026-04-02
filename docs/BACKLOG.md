# 王座 OZA — Backlog

Outstanding tasks, known issues, and design observations for future work.

**Status:** Prototype / pre-v1. Breaking changes are free. No backward-compatibility
tax yet — we can always pin to "version 1" once the spec and implementation are locked
down. Any conditionality, shims, or migration code can be stripped until then. The only
provision we maintain is the extensibility mechanism itself (unknown sections are
skippable) so future versions can coexist gracefully.

**Philosophy:** Long Now. Resilience. Simplicity. Orthogonality. Every feature must
justify its complexity budget. We are building the antidote to ZIM's accumulated arcana,
not its successor in spirit.

---

## Priority Tiers

- **P0 — Must fix:** Correctness, spec/code disagreements, security issues that affect
  integrity or enable exploitation.
- **P1 — Should fix:** Performance regressions under realistic load, code quality issues
  that impede contribution, testing gaps on critical paths.
- **P2 — Improve:** Ergonomic improvements, documentation polish, nice-to-have features.
- **P3 — Future:** Roadmap features requiring new sections or significant design work.

---

## P0 — Must Fix

*All P0 items have been resolved. See §Completed at the bottom of this file.*

---

## P1 — Should Fix

### Security

*All P1 security items have been resolved. See §Completed at the bottom of this file.*

### Performance

*All P1 performance items have been resolved. See §Completed at the bottom of this file.*

### Code Quality

*All P1 code quality items have been resolved. See §Completed at the bottom of this file.*

### Testing

*All P1 testing items have been resolved. See §Completed at the bottom of this file.*

### Documentation

*All P1 documentation items have been resolved. See §Completed at the bottom of this file.*

---

## P2 — Improve

### 2.29 mimeIndex is O(N) linear scan

`oza/archive.go:432-438`: `Archive.mimeIndex()` linearly scans the MIME table
on every call. Typically <20 entries so negligible, but a `map[string]uint16`
built at load time would be O(1) and trivial.

### 2.5 FrontArticles scan cost

`FrontArticles()` iterates all entries checking `is_front_article` — O(N). For
Wikipedia (6M+ entries), this is a full scan on every call.

**Fix:** Build a `[]uint32` of front-article IDs at load time (already done for
`ozaserve`'s random feature). Expose from the library.

### 2.6 Search ranking

Trigram search has no ranking beyond title-match > body-match > entry-ID order.

**Near-term:** BM25-lite scoring using content size (already in entry records) and
trigram hit count. No new data needed.

### 2.19 Configurable browse exclusions

Per-archive `browse_exclude` metadata key (glob patterns) for filtering non-article
entries from browse views. `zim2oza` could auto-detect ZIM source type and suggest
defaults.

### 2.21 Content dedup detection at read time

SHA-256 content hashes are stored per entry but not leveraged at read time. Exposing
this enables duplicate detection, storage analysis, and cross-archive dedup.

### 2.23 No writer-side index benchmarks

Missing `BenchmarkBuildIndex` covering V1 path/title index construction.

### 2.24 ozainfo CONTENT section compression reporting misleading

`ozainfo` reports the CONTENT section as `none` compression with equal compressed
and uncompressed sizes. This is technically correct (the section stores
pre-compressed chunks as opaque blobs and is not double-compressed), but
misleading — the chunks inside *are* Zstd-compressed. The overall Size Summary
shows the true ratio, but the per-section table gives the wrong impression.

**Fix:** Report per-chunk compression stats in the section table, or annotate
the CONTENT section row with the aggregate chunk compression ratio (e.g.
`zstd (chunks)` instead of `none`).

---

## P3 — Future

### MCP / AI Enhancements

Current tools: `list_archives`, `search_text`, `read_entry`, `get_entry_info`,
`browse_titles`, `get_random`, `get_archive_stats`.
Current resources: `oza://{slug}/metadata`, `oza://{slug}/entry/{id}`.

#### 3.1 AI section implementation priority

From LLM.md, ordered by value and build cost:

1. **PLAIN_TEXT (0x0101)** — Foundation. Everything else depends on clean text with
   stable passage boundaries. Medium build cost, critical value.
2. **CONTEXT_HINTS (0x0103)** — Extractive summaries + token counts. Enables
   `budget_context`. Medium build cost, high value.
3. **PROVENANCE (0x0105)** — Per-entry citation metadata. Low build cost, medium value.
4. **VECTOR_EMBEDDINGS (0x0100)** — Semantic search. High build cost (GPU), high value.
5. **KNOWLEDGE_GRAPH (0x0102)** — Structured intelligence. Medium build cost, medium
   value.
6. **TOOL_MANIFEST (0x0106)** — Self-describing domain tools. Low build cost, low
   value (until ecosystem matures).
7. **MULTIMODAL_EMBED (0x0104)** — Cross-modal search. High build cost. Defer.

#### 3.2 Future MCP tools (require new sections)

| Tool | Depends On | Purpose |
|------|-----------|---------|
| `search_semantic` | VECTOR_EMBEDDINGS (0x0100) | Vector similarity search |
| `search_hybrid` | VECTOR_EMBEDDINGS + trigram | RRF fusion of keyword + semantic |
| `budget_context` | CONTEXT_HINTS (0x0103) | Greedy-pack entries into token budget |
| `read_passage` | PLAIN_TEXT (0x0101) | Read specific passage with heading context |
| `browse_category` | KNOWLEDGE_GRAPH (0x0102) | Category-based navigation |
| `find_entity` | KNOWLEDGE_GRAPH (0x0102) | Entity lookup → entry list |
| `get_related` | KNOWLEDGE_GRAPH (0x0102) | Follow graph edges |

### Features

#### 3.3 ozaclean

A CLI tool to "clean" / repack an OZA file. Opportunities: re-optimize compression,
strip sections, upgrade to a new format version, add/remove signatures.

#### 3.4 Chrome section (FORMAT.md §7.2)

Implement the optional CHROME section:
- `ozawrite/chrome.go` — `AddChromeAsset(role, name, data)` on Writer
- `oza/chrome.go` — parse chrome section, enumerate assets by role
- `cmd/zim2oza/chrome.go` — extract `C/_mw_/` CSS/JS from ZIM into chrome section

Currently `categoryChrome` exists in the converter but entries are skipped.

#### 3.5b SVG minification

GIF→WebP and PNG→WebP transcoding are implemented (see §Completed 3.5/3.16), but SVG
images are served as-is. Minifying SVG (stripping comments, metadata, editor cruft,
collapsing whitespace) could yield meaningful savings for icon-heavy archives.

#### 3.6 Incremental / append mode

See `docs/INCREMENTAL.md`. Optimized rebuild with chunk-level copy. Estimated 6x
speedup for 95%-unchanged Wikipedia.

#### 3.7 Split archives

Multi-part archive support along the lines of ZIM's `.zimaa/.zimab` splits. A
section-type-based approach could be added without changing the core format.

#### 3.8 oza2jsonl export tool

Export structured training data from an OZA archive for LLM fine-tuning. Depends on
PLAIN_TEXT and optionally KNOWLEDGE_GRAPH sections.

#### 3.9 Additional converter sources

**EPUB → OZA implemented** (`epub2oza`). Supports single-book and collection mode
(`--collection`). Collection archives store a `catalog` metadata key (JSON) so
`ozaserve` surfaces individual books on the library page. See FORMAT.md §3.4.

Remaining future sources: static site → OZA, PDF collection → OZA, Markdown
corpus → OZA. A generic ingest pipeline with pluggable source readers would
reduce per-source effort.

### Compression

#### 3.10 Dictionary training panic

`zstd.BuildDict()` can panic on certain inputs. Caught by `defer/recover` in
`trainDictionary()`, but upstream fixes would be better. Track klauspost/compress.

#### 3.11 Minification semantics

HTML minification removes whitespace that may be significant in `<pre>` blocks. No
per-entry opt-out — only global `--no-minify-html`.

#### 3.12 PNG re-encode fidelity

The Go `image/png` round-trip is not guaranteed bit-exact for all valid PNGs.
Optimization is skipped if re-encoded is larger, but no pixel-level verification.

#### 3.13 Per-entry compression control

No way to mark an individual entry as "store uncompressed." MIME-based chunk grouping
handles 99% of cases. A per-entry flag adds complexity for diminishing returns.

### Platform

#### 3.15 No resume for interrupted conversions

A large ZIM → OZA conversion can take hours. If interrupted, must restart. No
checkpointing or resume capability.

**Design question:** Conversions run on build servers, not user laptops. A robust
retry (rerun the command) may be sufficient.

---

## Shelved

These items were investigated and deliberately set aside due to structural limitations or unfavorable cost/benefit tradeoffs. Revisit only if the underlying constraint changes.

### S.1 More aggressive search pruning for qa-forum

Lower `SearchPruneFreq` to 0.25 for `ProfileQAForum`.

**Shelved:** se_codegolf's 13.5% SEARCH_BODY overhead is actually below the corpus median (~19.5%). Benchmark data (25 files) shows 15–30% is the natural cost of trigram search on text-heavy content. Lowering the prune threshold would hurt normal Q&A forums (e.g., cooking.stackexchange) where common trigrams are legitimate search targets. se_codegolf is an unusually odd dataset (source code generates uniformly distributed trigrams); optimizing for it would over-prune everywhere else.

### S.2 Search index size budget

After building the trigram index, if the serialized body index exceeds N% of total content size, iteratively increase prune frequency and re-serialize until it fits.

**Shelved:** Benchmark data (25 files) shows SEARCH_BODY median is ~19.5% of OZA output (range 0.2–33.3%). A 10% budget would trigger on 17/25 archives; even 20% triggers on 10/25. This is not an outlier problem — it is the structural cost of trigram posting lists with roaring bitmaps on compressed text. Any reasonable budget threshold either degrades search quality across the board or is too loose to be worth the complexity. The right approach if search index size ever becomes critical is a more compact index format (FST, minimal perfect hashing), not pruning useful trigrams.

---

## Completed

### 0.1 FORMAT.md SectionDesc layout disagrees with code ~~P0~~

Updated `docs/FORMAT.md` §3.3 to match the implementation: `[40:48] reserved`,
`[48:80] SHA-256`.

### 0.2 CSP sandbox missing on served HTML content ~~P0~~

Added `Content-Security-Policy: sandbox` header to all HTML responses from archive
content in `cmd/ozaserve/handlers.go`.

### 0.3 README API overview is stale / won't compile ~~P0~~

Fixed `README.md`: `entry.BlobSize` → `entry.Size()`, `Search(query, limit)` →
`Search(query, SearchOptions{})`, removed `entry.ContentHash` (on `EntryRecord`, not
`Entry`), corrected cache size default to 8, corrected entry record size to
"variable-length (~15 bytes avg)".

### 0.4 Integer overflow in chunk table allocation ~~P0~~

Added validation in `oza/archive.go` `loadContentSection`: chunk count is checked
against section compressed size before allocating. Prevents int overflow on 32-bit
platforms and OOM from attacker-crafted counts.

### 0.5 Missing FuzzDecodePostingList ~~P0~~

Implemented `FuzzDecodePostingList` in `oza/search_test.go`. Fuzzes the roaring bitmap
deserialization path used by trigram search.

### 0.6 Decompression bomb check fires after full allocation ~~P0~~

Added `zstd.WithDecoderMaxMemory(1 GiB)` to all pooled decoders in `oza/compress.go`.
The limit is now enforced during decompression itself, preventing memory exhaustion
before the post-decompression size check.

### 1.1 Missing ReadHeaderTimeout (slowloris) ~~P1 Security~~

Added `ReadHeaderTimeout: 10 * time.Second` to the HTTP server in
`cmd/ozaserve/main.go`.

### 1.2 MCP resource template missing size guard ~~P1 Security~~

Added `maxReadContentSize` check before `ReadContent()` in the MCP resource template
handler in `cmd/internal/mcptools/mcptools.go`. Prevents excessive memory use from
large entries during HTML-to-markdown conversion.

### 1.3 readBlob always copies — no zero-copy path ~~P1 Performance~~

Added `readBlobSlice` (zero-copy sub-slice of cached chunk data) in `oza/chunk.go`.
Added `Entry.WriteTo(w io.Writer)` and `Entry.ReadContentSlice()` on the public API.
`ReadContent()` still copies for callers that expect to own the buffer. Snippet
extraction now uses the zero-copy path.

### 1.4 Roaring bitmaps deserialized on every search query ~~P1 Performance~~

Added a 512-entry LRU cache of deserialized `*roaring.Bitmap` keyed by trigram in
`oza/search.go`. Since posting lists are immutable, cached bitmaps are safe to reuse
across queries. Eliminates repeated heap allocation for frequently queried trigrams.

### 1.5 Browse page does O(N) title scan per request ~~P1 Performance~~

Pre-built a `letterOffsets` map (letter → title-index offset + count) at load time in
`computeLetterIndex`. `handleBrowse` now uses `BrowseTitles(offset, limit)` with the
pre-computed range — O(page_size) instead of O(N).

### 1.6 O(N²) StripHTML in snippet extraction ~~P1 Performance~~

Replaced `b.String()` call in `ensureSpace` with a `lastRune` variable — O(1) per
call instead of O(N). `ForEntry` now truncates HTML to 4 KB before parsing, and uses
the zero-copy `ReadContentSlice()` path.

### 1.7 HTML bar injection allocates full-body lowercase copy ~~P1 Performance~~

Replaced `injectHeaderBar`/`injectFooterBar` (two `bytes.ToLower(body)` calls) with a
single `injectBars` function that uses `indexCaseInsensitive` — a byte-by-byte scan
that lowercases only during comparison, eliminating two full-body allocations.

### 1.8 zim2oza buffers all content in memory for large conversions ~~P1 Performance~~

Replaced in-memory `buffered` slice with a two-pass temp-file approach: phase 2a writes
content to a temp file and keeps only metadata + offsets in RAM; phase 2c reads content
back one entry at a time via `ReadAt` with a reusable buffer. Memory usage is now
proportional to metadata size, not total content size.

### 1.9 Double content copy during dictionary training buffer ~~P1 Performance~~

`bufferForTraining` now makes a single copy of content shared between the dictionary
sample and the pending entry (both read-only), halving memory usage during the training
phase.

### 1.10 collectOZAPaths + makeSlug duplicated between CLIs ~~P1 Code Quality~~

Extracted `CollectOZAPaths` and `MakeSlug` into `cmd/internal/loadutil/`. Both
`ozamcp` and `ozaserve` now delegate to the shared package.

### 1.11 sectionTypeName duplicated — now a String() method ~~P1 Code Quality~~

Added `func (SectionType) String() string` and `func CompressionName(uint8) string`
in the `oza` package. Replaced duplicated switches in `ozainfo`, `ozaserve/info.go`,
and `mcptools` with calls to the canonical methods.

### 1.12 Sentinel errors returned without context ~~P1 Code Quality~~

`Index.Search` now wraps `ErrNotFound` with the search key:
`fmt.Errorf("oza: entry by key %q: %w", key, ErrNotFound)`.

### 1.13 WriterOptions zero-value turns off all features ~~P1 Code Quality~~

Exported `DefaultOptions()` and updated `NewWriter` to preserve defaults when a
zero-value `WriterOptions{}` is passed. Callers that set any field trigger explicit
boolean handling; otherwise all features remain enabled.

### 1.14 TrigramBuilder exported but should be internal ~~P1 Code Quality~~

Renamed to `trigramBuilder` (unexported). Only used within `ozawrite` internals.

### 1.15 ForEachEntryRecord silently swallows parse errors ~~P1 Code Quality~~

Added `ForEachEntryRecordErr` variant in `oza/archive.go` that propagates parse errors
and supports early termination via `fn` returning an error.

### 1.22 FORMAT.md index format missing string table fields ~~P1 Documentation~~

Updated FORMAT.md §3.7 to document the IDX1 string table header fields
(`string_table_count`, `string_table_size`) and token-encoded key format.

### 1.23 INDICES.md search version says 3, code uses 1 ~~P1 Documentation~~

Corrected `version uint32 = 3` to `version uint32 = 1` in INDICES.md.

### 1.24 OZAWRITE.md stale claims ~~P1 Documentation~~

Corrected `ZstdLevel` default from 19 to 6. Corrected entry record description
from "40 bytes each" to "variable-length (~15 bytes each on average)".

### 1.25 FORMAT.md test vectors promised but absent ~~P1 Documentation~~

Reworded §11 from "The specification includes a reference test.oza file" to
"Test vectors are planned but not yet included."

### 2.1 MD5 used for ETag generation ~~P2~~

Replaced MD5 with SHA-256 truncated to 16 bytes in `cmd/ozaserve/main.go` `makeETag`.

### 2.2 Insertion sort in searchTwoTier ~~P2~~

Replaced hand-rolled insertion sort in `searchTwoTier` with `slices.SortFunc`
(Go 1.21+). More idiomatic and equally performant for small N.

### 2.3 compressZstd creates a new encoder per call ~~P2~~

Replaced per-call `zstd.NewWriter` in `compressZstd` with a package-level
`sectionEncoderCache` that reuses encoders. Eliminates multi-MB allocation per
section compression call.

### 2.4 No streaming content API ~~P2~~

Added `Entry.WriteTo(w io.Writer)` (zero-copy via `readBlobSlice`) and
`Entry.ReadContentSlice()` in `oza/entry.go`. Resolved as part of P1 item 1.3.

### 2.8 Metadata format validation ~~P2~~

Added `ValidateMetadataStrict` in `oza/metadata.go` that checks value formats beyond
presence: `date` must be ISO 8601, `language` must be BCP-47, string keys must be
non-empty valid UTF-8, `favicon_entry`/`main_entry` must be decimal uint32. Returns
all issues as `[]ValidationError`. Writer enforces strict validation by default via
`StrictMetadata` option (default true); reader stays tolerant.

### 2.10 ozainfo uses raw os.Args instead of cobra ~~P2~~

Migrated `cmd/ozainfo/main.go` to cobra, consistent with all other CLI tools.

### 2.11 Duplicate isCJKRune and signatureRecordSize ~~P2~~

Exported `IsCJKRune` from `oza/search.go` and `SignatureRecordSize` from
`oza/signature.go`. Removed duplicates from `ozawrite/search.go` and
`ozawrite/signature.go`; both now reference the `oza` package.

### 2.12 Constant doc comments ~~P2~~

Added godoc-style doc comments to all constants in `oza/constants.go`: `SectionType`
values, `EntryType` values, compression values, header flags, and format constants.

### 2.13 CONTRIBUTING.md code layout incomplete ~~P2~~

Expanded the `docs/` section in CONTRIBUTING.md to list all documentation files.

### 2.14 EMBEDDINGS.md relative links broken from docs/ ~~P2~~

Fixed source links from `](ozawrite/...)` to `](../ozawrite/...)` and
`](oza/...)` to `](../oza/...)` so they resolve correctly from the `docs/` directory.

### 2.15 LLM.md missing status banner ~~P2~~

Added prominent banner: "Status: Design document. The AI-specific section types
described below (0x0100–0x0106) are not yet implemented."

### 2.16 Missing ozakeygen from README CLI section ~~P2~~

Added `ozamcp` and `ozakeygen` sections to the README CLI Tools area.

### 3.5 Image format conversion (PNG → WebP) ~~P3~~

Implemented GIF→WebP (via `gif2webp`) and lossless PNG→WebP (via `cwebp`) in
`ozawrite/transcode.go`. No CGo — uses external CLI tools discovered at startup.
Integrated into `zim2oza` with `--transcode` flag (auto/off/require). Skips
transcoding if output is larger than input.

### 3.16 Image optimization limited to JPEG ~~P3~~

GIF→WebP and PNG→WebP transcoding added alongside existing JPEG optimization.
SVG minification remains future work.

### 3.17 SVG carve-out from image MIME group ~~P3~~

Routed `image/svg+xml` to a `"svg"` MIME group instead of `"image"` in
`ozawrite/chunk.go` `mimeGroup()`. SVG chunks now get zstd compression +
dictionary training instead of `CompNone`. Result: wp_en_chemistry_maxi went
from +7.2% regression to -7.3% improvement (-68 MB).

### 3.18 Raise zstd level for text-heavy profiles ~~P3~~

Set `ZstdLevel: 11` for `ProfileQAForum` and `ProfileDocs` in
`cmd/internal/classify/profiles.go`. klauspost maps this to
`SpeedBestCompression`. Result: se_codegolf went from +2.4% regression to
-0.9% improvement (-12.3 MB).

### 3.19 Parallel image transcoding worker pool ~~P3~~

Pre-transcodes images in a goroutine worker pool (up to 8 workers) in
`cmd/zim2oza/convert.go` `addEntriesParallel()`, with a reorder buffer to
maintain sequential `AddEntry` calls. Result: xkcd transform phase 153.7s →
2.2s (69x faster). Total wall time: 157s → 28s (5.6x).

### 3.20 Pipe-based transcoding (eliminate temp files) ~~P3~~

`cwebp` uses stdin/stdout pipes (`runToolPipe`). `gif2webp` uses a temp file
for input and stdout for output (`runToolFile`). Eliminates most temp-file I/O
overhead during transcoding.

### 3.21 Skip transcoding for tiny images ~~P3~~

Early return in `ozawrite/transcode.go` `Transcode()` for PNGs < 2 KB and
GIFs < 1 KB (constants `minPNGTranscodeSize`, `minGIFTranscodeSize`). Avoids
tool-launch overhead for images too small to benefit.

### 3.22 Try-compress image chunks with zstd ~~P3~~

Image chunks are trial-compressed at `SpeedFastest` (level 1) in
`ozawrite/compress.go` `compressionWorker()`; the compressed version is kept
only if smaller than raw. Chunks of many small JPEG/WebP images share header
structure and compress ~5–9%. On a 2-book EPUB collection the image content
section shrank from 589 KiB to 536 KiB (9%), flipping the overall archive
from 1.06× to 0.98×.

### 3.23 JPEG→WebP lossy transcoding ~~P3~~

Opt-in via `--transcode-lossy-jpeg` in `ozawrite/transcode.go`. Uses
`cwebp -q 80 -m 4` via stdin/stdout pipe; keeps original if WebP output is
larger. Minimum size threshold: 1 KB. Photo-heavy encyclopedias with large
JPEGs benefit significantly (~25–35% savings per image). Requires
`brew install webp` / `apt install webp`.

### 3.24 Brotli as alternative compression codec ~~P3~~

`CompBrotli = 3` added to `oza/constants.go`. For non-dict text chunks, the
compression worker in `ozawrite/compress.go` trial-compresses with both zstd
and Brotli, keeping whichever is smaller. Brotli quality is mapped from the
zstd level. Reader decompression uses `github.com/andybalholm/brotli`
(pure Go). FORMAT.md updated: compression field values are now
`0=none, 1=zstd, 2=zstd+dict, 3=brotli`. On the 2-book EPUB test corpus:
635 KiB → 625 KiB (1.6% improvement).

### 3.25 AVIF image transcoding ~~P3~~

Opt-in via `--transcode-avif` in `ozawrite/transcode.go`. Discovered at
startup alongside gif2webp/cwebp. Uses `avifenc` with temp-file input and
stdout capture. PNG: `avifenc -s 6 --lossless`. JPEG: `avifenc -s 6 -q 70`.
Falls back to WebP if AVIF output is larger or avifenc is unavailable. Slower
than WebP (~3–10×) but leverages the existing parallel worker pool. AVIF
achieves 20–50% smaller files than WebP on photographic content. Requires
`brew install libavif` / `apt install libavif-bin`.

### 3.26 xxhash for dedup and content hashing ~~P3~~

Replaced `sha256.Sum256` with `xxhash.Sum64` (`github.com/cespare/xxhash/v2`)
for dedup map lookups and the 8-byte content hash stored in entry records in
`ozawrite/dedup.go`, `ozawrite/writer.go`, and `oza/checksum.go`. File-level
and section-level integrity checks remain SHA-256. xxhash is 5–10× faster than
SHA-256; the on-disk content hash was already truncated to 8 bytes (uint64) so
this is a pure implementation improvement with no format change.

### 1.16 No tests for ozaserve HTTP handlers ~~P1 Testing~~

Added `httptest`-based tests in `cmd/ozaserve/handlers_test.go` using a small
in-memory archive. Tests cover: content serving (HTML, CSS, images, redirects),
search (valid query, empty query, no-index archive, JSON format), browse
(pagination, letter filtering), random article, info pages (per-archive, JSON,
global), error pages (404, unknown archive), security headers (CSP, X-Frame-Options,
X-Content-Type-Options, Referrer-Policy), method check (405), ETag/conditional
requests, and favicon serving.

### 1.17 No tests for ozaserve URL utilities ~~P1 Testing~~

Added `cmd/ozaserve/urlutil_test.go` with targeted unit tests for `entryURL` and
`entryHref` including: path traversal attempts (`../`), percent-encoded paths,
query string and fragment characters (`?`, `#`), empty paths, Unicode paths,
and double-encoding verification.

### 1.18 CI coverage report only covers root module ~~P1 Testing~~

Added a second coverage step in `.github/workflows/ci.yml`:
`cd cmd && go test -coverprofile=../coverage-cmd.out -covermode=atomic ./...`
and updated the Codecov upload to include both `coverage.out` and `coverage-cmd.out`.

### 1.19 zim2oza tests depend on external fixture, likely skipped in CI ~~P1 Testing~~

Added `make testdata` step to the CI test job (Linux only) so that
`testdata/small.zim` and other Tier 1 fixtures are downloaded before tests run.
The `zimAvailable(t)` skip guard remains as a fallback for local development
without fixtures.

### 1.20 No concurrent access benchmarks ~~P1 Testing~~

Added four parallel benchmarks in `oza/bench_test.go`:
- `BenchmarkReadContentParallel` — concurrent content reads via `b.RunParallel`
- `BenchmarkEntryByPathParallel` — concurrent path lookups
- `BenchmarkCacheThrashing` — small cache (4 slots) with 50 diverse entries
- `BenchmarkSearchParallel` — concurrent search with varied queries

### 1.21 No fuzz targets in ozawrite ~~P1 Testing~~

Added three fuzz targets in `ozawrite/fuzz_test.go`:
- `FuzzTrigramBuilder` — fuzzes trigram index builder, feeds output to reader parser
- `FuzzStringTableSerialize` — fuzzes string table serialization round-trip
- `FuzzWriterRoundTrip` — fuzzes full write→open→read cycle with arbitrary content

Updated `Makefile` `fuzz` target to include the three new ozawrite fuzz targets.

### 2.22 No tests for ozawrite pipeline internals ~~P2~~

Added `ozawrite/pipeline_test.go` with direct unit tests for pipeline internals:
- **dedup.go**: `dedupMap` Check/Register/CheckHash, miss on empty map, hit after register
- **chunk.go**: `chunkBuilder` addBlob offsets, `uncompressedBytes` concatenation, empty chunk,
  `mimeGroup` classification (HTML/CSS/JS/SVG/image/other, MIME param stripping),
  `ChunkKey` small-entry threshold and image exemption, `marshalChunkDesc` round-trip
- **optimize_image.go**: `stripJPEGMetadata` with valid JPEG (APP0+COM stripped, DQT kept),
  non-JPEG bail, empty input, stuffed byte bail, truncated data bail, RST marker preservation,
  invalid segment length bail; `shouldKeepJPEGMarker` for all APP/COM/SOF/DHT/DQT markers;
  `isImageMIME` for jpeg/jpg/png/html
- **minify.go**: `minifyContent` with nil/empty content
- **compress.go**: `mapEncoderLevel` range boundaries, `mapBrotliQuality` mapping,
  `compressZstd` and `compressBrotli` round-trips, `encoderCache` reuse,
  `trainDictionary` with empty and too-small samples, `compressRawSection` tiny fallback
  and compressible section
- **assembly.go**: `buildMIMETable` mandatory index 0/1/2 + dedup, `buildRedirectSection`
  empty and with entries
- **pipeline.go** integration: chunk splitting (tiny ChunkTargetSize forcing multiple chunks),
  dedup (identical content under different paths), minification fallback (malformed HTML),
  dictionary training (low DictSamples threshold), image chunk handling (mixed MIME types)

### 2.7 Metadata duplicate keys ~~P2~~

`ParseMetadata()` now returns `ErrDuplicateMetadataKey` (wrapping the key name) when
a key appears more than once in the binary metadata section. Added sentinel error
`ErrDuplicateMetadataKey` in `oza/errors.go`. The writer (`SetMetadata`) uses a Go map
so duplicates are structurally impossible on the write path; this protects against
hand-crafted or corrupted archives on the read path. Tests added for both duplicate
and unique key parsing.

### 2.17 Structured access logging ~~P2~~

Added `cmd/ozaserve/accesslog.go` with a `responseRecorder` wrapper that captures
status code and bytes written, and an `accessLog` middleware that emits structured
per-request log lines via `slog`. Middleware is a no-op pass-through when the logger
is nil, so it composes cleanly with the existing server setup.

### 2.18 Markdown content rendering ~~P2~~

`ozaserve` now renders `text/markdown` entries via Goldmark (with table extension)
before serving. MIME type is rewritten to `text/html; charset=utf-8`, CSP sandbox
is applied, and the navigation/footer bars are injected — identical treatment to
native HTML entries. Falls back to `<pre>`-escaped plaintext if Goldmark fails.
Test added in `handlers_test.go`.

### 2.20 Entry enumeration by MIME type ~~P2~~

Built a `mimeToEntries map[uint16][]uint32` index in `Archive` at load time via
`buildMIMEIndex()` — a single O(N) pass over entry records using the existing
`ParseVarEntryRecord` pattern. Added four public APIs in `oza/iter.go`:
`EntriesByMIME(string) iter.Seq[Entry]`, `EntriesByMIMEErr(string) iter.Seq2[Entry,error]`,
`EntryCountByMIME(string) int`, and `MIMEEntryCount(uint16) int`. Memory overhead
is one `uint32` per content entry (~24 MB at Wikipedia scale).

### 3.14 Cross-platform CI ~~P3~~

The `test` job already ran on all three platforms via an OS matrix
(`ubuntu-latest`, `macos-latest`, `windows-latest`). Extended the `build` job with
the same matrix: `go build ./...` now runs natively on all three OSes, verifying
the library and all CLI tools compile on Windows and macOS. The Windows mmap stub
(`io_mmap_windows.go`) is exercised by the Windows test run. `make build` (which
uses Unix Makefile syntax) remains Linux-only as a full binary assembly step.

### 2.9 Benchmark regression tracking ~~P2~~

Added a `bench` job to `.github/workflows/ci.yml` using
`benchmark-action/github-action-benchmark@v1`. On every push to `main` the job
runs `go test -bench=. -benchmem -count=5 ./oza/ ./ozawrite/`, stores results in
the `gh-pages` branch, and builds a trend chart. On PRs it compares against the
stored baseline and posts a comment if any benchmark regresses beyond 200%.
`make bench` updated to use `-count=5` so local runs produce `benchstat`-compatible
output.

### 0.7 Integer overflow in posting list bounds on 32-bit ~~P0~~

Changed `end := int(postingOff) + int(postingLen)` to `uint64` arithmetic in
`oza/search.go` `lookup()`. The bounds check now uses `uint64` comparison,
preventing overflow on 32-bit platforms where two large `uint32` values could
wrap a signed `int` negative and bypass the guard.

### 0.8 Integer overflow in signature record allocation on 32-bit ~~P0~~

Added overflow guard in `oza/signature.go` `ReadSignatures()` before the
`make([]byte, int(count)*SignatureRecordSize)` allocation. If
`uint64(count) * uint64(SignatureRecordSize)` exceeds `math.MaxInt`, the
function returns an error instead of panicking or allocating a wrong-sized buffer.

### 0.9 Discarded error in trainAndFlushPending ~~P0~~

`ozawrite/pipeline.go` `trainAndFlushPending()` now checks the error from
`w.addToChunk()` during the pending-entry flush loop. On first error, stores it
as `w.pipelineErr` (matching the parallel pipeline pattern) and breaks out of
the loop. The error surfaces via `flushChunk` on subsequent `AddEntry` calls
or during `Close`.

### 1.26 fileReader.ReadAt missing negative offset check ~~P1 Security~~

Added `off < 0` guard to `oza/io.go` `fileReader.ReadAt`, matching the existing
check in `mmapReader.ReadAt`. Both I/O backends now consistently return
`io.EOF` for negative offsets.

### 1.27 Panics in sync.Pool decoders ~~P1 Security~~

Replaced `panic()` calls in `sync.Pool.New` callbacks in `oza/compress.go` with
error returns. Pool entries are now `any` (either `*zstd.Decoder` or `error`).
`decodeZstd` and `decodeZstdDict` type-switch on the pool result and propagate
errors to callers instead of crashing the host process.

### 1.28 ozaserve /_info exposes runtime internals ~~P1 Security~~

Added `--info-token` flag to `cmd/ozaserve`. When set, the `/_info`,
`/{archive}/-/info`, and `/{archive}/-/info.json` endpoints require the token
via `?token=` query parameter or `Authorization: Bearer` header. Returns 403
Forbidden on mismatch. When unset, endpoints remain open (backward compatible).

### 1.29 O(N) LRU scan in bitmap cache ~~P1 Performance~~

Replaced the `[]*bitmapCacheEntry` slice in `oza/search.go` with a
`container/list.List` + `map[[3]byte]*list.Element`. Cache hit promotion is now
O(1) via `list.MoveToFront`; eviction is O(1) via `list.Back` + `list.Remove`.
Previously O(N) per lookup with a 512-entry cache.

### 1.30 Unused `contentCount` field in Writer ~~P1 Code Quality~~

Removed the `contentCount` field and its three increment sites from
`ozawrite/writer.go`. The field was incremented but never read.

### 1.31 Encoder leak in validateDict on zero samples ~~P1 Code Quality~~

Moved `enc.Close()` to a `defer` immediately after creation in
`ozawrite/compress.go` `validateDict()`. Replaced the in-loop `enc.Close()`
with `enc.Flush()` and `enc.Reset()`. The encoder is now always closed,
including when the sample list is empty.

### 1.32 No tests for CLI tools beyond ozaserve ~~P1 Testing~~

Added smoke tests for five CLI tools using a shared `cmd/internal/testutil`
package that builds a small in-memory OZA archive:
- `cmd/ozainfo/main_test.go` — default output, `--json`, missing file
- `cmd/ozacat/main_test.go` — `-l` list, `-m` meta, extract, missing file
- `cmd/ozasearch/main_test.go` — default, `--json`, `--title-only`, no-index, missing file
- `cmd/ozaverify/main_test.go` — default, `--sections`, `--all`, missing file
- `cmd/ozakeygen/main_test.go` — file output, stdout output

### 1.33 Search query edge cases untested ~~P1 Testing~~

Added `oza/search_cjk_test.go` with three test functions:
- `TestCJKQueryGrams` — table-driven: empty, short ASCII, single/multi CJK,
  mixed CJK+Latin, Korean (Hangul), invalid UTF-8
- `TestCJKQueryGramsDedup` — verifies repeated characters don't produce
  duplicate grams
- `TestCJKQueryGramsNoPanicOnAllInputs` — edge-case fuzz: empty, truncated
  UTF-8, null bytes, mixed valid/invalid

### 1.34 ozacmp missing from README CLI section ~~P1 Documentation~~

Added `ozacmp` section to README.md under CLI Tools, with usage examples for
default, `--format md`, and `--deep` modes.

### 1.35 Design documents lack implementation status headers ~~P1 Documentation~~

Strengthened the status banner in `docs/LLM.md` to explicitly state section
types 0x0100–0x0106 "do not exist in code yet" and that the body "uses present
tense as design intent." Added a new status banner to `docs/INCREMENTAL.md`
marking it as a design document with implementation deferred.

### 2.25 JSON encoding errors unchecked in ozaserve ~~P2~~

All three `json.NewEncoder(w).Encode(results)` calls in
`cmd/ozaserve/handlers.go` (`handleSearchAll`, `handleSearchJSON`,
`handleSearchPage`) now check the error return and log failures via
`log.Printf`.

### 2.26 Search result limit not enforced across archives ~~P2~~

Added an inner-loop `if len(results) >= limit { break }` in
`handleSearchAll` so the per-result append loop also respects the global
cap. Previously the outer loop checked after processing all results from
an archive, which could overshoot by one archive's worth.

### 2.27 ozaserve archive loading duplicated with ozamcp ~~P2~~

Extracted `CollectFrontArticleIDs(a *oza.Archive) []uint32` into
`cmd/internal/loadutil/`. Both `cmd/ozaserve` and `cmd/ozamcp` now delegate
to the shared function instead of inlining identical `ForEachEntryRecord`
loops.

### 2.28 buildMIMEIndex uses append without pre-allocation ~~P2~~

Rewrote `oza/archive.go` `buildMIMEIndex()` as a two-pass algorithm: pass 1
counts entries per MIME index, pass 2 pre-allocates slices with exact
capacity and fills them. Eliminates intermediate slice growth and the
post-loop `slices.Clip` trim.

### 2.30 Writer accepts unbounded entry content ~~P2~~

Added `if len(content) > math.MaxUint32` guard at the top of
`ozawrite/writer.go` `AddEntry()`. Returns a descriptive error instead of
silently truncating `blobSize` to `uint32`.

### 2.31 CompressWorkers unbounded ~~P2~~

Added `const maxCompressWorkers = 32` in `ozawrite/writer.go` `NewWriter()`.
Values above the cap are silently clamped. Prevents callers from spawning
excessive goroutines and zstd encoders.

### 2.32 FORMAT.md section type 0x0008 gap undocumented ~~P2~~

Added `| 0x0008 | — | Reserved (not used in v1) |` row to the section type
table in `docs/FORMAT.md`. Added a comment `// 0x0008 is reserved and
intentionally unused.` above `SectionChrome` in `oza/constants.go`.

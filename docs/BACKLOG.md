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

#### 1.3 readBlob always copies — no zero-copy path

`oza/chunk.go:144-160` — `readBlob` unconditionally allocates and copies from the
decompressed chunk on every call. For the HTTP server, this means every request
allocates a blob-sized buffer that is immediately GC'd. For 50–200 KB articles under
load, this produces significant GC pressure.

**Fix:** Add an internal `readBlobSlice` that returns a sub-slice of cached chunk data
(zero-copy). Use in the HTTP handler where content is written immediately. Keep the
copying `ReadContent()` for the public API where callers expect to own the buffer.
Alternatively, add a `WriteTo(w io.Writer)` method on `Entry`.

#### 1.4 Roaring bitmaps deserialized on every search query

`oza/search.go:224-252` — `lookup` deserializes a roaring bitmap from wire format for
every trigram on every query. For a 3-word query, that's 30+ bitmap deserializations
with heap allocation and copying per query.

**Fix:** Add an LRU cache of deserialized `*roaring.Bitmap` keyed by trigram. Since
posting lists are immutable, cached bitmaps are safe to reuse. Even 256–512 entries
would cover the most common trigrams.

#### 1.5 Browse page does O(N) title scan per request

`cmd/ozaserve/handlers.go:298-331` — `handleBrowse` iterates every entry via
`EntriesByTitle()` to find entries matching a letter. For Wikipedia (6M+ entries),
this is a full scan on every browse page request.

**Fix:** Pre-build a letter-to-offset-range map at load time (letter counts are already
computed in `computeLetterCounts`). Use `BrowseTitles(offset, limit)` with the
pre-computed offset. Turns O(N) into O(page_size).

#### 1.6 O(N²) StripHTML in snippet extraction

`cmd/internal/snippet/snippet.go:43-49` — `ensureSpace` calls `b.String()` which
copies the entire builder content to check the last rune. Called hundreds of times per
article, this is O(N²) overall. Additionally, `ForEntry` at `:278-291` decompresses
and HTML-parses the full content for each of the 20 search results.

**Fix:** Track the last rune with a `lastRune rune` variable (O(1) per call). Truncate
content before HTML parsing — only the first ~1000 bytes are needed for a snippet.

#### 1.7 HTML bar injection allocates full-body lowercase copy

`cmd/ozaserve/handlers.go:452-483` — Both `injectHeaderBar` and `injectFooterBar` call
`bytes.ToLower(body)`, allocating a full copy of the HTML body (×2 calls) just to
search for `<body` and `</body` tags.

**Fix:** Search for both `<body` and `<BODY` (and `<Body`) directly with `bytes.Index`,
avoiding the full-body allocation. Combine both functions into a single pass.

#### 1.8 zim2oza buffers all content in memory for large conversions

`cmd/zim2oza/convert.go:348-448` — The converter reads all ZIM content entries into a
`buffered` slice, re-sorts by chunk key, then feeds the writer. For full Wikipedia
(6M+ entries, many GBs), this requires all decompressed content in RAM simultaneously.

**Fix:** Two-pass approach: (1) write entries to a temp file indexed by (chunkKey, path),
(2) read back in chunk-key order. Or feed entries in cluster order and let the writer
handle grouping. The MIME-locality sort could be done at the chunk level instead.

#### 1.9 Double content copy during dictionary training buffer

`ozawrite/pipeline.go:13-37` — `bufferForTraining` makes two copies of content: one
for the dictionary sample and one for the pending entry. For 2000+ entries in the
training phase, this doubles memory usage.

**Fix:** Share a single copy between the sample and pending entry. Both are read-only.

### Code Quality

*All P1 code quality items have been resolved. See §Completed at the bottom of this file.*

### Testing

#### 1.16 No tests for ozaserve HTTP handlers

`cmd/ozaserve/` has 8 source files with zero test coverage. All HTTP handlers (content
serving, search, browse, info, error pages) are untested. This is the most user-facing
surface area.

**Fix:** Add `httptest`-based tests using a small in-memory archive. Test:
- Content serving (HTML, CSS, images, redirects)
- Search (valid query, empty query, no-index archive)
- Browse (pagination, letter filtering)
- Error pages (404, invalid paths)
- Security headers (CSP, X-Frame-Options)

#### 1.17 No tests for ozaserve URL utilities

`cmd/ozaserve/urlutil.go` handles URL path parsing/normalization with no tests.
URL handling edge cases (path traversal, double-encoding, trailing slashes) are a
common source of bugs and security issues.

**Fix:** Add targeted unit tests including path traversal attempts (`../`),
percent-encoded paths, and empty/root paths.

#### 1.18 CI coverage report only covers root module

`.github/workflows/ci.yml` coverage job only runs `go test -coverprofile` from the
repo root, missing all `cmd/` module tests. Codecov coverage is incomplete.

**Fix:** Add a second step: `cd cmd && go test -coverprofile=../coverage-cmd.out ./...`
and upload both files.

#### 1.19 zim2oza tests depend on external fixture, likely skipped in CI

All 4 tests in `cmd/zim2oza/convert_test.go` call `zimAvailable(t)` which skips if
`testdata/small.zim` doesn't exist. CI has no step to download this fixture, so these
tests are likely always skipped.

**Fix:** Add `make testdata` as a CI step, or embed a minimal ZIM fixture in the repo.

#### 1.20 No concurrent access benchmarks

`oza/bench_test.go` — all benchmarks are single-goroutine. No benchmarks for concurrent
access (the primary HTTP server use case). Lock contention in the chunk cache and cache
thrashing under diverse access patterns are not measured.

**Fix:** Add `BenchmarkReadContentParallel` using `b.RunParallel`. Add a cache-thrashing
benchmark. Add concurrent search benchmark.

#### 1.21 No fuzz targets in ozawrite

All 7 fuzz targets are in `oza/` (reader). The writer has zero fuzz targets despite
complex serialization (trigram builder, index builder, string table, compression).

**Fix:** Add fuzz targets for `trigramBuilder.Build()`, string table serialization,
and the compression path. Fuzz the writer's output by feeding it to the parser.

### Documentation

*All P1 documentation items have been resolved. See §Completed at the bottom of this file.*

---

## P2 — Improve

### ~~2.1 MD5 used for ETag generation~~ — Resolved

*Replaced with SHA-256 truncated to 16 bytes. See §Completed.*

### 2.2 Insertion sort in searchTwoTier

`oza/archive.go:761-769` — Hand-rolled insertion sort. Fine for small result sets but
non-idiomatic.

**Fix:** Use `slices.SortFunc` (Go 1.21+). Optimized for small N as well.

### 2.3 compressZstd creates a new encoder per call

`ozawrite/compress.go:32-58` — The standalone `compressZstd` creates a new
`zstd.NewWriter` on every call. Each encoder allocates several MB. The `encoderCache`
exists but is only used for chunk compression.

**Fix:** Use `sync.Pool` or the existing `encoderCache` for section compression.

### 2.4 No streaming content API

`Entry.ReadContent()` returns `[]byte` — the entire decompressed blob. For large
entries (50 MB video), this is wasteful. An `io.Reader` or `WriteTo(w io.Writer)` API
would allow streaming without full materialization.

**Considerations:** Chunks are decompressed whole, so the benefit is mainly avoiding
the copy from cache to a new allocation.

### 2.5 FrontArticles scan cost

`FrontArticles()` iterates all entries checking `is_front_article` — O(N). For
Wikipedia (6M+ entries), this is a full scan on every call.

**Fix:** Build a `[]uint32` of front-article IDs at load time (already done for
`ozaserve`'s random feature). Expose from the library.

### 2.6 Search ranking

Trigram search has no ranking beyond title-match > body-match > entry-ID order.

**Near-term:** BM25-lite scoring using content size (already in entry records) and
trigram hit count. No new data needed.

### 2.7 Metadata duplicate keys

`oza/metadata.go` `ParseMetadata()` — if a key appears twice, the second value
silently overwrites the first.

**Fix:** Return error on duplicate keys (strict mode) or at least log a warning.

### 2.8 Metadata format validation

Required keys (`date`, `language`) are checked for presence but not format. `date`
should be ISO 8601, `language` should be BCP-47.

**Fix:** Add optional strict validation. Writer enforces format; reader tolerates
sloppiness but exposes `ValidateMetadata()`.

### 2.9 Benchmark regression tracking

Benchmarks exist but results aren't tracked across commits.

**Fix:** Consider `benchstat` in CI or a lightweight tracking solution.

### ~~2.10 ozainfo uses raw os.Args instead of cobra~~ — Resolved

*Migrated to cobra. See §Completed.*

### 2.11 Duplicate isCJKRune and signatureRecordSize

`oza/search.go:74` and `ozawrite/search.go:54` share identical `isCJKRune`.
`oza/signature.go:9` and `ozawrite/signature.go:16` share `signatureRecordSize`.

**Fix:** Export from `oza` and reference from `ozawrite`, or accept the small
duplication.

### ~~2.12 Constant doc comments~~ — Resolved

*Added godoc comments to all constants. See §Completed.*

### ~~2.13 CONTRIBUTING.md code layout incomplete~~ — Resolved

*Listed all docs files. See §Completed.*

### ~~2.14 EMBEDDINGS.md relative links broken from docs/~~ — Resolved

*Fixed relative links to use `../` prefix. See §Completed.*

### ~~2.15 LLM.md missing status banner~~ — Resolved

*Added prominent design-document status banner. See §Completed.*

### ~~2.16 Missing ozakeygen from README CLI section~~ — Resolved

*Added ozakeygen and ozamcp sections to README. See §Completed.*

### 2.17 Structured access logging

`ozaserve` has no access logging beyond Go's default logger. Structured JSON access
logs would be valuable for analytics, debugging, and cache tuning.

### 2.18 Markdown content rendering

`ozaserve` should render `text/markdown` entries through Goldmark before serving as
HTML. Currently only `text/html` entries render correctly.

### 2.19 Configurable browse exclusions

Per-archive `browse_exclude` metadata key (glob patterns) for filtering non-article
entries from browse views. `zim2oza` could auto-detect ZIM source type and suggest
defaults.

### 2.20 Entry enumeration by MIME type

API to enumerate entries by MIME type. Currently requires a full entry scan.

**Fix:** Build a MIME-to-entry-ID index at load time.

### 2.21 Content dedup detection at read time

SHA-256 content hashes are stored per entry but not leveraged at read time. Exposing
this enables duplicate detection, storage analysis, and cross-archive dedup.

### 2.22 No tests for ozawrite pipeline internals

`ozawrite/pipeline.go`, `assembly.go`, `dedup.go`, `minify.go`, `compress.go`,
`checksum.go`, `optimize_image.go` — no direct unit tests. Exercised indirectly through
integration tests but edge cases (dictionary training failure, JPEG metadata stripping,
minification fallback, chunk splitting) are not covered.

### 2.23 No writer-side index benchmarks

Missing `BenchmarkBuildIndex` covering V1 path/title index construction.

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

#### 3.5 Image format conversion (PNG → WebP)

Lossless PNG → WebP yields 25–35% savings. Blocked on CGo decision (`libwebp`). Could
be exposed as `--recompress-images` / `--lossy-images` flags on `zim2oza`.

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

Only ZIM → OZA exists. Future sources: EPUB → OZA, static site → OZA, PDF collection
→ OZA, Markdown corpus → OZA. A generic ingest pipeline with pluggable source readers
would reduce per-source effort.

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

#### 3.14 Cross-platform CI

No automated testing on Windows or macOS. The mmap/pread abstraction layer exists but
is untested in CI.

#### 3.15 No resume for interrupted conversions

A large ZIM → OZA conversion can take hours. If interrupted, must restart. No
checkpointing or resume capability.

**Design question:** Conversions run on build servers, not user laptops. A robust
retry (rerun the command) may be sufficient.

#### 3.16 Image optimization limited to JPEG

Only JPEG re-encoding is implemented. No PNG optimization, SVG minification, or WebP
conversion. See §3.5 for the CGo-blocked PNG → WebP path.

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

### 2.10 ozainfo uses raw os.Args instead of cobra ~~P2~~

Migrated `cmd/ozainfo/main.go` to cobra, consistent with all other CLI tools.

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

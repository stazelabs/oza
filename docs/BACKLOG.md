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

## 1. Security & Hardening

These are preconditions for exposing OZA to untrusted archives — the first thing an
attacker will try is a crafted file.

### 1.1 Decompression bomb (OOM) ✓ RESOLVED

`WithMaxDecompressedSize(n int64)` option added to `OpenWithOptions`. Default is
1 GiB. Any chunk or section whose decompressed output exceeds the limit is rejected
with `ErrDecompressedTooLarge`. Checked in both `readChunk` (`chunk.go`) and
`readSectionData` (`archive.go`).

### 1.2 Unbounded blob allocation ✓ RESOLVED

`WithMaxBlobSize(n int64)` (default 256 MiB) and `WithMaxMetadataValueSize(n int64)`
(default 16 MiB) options added. `readBlob` checks `blobSize` before allocation and
returns `ErrBlobTooLarge`. `parseMetadata` (archive-internal wrapper around
`ParseMetadata`) checks each value size and returns `ErrMetadataValueTooLarge`.

### 1.3 Blob extraction panic ✓ RESOLVED (pre-existing)

`readBlob` already returns `fmt.Errorf(...)` for out-of-bounds offsets — no panic
was ever present in the committed code. The backlog item was written against a
pre-commit draft.

### 1.4 Thread-safety contract ✓ RESOLVED

Documented in the `Archive` type godoc: safe for concurrent reads after `Open`
returns; `Close` must not be called concurrently with other methods. No mutex needed
— all state is write-once during `Open`. `TestConcurrentReads` (8 goroutines,
`-race`) enforces the invariant.

### 1.5 HTTP server hardening

`ozaserve` —
- No rate limiting or per-IP request throttling.
- No maximum request body size (relevant if POST support is added later).
- Content-Security-Policy is set on the index page but not on served HTML content.
  Served articles should get `Content-Security-Policy: sandbox` to prevent script
  execution in untrusted HTML.
- No read timeout on the HTTP server (uses `http.ListenAndServe` defaults).

**Fix:** Add `ReadTimeout`, `WriteTimeout`, `IdleTimeout` to the server. Add CSP
sandbox header to content responses. Consider optional rate limiting middleware.

---

## 2. Spec / Documentation Drift

The spec (FORMAT.md) and implementation must be byte-for-byte aligned. Any divergence
is a portability time bomb for third-party implementations.

### 2.1 Trigram index version ✓ RESOLVED

Canonicalized at v1. Writer emits `version=1`, reader accepts only `version=1`,
both godocs say "v1", FORMAT.md §4.3 updated to "Wire Format (v1)" with
`version (1)` in the header diagram.

### 2.2 Index format magic ✓ RESOLVED

Canonicalized to IDX1 (`0x49445831`). Renamed `IndexV3Magic` → `IndexV1Magic` in
`oza/constants.go`, updated godoc in `oza/index.go`, test helper comment in
`oza/index_test.go`, two writer call sites in `ozawrite/index.go`, and four test
function names in `ozawrite/index_test.go`. FORMAT.md §3.7 and §3.8 updated from
"IDX2" / `0x49445832` to "IDX1" / `0x49445831`.

### 2.3 Posting list encoding ✓ RESOLVED

FORMAT.md §4.3 now says "Serialized roaring bitmap (Roaring Bitmap portable format,
roaring.WriteTo)" — resolved alongside 2.1.

### 2.4 Reserved field validation ✓ RESOLVED

Added `Archive.warnings []string` (unexported) and `Warnings() []string` (public).
In `load()`, after parsing the header and section table, the raw byte buffers are
inspected for non-zero reserved bytes:
- Header bytes [60:64] (`Reserved uint32`)
- Section descriptor bytes [33:36] and [40:48] per entry

Non-zero values append a descriptive advisory message to `warnings`; the archive
opens successfully. `TestReservedFieldWarnings` covers the clean-archive (zero warnings)
and both modified-header and modified-section cases.

---

## 3. Correctness

### 3.1 Chunk table sort assumption ✓ RESOLVED

`loadContentSection` now validates that each descriptor's ID equals its position
(`desc.ID == uint32(i)`). Any gap, duplicate, or out-of-order ID returns
`ErrChunkTableUnsorted`, preventing silent index corruption.

### 3.2 MIME index bounds ✓ RESOLVED

`contentEntryRecord` validates `rec.MIMEIndex` against `len(a.mimeTypes)` after
parsing, returning `ErrInvalidEntry` for out-of-range values. The bounds check in
`MIMEType()` is now a documented safety net that is unreachable via normal Archive
methods.

### 3.3 ForEach error swallowing ✓ RESOLVED

Added `ForEachErr(fn func(uint32, string) error) error` to `Index`. Decode errors
and fn-returned errors are propagated immediately. `buildReverseMaps` now uses
`ForEachErr` and returns an error, which `load()` propagates — so index corruption
fails the `Open` call rather than silently producing a partial map. `ForEach` is
retained for callers that don't need error propagation, documented as silent-stop.

### 3.4 Metadata duplicate keys

`oza/metadata.go` `ParseMetadata()` — if a key appears twice, the second value
silently overwrites the first. This could mask corruption or writer bugs.

**Fix:** Return error on duplicate keys (strict mode) or at least log a warning.

### 3.5 Metadata format validation

Required keys (`date`, `language`, etc.) are checked for presence but not format.
`date` should be ISO 8601, `language` should be BCP-47. Invalid values pass silently.

**Fix:** Add optional strict validation (off by default for reading, on by default for
writing). The writer should enforce format; the reader should tolerate sloppiness but
expose a `ValidateMetadata()` method.

**Converter implications:** All five required metadata fields (`title`, `language`,
`creator`, `date`, `source`) are copied verbatim from the ZIM file's M/ namespace —
wild data from upstream authors. If OZA enforces format compliance on the writer,
`zim2oza` must act as a **sanitizing bridge**: normalize first, fall back to a sensible
default if normalization fails, and warn in both cases.

| Field | Normalize | Fallback if unparseable |
|-------|-----------|------------------------|
| `date` | Parse liberally (multiple date formats), emit ISO 8601 `YYYY-MM-DD` | `time.Now().Format("2006-01-02")` + warning |
| `language` | Map ISO 639-2/3 codes to BCP-47, strip whitespace, lowercase | `"und"` (BCP-47 "undetermined") + warning |

Note: the current language fallback is `"eng"` (ISO 639-3, not BCP-47). Should become
`"und"` to avoid a false claim about the content language.

---

## 4. API Design & Ergonomics

### 4.1 No streaming content API

`Entry.ReadContent()` returns `[]byte` — the entire decompressed blob in memory. For
a 50 MB video entry, this is wasteful. An `io.Reader` or `io.SectionReader` based API
would allow streaming to HTTP responses without full materialization.

**Considerations:** Chunks are decompressed whole (Zstd has no streaming sub-chunk
access), so the benefit is mainly on the caller side — avoid copying from cache to a
new allocation. A `WriteTo(w io.Writer)` method on `Entry` could write directly from
the cached chunk slice.

### 4.2 Iterator error propagation ✓ RESOLVED

Added `iter.Seq2[Entry, error]` variants for all five iterators: `EntriesErr()`,
`RedirectEntriesErr()`, `EntriesByPathErr()`, `EntriesByTitleErr()`,
`FrontArticlesErr()`. When a non-nil error is yielded, the Entry is zero-valued
and iteration stops after the error yield. The original `iter.Seq[Entry]` methods
are retained for callers that don't need error propagation, with godoc updated to
document the silent-stop behavior and point to the `*Err` variants.

### 4.3 Thread-safety documentation ✓ RESOLVED

Documented in the `Archive` type godoc (`oza/archive.go`): safe for concurrent use by
multiple goroutines after `Open` or `OpenWithOptions` returns; all internal state is
write-once during opening; the chunk cache is independently mutex-protected; `Close`
must not be called concurrently with any other method. See also §1.4.

### 4.4 FrontArticles scan cost

`FrontArticles()` iterates all entries checking the `is_front_article` flag — O(N).
For Wikipedia (6M+ entries), this is a full scan on every call.

**Fix:** Build a `[]uint32` of front-article IDs at load time (already done for
`ozaserve`'s random feature). Expose from the library. Cost: ~24 MB RAM for 6M
front articles.

### 4.5 Missing redirect count in header ✓ RESOLVED

Header expanded from 64 to 128 bytes. Added `redirect_count` (offset 60) and
`front_article_count` (offset 64). 60 bytes reserved for future use.

### 4.6 MIME index accessor ✓ RESOLVED

Added `Entry.MIMEIndex() uint` in `oza/entry.go`. Returns the raw MIME table index,
comparable against `oza.MIMEIndexHTML` (0), `oza.MIMEIndexCSS` (1), `oza.MIMEIndexJS`
(2) for allocation-free type checks. Returns 0 for redirect entries (meaningless;
callers should check `IsRedirect()` first).

Retrofitted `cmd/ozaserve/handlers.go` — the one OZA-entry comparison site — from
`entry.MIMEType() == "text/html"` to `entry.MIMEIndex() == oza.MIMEIndexHTML`. The
remaining `MIMEType()` calls in other commands are display/pass-through uses where the
string value is needed.

### 4.7 ozacat binary safety ✓ RESOLVED

Added `-o`/`--output <file>` flag: writes content to a file and prints the byte count
to stderr instead of emitting to stdout.

Added TTY detection via `os.Stdout.Stat()` + `ModeCharDevice` check. If stdout is a
terminal and the entry's MIME type is binary (not `text/*` and not a known text
`application/` type like `application/javascript`), `runCat` returns an error:

    refusing to write binary content (MIME: image/png) to terminal; use -o <file> to save

Content piped or redirected to a file/pipe bypasses the check, as there is no terminal
to corrupt.

---

## 5. MCP / AI Enhancements

Current tools: `list_archives`, `search_text`, `read_entry`.
Current resources: `oza://{slug}/metadata`, `oza://{slug}/entry/{id}`.

### 5.1 Near-term MCP tools (no new sections needed) ✓ RESOLVED

Added four tools to `internal/mcptools` (shared by both `ozamcp` and `ozaserve --mcp`):

- **`get_entry_info`** — Returns `entry_id`, `path`, `title`, `mime_type`, `size_bytes`,
  `is_redirect`, `is_front_article` (and `url` when served via ozaserve) without reading
  content. Cheaper than `read_entry` for triage.

- **`browse_titles`** — Returns up to 200 entries from the title index in alphabetical
  order with `offset`/`limit` pagination. Response includes `total_count` so callers
  know how many pages exist. Backed by `Archive.BrowseTitles(offset, limit)` (new
  method on `oza.Archive`, O(limit) per call using `Index.Record`).

- **`get_random`** — Picks a random front-article entry from `FrontArticleIDs` (new
  field on `mcptools.ArchiveInfo`, collected via `ForEachEntryRecord` at load time in
  both `ozamcp` and `ozaserve`). Accepts optional `archive` slug; omit to pick from any
  loaded archive.

- **`get_archive_stats`** — Returns `entry_count`, `redirect_count`, `chunk_count`,
  `has_search`, per-MIME-type entry counts and uncompressed byte totals (sorted by
  count), and a section inventory with compressed/uncompressed sizes and compression
  method.

Also added `Archive.TitleCount() int` to `oza/archive.go` (returns 0 if no title index)
and updated `docs/OZAMCP.md` with full parameter tables for all seven tools.

### 5.2 Existing tool enhancements ✓ RESOLVED

**`read_entry`:**
- ✓ Added `max_length` parameter — truncates output to N characters (rune-aware).
  When truncated, appends `\n\n[truncated]` marker. Applies after markdown conversion
  and header, so the header always appears.
- ✓ Added `section` parameter — extracts content under the first heading matching
  the given text (case-insensitive). Uses `internal/snippet.ExtractSection()` with
  `golang.org/x/net/html` tokenizer to find the heading and collect everything up to
  the next heading of equal or higher level. Returns an error if the section is not
  found. Works with `format=markdown` (section HTML is converted after extraction).

**`search_text`:**
- ✓ Added `mime_type` and `content_size` fields to every search result, enabling
  LLMs to estimate token cost before committing to `read_entry`.
- ✓ Added `snippets` and `snippet_length` parameters — see §7.2 "Search result
  snippets".

### 5.3 Future MCP tools (require new sections)

Mapped to the LLM.md roadmap:

| Tool | Depends On | Purpose |
|------|-----------|---------|
| `search_semantic` | VECTOR_EMBEDDINGS (0x0100) | Vector similarity search |
| `search_hybrid` | VECTOR_EMBEDDINGS + trigram | RRF fusion of keyword + semantic |
| `budget_context` | CONTEXT_HINTS (0x0103) | Greedy-pack entries into token budget |
| `read_passage` | PLAIN_TEXT (0x0101) | Read specific passage with heading context |
| `browse_category` | KNOWLEDGE_GRAPH (0x0102) | Category-based navigation |
| `find_entity` | KNOWLEDGE_GRAPH (0x0102) | Entity lookup → entry list |
| `get_related` | KNOWLEDGE_GRAPH (0x0102) | Follow graph edges |

### 5.4 MCP code quality

- Both `cmd/ozamcp/tools.go` and `cmd/ozaserve/mcp.go` ignore `json.MarshalIndent`
  errors. Should handle or use a helper that panics (initialization-time only).
- Significant code duplication between the two MCP implementations. The tool logic
  (list, search, read) is copy-pasted with minor differences (URL inclusion).
  **Fix:** Extract shared tool registration into a package (e.g., `ozamcp/tools`)
  parameterized by an optional URL builder.
- `read_entry` reads all HTML content into memory for markdown conversion. No size
  limit, no truncation. A 50 MB article will be fully converted.
- `ozamcp` SSE transport is documented in help text but not implemented.

### 5.5 AI section implementation priority

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

---

## 6. Performance

### 6.1 Chunk cache: FIFO → LRU ✓ RESOLVED

Replaced the `order []uint32` ring-slice with a `container/list` doubly-linked list
(`lru *list.List`) and changed the map from `map[uint32]*decompressedChunk` to
`map[uint32]*list.Element`. Each element value is a `cacheEntry{id, chunk}` so eviction
can `delete(m, back.Value.id)` in O(1). `get` calls `MoveToFront` under the lock for
O(1) LRU promotion. `put` evicts `lru.Back()` when at capacity.

### 6.2 Streaming file-level verify ✓ RESOLVED

Replaced the full-body allocation (`make([]byte, ChecksumOff)`) in both `Verify()`
and the file-tier block of `VerifyAll()` with streaming via `io.NewSectionReader` +
`io.Copy` into `sha256.New()`. Allocation is now a fixed 32 KB copy buffer regardless
of archive size.

### 6.3 mmap: MAP_SHARED → MAP_PRIVATE ✓ RESOLVED

Changed `syscall.MAP_SHARED` to `syscall.MAP_PRIVATE` in `oza/io_mmap_unix.go`.
No behavioral difference for read-only mappings; eliminates unnecessary dirty-page
tracking overhead in the kernel.

### 6.4 Browse page scan

`ozaserve` letter counts are precomputed at load time (good), but individual letter
pages (`/browse?letter=A`) do an O(N) scan of the title index per request.

**Fix:** Precompute letter-to-offset mapping at load time. Binary search to the start
of each letter in the sorted title index and store the offset. Then each browse request
is O(page_size), not O(N).

### 6.5 HTML injection per-request cost

`ozaserve/handlers.go` — injects a navigation bar and footer by searching for
`<body>` / `</body>` tags with `bytes.Index()` on every HTML response.

Low priority — byte search on a few hundred KB is fast. But for high-throughput
serving, caching the injection offsets per entry would eliminate redundant work.

---

## 7. Features

### 7.1 Existing backlog (preserved)

#### ozaclean

A CLI tool to "clean" / repack an OZA file. Opportunities: re-optimize compression,
strip sections, upgrade to a new format version, add/remove signatures.

#### Chrome section (FORMAT.md §7.2)

Implement the optional CHROME section:
- `ozawrite/chrome.go` — `AddChromeAsset(role, name, data)` on Writer
- `oza/chrome.go` — parse chrome section, enumerate assets by role
- `cmd/zim2oza/chrome.go` — extract `C/_mw_/` CSS/JS from ZIM into chrome section

Currently `categoryChrome` exists in the converter but entries are skipped.

#### Image format conversion (PNG → WebP)

Lossless PNG → WebP yields 25-35% savings. Lossy JPEG → WebP at quality 80 saves
another 25-35%. This is the single highest-impact size optimization. Blocked on CGo
decision (`libwebp`). Could be exposed as `--recompress-images` / `--lossy-images`
flags on `zim2oza`.

#### Incremental / append mode

See `docs/INCREMENTAL.md`. Optimized rebuild with chunk-level copy. Key methods:
`CopyChunk`, `AddFromArchive`. Estimated 6x speedup for 95%-unchanged Wikipedia.

#### Split archives

Evaluate multi-part archive support along the lines of ZIM's `.zimaa/.zimab` splits.
A section-type-based approach could be added without changing the core format.

### 7.2 New items

#### Markdown content rendering

`ozaserve` should render `text/markdown` entries through Goldmark before serving as
HTML. Currently only `text/html` entries get served correctly. Markdown is the
preferred format for extracted text (see ZIM_OBSERVATIONS.md) and will be the format
for the PLAIN_TEXT section.

#### Search result snippets ✓ RESOLVED

Added `internal/snippet` package with `StripHTML()` (using `golang.org/x/net/html`
tokenizer), `Extract()` (rune-aware windowed excerpt with word-boundary snapping and
ellipsis), and `ForEntry()` (convenience wrapper for OZA entries).

MCP `search_text` tool gains `snippets` (bool, default false) and `snippet_length`
(int, default 200, max 500) parameters. When enabled, each `searchHit` includes a
`snippet` field with a plain-text excerpt centered on the query match.

HTTP search endpoints (`/_search`, `/{archive}/_search`, `/{archive}/-/search`) gain
`?snippets=true&snippet_length=N` query parameters. The HTML search page renders
snippets below each result link. Snippets are only generated for `text/html` entries;
non-HTML entries return an empty snippet. Opt-in design avoids decompression cost by
default.

#### Structured access logging

`ozaserve` has no access logging beyond Go's default logger. For analytics, debugging,
and monitoring, structured JSON access logs would be valuable — especially for
understanding which entries are hot (informs cache tuning) and how search is used.

#### Configurable browse exclusions

Per-archive `browse_exclude` metadata key (glob patterns) for filtering non-article
entries from browse views. See `docs/ZIM_OBSERVATIONS.md` for source-specific patterns
(Gutenberg `_cover` entries, StackExchange tag pages, etc.).

The `zim2oza` converter could auto-detect ZIM source type from metadata and suggest
appropriate defaults.

#### Entry enumeration by MIME type

API to enumerate entries by MIME type — useful for "show all images", per-type
statistics, and MIME-aware batch operations. Currently requires a full entry scan with
per-entry MIME check.

**Fix:** Build a MIME-to-entry-ID index at load time (similar to the front-article
index).

#### Content dedup detection at read time

SHA-256 content hashes are stored per entry but not leveraged at read time. Two entries
with identical hashes are guaranteed to have identical content. Exposing this enables:
- Duplicate detection tools
- Storage optimization analysis
- Cross-archive dedup discovery

#### oza2jsonl export tool

Export structured training data from an OZA archive for LLM fine-tuning:
- Instruction tuning pairs (heading → passage text)
- Entity-fact pairs (from KNOWLEDGE_GRAPH)
- Domain-filtered text (by category or MIME type)

Depends on PLAIN_TEXT and optionally KNOWLEDGE_GRAPH sections.

---

## 8. Compression

### 8.1 Dictionary training panic (existing)

`zstd.BuildDict()` can panic on certain inputs ("can only encode up to 64K
sequences"). Caught by `defer/recover` in `trainDictionary()`, but upstream fixes
would be better. Track klauspost/compress issues.

### 8.2 Minification semantics (existing)

HTML minification (tdewolff) removes whitespace that may be significant in `<pre>`
blocks or inline formatting. CSS/JS minification is generally safe but may break code
that relies on `toString()` or source-level introspection. There is no per-entry
opt-out — only global `--no-minify-html` etc.

### 8.3 PNG re-encode fidelity (existing)

The Go `image/png` decoder + re-encoder round-trip is not guaranteed to be bit-exact
for all valid PNGs (16-bit color depth, unusual chunk ordering). Optimization is
skipped if re-encoded file is larger, but there's no pixel-level verification.

### 8.4 Per-entry compression control

No way to mark an individual entry as "store uncompressed." Currently handled by
MIME-based chunk grouping (images go in uncompressed chunks). But an explicit
per-entry flag would be cleaner for edge cases (pre-compressed data in an unexpected
MIME type).

**Design question:** Is this worth the complexity? MIME-based grouping handles 99% of
cases. A per-entry flag adds a bit to the entry record and complicates the writer's
chunk grouping logic.

---

## 9. Testing

### 9.1 HTTP handler tests

`ozaserve` has zero test coverage. All HTTP handlers (content serving, search, browse,
info, error pages) are untested. This is the most user-facing surface area.

**Fix:** Add `httptest`-based tests using a small in-memory archive. Test:
- Content serving (HTML, CSS, images, redirects)
- Search (valid query, empty query, no-index archive)
- Browse (pagination, letter filtering)
- Error pages (404, invalid paths)
- Security headers (CSP, X-Frame-Options, etc.)

### 9.2 MCP tool tests

Neither `ozamcp` nor `ozaserve` MCP tool handlers have tests. Test each tool with
valid/invalid inputs, missing archives, boundary conditions.

### 9.3 Concurrent access tests

No tests exercise concurrent `EntryByPath()` / `ReadContent()` / search from multiple
goroutines. Relevant for validating the thread-safety contract (§4.3).

**Fix:** Add `TestConcurrentReads` with `testing.T.Parallel()` and `-race` flag.

### 9.4 Adversarial archive corpus

No malformed/corrupted test archives exist. The fuzzing targets cover binary parsing
primitives but don't test end-to-end behavior with a corrupted-but-valid-looking
archive.

**Fix:** Create a corpus of crafted archives: truncated sections, out-of-bounds
offsets, decompression bombs, circular redirects, duplicate metadata keys, unsorted
chunk tables. Use these in integration tests.

### 9.5 Large archive integration test (existing)

The test suite uses small synthetic archives. No automated test converts a real ZIM
file and verifies with `ozaverify --all`. The Makefile has `bench-convert-large` for
manual runs.

### 9.6 Cross-platform CI

No automated testing on Windows or macOS. The mmap/pread abstraction layer exists but
is untested in CI.

### 9.7 Coverage reporting ✓ RESOLVED

Added `cover` and `cover-html` targets to the Makefile:

- `make cover` — runs `go test -coverprofile=coverage.out -covermode=atomic ./...`
  then prints a per-function summary via `go tool cover -func`.
- `make cover-html` — runs `cover`, then opens the annotated HTML report in the
  browser via `go tool cover -html`.

`coverage.out` is gitignored. A coverage badge requires CI (§10.1).

---

## 10. Infrastructure / CI/CD

### 10.1 GitHub Actions ✓ RESOLVED

Added `.github/workflows/ci.yml` with five parallel jobs triggered on push to `main`
and all pull requests:

- **test** — matrix across `ubuntu-latest`, `macos-latest`, `windows-latest`;
  runs `go test -race ./... -count=1`. Windows uses the fileReader fallback (no mmap).
- **lint** — `golangci-lint-action@v6` on Linux using `.golangci.yml`.
  `setup-go` cache disabled to let the action manage its own lint cache.
- **build** — `make build` on Linux; confirms all 8 binaries compile cleanly.
- **coverage** — `go test -coverprofile=coverage.out -covermode=atomic ./...` then
  `codecov/codecov-action@v4` (tokenless mode for public repos; upload failures are
  non-fatal so they don't block CI).
- **fuzz** — `make fuzz` on Linux; runs all 8 fuzz targets for 30s each (~4 min).

### 10.2 Linting ✓ RESOLVED

Added `.golangci.yml` enabling: `errcheck`, `gosimple`, `govet`, `ineffassign`,
`staticcheck`, `unused`, `gofmt`, `goimports`, `misspell`, `unconvert`, `unparam`,
`gosec`. `gosec` G304/G306 excluded (intentional in CLI tools); `gosec` and `unparam`
suppressed in test files and `cmd/` respectively.

Added `make lint` (`golangci-lint run ./...`) and `make lint-fix` (with `--fix`) to
the Makefile. Fixed all pre-existing `gofmt` violations across `oza/`, `ozawrite/`,
`internal/`, and `cmd/`.

### 10.3 Release automation ✓ RESOLVED

Added GoReleaser-based release pipeline:

- **`.goreleaser.yaml`** — builds all 8 binaries (`ozacat`, `ozainfo`, `ozasearch`,
  `ozaserve`, `ozaverify`, `ozamcp`, `ozakeygen`, `zim2oza`) for Linux, macOS, and
  Windows (amd64 + arm64) with `CGO_ENABLED=0`. Packages as `.tar.gz` (Linux/macOS)
  and `.zip` (Windows). Generates `checksums.txt`. Changelog grouped by feat/fix.
  Prereleases auto-detected from tags matching `v*-*`.

- **`.github/workflows/release.yml`** — triggers on `v*` tag push. Runs
  `go test -race ./...`, then calls `goreleaser release --clean` with
  `GITHUB_TOKEN` for GitHub Release publishing.

- **`make snapshot`** — runs `goreleaser release --snapshot --clean` for local
  testing without a tag or GitHub push. Output in `dist/`.

- **`CONTRIBUTING.md`** — documents the release process (push tag → CI builds →
  GitHub Release) and local snapshot workflow.

### 10.4 Pre-commit hooks ✓ RESOLVED

Added `.pre-commit-config.yaml` with three `local` hooks that run on every commit:

- **gofmt** — formats changed Go files in-place (`-w`)
- **go vet** — runs `go vet ./...` across the whole module
- **golangci-lint** — runs `golangci-lint run --fix ./...` (uses `.golangci.yml`)

All three use `language: system` so no additional Python environments are needed
beyond `pre-commit` itself (`brew install pre-commit` or `pip install pre-commit`).
Activate with `pre-commit install` in the repo root.

### 10.5 CONTRIBUTING.md ✓ RESOLVED

Added `CONTRIBUTING.md` covering: development setup (Go + golangci-lint + pre-commit),
Makefile command reference, code layout map, PR checklist (test-race + lint + spec
sync + godoc), spec-change process, and issue/security reporting.

### 10.6 Benchmark regression tracking

Benchmarks exist but results aren't tracked across commits. Performance regressions
are invisible.

**Fix:** Consider `benchstat` in CI, or a lightweight benchmark tracking solution.

---

## 11. Writer / Converter

### 11.1 No resume for interrupted conversions

A large ZIM → OZA conversion can take hours. If interrupted, it must restart from
scratch. No checkpointing or resume capability.

**Design question:** Is this worth the complexity? Conversions run on build servers,
not user laptops. A robust retry (rerun the command) may be sufficient.

### 11.2 Image optimization limited to JPEG

Only JPEG re-encoding is implemented. No PNG optimization (beyond re-encode which is
lossy for some PNGs), no SVG minification, no WebP conversion.

See §7.1 (image format conversion) for the CGo-blocked PNG → WebP path.

### 11.3 Single converter source

Only ZIM → OZA exists. Future sources to consider:
- **EPUB → OZA**: Book collections (Project Gutenberg native format)
- **Static site → OZA**: HTML directory tree → archive
- **PDF collection → OZA**: See ZIM_OBSERVATIONS.md for text extraction strategy
- **Markdown corpus → OZA**: Documentation sites, wiki exports

A generic ingest pipeline with pluggable source readers would reduce per-source
effort. The `zim2oza` converter's two-phase design (scan → write) is a reasonable
template.

---

## 12. Design Observations

Notes on architectural decisions worth revisiting as the project matures.

### 12.1 Writer file size ✓ RESOLVED

Decomposed `ozawrite/writer.go` (1,117 lines) into three focused files:

- **`writer.go`** (~720 lines) — public types, `NewWriter`, `AddEntry`, `AddRedirect`,
  `transformContent`, and `Close` (orchestration only).
- **`pipeline.go`** (~220 lines) — streaming chunk pipeline: `bufferForTraining`,
  `haveSufficientSamples`, `trainAndFlushPending`, `addToChunk`, `startPipeline`,
  `flushChunk`, `flushChunkSync`.
- **`assembly.go`** (~170 lines) — section serialization: `buildMIMETable`,
  `buildEntryTable`, `buildRedirectSection`, `buildIndexSections`, `buildDictSections`,
  `writeContentSection`, `contentSectionSize`, `cleanupTemp`, `newRawSection`,
  `compressRawSection`.

### 12.2 ozaserve inline HTML/JS ✓ RESOLVED

Refactored all inline HTML/CSS/JS from `fmt.Fprintf` calls into Go `html/template`
files under `cmd/ozaserve/templates/` with `embed.FS`. Added `templates.go` with
`initTemplates()`, `renderTemplate()`, and a FuncMap (`commaInt`, `formatBytes`,
`formatBytesShort`, `safeHTML`). Templates: `_head.html`, `_footer.html`,
`error.html`, `docs.html`, `search.html`, `browse.html`, `index.html`, `info.html`,
`info_global.html`. Handler files (`errorpage.go`, `docs.go`, `index.go`, `info.go`,
`handlers.go`) now build typed data structs and call `renderTemplate`.

### 12.3 Dual MCP implementations ✓ RESOLVED

Created `internal/mcptools` package with `ArchiveInfo` struct and:
- `RegisterTools(server, archives, archiveURL, entryURL)` — list_archives, search_text, read_entry
- `RegisterResources(server, archives, archiveURL, entryURL)` — metadata resources + entry template
- `ParseEntryURI(uri)` — shared URI parser (replaces both `parseEntryURI` and `parseMCPEntryURI`)

`archiveURL func(slug string) string` and `entryURL func(slug, path string) string` are the two
optional URL-builder parameters; passing nil omits URL fields from all results.

`cmd/ozamcp`: `tools.go` and `resources.go` reduced to `package main` stubs; `main.go` uses
`mcptools.ArchiveInfo` directly and calls shared functions with `nil, nil`.

`cmd/ozaserve/mcp.go`: replaced ~300 lines of duplicated logic with three thin functions
(`registerMCPTools`, `registerMCPResources`, `libToMCPArchives`) totalling ~35 lines.

### 12.4 Entry ID space ✓ RESOLVED

Added `MaxContentEntries` and `MaxRedirectEntries` constants (both `2,147,483,647`) to
`oza/constants.go` with a godoc block that names the tagged-ID scheme a permanent v1
format invariant. `AddEntry` and `AddRedirect` in `ozawrite/writer.go` now check these
limits and return an error rather than silently wrapping. FORMAT.md §3.11 was expanded
with an explicit capacity-constraint table and the requirement that readers reject
out-of-range counts and writers error rather than wrap.

### 12.5 Search ranking

Trigram search has no ranking beyond title-match > body-match > entry-ID order. For
many queries, alphabetical-by-ID is effectively random. Better ranking signals
(document length normalization, query-term density, link-based importance) could
improve results significantly without requiring new sections.

**Near-term:** BM25-lite scoring using content size (already in entry records) and
trigram hit count. No new data needed.

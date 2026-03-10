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

### 1.1 HTTP server hardening

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

## 2. Correctness

### 2.1 Metadata duplicate keys

`oza/metadata.go` `ParseMetadata()` — if a key appears twice, the second value
silently overwrites the first. This could mask corruption or writer bugs.

**Fix:** Return error on duplicate keys (strict mode) or at least log a warning.

### 2.2 Metadata format validation

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

## 3. API Design & Ergonomics

### 3.1 No streaming content API

`Entry.ReadContent()` returns `[]byte` — the entire decompressed blob in memory. For
a 50 MB video entry, this is wasteful. An `io.Reader` or `io.SectionReader` based API
would allow streaming to HTTP responses without full materialization.

**Considerations:** Chunks are decompressed whole (Zstd has no streaming sub-chunk
access), so the benefit is mainly on the caller side — avoid copying from cache to a
new allocation. A `WriteTo(w io.Writer)` method on `Entry` could write directly from
the cached chunk slice.

### 3.2 FrontArticles scan cost

`FrontArticles()` iterates all entries checking the `is_front_article` flag — O(N).
For Wikipedia (6M+ entries), this is a full scan on every call.

**Fix:** Build a `[]uint32` of front-article IDs at load time (already done for
`ozaserve`'s random feature). Expose from the library. Cost: ~24 MB RAM for 6M
front articles.

### 3.3 Search ranking

Trigram search has no ranking beyond title-match > body-match > entry-ID order. For
many queries, alphabetical-by-ID is effectively random. Better ranking signals
(document length normalization, query-term density, link-based importance) could
improve results significantly without requiring new sections.

**Near-term:** BM25-lite scoring using content size (already in entry records) and
trigram hit count. No new data needed.

---

## 4. MCP / AI Enhancements

Current tools: `list_archives`, `search_text`, `read_entry`, `get_entry_info`,
`browse_titles`, `get_random`, `get_archive_stats`.
Current resources: `oza://{slug}/metadata`, `oza://{slug}/entry/{id}`.

### 4.1 Future MCP tools (require new sections)

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

### 4.2 AI section implementation priority

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

## 5. Performance

### 5.1 Browse page scan

`ozaserve` letter counts are precomputed at load time (good), but individual letter
pages (`/browse?letter=A`) do an O(N) scan of the title index per request.

**Fix:** Precompute letter-to-offset mapping at load time. Binary search to the start
of each letter in the sorted title index and store the offset. Then each browse request
is O(page_size), not O(N).

### 5.2 HTML injection per-request cost

`ozaserve/handlers.go` — injects a navigation bar and footer by searching for
`<body>` / `</body>` tags with `bytes.Index()` on every HTML response.

Low priority — byte search on a few hundred KB is fast. But for high-throughput
serving, caching the injection offsets per entry would eliminate redundant work.

### 5.3 Benchmark regression tracking

Benchmarks exist but results aren't tracked across commits. Performance regressions
are invisible.

**Fix:** Consider `benchstat` in CI, or a lightweight benchmark tracking solution.

---

## 6. Features

### 6.1 ozaclean

A CLI tool to "clean" / repack an OZA file. Opportunities: re-optimize compression,
strip sections, upgrade to a new format version, add/remove signatures.

### 6.2 Chrome section (FORMAT.md §7.2)

Implement the optional CHROME section:
- `ozawrite/chrome.go` — `AddChromeAsset(role, name, data)` on Writer
- `oza/chrome.go` — parse chrome section, enumerate assets by role
- `cmd/zim2oza/chrome.go` — extract `C/_mw_/` CSS/JS from ZIM into chrome section

Currently `categoryChrome` exists in the converter but entries are skipped.

### 6.3 Image format conversion (PNG → WebP)

Lossless PNG → WebP yields 25-35% savings. Lossy JPEG → WebP at quality 80 saves
another 25-35%. This is the single highest-impact size optimization. Blocked on CGo
decision (`libwebp`). Could be exposed as `--recompress-images` / `--lossy-images`
flags on `zim2oza`.

### 6.4 Incremental / append mode

See `docs/INCREMENTAL.md`. Optimized rebuild with chunk-level copy. Key methods:
`CopyChunk`, `AddFromArchive`. Estimated 6x speedup for 95%-unchanged Wikipedia.

### 6.5 Split archives

Evaluate multi-part archive support along the lines of ZIM's `.zimaa/.zimab` splits.
A section-type-based approach could be added without changing the core format.

### 6.6 Markdown content rendering

`ozaserve` should render `text/markdown` entries through Goldmark before serving as
HTML. Currently only `text/html` entries get served correctly. Markdown is the
preferred format for extracted text (see ZIM_OBSERVATIONS.md) and will be the format
for the PLAIN_TEXT section.

### 6.7 Structured access logging

`ozaserve` has no access logging beyond Go's default logger. For analytics, debugging,
and monitoring, structured JSON access logs would be valuable — especially for
understanding which entries are hot (informs cache tuning) and how search is used.

### 6.8 Configurable browse exclusions

Per-archive `browse_exclude` metadata key (glob patterns) for filtering non-article
entries from browse views. See `docs/ZIM_OBSERVATIONS.md` for source-specific patterns
(Gutenberg `_cover` entries, StackExchange tag pages, etc.).

The `zim2oza` converter could auto-detect ZIM source type from metadata and suggest
appropriate defaults.

### 6.9 Entry enumeration by MIME type

API to enumerate entries by MIME type — useful for "show all images", per-type
statistics, and MIME-aware batch operations. Currently requires a full entry scan with
per-entry MIME check.

**Fix:** Build a MIME-to-entry-ID index at load time (similar to the front-article
index).

### 6.10 Content dedup detection at read time

SHA-256 content hashes are stored per entry but not leveraged at read time. Two entries
with identical hashes are guaranteed to have identical content. Exposing this enables:
- Duplicate detection tools
- Storage optimization analysis
- Cross-archive dedup discovery

### 6.11 oza2jsonl export tool

Export structured training data from an OZA archive for LLM fine-tuning:
- Instruction tuning pairs (heading → passage text)
- Entity-fact pairs (from KNOWLEDGE_GRAPH)
- Domain-filtered text (by category or MIME type)

Depends on PLAIN_TEXT and optionally KNOWLEDGE_GRAPH sections.

---

## 7. Compression

### 7.1 Dictionary training panic

`zstd.BuildDict()` can panic on certain inputs ("can only encode up to 64K
sequences"). Caught by `defer/recover` in `trainDictionary()`, but upstream fixes
would be better. Track klauspost/compress issues.

### 7.2 Minification semantics

HTML minification (tdewolff) removes whitespace that may be significant in `<pre>`
blocks or inline formatting. CSS/JS minification is generally safe but may break code
that relies on `toString()` or source-level introspection. There is no per-entry
opt-out — only global `--no-minify-html` etc.

### 7.3 PNG re-encode fidelity

The Go `image/png` decoder + re-encoder round-trip is not guaranteed to be bit-exact
for all valid PNGs (16-bit color depth, unusual chunk ordering). Optimization is
skipped if re-encoded file is larger, but there's no pixel-level verification.

### 7.4 Per-entry compression control

No way to mark an individual entry as "store uncompressed." Currently handled by
MIME-based chunk grouping (images go in uncompressed chunks). But an explicit
per-entry flag would be cleaner for edge cases (pre-compressed data in an unexpected
MIME type).

**Design question:** Is this worth the complexity? MIME-based grouping handles 99% of
cases. A per-entry flag adds a bit to the entry record and complicates the writer's
chunk grouping logic.

---

## 8. Testing

### 8.1 HTTP handler tests

`ozaserve` has zero test coverage. All HTTP handlers (content serving, search, browse,
info, error pages) are untested. This is the most user-facing surface area.

**Fix:** Add `httptest`-based tests using a small in-memory archive. Test:
- Content serving (HTML, CSS, images, redirects)
- Search (valid query, empty query, no-index archive)
- Browse (pagination, letter filtering)
- Error pages (404, invalid paths)
- Security headers (CSP, X-Frame-Options, etc.)

### 8.2 MCP tool tests

Neither `ozamcp` nor `ozaserve` MCP tool handlers have tests. Test each tool with
valid/invalid inputs, missing archives, boundary conditions.

### 8.3 Concurrent access tests

No tests exercise concurrent `EntryByPath()` / `ReadContent()` / search from multiple
goroutines. Relevant for validating the thread-safety contract.

**Fix:** Add `TestConcurrentReads` with `testing.T.Parallel()` and `-race` flag.

### 8.4 Large archive integration test

The test suite uses small synthetic archives. No automated test converts a real ZIM
file and verifies with `ozaverify --all`. The Makefile has `bench-convert-large` for
manual runs.

### 8.5 Cross-platform CI

No automated testing on Windows or macOS. The mmap/pread abstraction layer exists but
is untested in CI.

---

## 9. Writer / Converter

### 9.1 No resume for interrupted conversions

A large ZIM → OZA conversion can take hours. If interrupted, it must restart from
scratch. No checkpointing or resume capability.

**Design question:** Is this worth the complexity? Conversions run on build servers,
not user laptops. A robust retry (rerun the command) may be sufficient.

### 9.2 Image optimization limited to JPEG

Only JPEG re-encoding is implemented. No PNG optimization (beyond re-encode which is
lossy for some PNGs), no SVG minification, no WebP conversion.

See §6.3 (image format conversion) for the CGo-blocked PNG → WebP path.

### 9.3 Single converter source

Only ZIM → OZA exists. Future sources to consider:
- **EPUB → OZA**: Book collections (Project Gutenberg native format)
- **Static site → OZA**: HTML directory tree → archive
- **PDF collection → OZA**: See ZIM_OBSERVATIONS.md for text extraction strategy
- **Markdown corpus → OZA**: Documentation sites, wiki exports

A generic ingest pipeline with pluggable source readers would reduce per-source
effort. The `zim2oza` converter's two-phase design (scan → write) is a reasonable
template.

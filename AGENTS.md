# AGENTS.md -- 王座 OZA

## Project Overview

`oza` is a Go library and CLI toolset for reading and writing OZA (Open Zipped Archive) files -- a modern replacement for the ZIM file format. Pure Go, no CGo dependencies.

- **Module:** `github.com/stazelabs/oza` (library), `github.com/stazelabs/oza/cmd` (CLI binaries)
- **Go version:** 1.24+ (library), 1.25+ (cmd module)
- **Format spec:** `docs/FORMAT.md`
- **Reference project:** `github.com/stazelabs/gozim` (ZIM reader library; used only by `zim2oza`/`ozacmp`)
- **Branding:** 王座 (Japanese: oza, "throne") -- OZA takes the throne as ZIM's successor
- **Status:** v0.1.0 shipped; pre-v1, breaking changes are free

## Repository Structure

Multi-module repo. The root `go.mod` is the reader+writer library; `cmd/go.mod` is the CLI binaries, with a `replace github.com/stazelabs/oza => ../` directive so CLI tools always build against the in-tree library.

```
oza/
├── oza/                          # Reader library (no CLI deps)
│   ├── archive.go                # Archive type: Open, Close, lookup, options, chunk cache
│   ├── archive_test.go
│   ├── badoza_test.go            # Adversarial / corrupted-input tests
│   ├── bench_test.go             # Reader benchmarks
│   ├── checksum.go               # File/section/chunk SHA-256 verification
│   ├── chunk.go                  # Chunk reading, decompression, blob extraction
│   ├── compress.go               # Zstd decode with optional dictionary
│   ├── constants.go              # Magic, sizes, redirect-ID bit packing, compression IDs
│   ├── constants_test.go
│   ├── doc.go                    # Package godoc
│   ├── entry.go                  # Variable-length entry records (uvarint) + 5-byte redirect records
│   ├── entry_test.go
│   ├── errors.go                 # Sentinel errors
│   ├── example_test.go           # Runnable examples for godoc
│   ├── header.go                 # 128-byte header parse
│   ├── header_test.go
│   ├── index.go                  # Path/title index binary search
│   ├── index_test.go
│   ├── intern.go                 # MIME-type string interning
│   ├── io.go                     # reader interface (mmap + pread)
│   ├── io_mmap_unix.go           # mmap implementation (Unix)
│   ├── io_mmap_windows.go        # mmap implementation (Windows)
│   ├── io_test.go
│   ├── iter.go                   # iter.Seq + iter.Seq2 iterators
│   ├── metadata.go               # Length-prefixed key/value pairs
│   ├── mime.go                   # MIME table
│   ├── search.go                 # Trigram + body search reader
│   ├── search_cjk_test.go
│   ├── search_test.go
│   ├── section.go                # 80-byte section descriptors, SectionType enum
│   ├── section_test.go
│   └── signature.go              # Ed25519 signature verification (trailer past checksum)
├── ozawrite/                     # Writer library (imports oza/)
│   ├── assembly.go               # Final archive assembly from temp chunks + indexes
│   ├── bench_test.go
│   ├── checksum.go
│   ├── chunk.go                  # Chunk grouping, encoding
│   ├── compress.go               # Zstd encode + dictionary training
│   ├── dedup.go                  # xxhash content-addressed deduplication
│   ├── doc.go
│   ├── example_test.go
│   ├── fuzz_test.go              # Fuzz harnesses
│   ├── htmltext.go / *_test.go   # HTML → text extraction for body search
│   ├── index.go / *_test.go      # Path/title index builders
│   ├── minify.go                 # HTML/CSS/JS/SVG minification
│   ├── optimize_image.go         # Lossless image optimization
│   ├── pipeline.go / *_test.go   # Parallel compression pipeline
│   ├── search.go / *_test.go     # Trigram index builder (in-memory + disk-spilling)
│   ├── signature.go / *_test.go  # Ed25519 signing trailer
│   ├── strtable.go / *_test.go   # String-table builder
│   ├── transcode.go              # Image transcoding (JPEG → WebP/AVIF)
│   └── writer.go / *_test.go     # Public API: NewWriter, AddEntry, AddRedirect, Close
├── cmd/                          # CLI tools (separate module)
│   ├── go.mod / go.sum
│   ├── ozainfo/                  # Metadata + section table dump, --classify
│   ├── ozacat/                   # Extract content / list entries / metadata
│   ├── ozasearch/                # Trigram search CLI
│   ├── ozaverify/                # Three-tier verify + Ed25519 signature check
│   ├── ozaserve/                 # HTTP server with browse/search UI + optional MCP
│   │   ├── docs/ozaserve.md      # Full HTTP API and route reference
│   │   └── templates/            # HTML templates
│   ├── ozamcp/                   # Standalone MCP server (stdio)
│   ├── ozakeygen/                # Ed25519 keypair generator
│   ├── ozacmp/                   # Side-by-side ZIM vs. OZA comparison
│   ├── zim2oza/                  # ZIM → OZA converter (with --auto classifier)
│   ├── epub2oza/                 # EPUB → OZA converter, single-book or collection
│   ├── site2oza/                 # Filesystem directory → OZA converter
│   └── internal/                 # Shared internal packages (cmd-only)
│       ├── classify/             # Content-profile classifier (8 profiles)
│       ├── epubread/             # EPUB parser
│       ├── loadutil/             # Multi-archive loader (used by ozaserve, ozamcp)
│       ├── mcptools/             # Shared MCP tool definitions
│       ├── snippet/              # Search result snippet rendering
│       ├── stats/                # Archive statistics collection (used by ozainfo, classify)
│       └── testutil/             # Test helpers
├── docs/
│   ├── FORMAT.md                 # OZA binary format specification (normative)
│   ├── BRANDING.md               # 王座 branding guide
│   ├── BACKLOG.md                # Roadmap / pending work
│   ├── CLASSIFIER.md             # Content classifier design (drives --auto/--classify)
│   ├── OZAMCP.md                 # MCP server protocol surface
│   ├── OZAWRITE.md               # Writer pipeline architecture
│   ├── TESTING_PLAN.md           # Test strategy
│   ├── INDICES.md                # Index design notes
│   ├── PLAIN_TEXT.md             # Body-text extraction notes
│   ├── INCREMENTAL.md            # Incremental update plans
│   ├── EMBEDDINGS.md             # Embedding/index extensions
│   ├── HOMEBREW.md               # Homebrew packaging notes
│   ├── LANDSCAPE.md              # Competitive landscape
│   ├── LLM.md                    # LLM-facing notes
│   ├── ZIM_OBSERVATIONS.md       # ZIM format reverse-engineering notes
│   ├── ADVERSARIAL.md            # Adversarial-input handling notes
│   └── bench-report.md           # Latest benchmark report
├── scripts/                      # Helper scripts (testdata downloads, benchmark runners)
├── testdata/                     # Test fixtures (gitignored; populated via scripts)
├── CLAUDE.md                     # Claude-specific repo notes
├── README.md                     # User-facing readme
└── Makefile                      # Targets in both modules (test, lint, build, bench, fuzz, cover)
```

## Architecture & Key Decisions

### Two Modules: `oza` (Reader+Writer Library) + `cmd` (CLI)

The library is one Go module (`oza/` and `ozawrite/` packages). The CLI tools are a second module under `cmd/` with a `replace` directive pointing at the library. This keeps library consumers from picking up the CLI's heavier transitive deps (Cobra, MCP SDK, goldmark, html-to-markdown, gozim).

Within the library, `ozawrite` imports `oza` for shared type definitions — unidirectional dependency, no cycles. The writer ships heavier deps (Zstd encoding + dictionary training, image transcoding, minification) so the reader stays lightweight.

### OZA Format vs. ZIM

OZA content entries use **variable-length records**: a `type_and_flags` byte followed by uvarints for mime_index, chunk_id, blob_offset, and blob_size, plus a fixed 8-byte xxhash content hash (~15 bytes average). A uint32 offset table preceding the records gives O(1) random access by entry ID — `parseEntryTable` in `oza/archive.go` dispatches lookups through this table.

Redirects live in a separate REDIRECT_TABLE section as **5-byte records** (1-byte flags + uint32 target_id). Tagged IDs use bit 31 to distinguish content entries from redirects (`oza/constants.go: RedirectIDBit`), capping each namespace at 2³¹−1. Helpers `IsRedirectID`, `RedirectIndex`, `MakeRedirectID`.

Key differences from ZIM:

- No namespaces -- flat paths by convention (`Main_Page`, `_res/style.css`)
- `blob_size` stored per entry — HTTP `Content-Length` without decompressing the chunk
- Zstd only (with optional per-MIME dictionaries); no XZ/zlib/bzip2
- Three-tier SHA-256 checksums (file, section, chunk) instead of single MD5
- Built-in trigram + body search indexes (delta-encoded posting lists, Roaring bitmaps) instead of opaque Xapian
- Separate optional Chrome/UI section instead of mixing with content
- Content-addressed deduplication via xxhash (replaces SHA-256 from earlier drafts — 8 bytes per entry, much faster)

### I/O Strategy

Internal `reader` interface in `oza/io.go` with mmap (default on 64-bit Unix and Windows) and pread fallback. Chunk LRU cache guarded by `sync.Mutex`; size controlled via `oza.WithCacheSize(n)` (default 8).

### Concurrency

- Reader: `Archive` is safe for concurrent reads; chunk cache is mutex-protected
- Writer: single-threaded API (`AddEntry`/`AddRedirect`), but internally parallelizes Zstd compression via a worker pool (`WriterOptions.CompressWorkers`)
- HTTP server (ozaserve): concurrent request handlers, archives are loaded once at startup

### Patterns Carried from gozim

- Entry as value type (struct with `*Archive` back-pointer for lazy access)
- `iter.Seq[Entry]` iterators for range-over-func, with `iter.Seq2[Entry, error]` `…Err` variants
- Cobra for CLI tools
- Sentinel errors with `fmt.Errorf("%w", ...)` wrapping; prefix `oza:`
- Test helpers in `cmd/internal/testutil/`

### Patterns Introduced in OZA

- Writer builder pattern: `NewWriter` → `AddEntry`/`AddRedirect` → `Close` finalizes
- Section-based architecture: each section independently addressable and verifiable
- Trigram + body search: extract trigrams, delta-encode posting lists, intersect candidate sets via Roaring bitmaps, verify hits
- xxhash content-addressed dedup: identical blobs share the same chunk slot
- Content-profile classifier (`cmd/internal/classify`): 8 profiles driving `zim2oza --auto` and `ozainfo --classify`
- MCP integration: `ozaserve --mcp` runs HTTP + MCP on stdio simultaneously; `ozamcp` is a standalone MCP server

## OZA Format Quick Reference

- **Header:** 128 bytes, little-endian. Magic `0x01415A4F` ("OZA\x01" on disk). Version 1.0.
- **Section table:** 80-byte descriptors. Unknown types skippable via `offset + compressed_size`.
- **Entry table:** Variable-length records (`type_and_flags` byte + uvarints for mime/chunk/blob fields + 8-byte xxhash content_hash) preceded by a uint32 offset table for O(1) lookup. `entry_type` is 0=content or 2=metadata_ref. Redirects (value 1 reserved for in-memory dispatch) live in REDIRECT_TABLE as 5-byte records.
- **MIME table:** Length-prefixed strings. Index 0=text/html, 1=text/css, 2=application/javascript.
- **Content:** Chunks with per-chunk Zstd compression and optional per-MIME dictionaries. Images often stored uncompressed (already compressed formats).
- **Indexes:** Path and title indexes with offset tables for binary search.
- **Search:** Trigram title index + body index using delta-encoded posting lists with Roaring bitmaps for set ops.
- **Integrity:** SHA-256 at file, section, and chunk tiers. Optional Ed25519 SIGNATURES trailer past the file checksum.
- **Compression IDs (`oza/constants.go`):** `CompNone=0`, `CompZstd=1`, `CompBrotli=2`.

## Coding Conventions

- **Error handling:** Return `error`, use sentinel errors from `oza/errors.go`. Prefix `oza:`.
- **No panics** in library code.
- **Test naming:** `Test<FunctionName>` or `Test<FunctionName><Scenario>`. Fuzz targets: `Fuzz<Target>`.
- **Benchmarks:** `Benchmark<Operation>` in `*_test.go` files.
- **All integers:** little-endian on disk.
- **All strings:** UTF-8, NFC-normalized.
- **CLI banner format:** `王座 <toolname> v<version>`.
- **Imports:** writer never imports CLI internals; reader has zero CLI deps.

## Dependencies

### Library (`go.mod`)

| Package | Purpose |
|---------|---------|
| `github.com/klauspost/compress` | Zstd encode/decode + dictionary training |
| `github.com/andybalholm/brotli` | Brotli codec (writer-side) |
| `github.com/cespare/xxhash/v2` | Content-addressed dedup hash (replaces SHA-256 for per-entry dedup) |
| `github.com/RoaringBitmap/roaring/v2` | Compressed bitmaps for search index posting-list intersection |
| `github.com/tdewolff/minify/v2` (+ `parse/v2`) | HTML/CSS/JS/SVG minification |

### CLI (`cmd/go.mod`)

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/stazelabs/gozim` | ZIM reading (zim2oza, ozacmp) |
| `github.com/modelcontextprotocol/go-sdk` | MCP server runtime (ozaserve --mcp, ozamcp) |
| `github.com/yuin/goldmark` | Markdown → HTML for `text/markdown` rendering in ozaserve |
| `github.com/JohannesKaufmann/html-to-markdown/v2` | HTML → Markdown for MCP tool output |
| `golang.org/x/net` | HTML parsing helpers |

## Integration Points

- **`zim2oza --auto`** — runs `cmd/internal/classify` against the source ZIM, applies per-profile `WriterOptions` recommendations (chunk size, Zstd level, search/transcode/minify defaults). See [docs/CLASSIFIER.md](docs/CLASSIFIER.md).
- **`ozainfo --classify`** — runs the same classifier against a built OZA, prints detected profile + suggested re-conversion parameters.
- **`ozaserve`** — opens archives via the reader, serves HTTP routes, optionally exposes an MCP server on stdio simultaneously. `--info-token` gates `/_info` diagnostic pages; `--log-requests` emits structured JSON access logs.
- **`ozamcp`** — standalone stdio MCP server; reuses `cmd/internal/loadutil` and `cmd/internal/mcptools` with ozaserve.

## Testing

```bash
make test         # both modules, -count=1
make test-race    # with race detector
make lint         # golangci-lint (both modules)
make bench        # benchmarks for oza/ and ozawrite/
make fuzz         # all fuzz targets, 30s each
make cover        # coverage report
make testdata     # download test fixtures
```

Tests must pass in both modules — Makefile targets `cd cmd &&` for the second module. The CLI module's tests cover converters, MCP, HTTP routes; the library module covers format primitives, reader/writer round-trips, and benchmarks.

## Status

v0.1.0 has shipped. Format primitives, writer, reader, search, signing, chrome/UI separation, four converters (zim2oza, epub2oza, site2oza), HTTP server, MCP server, and classifier are all in `main`. Current focus is documented in [docs/BACKLOG.md](docs/BACKLOG.md).

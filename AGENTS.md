# AGENTS.md -- 王座 OZA

## Project Overview

`oza` is a Go library and CLI toolset for reading and writing OZA (Open Zipped Archive) files -- a modern replacement for the ZIM file format. Pure Go, no CGo dependencies.

- **Module:** `github.com/stazelabs/oza`
- **Go version:** 1.24+
- **Format spec:** `docs/FORMAT.md`
- **Reference project:** `github.com/stazelabs/gozim` (ZIM reader library)
- **Branding:** 王座 (Japanese: oza, "throne") -- OZA takes the throne as ZIM's successor

## Repository Structure

```
oza/
├── oza/                     # Core reader library
│   ├── archive.go           # Archive type -- Open, Close, entry lookup, chunk cache
│   ├── header.go            # 128-byte header parse/serialize
│   ├── section.go           # 80-byte section descriptors, SectionType enum
│   ├── entry.go             # Variable-length entry records (uvarint); 5-byte redirect records; EntryType enum
│   ├── metadata.go          # Length-prefixed key-value pairs
│   ├── mime.go              # MIME table (length-prefixed, index 0/1/2 convention)
│   ├── chunk.go             # Content chunk reading + decompression + blob extraction
│   ├── compress.go          # Zstd decompression with dictionary support
│   ├── index.go             # Path/title index binary search
│   ├── search.go            # Trigram index reader + query algorithm
│   ├── signature.go         # Ed25519 signature verification (trailer past file checksum)
│   ├── checksum.go          # SHA-256 at file/section/chunk tiers
│   ├── io.go                # reader interface (mmap + pread)
│   ├── iter.go              # iter.Seq[Entry] iterators
│   ├── errors.go            # Sentinel errors
│   ├── bench_test.go        # Reader benchmarks
│   └── *_test.go            # Tests per file
├── ozawrite/                # Writer library
│   ├── writer.go            # Builder API: NewWriter, AddEntry, AddRedirect, Close
│   ├── chunk.go             # Chunk grouping + compression
│   ├── compress.go          # Zstd compression + dictionary training
│   ├── index.go             # Path/title index builder
│   ├── search.go            # Trigram index builder
│   ├── dedup.go             # Content-addressed SHA-256 deduplication
│   ├── checksum.go          # SHA-256 computation
│   ├── signature.go         # Ed25519 signing
│   ├── bench_test.go        # Writer benchmarks
│   └── *_test.go            # Tests per file
├── cmd/
│   ├── ozainfo/             # CLI: dump OZA metadata and section table
│   ├── ozacat/              # CLI: extract content by path, list entries
│   ├── ozaserve/            # CLI: HTTP server for OZA content
│   ├── ozasearch/           # CLI: trigram search queries
│   ├── ozaverify/           # CLI: three-tier integrity verification
│   └── zim2oza/             # CLI: ZIM-to-OZA converter (critical tool)
├── testdata/                # Test files
├── docs/
│   ├── FORMAT.md            # OZA format specification
│   └── BRANDING.md          # 王座 branding guide
└── Makefile
```

## Architecture & Key Decisions

### Two Packages: `oza` (Reader) + `ozawrite` (Writer)

The writer has heavier dependencies (Zstd encoding + dictionary training, ed25519 signing, sort/hash infrastructure for index building). Separating them keeps the reader lightweight. The writer imports the reader for shared type definitions -- unidirectional dependency, no cycles.

### OZA Format vs ZIM

OZA content entries use **variable-length records** (uvarint-encoded mime_index, chunk_id, blob_offset, blob_size, plus a fixed 8-byte content_hash — ~15 bytes average) preceded by a uint32 offset table for O(1) random access by ID. Redirects live in a separate REDIRECT_TABLE section as **5-byte records** (1-byte flags + uint32 target_id). Tagged IDs use bit 31 to distinguish content entries from redirects, capping each namespace at 2³¹−1.

Key format differences from ZIM:
- No namespaces -- flat paths by convention (`Main_Page`, `_res/style.css`)
- `blob_size` stored per entry -- HTTP `Content-Length` without decompression
- Zstd-only compression with dictionary support (no XZ, zlib, bzip2)
- Three-tier SHA-256 checksums (file, section, chunk) instead of single MD5
- Built-in trigram search index instead of opaque Xapian
- Separate Chrome/UI section instead of mixing with content
- Content-addressed deduplication via SHA-256

### I/O Strategy

Same as gozim: internal `reader` interface with mmap (default on 64-bit) and pread fallback. Chunk LRU cache with `sync.Mutex`.

### Patterns Carried from gozim

- Entry as value type (struct with `*Archive` back-pointer for lazy access)
- `iter.Seq[Entry]` iterators for range-over-func
- Cobra for CLI tools
- Sentinel errors with `fmt.Errorf("%w", ...)` wrapping
- Test helpers: `testdataPath()`, `skipIfNoTestdata()`

### New Patterns in OZA

- Writer builder pattern: `NewWriter` -> `AddEntry`/`AddRedirect` -> `Close()` finalizes
- Section-based architecture: each section independently addressable and verifiable
- Trigram search: extract trigrams, delta-encode posting lists, intersect + verify
- Deduplication: SHA-256 content hash, identical entries share same blob

## OZA Format Quick Reference

- **Header:** 128 bytes, little-endian. Magic `0x01415A4F` ("OZA\x01" on disk). Version 1.0.
- **Section table:** 80-byte descriptors. Unknown types skippable via `offset + compressed_size`.
- **Entry table:** Variable-length records (`type_and_flags` byte + uvarints for mime/chunk/blob fields + 8-byte content_hash) with a uint32 offset table for O(1) lookup. `entry_type` is 0=content or 2=metadata_ref. Redirects (value 1 is reserved for in-memory dispatch) live in the REDIRECT_TABLE section as 5-byte records.
- **MIME table:** Length-prefixed strings. Index 0=text/html, 1=text/css, 2=application/javascript.
- **Content:** Chunks with per-chunk compression. Zstd level 19 for text, uncompressed for images.
- **Indexes:** Path and title indexes with offset tables for binary search.
- **Search:** Trigram index with delta-encoded posting lists.
- **Integrity:** SHA-256 at file, section, and chunk levels. Optional Ed25519 signatures.

## Coding Conventions

- **Error handling:** Return `error`, use sentinel errors from `errors.go`. Prefix: `oza:`.
- **No panics** in library code.
- **Test naming:** `Test<FunctionName>` or `Test<FunctionName><Scenario>`.
- **Benchmarks:** `Benchmark<Operation>` in `*_test.go` files.
- **All integers:** Little-endian on disk.
- **All strings:** UTF-8, NFC-normalized.
- **CLI banner format:** `王座 <toolname> v<version>`

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/klauspost/compress` | Zstd encode/decode + dictionary training |
| `github.com/spf13/cobra` | CLI framework (cmd/ tools only) |
| `github.com/stazelabs/gozim` | ZIM reading (zim2oza converter only) |

## Testing

```bash
go test ./...              # Run all tests
go test -race ./...        # With race detector
go test -bench=. ./oza/ ./ozawrite/  # Benchmarks
make testdata              # Download test files
```

## Implementation Phases

1. **Format Primitives** -- header, section, entry, metadata, MIME parsing + tests
2. **Writer** -- ozawrite: AddEntry/AddRedirect/Close, compression, indexes, dedup, checksums
3. **Reader** -- oza/archive: Open, EntryByPath, ReadContent, chunk cache, iterators
4. **zim2oza** -- full conversion pipeline, statistics reporting
5. **Core CLI** -- ozainfo, ozacat, ozaverify
6. **Search** -- trigram index read/write, ozasearch
7. **HTTP Server** -- ozaserve
8. **Advanced** -- Ed25519 signatures, chrome extraction, CJK bigrams
9. **Polish** -- fuzz tests, benchmarks, CI/CD, v0.1.0

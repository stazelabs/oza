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
│   ├── entry.go             # 40-byte fixed entry records, EntryType enum
│   ├── metadata.go          # Length-prefixed key-value pairs
│   ├── mime.go              # MIME table (length-prefixed, index 0/1/2 convention)
│   ├── chunk.go             # Content chunk reading + decompression + blob extraction
│   ├── compress.go          # Zstd decompression with dictionary support
│   ├── index.go             # Path/title index binary search
│   ├── search.go            # Trigram index reader + query algorithm
│   ├── redirect.go          # Redirect table + chain resolution
│   ├── chrome.go            # Chrome section reader
│   ├── signature.go         # Ed25519 signature verification
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

OZA entries are **fixed 40 bytes** with explicit `entry_type`, `blob_size`, and `content_hash`. No variable-length directory entries, no MIME sentinel overloading, no pointer indirection chains. Entry N is at `section_offset + N * 40` -- O(1) access.

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

- **Header:** 128 bytes, little-endian. Magic `0x415A4F01`. Version 1.0.
- **Section table:** 80-byte descriptors. Unknown types skippable via `offset + compressed_size`.
- **Entry table:** Fixed 40-byte records. `entry_type` field (0=content, 1=redirect, 2=metadata_ref).
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

<!-- ash:begin -->
# ash — agent guidance

This section is managed by `ash init`. Re-running `ash init` refreshes the
content between the `<!-- ash:begin -->` and `<!-- ash:end -->` markers; do
not edit between them. The rest of this file is yours.

## What ash is

`ash` is an agentic shell — a daemon-backed CLI that exposes filesystem,
search, git, and test operations as token-efficient verbs and records every
call to a per-repo SQLite ledger at `.ash/ledger.db`. The PreToolUse hook
installed alongside this section redirects the harness's built-in
`Grep`/`Glob`/`Read`/`Edit`/`Write` tools and several bash commands
(`find`, `grep`, `cat`, `head`, `tail`, `ls -R`, `git status`, `git log`,
`git diff`, `stat`) to their `ash` equivalents.

`ash help` is the authoritative verb list. `ash help --verb <name>` is the
authoritative arg schema. Markdown can drift; help is generated from code.

## When to use ash

Any `ash` invocation auto-starts the daemon. Subsequent calls reuse it over
a per-project Unix domain socket.

1. **Path or filename lookup across more than one directory** — `ash find --path <p> --glob '<pat>' --type file|dir|symlink`. Hidden directories and `.gitignore`d paths are skipped by default.
2. **Pattern search across files** — `ash grep --pattern '<re>' --path <p> --glob '<pat>'`. Smart-case by default; add `--fixed_string true` for literal matches, `--files_only true` for path-only output, `--no_text true` for `path:line:col` rows without excerpts.
3. **Read a file** — `ash read --path <p> [--range start:end]`. Default cap 256 KiB; UTF-8 returned as-is, binary base64-encoded.
4. **Stat paths** — `ash stat --paths a,b,c`. Uses `lstat`. Per-entry errors keep bulk calls alive when some paths are missing.
5. **Diff two files or contents** — `ash diff --path a --other b` or `ash diff --path f --content - < new`. Both inputs capped at 4000 lines.
6. **Write a file** — `ash write --path <p> --content - << 'EOF' … EOF`. Atomic via temp-file + rename.
7. **Edit a file** — `ash edit` with one of: `--old_string`/`--new_string`, `--range start:end --new_content`, or `--patch`. **Default to stdin** for any non-trivial content.
8. **Git status, log, diff, show** — `ash git --op status|log|diff|show`. Other git ops (commit, push, blame, rebase, checkout, etc.) stay in bash.
9. **Run Go tests** — `ash test [--packages <p>] [--run <name>] [--race true] [--short true]`. Failures arrive as a structured slice with `file:line` extracted; build failures land as `Status=build_failed`.
10. **Inspect the ledger** — `ash report` for synthesis, `ash metrics` for raw rows. `ash report --since 1h` is the most common form when a session feels heavy.

## The shell-quoting footgun

`ash edit` and `ash write` accept content via flags, but inline shell
arguments silently corrupt backticks, single quotes, backslashes, escape
sequences, and multiline blocks. **Default to stdin** for any non-trivial
content, in this order:

- Whole-file rewrite: `ash write --path <p> --content - << 'EOF' … EOF`
- Line-range edit: `ash edit --path <p> --range 5:10 --new_content - << 'EOF' … EOF`
- Cross-cutting or multi-region: `ash edit --path <p> --patch - << 'EOF' … EOF`

Inline `--new_content='…'` is for short ASCII-only swaps with no quoting
hazards. For hostile content (heredoc-with-EOF, mixed quotes), write a
Python fixer to `/tmp` via `ash write` and execute it — Python string
concat sidesteps all shell quoting.

## When ash doesn't fit

The hook is best-effort. Some operations stay in bash:

- `git` ops other than `status`, `log`, `diff`, `show`.
- `go build`, `go vet`, system package management, OS-level process management.
- Anything not yet shipped as an `ash` verb. Run `ash help` to check the live list.

If the hook gets in the way of a legitimate operation, run the bash command
anyway. The deny message is a nudge, not a wall.

## Daemon stickiness

Edits to `ash.toml` (jail policy, git backend, daemon limits) take effect
only on daemon restart. Run `ash stop`; the next `ash` invocation auto-starts
a fresh daemon. Don't `pkill ashd` — it bypasses graceful shutdown.

## Reference

- `ash help` — authoritative verb list.
- `ash help --verb <name>` — per-verb arg schema.
- `.ash/ledger.db` — SQLite ledger of every call (one row per invocation).
- The upstream ash project's `CLAUDE.md` and `docs/` for design depth.
<!-- ash:end -->

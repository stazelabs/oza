# 王座 OZA

[![CI](https://github.com/stazelabs/oza/actions/workflows/ci.yml/badge.svg)](https://github.com/stazelabs/oza/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/stazelabs/oza)](https://github.com/stazelabs/oza/releases/latest)
[![codecov](https://codecov.io/gh/stazelabs/oza/branch/main/graph/badge.svg)](https://codecov.io/gh/stazelabs/oza)
[![Go Report Card](https://goreportcard.com/badge/github.com/stazelabs/oza)](https://goreportcard.com/report/github.com/stazelabs/oza)
[![Go Reference](https://pkg.go.dev/badge/github.com/stazelabs/oza/oza.svg)](https://pkg.go.dev/github.com/stazelabs/oza/oza)
[![Go 1.24+](https://img.shields.io/github/go-mod/go-version/stazelabs/oza)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

A modern replacement for the [ZIM file format](https://wiki.openzim.org/wiki/ZIM_file_format). Pure Go library and CLI tools for reading, writing, and serving OZA archives.

> *王座 (oza) -- "throne." OZA takes the throne as the successor to ZIM, with extensible section tables, Zstd compression, SHA-256 integrity, trigram search, and content-addressed deduplication.*

## Why OZA?

ZIM has served the offline content community since 2007, but its design has aged:

- **Frozen header** -- no extensibility without format hacks
- **Namespace overloading** -- entry types smuggled into MIME index sentinels
- **Single MD5** -- one hash for an entire 90 GB file, no corruption localization
- **Xapian search** -- 150K lines of C++ with no binary spec, impossible to implement without `libxapian`
- **No content sizes** -- `Content-Length` requires decompressing entire clusters
- **Four compression formats** -- readers must carry zlib, bzip2, XZ, and Zstd
- **Chrome entanglement** -- HTML assumes a specific application shell at runtime

OZA addresses all of these with a clean-break redesign. See [docs/FORMAT.md](docs/FORMAT.md) for the full specification.

## Format Highlights

| Feature | ZIM | OZA |
|---------|-----|-----|
| Header | Fixed 80 bytes, no extensibility | 128 bytes + section table |
| Entry records | Variable length, 3 pointer indirections | Variable-length (~15 bytes avg), O(1) by ID |
| Content size | Must decompress cluster | `blob_size` in every entry |
| Compression | XZ/Zstd/zlib/bzip2 | Zstd only + dictionaries |
| Integrity | Single MD5 | SHA-256 at file/section/chunk |
| Search | Opaque Xapian C++ database | Trigram index (fully specified) |
| Deduplication | None | Content-addressed via SHA-256 |
| Signatures | None | Optional Ed25519 |
| Chrome/UI | Mixed with content | Separate optional section |

## Install

```bash
# Reader library
go get github.com/stazelabs/oza/oza
# Writer library
go get github.com/stazelabs/oza/ozawrite
```

## Usage

```go
package main

import (
    "fmt"
    "log"

    "github.com/stazelabs/oza/oza"
)

func main() {
    a, err := oza.Open("archive.oza")
    if err != nil {
        log.Fatal(err)
    }
    defer a.Close()

    // Read metadata
    title, _ := a.Metadata("title")
    fmt.Println("Archive:", title)
    fmt.Println("Entries:", a.EntryCount())

    // Look up an entry by path
    entry, err := a.EntryByPath("Main_Page")
    if err != nil {
        log.Fatal(err)
    }

    // Read content (resolves redirects automatically)
    data, err := entry.ReadContent()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Content-Type: %s\n", entry.MIMEType())
    fmt.Printf("Size: %d bytes\n", len(data))
    fmt.Printf("Blob size: %d bytes\n", entry.Size()) // no decompression needed

    // Iterate all front articles
    for e := range a.FrontArticles() {
        fmt.Println(e.Path())
    }
}
```

## Writing OZA Files

```go
package main

import (
    "log"
    "os"

    "github.com/stazelabs/oza/ozawrite"
)

func main() {
    f, err := os.Create("output.oza")
    if err != nil {
        log.Fatal(err)
    }
    defer f.Close()

    w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
        ZstdLevel:       6,  // 1=fastest, 6=default, 19=best
        BuildSearch:     true,
        CompressWorkers: 0,  // 0 = min(NumCPU, 4)
    })

    w.SetMetadata("title", "My Archive")
    w.SetMetadata("language", "en")
    w.SetMetadata("creator", "Example")
    w.SetMetadata("date", "2026-03-07")
    w.SetMetadata("source", "https://example.com")

    id, _ := w.AddEntry("Main_Page", "Main Page", "text/html",
        []byte("<h1>Hello, World</h1>"), true)

    w.AddRedirect("Home", "Home", id)

    if err := w.Close(); err != nil {
        log.Fatal(err)
    }
}
```

## CLI Tools

Pass `-h` to any tool for its own help output. Defaults and behavior reflect the current `main` branch.

### ozainfo

Dump metadata, section table, and statistics from an OZA file.

```bash
go run ./cmd/ozainfo archive.oza
go run ./cmd/ozainfo --classify archive.oza   # detect content profile
go run ./cmd/ozainfo --json archive.oza       # machine-readable
```

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | false | Output statistics as JSON |
| `--classify` | false | Run the content-profile classifier and print recommendations |

### ozacat

Extract content from an OZA file.

```bash
# Extract an article to stdout
go run ./cmd/ozacat archive.oza Main_Page

# List all entries
go run ./cmd/ozacat -l archive.oza

# Write content to a file instead of stdout
go run ./cmd/ozacat -o page.html archive.oza Main_Page
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--list` | `-l` | false | List all entries (path, type, MIME, size) |
| `--meta` | `-m` | false | Show all metadata key/value pairs |
| `--info` | `-t` | false | Show entry info without extracting content |
| `--output` | `-o` | (stdout) | Write content to a file instead of stdout |

### ozasearch

Full-text trigram search.

```bash
go run ./cmd/ozasearch archive.oza "quantum mechanics"
go run ./cmd/ozasearch -t -l 50 archive.oza "newton"   # titles only, top 50
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--limit` | `-l` | 20 | Maximum number of results to return |
| `--json` | `-j` | false | Output results as JSON |
| `--title-only` | `-t` | false | Search article titles only (skip body index) |

### ozaverify

Three-tier integrity verification with optional Ed25519 signature checking.

```bash
# File-level SHA-256 check
go run ./cmd/ozaverify archive.oza

# All three integrity tiers (file + per-section + per-entry chunks)
go run ./cmd/ozaverify --all archive.oza

# Verify Ed25519 signatures against a trusted public key (see ozakeygen)
go run ./cmd/ozaverify --signatures --pubkey <hex> archive.oza
```

| Flag | Default | Description |
|------|---------|-------------|
| `--all` | false | Verify all three integrity tiers (sections + chunks) |
| `--sections` | false | Verify per-section SHA-256 checksums |
| `--chunks` | false | Verify per-entry content hashes |
| `--signatures` | false | Verify Ed25519 signatures (requires `--pubkey`) |
| `--pubkey` | | Trusted Ed25519 public key in hex (repeatable) |
| `--quiet` | false | Suppress OK lines; print only failures and summary |

### ozaserve

Serve OZA files over HTTP with a browse/search UI and optional MCP server. See [cmd/ozaserve/docs/ozaserve.md](cmd/ozaserve/docs/ozaserve.md) for routes and full documentation.

```bash
# Serve a single file on :8080
go run ./cmd/ozaserve archive.oza

# Serve a directory of archives recursively
go run ./cmd/ozaserve -d ./archives -r

# Public-facing: token-gate the diagnostic /_info pages
go run ./cmd/ozaserve --info-token "$INFO_TOKEN" --log-requests -d ./archives
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--addr` | `-a` | `:8080` | Listen address (`host:port`) |
| `--cache` | `-c` | 64 | Decompressed chunk cache size per archive |
| `--dir` | `-d` | | Directory of OZA files to serve (repeatable) |
| `--recursive` | `-r` | false | Scan `--dir` directories recursively |
| `--no-info` | | false | Disable all `/_info` diagnostic pages |
| `--info-token` | | | Require this bearer token to access `/_info` endpoints |
| `--log-requests` | | false | Emit structured JSON access logs for every request |
| `--mcp` | | false | Also expose an MCP server on stdio |

### ozamcp

Standalone MCP server for LLM agents (see [docs/OZAMCP.md](docs/OZAMCP.md)).

```bash
go run ./cmd/ozamcp archive.oza
go run ./cmd/ozamcp -d ./archives -r
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | | Directory of OZA files (repeatable) |
| `--recursive` | `-r` | false | Scan `--dir` directories recursively |
| `--cache` | `-c` | 64 | Chunk cache size per archive |
| `--transport` | `-t` | `stdio` | Transport (only `stdio` supported) |

### ozakeygen

Generate an Ed25519 signing keypair. The private key is written as PEM to the
`--out` target (or stdout if omitted); the public key is printed to **stdout**
as hex (prefixed with `Public key (hex):`) for use with `ozaverify --pubkey`.

```bash
# Write private key PEM to signer.key; public key hex printed on stdout
go run ./cmd/ozakeygen --out signer.key
```

| Flag | Default | Description |
|------|---------|-------------|
| `--out` | (stdout) | Path to write the PEM-encoded private key |

See [Signing & verifying](#signing--verifying) below for the end-to-end workflow.

### ozacmp

Compare a ZIM file and its OZA conversion side-by-side.

```bash
go run ./cmd/ozacmp source.zim converted.oza
go run ./cmd/ozacmp --format md source.zim converted.oza
go run ./cmd/ozacmp --deep source.zim converted.oza   # slow: per-entry byte sizes
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format: `text`, `json`, or `md` |
| `--deep` | false | Read all ZIM content to compute per-entry byte sizes (slow) |

### zim2oza

Convert ZIM files to OZA format. With `--auto`, the converter runs the content classifier and applies per-profile recommended parameters (see [docs/CLASSIFIER.md](docs/CLASSIFIER.md)).

```bash
go run ./cmd/zim2oza wikipedia.zim wikipedia.oza
go run ./cmd/zim2oza --auto wikipedia.zim wikipedia.oza
go run ./cmd/zim2oza --verbose --json-stats wikipedia.zim wikipedia.oza
go run ./cmd/zim2oza --dry-run wikipedia.zim
```

| Flag | Default | Description |
|------|---------|-------------|
| `--auto` | false | Auto-detect content profile and apply recommended parameters |
| `--zstd-level` | 6 | Zstd compression level (1=fastest, 6=default, 19=best) |
| `--chunk-size` | 4194304 | Target uncompressed chunk size in bytes |
| `--compress-workers` | 0 | Parallel compression workers (0 = min(NumCPU, 4)) |
| `--no-dict` | false | Disable Zstd dictionary training |
| `--dict-samples` | 2000 | Max samples for dictionary training |
| `--no-search` | false | Disable trigram search indexes |
| `--search-prune` | 0.5 | Prune trigrams appearing in >= this fraction of docs |
| `--minify` | false | Enable content minification (HTML, CSS, JS, SVG) |
| `--no-optimize-images` | false | Disable lossless image optimization (JPEG metadata strip) |
| `--transcode` | `auto` | Image transcoding mode: `auto`, `off`, `require` |
| `--transcode-lossy-jpeg` | false | Enable lossy JPEG→WebP transcoding (~25-35% savings) |
| `--transcode-avif` | false | Prefer AVIF over WebP (requires `avifenc`) |
| `--dry-run` | false | Scan and report statistics without writing output |
| `--verbose` | false | Print detailed progress and statistics |
| `--json-stats` | false | Emit statistics as JSON (implies `--verbose`) |
| `--profile` | | Write CPU and memory profiles to this directory |

### epub2oza

Convert EPUB books to OZA format. Supports single-book and collection modes.

```bash
# Single book
go run ./cmd/epub2oza book.epub book.oza

# Collection: bundle every EPUB in a directory into one searchable archive
go run ./cmd/epub2oza --collection --title "My Library" ./epubs/ library.oza
```

| Flag | Default | Description |
|------|---------|-------------|
| `--collection` | false | Treat input as a directory of `.epub` files |
| `--title` | | Collection title (collection mode only; default = directory name) |
| `--recursive` | false | In collection mode, recurse into subdirectories |
| `--zstd-level` | 6 | Zstd compression level |
| `--chunk-size` | 4194304 | Target uncompressed chunk size in bytes |
| `--compress-workers` | 0 | Parallel compression workers |
| `--no-dict` | false | Disable Zstd dictionary training |
| `--dict-samples` | 2000 | Max samples for dictionary training |
| `--no-search` | false | Disable trigram search indexes |
| `--search-prune` | 0.5 | Prune trigrams >= this fraction of docs |
| `--minify` | false | Enable content minification |
| `--no-optimize-images` | false | Disable lossless image optimization |
| `--transcode` | `auto` | Image transcoding: `auto`, `off`, `require` |
| `--transcode-lossy-jpeg` | false | Enable lossy JPEG→WebP transcoding |
| `--transcode-avif` | false | Prefer AVIF over WebP |
| `--verbose` | false | Print detailed progress and statistics |
| `--profile` | | Write CPU and memory profiles to this directory |

### site2oza

Convert a directory of HTML and asset files into an OZA archive.

```bash
go run ./cmd/site2oza ./mysite/ mysite.oza
go run ./cmd/site2oza --title "My Site" --language en ./mysite/ mysite.oza
```

| Flag | Default | Description |
|------|---------|-------------|
| `--title` | (directory name) | Archive title |
| `--language` | `en` | BCP-47 language code |
| `--creator` | `Unknown` | Author or organization |
| `--date` | (today) | ISO date for the archive |
| `--source` | (input path) | Source URL or path |
| `--exclude` | | Glob patterns for files to exclude (repeatable) |
| `--zstd-level` | 6 | Zstd compression level |
| `--chunk-size` | 4194304 | Target uncompressed chunk size in bytes |
| `--compress-workers` | 0 | Parallel compression workers |
| `--no-dict` | false | Disable Zstd dictionary training |
| `--dict-samples` | 2000 | Max samples for dictionary training |
| `--no-search` | false | Disable trigram search indexes |
| `--search-prune` | 0.5 | Prune trigrams >= this fraction of docs |
| `--minify` | false | Enable content minification |
| `--no-optimize-images` | false | Disable lossless image optimization |
| `--transcode` | `auto` | Image transcoding mode |
| `--transcode-lossy-jpeg` | false | Enable lossy JPEG→WebP transcoding |
| `--transcode-avif` | false | Prefer AVIF over WebP |
| `--verbose` | false | Print detailed progress and statistics |
| `--profile` | | Write CPU and memory profiles to this directory |

## Signing & verifying

OZA archives can carry one or more Ed25519 signatures appended after the file checksum. The full workflow is:

```bash
# 1. Generate a keypair (private key PEM to file, public key hex to stdout)
go run ./cmd/ozakeygen --out signer.key
# → Public key (hex): 1a2b3c…
```

```go
// 2. Build a signed archive in Go:
pemBytes, _ := os.ReadFile("signer.key")
block, _ := pem.Decode(pemBytes)
priv := ed25519.PrivateKey(block.Bytes)

w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
    SigningKeys: []ozawrite.SigningKey{{Key: priv, KeyID: 1}},
})
```

```bash
# 3. Verify on the consumer side
go run ./cmd/ozaverify --signatures --pubkey 1a2b3c… archive.oza
```

Signatures live in a trailer past the file-level checksum, so they do not affect `oza.Verify()` of the underlying data. Multiple trusted keys can be passed by repeating `--pubkey`.

## Auto-classification

OZA ships a content-profile classifier (encyclopedia, dictionary, books, qa-forum, docs, media-heavy, pdf-container, mixed-scrape) with two integration points:

- **`ozainfo --classify`** -- runs the classifier against a built OZA archive and prints the detected profile, confidence, and recommended `WriterOptions`.
- **`zim2oza --auto`** -- runs the classifier against the source ZIM before conversion and applies the per-profile recommended chunk size, Zstd level, search-index choices, and image-transcoding mode.

See [docs/CLASSIFIER.md](docs/CLASSIFIER.md) for the taxonomy, feature vector, and decision rules.

## API Overview

Full reference: [pkg.go.dev/github.com/stazelabs/oza/oza](https://pkg.go.dev/github.com/stazelabs/oza/oza) (reader) and [pkg.go.dev/github.com/stazelabs/oza/ozawrite](https://pkg.go.dev/github.com/stazelabs/oza/ozawrite) (writer).

### Reader (`oza`)

Open archives with `oza.Open(path)` (defaults) or `oza.OpenWithOptions(path, opts...)`. Available options:

| Option | Purpose |
|--------|---------|
| `WithMmap(bool)` | Enable/disable memory-mapped I/O (default: true on 64-bit) |
| `WithCacheSize(n)` | Decompressed chunk LRU cache size (default: 8) |
| `WithVerifyOnOpen()` | Verify section checksums during `Open` |
| `WithMaxDecompressedSize(n)` | Cap decompressed chunk size to defend against decompression bombs |
| `WithMaxBlobSize(n)` | Cap per-entry blob size |
| `WithMaxMetadataValueSize(n)` | Cap individual metadata value length |

**Archive** — high-level access to a single file:

- *Lookup:* `EntryByPath`, `EntryByTitle`, `EntryByID`, `MainEntry`, `Metadata`, `AllMetadata`
- *Iteration:* `Entries`, `EntriesByPath`, `EntriesByTitle`, `FrontArticles`, `RedirectEntries`, `EntriesByMIME(mime)`; each has an `…Err` (`iter.Seq2[Entry, error]`) variant that surfaces per-record errors instead of silently skipping
- *Counts & stats:* `EntryCount`, `RedirectCount`, `ChunkCount`, `MIMEEntryCount`, `EntryCountByMIME`, `TitleCount`, `CacheStats`, `Warnings`
- *Structure:* `FileHeader`, `Sections`, `UUID`, `MIMETypes`
- *Search:* `Search(q, SearchOptions{})`, `SearchTitles(q, limit)`, `BrowseTitles(offset, limit)`; presence checks `HasSearch`, `HasTitleSearch`, `HasBodySearch`; index sizes `TitleSearchDocCount`, `BodySearchDocCount`
- *Title-index streaming:* `ForEachTitleKey`, `ForEachEntryRecord`, `ForEachEntryRecordErr`
- *Integrity:* `Verify` (file-level SHA-256), `VerifySection(sectionType)`, `VerifyAll` (all tiers)
- *Signatures:* `HasSignatures`, `ReadSignatures`, `VerifySignatures(trustedKeys)`
- *Lifecycle:* `Close`

**Entry** — value type:

- `ID`, `Path`, `Title`, `MIMEType`, `MIMEIndex`, `Size`
- `IsRedirect`, `IsFrontArticle`, `Resolve` (follow redirect chain)
- `ReadContent` (allocates, resolves redirects), `ReadContentSlice` (zero-copy when possible), `ContentReader` (`io.Reader`), `WriteTo(io.Writer)`

**Sentinel errors** (`oza/errors.go`): `ErrNotFound`, `ErrIsRedirect`, `ErrNotRedirect`, `ErrInvalidMagic`, `ErrUnsupportedVersion`, `ErrChecksumMismatch`, `ErrCorruptedSection`, `ErrCorruptedChunk`, `ErrInvalidEntry`, `ErrRedirectLoop`, `ErrMissingMetadata`, `ErrDecompressedTooLarge`, `ErrBlobTooLarge`, `ErrMetadataValueTooLarge`, `ErrChunkTableUnsorted`, `ErrInvalidMetadataValue`, `ErrDuplicateMetadataKey`.

### Writer (`ozawrite`)

```go
w := ozawrite.NewWriter(f, ozawrite.DefaultOptions())   // or WriterOptions{...}
w.SetMetadata(key, value)
id, err := w.AddEntry(path, title, mimeType, content, isFrontArticle)
_, err   = w.AddRedirect(path, title, targetID)
err      = w.Close()                                    // finalizes; Timings() valid afterward
```

`WriterOptions` covers compression (`ZstdLevel`, `ChunkTargetSize`, `CompressWorkers`), dictionary training (`TrainDict`, `DictSamples`), search (`BuildSearch`, `BuildTitleSearch`, `BuildBodySearch`, `SearchPruneFreq`), minification (`MinifyHTML`/`CSS`/`JS`/`SVG`), image optimization (`OptimizeImages`, `TranscodeTools`), metadata strictness (`StrictMetadata`), signing (`SigningKeys []SigningKey`), and a `Progress ProgressFunc` callback. Prefer `DefaultOptions()` as a baseline over the zero value — most boolean defaults are `true`.

`Writer.Timings()` (after `Close`) returns per-phase wall-clock data useful for benchmarking conversion pipelines.

## Benchmarks

Run all benchmarks:

```bash
make bench
```

Run a specific benchmark or subset:

```bash
go test -bench=BenchmarkOpen -benchmem ./oza/
go test -bench=BenchmarkWrite -benchmem ./ozawrite/
```

Compare performance across changes with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```bash
go test -bench=. -benchmem -count=6 ./oza/ ./ozawrite/ > old.txt
# ... make changes ...
go test -bench=. -benchmem -count=6 ./oza/ ./ozawrite/ > new.txt
benchstat old.txt new.txt
```

### Reader benchmarks (`oza/bench_test.go`)

| Benchmark | What it measures |
|-----------|-----------------|
| `BenchmarkOpen` | Header parsing, section loading, index construction |
| `BenchmarkEntryByPath` | Binary search on path index |
| `BenchmarkEntryByID` | O(1) entry lookup by numeric ID |
| `BenchmarkReadContent` | Chunk decompression (cached and uncached sub-benchmarks) |
| `BenchmarkVerify` | File-level SHA-256 verification |
| `BenchmarkVerifyAll` | Three-tier integrity check (file + section + entry) |
| `BenchmarkSearch` | Trigram full-text search |

### Writer benchmarks (`ozawrite/bench_test.go`)

| Benchmark | What it measures |
|-----------|-----------------|
| `BenchmarkWriteSmall` | End-to-end archive creation (100 entries) |
| `BenchmarkWriteMedium` | End-to-end archive creation (10K entries) |
| `BenchmarkWriteWithDict` | Archive creation with dictionary training (500 entries) |
| `BenchmarkCompressChunk` | Zstd compression throughput (64 KB chunk) |
| `BenchmarkTrainDictionary` | Zstd dictionary training from HTML samples |
| `BenchmarkBuildTrigramIndex` | Trigram index construction (1K entries, in-memory) |
| `BenchmarkBuildTrigramIndexLarge` | Trigram index construction (5K entries, disk spilling) |

### Conversion benchmarks

```bash
make bench-convert                          # convert small.zim (downloads test data)
make bench-convert-large ZIM=/path/to.zim   # convert a large ZIM file
```

## Development

```bash
make test        # run tests
make test-race   # run with race detector
make bench       # run benchmarks
make testdata    # download test files
make build       # build all CLI tools
```

## License

Apache 2.0 -- see [LICENSE](LICENSE) for details.

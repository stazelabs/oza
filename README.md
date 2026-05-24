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

OZA addresses all of these with a clean-break redesign. See [docs/SPEC.md](docs/SPEC.md) for the full specification.

## Format Highlights

| Feature | ZIM | OZA |
|---------|-----|-----|
| Header | Fixed 80 bytes, no extensibility | 128 bytes + section table |
| Entry records | Variable length, 3 pointer indirections | Variable length (~15 B avg), O(1) by ID via offset table |
| Content size | Must decompress cluster | `blob_size` in every entry |
| Compression | XZ/Zstd/zlib/bzip2 | Zstd only + dictionaries |
| Integrity | Single MD5 | SHA-256 at file/section/chunk |
| Search | Opaque Xapian C++ database | Trigram index (fully specified) |
| Deduplication | None | Content-addressed via SHA-256 |
| Signatures | None | Optional Ed25519 |
| Chrome/UI | Mixed with content | Separate optional section |

## Install

The library lives in the `oza` (reader) and `ozawrite` (writer) subpackages:

```bash
go get github.com/stazelabs/oza/oza        # reader
go get github.com/stazelabs/oza/ozawrite   # writer
```

Prebuilt CLI binaries are published on the [releases page](https://github.com/stazelabs/oza/releases). To install from source:

```bash
go install github.com/stazelabs/oza/cmd/ozainfo@latest
go install github.com/stazelabs/oza/cmd/ozaserve@latest
# ...etc — see ./cmd/ for the full list
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

### ozainfo

Dump metadata and section table of an OZA file:

```bash
go run ./cmd/ozainfo archive.oza
```

### ozacat

Extract content from an OZA file:

```bash
# Extract an article to stdout
go run ./cmd/ozacat archive.oza Main_Page

# List all entries
go run ./cmd/ozacat -l archive.oza

# Show metadata
go run ./cmd/ozacat -m archive.oza
```

### ozasearch

Full-text trigram search. By default searches both title and body indices with title matches ranked first:

```bash
go run ./cmd/ozasearch archive.oza "quantum mechanics"
go run ./cmd/ozasearch -l 50 archive.oza "quantum"     # raise result limit (default 20)
go run ./cmd/ozasearch -t archive.oza "quantum"        # title index only
go run ./cmd/ozasearch -j archive.oza "quantum"        # JSON output for scripting
```

### ozaverify

Tiered integrity verification (file → section → entry SHA-256) plus optional Ed25519 signature verification:

```bash
# File-level SHA-256 check (fastest, single-pass)
go run ./cmd/ozaverify archive.oza

# Full integrity verification (file + section + entry)
go run ./cmd/ozaverify --all archive.oza

# Section-level only / entry-level only
go run ./cmd/ozaverify --sections archive.oza
go run ./cmd/ozaverify --chunks archive.oza

# Verify Ed25519 signatures against trusted public keys (hex-encoded, repeat for multiple)
go run ./cmd/ozaverify --signatures --pubkey <hex> archive.oza

# Quiet mode (exit code only, suppresses progress)
go run ./cmd/ozaverify --quiet --all archive.oza
```

### ozaserve

Serve OZA files over HTTP (and optionally as an MCP server). See [cmd/ozaserve/docs/ozaserve.md](cmd/ozaserve/docs/ozaserve.md) for the full guide.

```bash
# Single archive
go run ./cmd/ozaserve -a :8080 archive.oza

# Directory of archives (each served at its own slug)
go run ./cmd/ozaserve -a :8080 -d /path/to/archives/

# Recursive directory scan
go run ./cmd/ozaserve -a :8080 -d /path/to/archives/ -r

# Run as an MCP server alongside HTTP (recommended for AI assistants)
go run ./cmd/ozaserve -a :8080 -d ./archives/ --mcp

# Tune chunk cache size (entries; default 64)
go run ./cmd/ozaserve -a :8080 -c 256 archive.oza

# Suppress the /_info page
go run ./cmd/ozaserve -a :8080 --no-info archive.oza
```

### zim2oza

Convert ZIM files to OZA format. The default settings target compatibility; pass `--minify` for tighter HTML and adjust `--zstd-level` to trade size for build time.

```bash
go run ./cmd/zim2oza wikipedia.zim wikipedia.oza

# Verbose progress + JSON statistics
go run ./cmd/zim2oza --verbose wikipedia.zim wikipedia.oza
go run ./cmd/zim2oza --json-stats stats.json wikipedia.zim wikipedia.oza

# Dry run (analyze without writing)
go run ./cmd/zim2oza --dry-run wikipedia.zim

# Compression tuning
go run ./cmd/zim2oza --zstd-level 19 --minify wikipedia.zim wikipedia.oza
go run ./cmd/zim2oza --no-dict wikipedia.zim wikipedia.oza           # skip dictionary training
go run ./cmd/zim2oza --no-search wikipedia.zim wikipedia.oza         # skip trigram indices
go run ./cmd/zim2oza --no-optimize-images wikipedia.zim wikipedia.oza # skip JPEG re-encode

# Parallel compression workers (default: min(NumCPU, 4))
go run ./cmd/zim2oza --compress-workers 8 wikipedia.zim wikipedia.oza

# Chunk size in MB (default 4)
go run ./cmd/zim2oza --chunk-size 8 wikipedia.zim wikipedia.oza

# Dictionary training samples per group (default 1000)
go run ./cmd/zim2oza --dict-samples 2000 wikipedia.zim wikipedia.oza

# pprof CPU profile
go run ./cmd/zim2oza --profile convert.pprof wikipedia.zim wikipedia.oza
```

### ozakeygen

Generate Ed25519 keypairs for signing OZA archives. Pair with `ozaverify --signatures` for publisher authentication.

```bash
# Print private key PEM to stdout (also prints the public key hex on stderr)
go run ./cmd/ozakeygen

# Write private key to file
go run ./cmd/ozakeygen --out signer.key
```

Sign at write time via `ozawrite.WriterOptions.SigningKeys`:

```go
priv, _ := os.ReadFile("signer.key")  // PEM-decoded to ed25519.PrivateKey
w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
    SigningKeys: []ozawrite.SigningKey{{Key: priv, KeyID: 1}},
})
```

Verify at read time:

```bash
go run ./cmd/ozaverify --signatures --pubkey <hex-from-ozakeygen> archive.oza
```

### ozamcp

Standalone Model Context Protocol server exposing one or more OZA archives as MCP tools (`list_archives`, `search_text`, `read_entry`, `get_entry_info`, `browse_titles`, `get_random`, `get_archive_stats`). For an HTTP-backed alternative that pairs MCP with browseable URLs, use `ozaserve --mcp`. See [docs/OZAMCP.md](docs/OZAMCP.md) for Claude Desktop config.

```bash
# Serve a directory of archives over MCP stdio (default transport)
go run ./cmd/ozamcp -d /path/to/archives/

# Recursive scan, larger cache
go run ./cmd/ozamcp -d /path/to/archives/ -r -c 256
```

## API Overview

### Archive (Reader)

```go
oza.Open(path) (*Archive, error)
oza.OpenWithOptions(path, ...Option) (*Archive, error)

archive.EntryByPath("Main_Page") (Entry, error)
archive.EntryByTitle("Main Page") (Entry, error)
archive.EntryByID(0) (Entry, error)
archive.MainEntry() (Entry, error)
archive.Metadata("title") (string, error)
archive.Entries() iter.Seq[Entry]
archive.EntriesByTitle() iter.Seq[Entry]
archive.FrontArticles() iter.Seq[Entry]
archive.Search(query string, opts SearchOptions) ([]SearchResult, error)
archive.SearchTitles(query string, opts SearchOptions) ([]SearchResult, error)
archive.HasSearch() bool
archive.HasSignatures() bool
archive.Verify() error
archive.VerifyAll() ([]VerifyResult, error)
archive.VerifySignatures(trusted []ed25519.PublicKey) ([]SignatureVerifyResult, error)
```

### Entry

```go
entry.ID() uint32
entry.Path() string
entry.Title() string
entry.Size() uint32                  // content size without decompression
entry.IsRedirect() bool
entry.IsFrontArticle() bool
entry.MIMEType() string
entry.MIMEIndex() uint
entry.ReadContent() ([]byte, error)  // resolves redirects
entry.ContentReader() (io.Reader, error)
entry.Resolve() (Entry, error)       // follow redirect chain
```

### Options

```go
oza.WithMmap(false)                 // disable memory mapping
oza.WithCacheSize(32)               // chunk cache size (default: 8)
oza.WithVerifyOnOpen()              // verify section checksums on open
oza.WithMaxDecompressedSize(1<<30)  // reject decompressions above this (default: 1 GiB)
oza.WithMaxBlobSize(256<<20)        // reject blobs above this (default: 256 MiB)
oza.WithMaxMetadataValueSize(16<<20)// reject metadata values above this (default: 16 MiB)
```

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

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
go get github.com/stazelabs/oza
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

Full-text trigram search:

```bash
go run ./cmd/ozasearch archive.oza "quantum mechanics"
```

### ozaverify

Three-tier integrity verification:

```bash
# File-level SHA-256 check
go run ./cmd/ozaverify archive.oza

# Full verification (file + section + chunk)
go run ./cmd/ozaverify --all archive.oza
```

### ozaserve

Serve OZA files over HTTP:

```bash
go run ./cmd/ozaserve -a :8080 archive.oza
```

### ozamcp

Standalone MCP server for LLM agents (see [docs/OZAMCP.md](docs/OZAMCP.md)):

```bash
go run ./cmd/ozamcp archive.oza
```

### ozakeygen

Generate Ed25519 signing key pairs for archive signatures:

```bash
go run ./cmd/ozakeygen -o mykey
# Creates mykey.pub and mykey.key
```

### ozacmp

Compare a ZIM file and its OZA conversion side-by-side:

```bash
go run ./cmd/ozacmp source.zim converted.oza

# Markdown table output
go run ./cmd/ozacmp --format md source.zim converted.oza

# Deep per-entry comparison
go run ./cmd/ozacmp --deep source.zim converted.oza
```

### zim2oza

Convert ZIM files to OZA format:

```bash
go run ./cmd/zim2oza wikipedia.zim wikipedia.oza

# With verbose statistics
go run ./cmd/zim2oza --verbose wikipedia.zim wikipedia.oza

# Dry run (analyze without writing)
go run ./cmd/zim2oza --dry-run wikipedia.zim

# Control parallel compression (default: number of CPUs)
go run ./cmd/zim2oza --compress-workers 4 wikipedia.zim wikipedia.oza
```

### epub2oza

Convert EPUB books to OZA format:

```bash
# Single book
go run ./cmd/epub2oza book.epub book.oza

# Collection: bundle all EPUBs in a directory into one searchable archive
go run ./cmd/epub2oza --collection --title "My Library" ./epubs/ library.oza

# With verbose statistics and minification
go run ./cmd/epub2oza --verbose --minify book.epub book.oza
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
archive.Search("query", SearchOptions{}) ([]SearchResult, error)
archive.Verify() error
archive.VerifyAll() ([]VerifyResult, error)
```

### Entry

```go
entry.Path() string
entry.Title() string
entry.Size() uint32                  // content size without decompression
entry.IsRedirect() bool
entry.IsFrontArticle() bool
entry.MIMEType() string
entry.ReadContent() ([]byte, error)  // resolves redirects
entry.Resolve() (Entry, error)       // follow redirect chain
```

### Options

```go
oza.WithMmap(false)      // disable memory mapping
oza.WithCacheSize(32)    // chunk cache size (default: 8)
oza.WithVerifyOnOpen()   // verify section checksums on open
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

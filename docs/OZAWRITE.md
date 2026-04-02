# 王座 OZA -- ozawrite — OZA Archive Writer

The `ozawrite` package constructs OZA archive files. It implements a streaming
pipeline: content is transformed, deduplicated, compressed, and flushed to a
temporary file during `AddEntry()`. The final archive is assembled from metadata
and the temp file during `Close()`.

## Quick start

```go
f, _ := os.Create("output.oza")
w := ozawrite.NewWriter(f, ozawrite.WriterOptions{
    ZstdLevel:   19,
    TrainDict:   true,
    BuildSearch: true,
    MinifyHTML:  true,
})

w.SetMetadata("title", "My Archive")
w.SetMetadata("language", "eng")
w.SetMetadata("creator", "Example")
w.SetMetadata("date", "2026-03-08")
w.SetMetadata("source", "https://example.com")

id, _ := w.AddEntry("Main_Page", "Main Page", "text/html", htmlBytes, true)
w.AddRedirect("Home", "Home", id)

w.Close()
```

## Lifecycle

1. **NewWriter** — configures options, initialises internal state
2. **SetMetadata** — store key-value metadata (call before Close)
3. **AddEntry / AddRedirect** — populate content (streaming, repeatable)
4. **Close** — assemble the archive, write to the underlying writer

After `Close()` returns, the Writer is spent. Calling any method again returns
an error.

## WriterOptions

All boolean options default to their zero value (false) at the Go level. The
constructor applies sensible defaults for numeric fields only when the caller
passes zero.

| Option | Default | Description |
|--------|---------|-------------|
| ZstdLevel | 6 | Zstd compression level (1–22). Mapped to klauspost's 4 discrete speed levels: ≤1 → Fastest, ≤4 → Default, ≤8 → BetterCompression, ≥9 → BestCompression. |
| ChunkTargetSize | 4 MiB | Uncompressed byte threshold per chunk. Each MIME group has its own open chunk; once it reaches this size it is compressed and flushed. |
| TrainDict | true | Train per-MIME-group Zstd dictionaries from early entries. |
| DictSamples | 2000 | Maximum content samples collected per MIME group before training triggers. |
| BuildSearch | true | Convenience flag: enables both title and body trigram search. |
| BuildTitleSearch | true | Build a trigram index over front-article titles. |
| BuildBodySearch | true | Build a trigram index over front-article body content. |
| MinifyHTML | true | Minify `text/html` content (tdewolff/minify). |
| MinifyCSS | true | Minify `text/css` content. |
| MinifyJS | true | Minify `application/javascript` and `text/javascript`. |
| MinifySVG | true | Minify `image/svg+xml` content. |
| OptimizeImages | true | Lossless JPEG metadata stripping. |
| Progress | nil | Optional callback for progress reporting. |

## What happens during AddEntry

Each call to `AddEntry(path, title, mimeType, content, isFrontArticle)`
performs the following steps **before returning**:

1. **Transform** — minify HTML/CSS/JS/SVG if enabled; strip JPEG metadata if
   enabled. Errors are silent: the original content is kept.

2. **Hash** — SHA-256 of the transformed content. The first 8 bytes (as a
   little-endian uint64) are stored in the entry record for per-entry
   verification.

3. **Search index** — if `isFrontArticle` is true, the title and body are fed
   to the trigram builders. Non-front-article entries are never indexed.

4. **Dedup** — the hash is checked against previously seen content. If a match
   is found, the entry reuses the existing chunk/blob reference. No new data is
   written.

5. **Dictionary training buffer** — during the early phase (before dictionaries
   are trained), the entry and its content are buffered in memory. Training
   triggers once 2000 HTML samples are collected, or 4000 total entries are
   buffered, whichever comes first.

6. **Chunk assignment** — the content is appended to the open chunk for its
   MIME group. If the chunk exceeds `ChunkTargetSize`, it is compressed and
   flushed to a temporary file on disk.

After AddEntry returns, the content bytes are no longer referenced by the
Writer. The caller is free to reuse or discard the slice.

### Entry IDs

Content entry IDs are assigned sequentially starting from 0 in the order
entries are added via `AddEntry()`. Redirect IDs are assigned separately via
`AddRedirect()` and use a **tagged ID** scheme: bit 31 is set, and the lower
31 bits are the redirect's index in the redirect table. For example, the first
redirect gets ID `0x80000000`, the second `0x80000001`, etc.

Use `oza.IsRedirectID(id)` to distinguish content from redirect IDs, and
`oza.RedirectIndex(id)` to extract the redirect table index.

## What happens during Close

Close is a multi-phase pipeline:

1. **Train dictionaries** — if training didn't trigger during AddEntry (small
   archive), it happens now. All buffered entries are then flushed through the
   chunk pipeline.

2. **Flush remaining chunks** — any open chunks that haven't reached the target
   size are compressed and written to the temp file.

3. **Sort chunks** — chunk descriptors are sorted by ID. This is necessary
   because different MIME groups flush chunks at different times, so the flush
   order may not match creation order.

4. **Build MIME table** — collects all MIME types seen. The first three slots
   are always `text/html` (0), `text/css` (1), `application/javascript` (2),
   regardless of whether any entries use them.

5. **Build entry table** — serialises variable-length content entry records
   (~15 bytes each on average) in ID order with an offset table for O(1) access.
   Redirect entries are excluded from the entry table.

6. **Build redirect section** — serialises redirect records (5 bytes each:
   1-byte flags + 4-byte target ID) prefixed by a uint32 count. Redirect
   targets are always content entry IDs (bit 31 clear). Redirect chains are
   flattened so no multi-hop resolution is needed at read time.

7. **Build path/title indices** — sorts all entries (content and redirect) by
   path and title, produces front-coded indices with restart blocks every 64
   entries. Redirect entries are stored with tagged IDs (bit 31 set).

8. **Build search indices** — serialises the trigram posting lists accumulated
   during AddEntry. This is the most memory-intensive step for large archives.

9. **Stream content section** — the CONTENT section is written directly from
   the temp file to the output, computing its SHA-256 on the fly. This avoids
   materialising all compressed chunks in RAM.

10. **Write header** — a placeholder header is written first; after all sections
    are written, the Writer seeks back and overwrites it with final offsets.

11. **File checksum** — the Writer reads back all bytes before the checksum
    offset, computes SHA-256, and writes the 32-byte hash at the end.

12. **Cleanup** — the temp file is deleted.

## MIME groups

Content is grouped into chunks by MIME type for better compression. The groups
are:

| Group | MIME types |
|-------|-----------|
| html | `text/html` |
| css | `text/css` |
| js | `application/javascript`, `text/javascript` |
| svg | `image/svg+xml` |
| image | all other `image/*` |
| other | everything else |

MIME parameters (e.g., `; charset=utf-8`) are stripped before classification.

**Image chunks are trial-compressed** with Zstd at `SpeedFastest` (level 1); the
compressed version is kept only if smaller, otherwise stored as CompNone. Chunks
of many small images share header structure and typically compress 5-9%.
**SVG chunks are compressed with Zstd** because SVG is XML text and compresses
very well (typically 60-80% reduction). All other groups also use Zstd,
optionally with a trained dictionary.

## Content transforms

### Minification

Uses [tdewolff/minify](https://github.com/tdewolff/minify). Applied to
text/html, text/css, application/javascript, text/javascript, and image/svg+xml
when the corresponding option is enabled.

**Behaviour on error**: the original content is returned unchanged. No error is
reported.

**Caveat**: HTML minification removes whitespace that may be significant in
`<pre>` blocks. CSS/JS minification may break code relying on source-level
introspection.

### Image optimisation

Applied to `image/png` and `image/jpeg` (`image/jpg`) only. Other image types
pass through unchanged.

**PNG**: decoded via Go's `image/png`, re-encoded with `BestCompression`. This
strips all metadata chunks (tEXt, zTXt, iTXt, eXIf, iCCP, tIME). The
re-encoded version is only used if it's smaller than the original.

**JPEG**: byte-level marker stripping (no re-encoding). Removes APP0–APP15
(EXIF, JFIF, ICC profiles) and COM (comments). Preserves all markers required
for decoding: SOF, DHT, DAC, DQT, DRI, SOS. Returns the original on any parse
error.

## Deduplication

Content is deduplicated by SHA-256 hash of the transformed content. If two
entries produce identical content after minification/optimisation, only one copy
is stored. The second entry's metadata points to the same chunk and blob offset
as the first.

Deduplication is checked after transforms but before chunk assignment. During
the dictionary training buffer phase, dedup only catches entries whose identical
content was already flushed to a chunk — duplicates within the training buffer
are not caught until flush.

## Dictionary training

When `TrainDict` is enabled, the Writer collects content samples during
AddEntry. Training triggers when:

- 2000 HTML samples are collected, OR
- 4000 total entries are buffered (whichever comes first)

Dictionaries are trained per MIME group (html, css, js, svg, other). Raster
image content is excluded from sampling.

**Dictionary IDs**: html=1, css=2, js=3, other=4.

**Validation**: each trained dictionary is tested with a compress→decompress
round-trip on up to 5 samples. If any round-trip fails, the dictionary is
discarded and plain Zstd is used for that group.

**Panic recovery**: `zstd.BuildDict()` can panic on pathological inputs. This
is caught with `defer/recover`; the dictionary is silently discarded.

**Minimum history**: at least 128 KiB of concatenated sample data is required.
If samples are too small, no dictionary is trained.

## Search indexing

Trigram search indices are built incrementally during `AddEntry()` for entries
with `isFrontArticle=true`. Non-front-article entries are never indexed.

**Title index**: trigrams extracted from the entry title.

**Body index**: trigrams extracted from the title, path, and full content body.

ASCII bytes are lowercased; non-ASCII bytes pass through unchanged. Each
(trigram, entryID) pair is recorded at most once per IndexEntry call.

The indices are serialised during Close into a binary format with a sorted
trigram table and delta-encoded posting lists (LEB128 varints).

## Compression details

Zstd compression uses:

- Window size: 8 MiB
- Concurrency: 1 (single-threaded per chunk)
- Full literal entropy compression enabled

The `encoderCache` reuses `zstd.Encoder` instances across chunks with the same
(level, dictID) to avoid re-allocating internal encoder state.

Sections (entry table, path index, title index, search indices) are
independently compressed with Zstd at level 19. Sections smaller than 256
bytes are stored uncompressed. Sections where compression doesn't reduce size
are stored uncompressed.

## Already-compressed content

The following MIME types are stored without Zstd compression:

- `image/jpeg`, `image/png`, `image/webp`, `image/gif`, `image/avif`, `image/heic`
- `video/mp4`, `video/webm`, `video/ogg`
- `audio/mpeg`, `audio/ogg`, `audio/mp4`, `audio/webm`
- `font/woff`, `font/woff2`
- `application/zip`, `application/gzip`, `application/x-bzip2`, `application/x-zstd`, `application/zstd`

These are placed in "image" group chunks with `CompNone`.

## Temp file

During AddEntry, compressed chunks are written to a temporary file created with
`os.CreateTemp("", "ozawrite-chunks-*")`. This file is cleaned up automatically
when Close returns (including on error).

The temp file lives in the system's default temp directory (`$TMPDIR` or
`/tmp`). For very large archives, ensure this filesystem has enough space — the
temp file will be roughly the size of the compressed content section.

## Required metadata

`Close()` validates that the following metadata keys are set:

- `title`
- `language`
- `creator`
- `date`
- `source`

Missing keys cause Close to return an error.

## Progress callback

The `Progress` function is called with `(phase string, n int, total int)`:

| Phase | When | n | total |
|-------|------|---|-------|
| `dict-train` | Dictionary training starts | 0 | 1 |
| `dict-train` | Dictionary training complete | 1 | 1 |
| `compress` | Chunk flushed during AddEntry | chunk count | 0 (unknown) |
| `compress` | All chunks flushed (in Close) | final count | final count |
| `index-path` | Path/title index build starts | 0 | 1 |
| `index-path` | Path/title index build done | 1 | 1 |
| `index-search-title` | Title search index build starts | 0 | 1 |
| `index-search-title` | Title search index build done | 1 | 1 |
| `index-search-body` | Body search index build starts | 0 | 1 |
| `index-search-body` | Body search index build done | 1 | 1 |
| `assemble` | Final assembly starts | 0 | 1 |
| `assemble` | Final assembly done | 1 | 1 |

The `dict-train` and `compress` callbacks may fire during `AddEntry()`, not
just during `Close()`.

## Timings

After Close, `w.Timings()` returns per-phase durations:

- `ChunkBuild` — not currently populated (chunking is interleaved with AddEntry)
- `DictTrain` — not currently populated (training is interleaved with AddEntry)
- `Compress` — not currently populated (compression is interleaved with AddEntry)
- `SearchIndex` — time spent in trigram Build() during Close
- `Assemble` — time spent in Close from MIME table build through file write

## Thread safety

The Writer is **not** safe for concurrent use. All calls to AddEntry,
AddRedirect, SetMetadata, and Close must be serialised by the caller.

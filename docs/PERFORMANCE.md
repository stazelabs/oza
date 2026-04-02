# Performance Improvements

Based on benchmark analysis of 25 ZIM files (3.2 GB total, 18.3% average savings).

## Current Issues

**Two size regressions (both resolved):**
- ~~`se_codegolf` (+2.4%)~~ now -0.9% (fixed by #1 SVG carve-out + #2 zstd level 11)
- ~~`wp_en_chemistry_maxi` (+7.2%)~~ now -7.3% (fixed by #1 SVG carve-out)

**Speed bottleneck (resolved):**
- ~~xkcd: 157s~~ now 28s (5.6x faster, fixed by #4 parallel transcoding + #5 pipe I/O)
- ~~wp_en_chemistry_maxi: 124s~~ now 56s (2.2x faster)

---

## Tier 1 -- Fix Regressions

### 1. SVG carve-out from image MIME group -- DONE

Route `image/svg+xml` to a `"svg"` MIME group instead of `"image"` in `ozawrite/chunk.go` `mimeGroup()`. SVG chunks now get zstd compression + dictionary training instead of `CompNone`.

**Result:** wp_en_chemistry_maxi went from +7.2% regression to -7.3% improvement (-68 MB).

### 2. Raise zstd level for text-heavy profiles -- DONE

Set `ZstdLevel: 11` for `ProfileQAForum` and `ProfileDocs` in `cmd/internal/classify/profiles.go`. klauspost maps this to `SpeedBestCompression`.

**Result:** se_codegolf went from +2.4% regression to -0.9% improvement (-12.3 MB).

### 3. More aggressive search pruning for qa-forum

Lower `SearchPruneFreq` to 0.25 for qa-forum in `cmd/internal/classify/profiles.go` (prune trigrams appearing in >25% of docs instead of >50%). se_codegolf's SEARCH_BODY is 54 MB (13.5% of file).

Code content generates many unique trigrams that still survive 50% pruning. More aggressive pruning directly shrinks the dominant overhead section.

**Impact:** MEDIUM-HIGH size | **Effort:** Low

---

## Tier 2 -- Speed Wins

### 4. Parallel image transcoding -- DONE

Pre-transcode images in a goroutine worker pool (up to 8 workers) in `cmd/zim2oza/convert.go` `addEntriesParallel()`, with reorder buffer to maintain sequential `AddEntry` calls.

**Result:** xkcd transform phase 153.7s -> 2.2s (69x faster). Total: 157s -> 28s (5.6x).

### 5. Pipe-based transcoding (eliminate temp files) -- DONE

`cwebp` uses stdin/stdout pipes (`runToolPipe`). `gif2webp` uses temp file for input, stdout for output (`runToolFile`). Eliminates most temp file I/O.

### 6. Skip transcoding for tiny images -- DONE

Early return in `ozawrite/transcode.go` `Transcode()` for PNGs < 2 KB and GIFs < 1 KB (constants `minPNGTranscodeSize`, `minGIFTranscodeSize`).

---

## Tier 3 -- Additional Size Wins

### 7. Try-compress image chunks with zstd ✅

**Implemented** in `ozawrite/compress.go` `compressionWorker()`. Image chunks are
trial-compressed at `SpeedFastest` (level 1); the compressed version is kept only
if it is smaller than the raw data. Falls back to `CompNone` when compression
doesn't help (e.g. a chunk containing a single large JPEG).

Observed results: chunks of many small JPEG/WebP images share header structure
and compress ~5-9% at SpeedFastest. On a 2-book EPUB collection the image
content section shrank from 589 KiB to 536 KiB (9%), flipping the overall
archive from 1.06x (larger than input) to 0.98x (smaller).

### 8. Search index size budget

After building the trigram index in `ozawrite/search.go` `Build()`, if the serialized body index exceeds N% of total content size (e.g., 10%), iteratively increase prune frequency and re-serialize until it fits. The in-memory maps make re-pruning cheap.

Generalizes fix #3 to all profiles. Prevents search index from ever dominating file size.

**Impact:** MEDIUM | **Effort:** Medium

---

## Tier 4 -- Experimental

### 9. JPEG→WebP lossy transcoding ✅

**Implemented** in `ozawrite/transcode.go`. Opt-in via `--transcode-lossy-jpeg`.
Uses `cwebp -q 80 -m 4` via stdin/stdout pipe. Keeps original if WebP output is
larger. Minimum size threshold: 1 KB.

On the 2-book EPUB test corpus (small Gutenberg cover JPEGs), lossy transcoding
produced slightly larger output due to WebP container overhead exceeding the
compression gain. Photo-heavy encyclopedias with large JPEGs would benefit
significantly (~25-35% savings per image).

Install: `brew install webp` (macOS) / `apt install webp` (Ubuntu)

### 10. Brotli as alternative compression codec ✅

**Implemented** in `ozawrite/compress.go` and `oza/compress.go`. `CompBrotli = 3`
added to `oza/constants.go`. For non-dict text chunks, the compression worker
trial-compresses with both Zstd and Brotli, keeping whichever is smaller.
Brotli quality is mapped from the Zstd level (e.g. zstd 6 → brotli 6).

Reader decompression uses `github.com/andybalholm/brotli` (pure Go).
FORMAT.md updated: compression field values are now `0=none, 1=zstd, 2=zstd+dict, 3=brotli`.

On the 2-book EPUB test corpus: baseline 635 KiB → 625 KiB with Brotli trial
(1.6% improvement, Brotli winning on some text chunks).

### 11. AVIF transcoding for images ✅

**Implemented** in `ozawrite/transcode.go`. Opt-in via `--transcode-avif`.
Discovered at startup alongside gif2webp/cwebp. Uses `avifenc` with temp file
input and stdout capture. PNG: `avifenc -s 6 --lossless`. JPEG: `avifenc -s 6 -q 70`.
Falls back to WebP if AVIF output is larger or avifenc is unavailable.

Install: `brew install libavif` (macOS) / `apt install libavif-bin` (Ubuntu)

Encoding is slower than WebP (~3-10x) but leverages the existing parallel
transcoding worker pool (#4). AVIF achieves 20-50% smaller files than WebP on
photographic content.

### 12. Faster dedup hash (xxhash instead of SHA-256) ✅

**Implemented** in `ozawrite/dedup.go`, `ozawrite/writer.go`, `oza/checksum.go`.
Replaced `sha256.Sum256` with `xxhash.Sum64` (`github.com/cespare/xxhash/v2`)
for both dedup map lookups and the 8-byte content hash stored in entry records.
File-level and section-level integrity checks remain SHA-256.

xxhash is 5-10x faster than SHA-256 for the same data. The on-disk content hash
was already truncated to 8 bytes (uint64), so switching to xxhash is a pure
implementation improvement with no format change.

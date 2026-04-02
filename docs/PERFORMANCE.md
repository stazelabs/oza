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

### 7. Try-compress image chunks with zstd

Trial-compress image chunks at `SpeedFastest` in `ozawrite/compress.go` `compressionWorker()`; keep the compressed version only if it is smaller. The same pattern is already used in `compressRawSection` (assembly.go).

Chunks of many small WebP/JPEG images share header structure. Even 1-3% savings on 340 MB of images is meaningful.

**Impact:** LOW-MEDIUM size | **Effort:** Low

### 8. Search index size budget

After building the trigram index in `ozawrite/search.go` `Build()`, if the serialized body index exceeds N% of total content size (e.g., 10%), iteratively increase prune frequency and re-serialize until it fits. The in-memory maps make re-pruning cheap.

Generalizes fix #3 to all profiles. Prevents search index from ever dominating file size.

**Impact:** MEDIUM | **Effort:** Medium

---

## Tier 4 -- Experimental

### 9. JPEG -> WebP lossy transcoding

Add `cwebp -q 80` for JPEG input in `ozawrite/transcode.go`. Gate behind a `--transcode-lossy-jpeg` flag (opt-in since it's lossy).

JPEG->WebP lossy at q80 saves 25-35% with no visible quality loss. xkcd has 173 JPEGs (13 MB), but photo-heavy encyclopedias would benefit much more.

**Impact:** MEDIUM-HIGH size (for photo archives) | **Effort:** Low

### 10. Brotli as alternative compression codec

Add `CompBrotli = 3` in `oza/constants.go`. Trial-compress text chunks with both zstd and brotli, keep whichever is smaller. Pure Go: `github.com/andybalholm/brotli`.

Brotli achieves 10-15% better compression than zstd on text at comparable decode speed. Could definitively beat ZIM's LZMA ratios.

**Impact:** MEDIUM size | **Effort:** Medium (format change required in reader+writer+FORMAT.md)

### 11. AVIF transcoding for images

Add `avifenc` support in `ozawrite/transcode.go`. PNG: `avifenc --lossless`. JPEG: `avifenc -q 70`. Gate behind `--transcode-avif` flag.

AVIF achieves 20-50% smaller files than WebP. Encoding is 10-100x slower, so requires parallel transcoding (#4) first.

**Impact:** HIGH size | **Effort:** Medium | **Prerequisite:** #4

### 12. Faster dedup hash (xxhash instead of SHA-256)

Replace `sha256.Sum256` with `xxhash.Sum64` for dedup lookups in `ozawrite/writer.go`. The on-disk hash is already truncated to 8 bytes, so this is purely an implementation detail.

xxhash is 5-10x faster. Helps archives with 200K+ entries (wp_simple_mini: 392K entries).

**Impact:** LOW speed | **Effort:** Low

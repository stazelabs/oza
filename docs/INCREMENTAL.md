# Incremental / Append Mode for OZA

## Context

The Writer currently must produce a complete archive in one shot. For large corpus updates (e.g., Wikipedia monthly dumps where 95% of articles are unchanged), this means hours of reprocessing and recompressing content that hasn't changed. The goal is to eliminate that waste.

FORMAT.md explicitly states "OZA files are immutable once created" and "OZA is a distribution format, not a database." Any solution must respect this philosophy.

## Approach: Optimized Rebuild with Chunk-Level Copy

After evaluating four approaches (delta files, in-place append, optimized rebuild, multi-file overlay), **optimized rebuild** is the clear winner:

- **Zero reader changes** -- output is a standard OZA file
- **All format invariants preserved** -- immutability, three-tier integrity, signatures
- **Crash-safe** -- old file is never modified
- **Composes cleanly** with split archives and future signatures
- The other approaches either break file integrity (in-place append), require large reader changes (delta/overlay), or strain the "distribution format" philosophy

The key optimization: copy compressed chunks byte-for-byte from the old archive into the new one, skipping decompression and recompression for unchanged content.

**Trade-off:** Requires 2x disk during the update (old + new file coexist). Acceptable for a build-server tool that runs monthly.

## Signatures: Clean Story

- Old signed archive remains valid and verifiable
- New archive gets its own fresh signature
- No concept of "appending to a signed file" -- each release is independently signed
- Signing keys can rotate between releases
- No special interaction needed

## Split Archives: Orthogonal

- Output is a normal OZA file that can be split afterward
- `CopyChunk` works regardless of whether the source is split or monolithic
- No special interaction between incremental updates and split archives

## Implementation Phases

### Phase 1: Reader API for raw chunk access (`oza/`)

Add read-only accessors to `Archive` that expose what the writer needs:

| Method | Purpose |
|--------|---------|
| `ChunkCount() int` | Number of chunks in CONTENT section |
| `ChunkRawData(chunkID uint32) ([]byte, ChunkDesc, error)` | Compressed bytes + descriptor, no decompression |
| `DictByID(id uint32) ([]byte, bool)` | Raw dictionary bytes by ID |
| Export `contentEntryRecord()` | Already exists, just unexported |

**Files:** `oza/archive.go`, `oza/chunk.go`

### Phase 2: Writer `AddFromArchive` + `CopyChunk` (`ozawrite/`)

New methods on `Writer`:

```go
// CopyChunk copies a compressed chunk byte-for-byte from src, returns new chunk ID.
func (w *Writer) CopyChunk(src *oza.Archive, chunkID uint32) (uint32, error)

// AddFromArchive copies an entry from src, referencing a previously-copied chunk.
func (w *Writer) AddFromArchive(src *oza.Archive, entryID uint32) (uint32, error)
```

Internal state additions:
- `copiedChunks map[sourceArchive]map[uint32]uint32` -- source chunk ID → new chunk ID
- `copiedDicts map[uint32]uint32` -- source dict ID → new dict ID

**Chunk granularity:** A chunk contains multiple entries. If *any* entry in a chunk changed, the whole chunk must be rebuilt (decompress, replace entries, recompress). Unchanged chunks are copied as-is.

**Search indexing:** Copied front-article entries still need their content decompressed for trigram indexing (title + body text). This is the remaining per-entry cost, but only for front articles with search enabled.

**Files:** `ozawrite/writer.go`, `ozawrite/chunk.go`

### Phase 3: `ozaupdate` CLI tool (`cmd/ozaupdate/`)

```
ozaupdate --base old.oza --changes ./updates/ --output new.oza
```

Workflow:
1. Open base archive, scan path index
2. Compare against update manifest (new/modified/deleted paths)
3. Identify unchanged chunks (all entries present and unmodified)
4. For unchanged chunks → `CopyChunk` + `AddFromArchive` per entry
5. For partially-changed chunks → decompress, `AddEntry` per entry (new content for modified, skip deleted)
6. For new entries → `AddEntry` normally
7. `Close()` rebuilds all indexes and writes a clean OZA file

### Phase 4 (future): Body search optimization

Rebuilding the body trigram index is the remaining expensive operation (7+ GB RAM for Wikipedia). Two mitigations already in backlog:
- `--search=title-only` flag to skip body index
- Deferred body search: build from the newly written file in a second pass (avoids holding all trigrams in RAM during the main write)

## Remaining Cost Breakdown (95% unchanged Wikipedia)

| Operation | Full rebuild | With chunk copy |
|-----------|-------------|-----------------|
| Decompress old content | ~30 min | ~2 min (5% only) |
| Transforms (minify, etc.) | ~20 min | ~1 min (5% only) |
| Compress content | ~60 min | ~3 min (5% only) |
| Copy unchanged chunks | N/A | ~5 min (byte copy) |
| Rebuild indexes | ~10 min | ~10 min (same) |
| Rebuild body search | ~15 min | ~15 min (same) |
| **Total** | **~2-3 hours** | **~30-35 min** |

## Verification

- Write unit tests for `CopyChunk` and `AddFromArchive` using small synthetic archives
- Round-trip test: create archive A, create archive B via `AddFromArchive` from A + new entries, verify B with `ozaverify --all`
- Compare byte-for-byte: archive built from scratch vs. archive built incrementally should produce identical content (though offsets may differ due to chunk ordering)
- Run `ozainfo` on the output to verify section structure
- Benchmark: measure time savings on a medium-sized archive (~10K entries)

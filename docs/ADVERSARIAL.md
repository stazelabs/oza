# Adversarial Archive Corpus

Crafted-bad `.oza` files for testing parser robustness. Each "recipe" builds a
valid archive with `ozawrite.Writer`, then applies a targeted corruption that
exercises a specific failure mode end-to-end.

## Design Principles

1. **Build on the writer** — valid archives come from `ozawrite.Writer`, so they
   always reflect the current format version. No hand-assembled byte arrays.

2. **Parse to find targets** — recipes call `oza.ParseHeader` /
   `oza.ParseSectionDesc` to locate mutation points. When the format changes the
   parsers change, and recipes adapt automatically.

3. **Corrupt at the semantic level** — "set SectionCount to N" rather than
   "write 0xFF at byte 24". Recipes describe *what* to break, not *where*.

4. **Assert on error types** — check functions test for sentinel errors
   (`ErrChecksumMismatch`, `ErrInvalidMagic`, …), not message strings.

5. **Builders isolate config** — `buildMinimal`, `buildWithRedirects`,
   `buildWithSearch`, `buildWithSignatures` encapsulate archive configs.

## Running

```bash
# All adversarial tests
go test ./oza/ -run TestAdversarialArchives -v

# Single recipe
go test ./oza/ -run TestAdversarialArchives/D1_SelfRedirect -v

# With race detector
go test ./oza/ -run TestAdversarialArchives -race
```

## Recipe Catalog

### P. Header Edge Cases

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| P1 | `InvalidMagic` | Replace magic bytes with `0xDEADBEEF` | `ErrInvalidMagic` |
| P2 | `FutureVersion` | Set `MajorVersion` to 99 | `ErrUnsupportedVersion` |
| P3 | `NonZeroReserved` | Fill reserved bytes `[68:128]` with `0xFF` | Open succeeds, warnings emitted |

### A. Structural Truncation

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| A1 | `TruncatedHeader` | Cut file at byte 32 (half the 128-byte header) | Open fails |
| A2 | `TruncatedSectionTable` | Cut in the middle of the section table | Open fails |
| A3 | `TruncatedContentSection` | Cut halfway through the content section | Open fails |
| A4 | `TruncatedChecksum` | Cut halfway through the 32-byte checksum | Verify fails |

### B. Out-of-Bounds Offsets

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| B1 | `SectionOffsetPastEOF` | Content section offset → past EOF | Open fails |
| B2 | `SectionTableOffPastEOF` | Header `SectionTableOff` → past EOF | Open fails |
| B3 | `ChecksumOffPastEOF` | Header `ChecksumOff` → past EOF | Verify fails |
| B4 | `ChunkOffsetPastContent` | Chunk descriptor `CompressedOff` → huge | ReadContent fails |
| B5 | `EntryBlobPastChunk` | Chunk descriptor `CompressedSize` → 1 | ReadContent fails |

### C. Decompression Attacks

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| C2 | `InvalidCompressionType` | Set section compression byte to `0xFF` | Open fails |

### D. Redirect Attacks

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| D1 | `SelfRedirect` | Redirect 0 target → its own tagged ID | `ErrRedirectLoop` |
| D2 | `RedirectCycle` | Redirect 0 → redirect 1, redirect 1 → redirect 0 | `ErrRedirectLoop` |
| D3 | `RedirectToNonexistent` | Redirect target → `0x7FFFFF` | Resolve fails |

### E. Checksum Corruption

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| E1 | `FileChecksumFlipped` | XOR file checksum byte | `ErrChecksumMismatch` |
| E2 | `SectionChecksumFlipped` | XOR section descriptor SHA-256 byte | VerifyAll fails |
| E3 | `ContentByteFlipped` | XOR byte in content section data | VerifyAll detects |

### G. Section-Level Confusion

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| G1 | `SectionTypeSwap` | Swap metadata ↔ MIME table type fields | Open fails |
| G2 | `DuplicateSectionType` | Two descriptors with same type | MIME table missing |
| G3 | `OverlappingSections` | Two sections share same offset | Data garbled, VerifyAll fails |

### H. Count Overflow

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| H1 | `MassiveSectionCount` | `SectionCount` exceeds file / `SectionSize` | Open fails |
| H2 | `MassiveChunkCount` | Chunk count exceeds file / `ChunkDescSize` | Open fails |

### I. MIME Table

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| I1 | `MIMEConventionViolation` | Index 0 → "text/plai" instead of "text/html" | Open fails |
| I2 | `MIMETableTruncated` | Section `CompressedSize` → 1 | Open fails |

### J. Index Corruption

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| J1 | `IndexZeroRestartInterval` | IDX1 `restart_interval` → 0 | ParseIndex fails |
| J2 | `IndexBadMagic` | Overwrite IDX1 magic | ParseIndex fails |

### K. Search/Trigram Corruption

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| K1 | `TrigramCorruptBitmap` | XOR 16 bytes in posting list region | Search returns nil (panic recovery) |
| K2 | `TrigramBadVersion` | Version field → 99 | Open/Search fails |

### L. Chunk Table

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| L1 | `ChunkTableUnsorted` | Swap two chunk descriptors | `ErrChunkTableUnsorted` |
| L2 | `ZeroLengthChunk` | Chunk `CompressedSize` → 0 | ReadContent fails |

### M. Signature Corruption

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| M1 | `SignatureTampered` | Flip bit in Ed25519 signature | VerifySignatures reports invalid |

### N. Metadata

| ID | Name | Mutation | Expected |
|----|------|----------|----------|
| N1 | `MetadataDuplicateKeys` | Inflate `pair_count` beyond data | Open fails or gracefully handles |

## Adding a New Recipe

1. Pick a builder (`buildMinimal`, `buildWithRedirects`, etc.) or write one.
2. Write a `Corrupt` function that:
   - Calls `parseHdr` / `findSection` to locate the target semantically.
   - Mutates a copy of the valid archive bytes.
3. Write a `Check` function using `mustFailOpen`, `mustFailVerify`, or custom logic.
4. Append to `allRecipes()` in `oza/badoza_test.go`.
5. Add a row to the table above.

## File

All recipes live in [`oza/badoza_test.go`](../oza/badoza_test.go).

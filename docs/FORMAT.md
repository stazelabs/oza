# 王座 -- OZA: Open Zipped Archive -- Format Specification

*This is the in-repo copy of the OZA format specification.*

*Draft 0.1 -- 2026-03-06*

File extension: `.oza`

---

## 1. Why Redesign ZIM?

The ZIM format has served the offline content community since 2007. Billions of articles
have been distributed in ZIM files. But after nearly two decades, its design shows its age
in ways that incremental patches cannot fix.

### The Header Problem

ZIM's 80-byte header is frozen. No extensibility field, no section table, no way to add
capabilities without breaking readers. Fields like `paramLen` (always zero in every ZIM
ever created), `revision` (no defined semantics), and `layoutPage` (deprecated) occupy
permanent real estate. When the format needed full-text search, the only option was to
stuff Xapian databases into the `X` namespace as opaque binary blobs -- a hack that became
load-bearing infrastructure.

### The Namespace Trap

A single byte (`C`, `M`, `W`, `X`) conflates organizational category with entry type.
Content entries and metadata entries are distinguished only by namespace letter, not by an
explicit type field. The MIME index value `0xFFFF` is overloaded to mean "redirect" --
a type indicator smuggled into a content-type field. Adding a new category of entry means
burning a namespace byte from a pool of 256, with no registry and no hierarchy.

### Broken Integrity

One MD5 hash covers the entire file. MD5 has been cryptographically broken since 2004.
There are no per-cluster or per-entry checksums. If a single byte is corrupted in a 90 GB
Wikipedia archive, the only diagnostic is "checksum failed" -- no indication of where.
No signatures. No authentication of publisher identity.

### The Xapian Problem

Full-text search in ZIM means `X/fulltext/xapian` -- a serialized Xapian C++ database
with no formal binary specification. It is defined solely by ~150,000 lines of C++ source
code. The on-disk format changes between Xapian versions (Glass, Honey) without any
version indicator in the ZIM. Any non-C++ reader must either link `libxapian` via FFI or
reverse-engineer an undocumented binary format. This is the single biggest barrier to
implementing ZIM readers in any language other than C++.

### Missing Content Length

ZIM directory entries do not store blob size. To answer "how big is this article?" you must
decompress the entire cluster (which can be hundreds of megabytes) and read the blob offset
table. An HTTP server cannot set `Content-Length` without decompression. Range requests are
impossible without knowing content boundaries.

### Chrome Entanglement

ZIM HTML produced by mwoffliner assumes it will be served by kiwix-serve, which injects a
Vue.js application shell at runtime. The raw HTML contains dead `Special:Search` links,
invisible Codex components that require MediaWiki's ResourceLoader JS, and root-relative
URLs like `/wiki/` that only work under a specific mount point. Navigation, search UI, and
article content are mixed in the `C` namespace with no formal separation. A third-party
reader must either replicate Kiwix's entire app shell or serve broken HTML.

### Compression Baggage

Four compression formats: none, zlib (deprecated), bzip2 (deprecated), XZ/LZMA, and Zstd.
Readers must carry code for all of them. XZ decompresses 5-10x slower than Zstd at
comparable ratios. There is no dictionary support despite Zstd being designed around it.
Compression is per-cluster with no option to skip already-compressed blobs (JPEG, PNG,
WebP). A cluster mixing text and images compresses everything together, wasting CPU on
incompressible data.

### Pointer Indirection

Looking up an entry by title requires three I/O operations: read the title pointer list
to get an entry index, read the URL pointer list to get a file offset, then read the
directory entry at that offset. This was designed for sequential media; on modern SSDs,
the indirection just adds latency.

---

## 2. Design Goals

1. **Self-describing and extensible.** A section table where unknown sections are
   skippable. New capabilities never require format hacks.

2. **Content-addressed storage.** Deduplicate identical content using cryptographic
   hashes. A CSS file shared by 6 million Wikipedia articles is stored once.

3. **Strong integrity.** SHA-256 at file, section, and chunk levels. Corruption is
   localizable. Optional Ed25519 signatures for publisher authentication.

4. **Built-in search.** A trigram index that any language can implement in a few hundred
   lines. No dependency on any external search engine library.

5. **Clean separation of concerns.** Content, metadata, chrome/UI, and search are
   distinct sections. A reader that only wants articles never touches search data.

6. **Streaming-friendly.** The table of contents is at the start. Sections are
   independently addressable. HTTP range requests work without decompressing unrelated
   data.

7. **Modern compression.** Zstd with dictionary support. No legacy formats.

8. **Simple to implement.** A read-only parser in any language in under a week. The
   spec fits in one document with test vectors.

### Non-Goals

- **Write path optimization.** OZA is a distribution format, not a database. Writers are
  tools; the spec optimizes for readers.
- **Incremental updates.** OZA files are immutable once created.
- **DRM.** Content is open.
- **Locale-aware sorting.** Title sort uses UTF-8 binary order. Locale-aware collation
  is a UI concern. The search index handles fuzzy matching.
- **Backward compatibility with ZIM.** Clean break. Converters bridge the gap.

---

## 3. File Format Specification

### 3.1 Layout Overview

```
+-------------------+
| File Header       |  64 bytes fixed
+-------------------+
| Section Table     |  Array of section descriptors (80 bytes each)
+-------------------+
| Metadata          |  Structured key-value pairs
+-------------------+
| MIME Table        |  Deduplicated MIME type strings
+-------------------+
| Entry Table       |  Variable-length content entry records + offset table
+-------------------+
| Redirect Table    |  Compact 5-byte redirect records
+-------------------+
| Path Index        |  Sorted paths + offset table for binary search
+-------------------+
| Title Index       |  Sorted titles + offset table for binary search
+-------------------+
| Content           |  Compressed content chunks
+-------------------+
| Search (Title)    |  Trigram index of titles (optional)
+-------------------+
| Search (Body)     |  Trigram index of content (optional)
+-------------------+
| Chrome            |  UI assets (optional)
+-------------------+
| Zstd Dictionaries |  Shared compression dictionaries (optional)
+-------------------+
| Signatures        |  Ed25519 signatures (optional)
+-------------------+
| File Checksum     |  32-byte SHA-256
+-------------------+
```

All integers are **little-endian**. All strings are **UTF-8, NFC-normalized**.

### 3.2 File Header (64 bytes)

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0 | 4 | `magic` | `0x01415A4F` ("OZA\x01" on disk, little-endian) |
| 4 | 2 | `major_version` | 1 |
| 6 | 2 | `minor_version` | 0 |
| 8 | 16 | `uuid` | Random UUID v4 |
| 24 | 4 | `section_count` | Number of sections |
| 28 | 4 | `entry_count` | Content entries (excludes redirects) |
| 32 | 8 | `content_size` | Total uncompressed content bytes |
| 40 | 8 | `section_table_offset` | Offset to section table |
| 48 | 8 | `checksum_offset` | Offset to trailing SHA-256 |
| 56 | 4 | `flags` | Bit flags (see below) |
| 60 | 4 | `reserved` | Must be zero |

**Flags:**

| Bit | Name | Meaning |
|-----|------|---------|
| 0 | `has_search` | Search section present |
| 1 | `has_chrome` | Chrome section present |
| 2 | `has_signatures` | Signature section present |
| 3-31 | -- | Reserved (must be zero; readers ignore unknown flags) |

### 3.3 Section Table

Each section descriptor is **80 bytes**:

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0 | 4 | `section_type` | Enum (see below) |
| 4 | 4 | `flags` | Section-specific flags |
| 8 | 8 | `offset` | Absolute file offset |
| 16 | 8 | `compressed_size` | On-disk size |
| 24 | 8 | `uncompressed_size` | Decompressed size |
| 32 | 1 | `compression` | 0=none, 1=zstd, 2=zstd+dict |
| 33 | 3 | `reserved` | Must be zero |
| 36 | 4 | `dict_id` | Dictionary ID (0 if none) |
| 40 | 32 | `sha256` | SHA-256 of compressed section bytes |
| 72 | 8 | `reserved2` | Must be zero |

**Section types:**

| Value | Name | Description |
|-------|------|-------------|
| 0x0001 | METADATA | Structured key-value pairs |
| 0x0002 | MIME_TABLE | MIME type string table |
| 0x0003 | ENTRY_TABLE | Variable-length entry records + offset table |
| 0x0004 | PATH_INDEX | Path lookup index |
| 0x0005 | TITLE_INDEX | Title lookup index |
| 0x0006 | CONTENT | Content chunks |
| 0x0007 | REDIRECT_TABLE | Redirect mappings |
| 0x0009 | CHROME | UI/navigation assets |
| 0x000A | SIGNATURES | Cryptographic signatures |
| 0x000B | ZSTD_DICT | Shared Zstd dictionaries |
| 0x000C | SEARCH_TITLE | Trigram index of front-article titles |
| 0x000D | SEARCH_BODY | Trigram index of front-article body content |
| 0x0100+ | -- | Reserved for extensions |

**A reader that encounters an unknown section type skips it** using
`offset + compressed_size`. This is the entire extensibility mechanism -- no TLV nesting,
no protobuf. Just a flat table with self-describing entries.

### 3.4 Metadata Section

Length-prefixed key-value pairs. No JSON, no XML.

```
4 bytes: pair_count

Per pair:
  2 bytes: key_length
  key_length bytes: key (UTF-8)
  4 bytes: value_length
  value_length bytes: value (UTF-8 or raw bytes)
```

**Required keys:** `title`, `language` (BCP-47), `creator`, `date` (ISO 8601), `source`.

**Optional well-known keys:** `description`, `long_description`, `license` (SPDX),
`favicon_entry` (uint32 entry ID), `main_entry` (uint32 entry ID), `article_count`,
`scraper` (tool name + version).

### 3.5 MIME Table

```
2 bytes: count

Per type:
  2 bytes: string_length
  string_length bytes: MIME type string
```

Index 0 is always `text/html`. Index 1 is always `text/css`. Index 2 is always
`application/javascript`. Fixed by convention for fast checks without string comparison.

The value `0xFFFF` is **not** used for redirects. MIME indices are purely MIME indices.
Redirects are a separate entry type.

### 3.6 Entry Table

Variable-length entry records with an offset table for O(1) random access. Content
entries only — redirects are stored separately in the Redirect Table (§3.11). Paths and
titles live in the index sections.

```
Section layout:

  uint32  entry_count           Number of content entries
  uint32  record_data_offset    Byte offset from section start to first record
                                (= 8 + entry_count * 4)

  uint32[entry_count]           Offset table: byte offset of each record
                                relative to record_data_offset

  Per record (variable length, ~15 bytes average):
    uint8   type_and_flags      Bits 0-3: entry_type (0=content, 2=metadata_ref)
                                Bits 4-7: flags (bit 4 = is_front_article)
    uvarint mime_index          Index into MIME table
    uvarint chunk_id            Content chunk ID
    uvarint blob_offset         Byte offset within decompressed chunk
    uvarint blob_size           Decompressed content size in bytes
    uint64  content_hash        Truncated SHA-256 (first 8 bytes, LE)
```

Entry ID is implicit: the index into the offset table. Uvarints use unsigned LEB128
encoding (same as Go `encoding/binary.PutUvarint`).

Key properties:

- **O(1) access:** Entry N is at `record_data[offset_table[N]]`. One extra indirection
  vs fixed-size records, but the offset table stays cache-hot.
- **~60% smaller** than fixed 40-byte records. Average record is ~15 bytes + 4 bytes
  offset table entry = ~19 bytes/entry.
- **`content_hash`** is fixed 8 bytes (not varint) because hash values are uniformly
  distributed — varint encoding would be worse.
- **`blob_size` is in the entry.** HTTP `Content-Length` without decompression.
- **`is_front_article`** replaces namespace-based heuristics for "is this user-visible?"

### 3.6a Redirect Table

Redirects are stored in a dedicated `REDIRECT_TABLE` section (0x0007) using a compact
**5-byte** record format — far more efficient than the 40-byte entry records they
previously occupied.

```
4 bytes: count (uint32)

Per redirect record (5 bytes):
  1 byte:  flags      (bit 0: is_front_article)
  4 bytes: target_id  (uint32, content entry ID — bit 31 always clear)
```

**Tagged ID convention:** Entry IDs use bit 31 to distinguish content from redirect
entries:

- Bit 31 clear → content entry ID, indexes into the entry table
- Bit 31 set → redirect index (`id & 0x7FFFFFFF` indexes into the redirect table)

Path and title indices store tagged IDs, so lookups transparently dispatch to the
correct table. Redirect targets are always content entry IDs (chains are flattened at
write time).

**Capacity constraint (v1 format invariant).** Because bit 31 is permanently reserved
as a type tag, each namespace is capped at 2,147,483,647 entries (2³¹ − 1):

| Namespace | Maximum |
|-----------|---------|
| Content entries | 2,147,483,647 |
| Redirect entries | 2,147,483,647 |

This exceeds any foreseeable archive size (Wikipedia ~6 M articles as of 2026) and is
not expected to be a practical constraint. However, implementations **must** reject
archives whose `entry_count` field or redirect table `count` field exceeds this limit,
and writers **must** return an error rather than silently wrap the ID space.

At Wikipedia scale (~10 M redirects), this saves ~350 MB compared to storing redirects
as 40-byte entry records.

### 3.7 Path Index

Front-coded index sorted by path for binary search. Uses the IDX1 format with
restart blocks every 64 entries for efficient random access.

```
Header (16 + restart_count * 4 bytes):
  4 bytes: magic (0x49445831 = "IDX1" little-endian)
  4 bytes: count (total number of entries)
  4 bytes: restart_interval (64)
  4 bytes: restart_count
  restart_count * 4 bytes: restart_offsets (byte offset from section start)

Records (front-coded within restart blocks):

  Restart record (first in each block of 64):
    4 bytes: entry_id
    2 bytes: key_length
    key_length bytes: full key (UTF-8, NFC)

  Non-restart record:
    4 bytes: entry_id
    2 bytes: prefix_length (bytes shared with previous key)
    2 bytes: suffix_length
    suffix_length bytes: suffix bytes
```

**No namespaces.** Paths are flat: `Main_Page`, `_res/style.css`, `_meta/Title`.
Content organization is by convention (path prefix), not by format-level namespace.

Binary search uses restart offsets for O(1) block access, then linear scan within the
block. Overall lookup is O(log(count / 64) + 64) string comparisons.

### 3.8 Title Index

Same IDX1 format as the path index, but sorted by title. Entries without an explicit
title use their path. The writer populates this at creation time.

### 3.9 Content Section

```
Chunk Table (at section start):
  4 bytes: chunk_count

  Per chunk descriptor (28 bytes):
    4 bytes:  chunk_id          (uint32)
    8 bytes:  compressed_offset (uint64, byte offset from start of chunk data area)
    8 bytes:  compressed_size   (uint64)
    4 bytes:  dict_id           (uint32, 0 if not dict-compressed)
    1 byte:   compression       (0=none, 1=zstd, 2=zstd+dict)
    3 bytes:  reserved

[Compressed chunk data follows immediately after the chunk table]
```

Each chunk is independently compressed. Each chunk has its own compression type — HTML
chunks use Zstd level 19, image-only chunks store uncompressed.

Chunk descriptors are sorted by `chunk_id`. Entry records reference chunks by ID;
`compressed_offset` is relative to the start of the chunk data area (immediately after
the chunk table).

**Chunk sizing guidance for writers:**
- Group entries by MIME type (HTML with HTML, images with images)
- Target 1-4 MB uncompressed per chunk for text
- Store large media (video, large images) as single-blob chunks, uncompressed
- Group small entries (< 1 KB) aggressively to amortize overhead

### 3.10 Zstd Dictionary Section

Each trained dictionary is stored in its own `ZSTD_DICT` (0x000B) section. A typical
archive has 2-4 dictionary sections (one per MIME group: html=1, css=2, js=3, other=4).

```
Per ZSTD_DICT section:
  4 bytes: dict_id (uint32)
  remaining bytes: raw Zstd dictionary
```

There is no `dict_count` header — the number of dictionaries equals the number of
`ZSTD_DICT` sections in the section table. Dictionary IDs are referenced by chunk
descriptors (`dict_id` field) and section descriptors (`dict_id` field for
zstd+dict compressed sections).

Zstd dictionaries improve compression 2-3x for blobs under 16 KB — transformative for
archives with millions of small entries (Wiktionary, Stack Overflow).

---

## 4. Search Index Design

### 4.1 Rationale

Xapian is 150,000 lines of C++ with no binary specification. OZA replaces it with a
**trigram index** -- the same approach used by Google Code Search (Russ Cox, 2012),
OpenGrok, and Sourcegraph. It maps every 3-byte substring to the list of documents
containing it. Any language can implement it in 300-500 lines.

### 4.2 Two-Index Architecture

OZA uses **two separate trigram indices** stored in distinct sections:

- **SEARCH_TITLE (0x000C)**: Indexes only the `title` field of front-article entries.
  Small (~3-5 MB for Wikipedia-scale archives). Designed to stay memory-resident for
  fast autocomplete and title search.

- **SEARCH_BODY (0x000D)**: Indexes `title + path + content` of all front-article entries.
  Large (~200-400 MB for Wikipedia). Loaded lazily for full-text content search.

Both sections share the same wire format.

**Why two sections instead of flags?** OZA's extensibility model is "unknown section
types are skipped." Two distinct types are self-describing. An old reader skips both
gracefully. Section-level flags would require every reader to understand the flag scheme.

**Why front articles only?** The `is_front_article` entry flag is the authoritative
marker for user-visible content. CSS, JS, images, and internal resources should never
appear in search results.

### 4.3 Wire Format (v1)

Both SEARCH_TITLE and SEARCH_BODY use the same binary format:

```
Header (16 bytes):
  4 bytes: version (1)
  4 bytes: flags (bit 0: bigram mode for CJK)
  4 bytes: trigram_count
  4 bytes: doc_count (number of distinct entry IDs indexed)

Trigram Table (sorted for binary search):
  Per trigram (12 bytes):
    3 bytes: trigram (UTF-8 bytes, lowercased)
    1 byte:  reserved (0)
    4 bytes: posting_list_offset (byte offset from section start)
    4 bytes: posting_list_length (byte count of posting list data)

Posting Lists:
  Serialized roaring bitmap (Roaring Bitmap portable format, roaring.WriteTo).
```

### 4.4 Query Algorithm

1. Normalize query: NFC, lowercase.
2. Extract all trigrams from the query.
3. Look up each trigram in the trigram table (binary search).
4. Intersect posting lists (sorted merge, smallest-first).
5. **Two-tier ranking**:
   - Search the title index first; collect matching entry IDs.
   - Search the body index; collect matching entry IDs.
   - Title matches sort before body-only matches.
   - Within each tier, results are sorted by entry ID.

### 4.5 Index Size

For English Wikipedia (~6M front articles):
- **Title index**: ~3-5 MB (titles average ~25 bytes)
- **Body index**: ~200-400 MB (~3-5% of uncompressed content)

The title index is small enough to keep permanently memory-resident, enabling
sub-millisecond autocomplete without touching the larger body index.

### 4.6 CJK Bigram Mode

Chinese, Japanese, and Korean (CJK) scripts are multi-byte in UTF-8 (3 bytes per
character for the common ranges). A raw 3-byte sliding window splits characters at
byte boundaries, producing cross-character grams like `[0x97, 0xA5, 0xE6]` for the
boundary between 日 and 本. These artificial grams pollute the index and make
multi-character queries less precise.

When CJK content is detected (`flags` bit 0 = 1), the writer uses
**character-aligned grams** for CJK runs:

| Gram type | Source | Key bytes |
|-----------|--------|-----------|
| Unigram | Single CJK character c | `c[0], c[1], c[2]` (the character's 3 UTF-8 bytes) |
| Bigram | Adjacent CJK pair (c1, c2) | `c1[last], c2[0], c2[1]` |

Where `c1[last]` is the last UTF-8 byte of c1, and `c2[0]`, `c2[1]` are the first
two bytes of c2. The bigram key is the cross-character-boundary byte trigram that
the raw sliding window would have produced at that exact position — it is already
in the index without any additional storage.

**Example** — indexing "日本語":

- 日 = `[E6 97 A5]`, 本 = `[E6 9C AC]`, 語 = `[E8 AA 9E]`
- Unigrams: `[E6 97 A5]`, `[E6 9C AC]`, `[E8 AA 9E]`
- Bigram 日本: `[A5 E6 9C]`  (last of 日, first two of 本)
- Bigram 本語: `[AC E8 AA]`  (last of 本, first two of 語)

Non-CJK runs within the same text continue to use the standard 3-byte sliding window.
At runtime the reader applies the same character-aligned extraction to the query when
`flags` bit 0 is set.

**Unicode ranges detected as CJK:**

| Range | Block |
|-------|-------|
| U+3000–U+9FFF | CJK Symbols, Hiragana, Katakana, CJK Unified Ideographs |
| U+AC00–U+D7AF | Hangul Syllables |
| U+F900–U+FAFF | CJK Compatibility Ideographs |

The `hasCJK` flag is set automatically by `TrigramBuilder` when any CJK rune is
encountered. Writers must not set bit 0 unless they use character-aligned grams;
readers must not assume character-aligned grams unless bit 0 is set.

### 4.7 Trade-offs

- **No stemming.** "running" won't match "run". Intentional -- stemming is
  language-dependent and complex. Users search for stems manually.
- **No BM25 ranking.** Title-match vs body-match tiers provide relevance signal.
  For offline archives, finding the right article matters more than ranking order.
- **False positives < 5%** for queries longer than 4 characters. Can be eliminated by a
  verification pass against actual content.

---

## 5. Compression Strategy

### 5.1 Zstd Only

No zlib. No bzip2. No XZ/LZMA.

- Zstd decompresses 5-10x faster than XZ at comparable ratios
- First-class dictionary support
- Pure implementations exist in Go, Rust, JavaScript, Python, Java
- LZMA's only advantage is ~5-10% better ratio at ultra settings, not worth 10x slowdown

### 5.2 Recommended Levels

| Content type | Strategy |
|-------------|----------|
| HTML/text | Zstd level 19, with dictionary for small articles |
| CSS/JS | Zstd level 19, with dictionary |
| JPEG/PNG/WebP | Store uncompressed (already compressed) |
| SVG | Zstd level 19 |
| Video | Store uncompressed (single-blob chunks) |

### 5.3 Dictionary Training

Writers train dictionaries on 100-1000 representative samples using `zstd --train`.
Dictionaries are most valuable for archives with many small, similar entries (Wiktionary:
millions of 2-5 KB entries). For Wikipedia with its long articles, dictionaries provide
marginal benefit but don't hurt.

---

## 6. Integrity and Security

### 6.1 Three-Tier Checksums

**File-level:** SHA-256 of everything before the trailing 32-byte checksum. One pass
over the file for quick verification.

**Section-level:** SHA-256 of each section's on-disk bytes, stored in the section
descriptor. Verify any section independently.

**Chunk-level:** Derived from chunk table entries. Pinpoint exactly which chunk is
corrupted.

If the file-level check fails, drill into section-level, then chunk-level, to localize
the damage. Compare this to ZIM's single MD5: "something's wrong somewhere."

### 6.2 Signatures

Optional `SIGNATURES` section with Ed25519:

```
4 bytes: signature_count

Per signature (128 bytes):
  32 bytes: public_key
  64 bytes: signature (of the file-level SHA-256)
  4 bytes:  key_id
  28 bytes: reserved
```

The signed payload is the SHA-256 hash, not raw file bytes. Signatures can be verified
without re-reading the entire file if the hash is already known.

OZA does not define a PKI. Key distribution is out of scope. A reader obtains trusted
public keys externally (config file, well-known URL, TOFU).

### 6.3 Content Sandboxing Guidance

Not a format feature, but the spec recommends:
- Set `Content-Security-Policy: sandbox` on HTML responses
- Disable JavaScript execution by default
- Block external resource loading in offline mode

---

## 7. Chrome/UI Separation

### 7.1 The Contract

Article content in the CONTENT section is **pure content**. No application shell, no
navigation framework, no search forms, no dead `Special:Search` links, no Vue components
that require MediaWiki's JS. If an article references `style.css`, that file is a content
entry.

### 7.2 Chrome Section

The optional CHROME section contains UI assets that a reader **may** use:

```
4 bytes: asset_count

Per asset:
  2 bytes: role (0=stylesheet, 1=script, 2=template, 3=icon, 4=font)
  2 bytes: name_length
  name_length bytes: asset name
  4 bytes: data_length
  data_length bytes: asset data
```

A minimal reader ignores chrome and serves raw HTML. A full-featured reader wraps articles
in a navigation template, adds search via the trigram index, provides browse-by-letter
via the title index.

### 7.3 Link Convention

Internal links are relative paths:

```html
<a href="Quantum_mechanics">Quantum mechanics</a>
<link rel="stylesheet" href="_res/style.css">
```

No namespace prefixes. No `/wiki/` roots. The reader maps paths to entries via the path
index.

### 7.4 Writer Obligations

Writers must produce self-contained HTML:
- No references to `Special:*` or any CMS-specific URLs
- No components requiring external JS bundles to render
- All internal links are relative paths
- CSS/JS dependencies included as content entries

---

## 8. Comparison

| Aspect | ZIM v5/v6 | OZA v1 |
|--------|-----------|--------|
| Header | Fixed 80 bytes, no extensibility | 64 bytes + variable section table |
| Extensibility | None (namespace abuse) | Unknown sections skippable |
| Namespaces | Single byte (C/M/W/X) | None -- flat paths by convention |
| Entry types | Overloaded MIME index (0xFFFF) | Explicit `entry_type` field |
| Content size | Not stored (must decompress) | `blob_size` in every entry |
| Entry record | Variable length + null-terminated strings | Variable-length (~15 bytes avg) + offset table, 5 bytes (redirect) |
| Entry lookup | 3 indirections | 1 indirection (O(1) by ID) |
| Compression | XZ/Zstd/zlib/bzip2 | Zstd only + dictionaries |
| Integrity | Single MD5 over file | SHA-256 at file/section/chunk |
| Corruption localization | No | Yes |
| Signatures | None | Optional Ed25519 |
| Search | Xapian (opaque C++, no spec) | Trigram index (fully specified) |
| Chrome/UI | Mixed with content | Separate optional section |
| MIME storage | Null-terminated list + sentinel | Length-prefixed table, no sentinel |
| Deduplication | None | Content-addressed via SHA-256 |
| Streaming | Requires random access | Section table up front |
| Unused fields | paramLen, revision, layoutPage | None |
| Encoding | UTF-8 assumed | UTF-8 NFC required |

---

## 9. Migration

### 9.1 ZIM-to-OZA Converter

A `zim2oza` tool:

1. Read all ZIM entries
2. Decompress clusters, compute `blob_size` for each entry
3. Hash content for deduplication
4. Regroup blobs into OZA chunks by MIME type
5. Recompress with Zstd (train dictionaries on HTML samples)
6. Build path, title, and trigram indexes
7. Strip chrome from content (remove dead Special:* links, rewrite URLs)
8. Write OZA file

**Expected results for English Wikipedia (~90 GB ZIM):** 2-4 hours conversion time.
10-20% smaller output due to Zstd and deduplication.

### 9.2 Dual-Format Reader

During transition, a reader detects format by magic number:

```go
func Open(path string) (*Archive, error) {
    // 0x044D495A -> ZIM  ("ZIM\x04" on disk, little-endian)
    // 0x01415A4F -> OZA  ("OZA\x01" on disk, little-endian)
}
```

The `Archive` API (`EntryByPath`, `ReadContent`, etc.) abstracts over both formats.

### 9.3 What We Lose

- **Xapian compatibility.** Indexes must be rebuilt as trigram indexes. Intentional.
- **Backward-compatible readers.** ZIM-only readers can't read OZA. Dual-format bridges
  this.
- **Ecosystem momentum.** ZIM has 15 years of tooling. Conversion tools and dual-format
  support are the migration strategy.

---

## 10. Open Questions for v1.1

1. **Multi-part archives.** ZIM supports `.zimaa/.zimab` splits. OZA v1 does not.
   Recommendation: use filesystem-level splitting. A multi-part section type can be added
   later without changing the core format.

2. **Video streaming.** Large video blobs are poorly served by the chunk model. A future
   `MEDIA_STREAM` section type could support byte-range-addressable video segments.

3. **Semantic search.** The trigram index is good for substring search but not semantic
   queries. A future section type could hold vector embeddings for AI-powered search.

4. **Incremental updates.** Could a delta/patch format update an OZA without full rewrite?
   The section-based design makes it possible to replace individual sections.

---

## 11. Test Vectors

The specification includes a reference `test.oza` file with known contents:

- 4 entries: one HTML article, one CSS file, one redirect, one metadata entry
- 2 chunks: one Zstd-compressed HTML chunk, one uncompressed CSS chunk
- Trigram index covering the HTML article
- All checksums pre-computed
- Small enough to include as a hex dump (< 4 KB)
- Accompanied by a JSON file listing expected parse results for every field

Any implementation that correctly parses `test.oza` and produces the expected JSON is
conformant.

---

*This document is a design proposal, not a ratified standard. It reflects what we would
build if starting from zero with two decades of hindsight about what works, what doesn't,
and what the offline content community actually needs.*

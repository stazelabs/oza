# 王座 -- OZA: Open Zipped Archive -- Format Specification

*This is the in-repo copy of the OZA format specification.*

*Draft 0.2 -- 2026-05-24*

File extension: `.oza`

---

## Conformance

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119).

---

## Introduction

OZA (Open Zipped Archive) is a binary archive format for offline content distribution
and long-term preservation. It packages web content — articles, images, stylesheets, and
search indices — into a single self-contained file that can be served over HTTP, read
directly from disk, or stored indefinitely. The format is designed to stay readable
without ecosystem maintenance: a complete read-only implementation requires no native
dependencies and fits comfortably in a week of work in any language.

---

## 1. From ZIM to OZA

The [ZIM format](https://wiki.openzim.org/wiki/OpenZIM) has served the offline content
community since 2007. Billions of articles have been distributed in ZIM files through the
Kiwix platform, reaching readers on every continent without an internet connection. That
track record is a genuine achievement, and OZA is not a rejection of it.

Two decades of deployment at scale are unusually valuable data. They reveal which design
decisions age well, which become load-bearing hacks, and which simply run out of headroom.
Redesigning in hindsight is easier than designing from scratch — the failure modes are
visible, the workarounds are documented, and the cost of each tradeoff is known. OZA
benefits from that knowledge directly.

The goal is a format that works as a **practical daily driver**: fast path lookup,
HTTP-range-friendly serving, straightforward to implement in any language without native
dependencies. It should also hold up as a **long-term archival medium**: localizable
integrity, stable and well-defined addressing, a version and extension model that can
evolve without breaking existing readers. Those two purposes reinforce each other — a
format that is easy to implement correctly gets deployed widely, and wide deployment is
what makes archival meaningful.

The following sections document where ZIM's design stops short of those goals, and why
the gaps cannot be closed with incremental patches.

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

### Versioning Policy

ZIM's history shows the cost of leaving this unstated: extension attempts were made
inside v5 files using overloaded fields because a major-version bump was seen as too
disruptive, fragmenting the ecosystem. OZA states its version philosophy before v1
freezes.

**Two-field version scheme**

OZA archives carry `major_version` and `minor_version` in the file header. The two
fields have distinct meanings and distinct reader obligations.

**Minor version increments are additive only**

A minor version increment signals a backwards-compatible addition:

- New optional section types (not marked `SECTION_CRITICAL`)
- New optional metadata keys

A v1 reader MUST NOT reject a v1 archive whose `minor_version` is greater than zero.
Unknown minor features that are skippable by design do not justify aborting. A reader
that does not understand a new optional section simply skips it.

**Major version increments signal breaking changes**

A major version increment means a v(N) reader cannot safely open the file without
explicit adaptation. A v1 reader that encounters `major_version != 1` MUST return
`ErrUnsupportedVersion`; this applies equally to past values (0) and future values (>1).

**Forward-compatibility obligation**

A v(N+1) reader SHOULD accept v(N) files for at least one full major version. Stating
this obligation in advance lowers the cost of future major versions and removes the
incentive to overload fields in the current one — the failure mode ZIM encountered.

**Extension sections across major versions**

Section types `0x0100` and above are the extensibility path within any major version.
The range is partitioned into three bands (see also `docs/EXTENSION_REGISTRY.md`):

- **`0x0100–0x01FF` — spec-sanctioned extensions.** Allocated via `FORMAT.md` PRs or
  the extension registry. These types carry no vendor prefix in the payload.
- **`0x0200–0xFEFF` — vendor/community extensions.** Writers MUST embed a 4-byte
  ASCII vendor prefix as the first 4 bytes of the section payload (e.g. `KWIX` for
  Kiwix, `WIKI` for Wikimedia). This prevents collisions between independent implementors.
- **`0xFF00–0xFFFF` — private/experimental use.** MUST NOT appear in distributed
  archives. Reserved for local testing and draft implementations.

Writers MUST set `SECTION_CRITICAL` on extension sections that a reader must understand
to open the archive correctly, and MUST leave the flag clear when skipping is safe (see
§3.3 for full `SECTION_CRITICAL` semantics). This mechanism is designed to carry forward
across future major versions.

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

The diagram below is the **RECOMMENDED canonical streaming order**. Sections in v1
may technically appear at any file offset (the section table records each section's
absolute offset, so readers do not depend on layout), but writers SHOULD emit them in
this order so that a single-pass reader serving an archive over HTTP, from an SD card,
or from spinning disk can satisfy lookups as soon as each section streams in.

```
+----------------------+
| File Header          |  128 bytes fixed (§3.2)
+----------------------+
| Section Table        |  Array of section descriptors (80 bytes each)
|                      |  MUST start at offset 128 (§3.2; OZA-53)
+----------------------+
| METADATA             |  Structured key-value pairs (§3.4)
+----------------------+
| MIME_TABLE           |  Deduplicated MIME type strings (§3.5)
+----------------------+
| LANGUAGE_TABLE       |  BCP-47 language strings (§3.5a; multilingual archives only)
+----------------------+
| ENTRY_TABLE          |  Variable-length content entry records + offset table (§3.6)
+----------------------+
| REDIRECT_TABLE       |  Compact 5-byte redirect records (§3.6a; if redirects exist)
+----------------------+
| PATH_INDEX           |  Sorted paths + offset table (§3.7)
+----------------------+
| TITLE_INDEX          |  Sorted titles + offset table (§3.8; optional)
+----------------------+
| MIME_INDEX           |  Roaring posting lists per MIME (§3.8a; optional)
+----------------------+
| ZSTD_DICT × N        |  Shared compression dictionaries (§3.10; optional)
|                      |  MUST precede CONTENT under the canonical order
+----------------------+
| CONTENT              |  Compressed content chunks (§3.9; large)
+----------------------+
| SEARCH_TITLE         |  Trigram index of titles (§4; optional)
+----------------------+
| SEARCH_BODY          |  Trigram index of content (§4; optional)
+----------------------+
| CHROME               |  UI assets (§3.x; optional)
+----------------------+
| File Checksum        |  32-byte SHA-256 (§6.1)
+----------------------+
| Signatures           |  Ed25519 signature trailer (§6.2; optional)
+----------------------+
```

**Why this order.** A reader receiving the bytes sequentially can begin answering
requests as each section finishes streaming:

- **Header + section table (`offset 128`)** — the reader knows the size, version,
  and offset of every other section before reading any of them.
- **METADATA → MIME_TABLE → LANGUAGE_TABLE** — small, required for parsing
  ENTRY_TABLE records (`mime_index`, `lang_index`). Together they typically fit
  in a single TCP window.
- **ENTRY_TABLE + REDIRECT_TABLE** — the universe of addressable entries.
- **PATH_INDEX + TITLE_INDEX + MIME_INDEX** — once these arrive, the reader can
  answer "where is `/wiki/Foo`?", "what title begins with `Bar`?", "list all
  images" without waiting for CONTENT.
- **ZSTD_DICT before CONTENT** — chunks reference dictionaries by `dict_id`;
  emitting dictionaries first lets the reader prime its decoder before the first
  byte of compressed content arrives.
- **CONTENT** — the bulk of the archive, streamed last among the primary
  sections so smaller indices are usable before it completes.
- **SEARCH_TITLE / SEARCH_BODY / CHROME** — large, optional, and used only by
  certain client types (search-capable UIs, branded viewers). Streaming them
  after CONTENT keeps the time-to-first-useful-byte short for clients that don't
  need them.
- **File checksum + signatures (trailer)** — verification artifacts that
  intrinsically need the rest of the file first.

**Cardinality interaction.** A few sections are optional or absent depending on
content (§3.3 cardinality table); when absent they simply have no slot, and the
relative order of present sections still follows this list. Multiple ZSTD_DICT
sections (one per dictionary) MUST appear contiguously in the dictionary slot,
above CONTENT.

**Advisory, not required.** A reader MUST NOT assume canonical order unless the
`STREAMING` header flag is set (§3.2). The section table is the single source of
truth for section offsets; out-of-order archives remain valid v1.

All integers are **little-endian**. All strings are **UTF-8, NFC-normalized**.

### 3.2 File Header (128 bytes)

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0 | 4 | `magic` | `0x00415A4F` ("OZA\x00" on disk, little-endian) |
| 4 | 2 | `major_version` | 1 |
| 6 | 2 | `minor_version` | 0 |
| 8 | 16 | `uuid` | UUID v5 content identity (see below) |
| 24 | 4 | `section_count` | Number of sections |
| 28 | 4 | `entry_count` | Content entries (excludes redirects) |
| 32 | 8 | `content_size` | Total uncompressed content bytes |
| 40 | 8 | `section_table_offset` | Offset to section table; MUST equal 128 for v1 |
| 48 | 8 | `checksum_offset` | Offset to trailing SHA-256 |
| 56 | 4 | `flags` | Bit flags (see below) |
| 60 | 4 | `redirect_count` | Number of redirect entries |
| 64 | 4 | `front_article_count` | Number of front-article entries (content + redirect) |
| 68 | 60 | `reserved` | MUST be zero |

**`section_table_offset` placement.** For v1, `section_table_offset` MUST equal 128 (immediately after the fixed-size header). Writers MUST NOT place the section table at any other offset. Readers MUST reject an archive whose `section_table_offset` is not 128. Fixing the offset enables single-pass reads — a reader can parse the header and immediately stream the section table without a seek — and eliminates an entire class of parser ambiguity. The field is retained in the header layout for forward-compatibility with potential future major versions where a different placement policy might apply.

The `magic` field encodes only format identity, never version. The null byte (`\x00`) is
a permanent sentinel: v2, v3, and all future major versions use the same magic value. A
reader MUST check `magic` first (identity gate), then `major_version` (version gate). This
keeps `file(1)`, OS file-type associations, and naive format detectors working correctly
across all future major versions.

**Flags:**

| Bit | Name | Meaning |
|-----|------|---------|
| 0 | `has_search` | Search section present |
| 1 | `has_chrome` | Chrome section present |
| 2 | `has_signatures` | Signature section present |
| 3 | `streaming` | Sections are emitted in the §3.1 RECOMMENDED canonical order |
| 4-31 | -- | Reserved (MUST be zero; readers ignore unknown flags) |

**`streaming` flag (bit 3).** When set, the writer asserts that the archive's
sections are laid out in the §3.1 canonical order (header → section table at offset
128 → METADATA → MIME_TABLE → LANGUAGE_TABLE → ENTRY_TABLE → REDIRECT_TABLE →
PATH_INDEX → TITLE_INDEX → MIME_INDEX → ZSTD_DICT(s) → CONTENT → SEARCH_TITLE →
SEARCH_BODY → CHROME → checksum → optional signature trailer), with the
section_table entries also sorted ascending by `offset`. Optional sections that are
absent simply have no slot; the relative order of present sections still matches.

Readers MAY use this flag to enable single-pass streaming optimisations (begin
decoding sections in section-table order without random seeks; prime the Zstd
decoder before CONTENT bytes arrive). When the flag is clear, readers MUST treat
section offsets as arbitrary and use the section table as the single source of
truth.

Writers MUST NOT set `streaming` unless every section in the archive obeys the
canonical order. Readers MUST reject an archive whose `streaming` flag is set but
whose actual section layout (sorted by `offset`) deviates from the canonical order.
The order check is purely positional: a reader walks the section table sorted by
`offset` and verifies the type sequence matches the canonical sequence filtered to
the section types actually present.

**Count field semantics.** The three header count fields use different inclusion rules,
which is a common source of off-by-one errors in implementations:

| Field | Counts content entries | Counts redirect entries | Purpose |
|-------|----------------------|------------------------|---------|
| `entry_count` | All | None | Size of ENTRY_TABLE |
| `redirect_count` | None | All | Size of REDIRECT_TABLE |
| `front_article_count` | Those with `is_front_article` set | Those with `is_front_article` set | Navigation and search index sizing |

`entry_count + redirect_count` is the total number of addressable entries in the
archive. `front_article_count` MUST be less than or equal to this sum.

`front_article_count` deliberately spans both namespaces because navigability is
orthogonal to the content-vs-redirect distinction. A redirect from a common spelling to
the canonical title is user-visible and belongs in title search and "random article" —
splitting the front-article count by entry type would force all callers to sum two
fields and invite off-by-one errors. The per-entry flag that populates this count is
`is_front_article` (bit 4 of `type_and_flags` in content entries; bit 0 of `flags` in
redirect entries — see §3.6 and §3.6a).

Writers MUST ensure consistency between these header fields and the sections they
describe:
- `header.entry_count` MUST equal the `entry_count` field embedded in the ENTRY_TABLE
  section (§3.6).
- `header.redirect_count` MUST equal the `count` field embedded in the REDIRECT_TABLE
  section (§3.6a), or zero if that section is absent.
- `header.front_article_count` MUST equal the number of content entries with
  `is_front_article` set plus the number of redirect entries with `is_front_article`
  set.

**Content UUID (`uuid` field).** `header.uuid` is a deterministic UUID v5 (RFC 4122
§4.3) that identifies the archive content independently of when it was built. Writers
MUST compute it as:

    uuidV5(ArchiveIdentityNamespace, source + "\x00" + language)

where `source` and `language` are the values of the required metadata keys, and
`ArchiveIdentityNamespace` is the fixed 16-byte value
`c0a8f6e2-4b73-4d92-8a15-3e7f6c9b1d04`. Monthly rebuilds of the same archive
(same source, same language) produce the same `uuid`. The per-build random UUID v4
that identifies the specific build artifact is stored separately in the optional
`build_uuid` metadata key (§3.4).

### 3.3 Section Table

Each section descriptor is **80 bytes**:

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0 | 4 | `section_type` | Enum (see below) |
| 4 | 4 | `flags` | Section descriptor flags (see below) |
| 8 | 8 | `offset` | Absolute file offset |
| 16 | 8 | `compressed_size` | On-disk size |
| 24 | 8 | `uncompressed_size` | Decompressed size |
| 32 | 1 | `compression` | 0=none, 1=zstd, 2=zstd+dict, 3=brotli |
| 33 | 3 | `reserved` | MUST be zero |
| 36 | 4 | `dict_id` | Dictionary ID (0 if none) |
| 40 | 8 | `reserved2` | MUST be zero |
| 48 | 32 | `sha256` | SHA-256 of compressed section bytes |

**Section descriptor flags** (the `flags` field at offset 4):

| Bit | Name | Meaning |
|-----|------|---------|
| 0 | `SECTION_CRITICAL` | Reader MUST reject the archive if it does not recognise this section type |
| 1-31 | -- | Reserved (MUST be zero) |

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
| 0x0008 | — | Reserved (not used in v1) |
| 0x0009 | CHROME | UI/navigation assets |
| 0x000A | — | Reserved (not used in v1) |
| 0x000B | ZSTD_DICT | Shared Zstd dictionaries |
| 0x000C | SEARCH_TITLE | Trigram index of front-article titles |
| 0x000D | SEARCH_BODY | Trigram index of front-article body content |
| 0x000E | LANGUAGE_TABLE | BCP-47 language string table (multilingual archives) |
| 0x000F | MIME_INDEX | Secondary index mapping each MIME type to its entry IDs |
| 0x0100–0x01FF | -- | Spec-sanctioned extensions (see `docs/EXTENSION_REGISTRY.md`) |
| 0x0200–0xFEFF | -- | Vendor/community extensions (4-byte vendor prefix required in payload) |
| 0xFF00–0xFFFF | -- | Private/experimental use (MUST NOT appear in distributed archives) |

**Section cardinality:** Writers MUST NOT produce archives that violate these rules.
Readers MUST reject archives that do.

| Section | Cardinality | Notes |
|---------|-------------|-------|
| METADATA (0x0001) | Exactly once | Required |
| MIME_TABLE (0x0002) | Exactly once | Required |
| ENTRY_TABLE (0x0003) | Exactly once | Required |
| PATH_INDEX (0x0004) | At most once | RECOMMENDED in general; **REQUIRED if `header.redirect_count > 0`** (see §3.6a). Omission otherwise degrades path lookup to a linear scan |
| TITLE_INDEX (0x0005) | At most once | Optional |
| CONTENT (0x0006) | Exactly once | Required |
| REDIRECT_TABLE (0x0007) | At most once | MUST be present if `header.redirect_count > 0`; MUST be absent otherwise |
| CHROME (0x0009) | At most once | Optional |
| ZSTD_DICT (0x000B) | Zero or more | One section per dictionary; chunk descriptors reference by `dict_id` |
| SEARCH_TITLE (0x000C) | At most once | Optional |
| SEARCH_BODY (0x000D) | At most once | Optional |
| LANGUAGE_TABLE (0x000E) | At most once | MUST be present if any entry has `lang_index > 0`; otherwise MAY be omitted |
| MIME_INDEX (0x000F) | At most once | Optional secondary index (§3.8a); omission means MIME-filtered enumeration falls back to a linear scan of ENTRY_TABLE |

When a reader encounters an unknown section type it checks the `SECTION_CRITICAL` flag
before deciding how to proceed:

- If `SECTION_CRITICAL` is **clear**: skip using `offset + compressed_size` and continue
  opening the archive. This is the extensibility path for optional enhancements.
- If `SECTION_CRITICAL` is **set**: the reader MUST reject the archive with an error.
  This allows future writers to emit a section that old readers must not silently ignore
  (for example, a replacement index format that supersedes an optional one).

No TLV nesting, no protobuf. Just a flat table with self-describing entries.

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

**Required keys:** `title`, `language` (BCP-47, see below), `creator`, `date` (ISO 8601 subset — see below), `source`.

**`language` format.** The `language` value is a comma-separated list of BCP-47 tags
(no whitespace between tags). The first tag is the archive's **dominant language**
and is the value used for archive identity derivation (§3.4, `header.uuid`). Additional
tags declare other primary languages present in the archive — for example, a
multilingual wiki bundle or a polyglot documentation site.

Examples:

- `en` — single-language archive
- `en,fr,de` — archive whose dominant language is English with French and German
  also represented as primary languages
- `zh-Hans,zh-Hant` — Simplified and Traditional Chinese

Single-language archives MUST omit the comma form (the value is exactly one BCP-47
tag). For mixed-language archives, individual entries MAY declare a language
override via the entry record's `lang_index` field; see §3.6.

**`date` format.** "ISO 8601" encompasses many representations that parsers cannot
interchangeably handle. To prevent implementation divergence, `date` MUST conform to
exactly one of the following two forms:

- `YYYY-MM-DD` — calendar date only (e.g. `2024-01-15`)
- `YYYY-MM-DDThh:mm:ssZ` — UTC datetime with second precision (e.g. `2024-01-15T00:00:00Z`)

Any other ISO 8601 representation — year-only, month-only, week dates, fractional seconds,
or a non-UTC offset — is a conformance error. Writers MUST NOT produce a `date` value
outside these two forms. Readers SHOULD emit a warning when the `date` value does not
match either pattern.

**Optional well-known keys:** `description`, `long_description`, `license` (SPDX),
`favicon_entry` (entry path string — same lookup mechanism as `main_entry`; existence is checked by the reader via `EntryByPath`), `main_entry` (entry path string — any non-empty UTF-8; existence is checked by the reader via `EntryByPath`), `article_count`,
`scraper` (tool name + version), `catalog` (JSON array, see below),
`build_uuid` (UUID v4 string identifying the specific build artifact — distinct from
the content UUID in `header.uuid`),
`identifier` (stable external ID — DOI, ISBN, ISSN, URN, or ISNI),
`publisher` (organisation distributing the archive — distinct from `creator` which is
the original author or content producer),
`relation` (Dublin Core relation expressed as `<type>:<uri>`, e.g.
`Is-Version-Of:https://example.org/v1`, `Is-Part-Of:<uri>`, `Replaces:<uri>`),
`rights` (access restrictions or rights statement — distinct from `license` which
carries the SPDX reuse terms).

**Catalog metadata.** Archives that bundle multiple logical items (e.g. a book
collection) MAY set `catalog` to a JSON array of item descriptors. Each element
is an object with the following fields:

| Field      | Type   | Description                                        |
|------------|--------|----------------------------------------------------|
| `slug`     | string | Namespace prefix for the item's entries             |
| `title`    | string | Display title                                       |
| `creator`  | string | Author / creator                                    |
| `language` | string | BCP-47 language code                                |
| `entry`    | string | Path to the item's index page within the archive    |
| `entries`  | int    | Number of entries belonging to this item             |

Viewers (e.g. `ozaserve`) use the catalog to surface individual items on the
library page instead of showing a single opaque archive row. The convention is
converter-agnostic: any tool that bundles multiple items into one archive can
write a `catalog` key.

**Key namespacing.** The metadata key space is partitioned to prevent collisions between
the spec-defined vocabulary and community extensions:

- **Unprefixed keys** (e.g. `title`, `language`, `creator`) are reserved for this
  specification. Writers MUST NOT use unprefixed keys that are not defined here.
- **Reverse-domain keys** use the form `com.example:key_name`. This is the standard
  path for vendor- or community-specific metadata:
  `com.kiwix:scrape_version`, `org.wikimedia:dump_date`.
- **Short-prefix keys** use the form `prefix:key_name` for well-known vocabularies
  with broadly accepted short identifiers: `dc:identifier`, `schema:author`.

Writers adding custom keys MUST use a prefixed form. Readers MUST ignore keys with
unrecognised prefixes rather than failing.

### 3.5 MIME Table

```
2 bytes: count

Per type:
  2 bytes: string_length
  string_length bytes: MIME type string
```

The MIME table is fully dynamic — no index value is reserved or required. Writers
assign MIME types in any order. Writers MAY sort their most-referenced types toward the
front (lower indices compress and cache slightly better in practice), but this is
advisory only. Archives that contain no HTML, CSS, or JavaScript are not required to
include those types.

The value `0xFFFF` is **not** used for redirects. MIME indices are purely MIME indices.
Redirects are a separate entry type.

### 3.5a Language Table

Optional section (0x000E) carrying BCP-47 language tags referenced by entry records.
Same wire format as the MIME table.

```
2 bytes: count

Per tag:
  2 bytes: string_length
  string_length bytes: BCP-47 tag (UTF-8, NFC)
```

LANGUAGE_TABLE is required only when at least one entry record carries
`lang_index > 0` (see §3.6). Single-language archives, and multilingual archives
whose entries all inherit the archive's dominant language, MUST omit this section.

Indexing is 1-based from the perspective of `lang_index`: an entry with
`lang_index == N` (for `N ≥ 1`) refers to the tag stored at table position `N - 1`.
The reserved value `lang_index == 0` means **inherit the archive's dominant
language** (the first tag in the metadata `language` value) and never performs a
LANGUAGE_TABLE lookup.

Writers SHOULD order tags by descending entry-count so that the most-referenced
languages produce the smallest uvarint encoding, but this is advisory only. Tag
strings MUST be valid BCP-47 and SHOULD appear in NFC. Duplicate tags are a
conformance error.

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

  Per record (variable length, ~15 bytes average without optional trailing fields):
    uint8   type_and_flags      Bits 0-3: entry_type (0=content, 2=metadata_ref)
                                Bits 4-7: flags (bit 4 = is_front_article)
    uvarint mime_index          Index into MIME table
    uvarint chunk_id            Content chunk ID
    uvarint blob_offset         Byte offset within decompressed chunk
    uvarint blob_size           Decompressed content size in bytes
    uvarint hash_algorithm      Hash algorithm (0=xxhash64, 1=blake3-64; see below)
    <bytes> content_hash        Hash bytes; length determined by hash_algorithm
                                (8 bytes for algorithm 0 and 1)
    uvarint mtime               Unix epoch seconds (UTC); 0 = unknown / not applicable.
                                Writers for sources with per-document timestamps
                                (wikis, crawlers, git-tracked sites) SHOULD populate
                                this field. Readers that exhaust the record bytes before
                                reaching mtime MUST treat it as 0 (absent in archives
                                produced before this field was defined).
    uvarint lang_index          0 = inherit the archive's dominant language (default).
                                N ≥ 1 = index into LANGUAGE_TABLE at position N - 1.
                                Readers that exhaust the record bytes before reaching
                                lang_index MUST treat it as 0.
```

Entry ID is implicit: the index into the offset table. Uvarints use unsigned LEB128
encoding (same as Go `encoding/binary.PutUvarint`).

**Entry IDs are transient.** An ID is a build-specific index into the offset table of
a particular archive build. Monthly rebuilds reassign every ID: entries added, removed,
or reordered in any position produce entirely different assignments. Applications
MUST NOT store entry IDs as persistent references across archive versions. The canonical
persistent identifier for an entry is its NFC-normalised path. Applications that need
long-term or cross-archive entry references SHOULD use paths, not IDs. (A future
`STABLE_IDS` section could carry per-entry UUIDs or content-addressed identifiers for
archives that require stable links.)

**Path uniqueness.** Each path in an archive **MUST** be unique. Path uniqueness is
case-sensitive (UTF-8 NFC byte equality, matching the NFC normalisation already required
for all paths). Writers **MUST NOT** produce archives with duplicate paths. Readers
**MUST** reject archives that contain duplicate paths.

Key properties:

- **O(1) access:** Entry N is at `record_data[offset_table[N]]`. One extra indirection
  vs fixed-size records, but the offset table stays cache-hot.
- **~60% smaller** than fixed 40-byte records. Average record is ~15 bytes + 4 bytes
  offset table entry = ~19 bytes/entry.
- **`hash_algorithm` / `content_hash`**: `hash_algorithm` is a uvarint discriminator
  that precedes `content_hash` and determines its length and interpretation. Defined values:

  | Value | Algorithm  | Hash length | Notes                          |
  |-------|------------|-------------|--------------------------------|
  | `0`   | xxhash64   | 8 bytes     | Default; little-endian uint64  |
  | `1`   | blake3-64  | 8 bytes     | First 8 bytes of BLAKE3        |

  All other `hash_algorithm` values are reserved. Writers MUST use `0` (xxhash64) unless
  they have a specific reason to use a defined alternative. Readers that encounter an
  unknown `hash_algorithm` MUST treat the entry as having an **unverified hash** — they
  MUST NOT fail to parse the record, but they cannot verify integrity for that entry.
  `content_hash` provides **error-detection** against accidental bit corruption and
  enables deduplication; it is non-cryptographic and provides **no tamper-resistance**.
  For tamper-resistance, rely on the SHA-256 checksums (§6.1).
- **`blob_size` is in the entry.** HTTP `Content-Length` without decompression.
- **`is_front_article`** replaces namespace-based heuristics for "is this user-visible?"
- **`mtime`**: Optional per-entry modification timestamp as Unix epoch seconds (UTC). A
  value of `0` means unknown or not applicable. The field uses the trailing-uvarint
  forward-compatibility pattern: each record is bounded by the offset table, so old
  readers silently ignore trailing bytes, and new readers that exhaust the record
  before reaching `mtime` treat it as `0`. Cost: 1 byte (`0x00`) when absent; typically
  5 bytes for a current Unix timestamp. For archives derived from sources with
  independent update rates (wikis, crawlers, git-tracked sites), `mtime` enables
  "show me what changed since X" queries without full content comparison.
- **`lang_index`**: Optional per-entry language override for mixed-language archives. A
  value of `0` (the default, and the value produced by readers that exhaust the record
  before reaching this field) means the entry inherits the archive's dominant language
  — the first BCP-47 tag in the metadata `language` value (§3.4). A value `N ≥ 1` is
  a 1-based reference into LANGUAGE_TABLE (§3.5a): the entry's language is the tag at
  table position `N - 1`. Single-language archives MUST leave this field at `0` for
  every entry and MUST omit LANGUAGE_TABLE. Same trailing-uvarint forward-compatibility
  rule as `mtime`. Cost: 1 byte (`0x00`) when absent; 1–2 bytes when present for any
  reasonable number of distinct languages.

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

**PATH_INDEX is required when this table is non-empty.** A redirect record carries no
path of its own — only its target ID. The address a redirect answers to is recoverable
only by reverse lookup through the path index (scanning PATH_INDEX for the tagged ID
whose high bit is set and low 31 bits equal this record's position). Consequently,
writers **MUST** emit PATH_INDEX (0x0004) whenever `header.redirect_count > 0`, and
readers **MUST** reject archives that contain a non-empty REDIRECT_TABLE but lack a
PATH_INDEX. This constraint is reflected in the cardinality table in §4. Archives with
zero redirects MAY still omit PATH_INDEX (with the usual lookup-degradation tradeoff).

**Capacity constraint (v1 format invariant).** Because bit 31 is permanently reserved
as a type tag, each namespace is capped at 2,147,483,647 entries (2³¹ − 1):

| Namespace | Maximum |
|-----------|---------|
| Content entries | 2,147,483,647 |
| Redirect entries | 2,147,483,647 |

This exceeds any foreseeable archive size (Wikipedia ~6 M articles as of 2026) and is
not expected to be a practical constraint. However, implementations **MUST** reject
archives whose `entry_count` field or redirect table `count` field exceeds this limit,
and writers **MUST** return an error rather than silently wrap the ID space.

At Wikipedia scale (~10 M redirects), this saves ~350 MB compared to storing redirects
as 40-byte entry records.

### 3.7 Path Index

Front-coded index sorted by path for binary search. Uses the IDX1 format with
restart blocks every 64 entries for efficient random access.

```
Header (24 + restart_count * 4 + string_table_size bytes):
  4 bytes: magic (0x49445831 = "IDX1" little-endian)
  4 bytes: count (total number of entries)
  4 bytes: restart_interval (64)
  4 bytes: restart_count
  4 bytes: string_table_count (number of interned strings)
  4 bytes: string_table_size  (byte size of the serialized string table)
  restart_count * 4 bytes: restart_offsets (byte offset from section start)

String Table (string_table_size bytes):
  Per interned string:
    2 bytes: string_length
    string_length bytes: string data (UTF-8)

Records (front-coded within restart blocks, using token-encoded keys):

  Restart record (first in each block of 64):
    4 bytes: entry_id
    1 byte:  token_count
    token_count tuples of:
      2 bytes: table_index (0xFFFF = no table lookup)
      2 bytes: literal_length
      literal_length bytes: literal data

  Non-restart record:
    4 bytes: entry_id
    2 bytes: prefix_length (bytes shared with previous key)
    1 byte:  token_count
    token_count tuples of:
      2 bytes: table_index (0xFFFF = no table lookup)
      2 bytes: literal_length
      literal_length bytes: literal data
```

The string table interns frequently repeated path/title components (e.g. `.html`,
`/wiki/`). Token-encoded keys reference table entries by index, with optional literal
suffixes. This reduces index size by ~30% for large archives with repetitive paths.

**No namespaces.** Paths are flat: `Main_Page`, `_res/style.css`, `_meta/Title`.
Content organization is by convention (path prefix), not by format-level namespace.

Binary search uses restart offsets for O(1) block access, then linear scan within the
block. Overall lookup is O(log(count / 64) + 64) string comparisons.

**Path uniqueness invariant.** The PATH_INDEX is a front-coded sorted structure that
assumes each key is unique. Writers **MUST NOT** emit a PATH_INDEX with duplicate paths.
Readers **MUST** reject archives whose path index contains duplicate paths. Uniqueness
is defined as UTF-8 NFC byte equality (case-sensitive), consistent with the NFC
normalisation requirement in §3.6.

**Required when redirects exist.** PATH_INDEX is RECOMMENDED in general, but
**REQUIRED** whenever `header.redirect_count > 0`. Redirect records (§3.6a) carry only
a target ID, not a path; the URL a redirect answers to is recoverable only by reverse
lookup through this index. Archives that violate this rule (non-empty REDIRECT_TABLE
without PATH_INDEX) MUST be rejected by readers and MUST NOT be produced by writers.

### 3.8 Title Index

Same IDX1 format as the path index, but sorted by title. Entries without an explicit
title use their path. The writer populates this at creation time.

### 3.8a MIME Index

Optional secondary index (section type `0x000F`, `MIME_INDEX`) that maps each MIME
table index to the set of content entry IDs whose `mime_index` field equals that value.
PATH_INDEX answers "where is path `p`?" — MIME_INDEX answers "which entries have MIME
type `t`?" without a linear scan of ENTRY_TABLE. This matters at Wikipedia scale (~4 M
entries, ~90 GB on disk), where "list every image" or "enumerate all HTML pages"
otherwise touches hundreds of MB of entry records just to read the `mime_index` field.

Posting lists use the **Roaring Bitmap portable format** (`roaring.WriteTo`), matching
the encoding already used for SEARCH_TITLE / SEARCH_BODY posting lists (§4.3). This
lets readers reuse a single posting-list decoder for both subsystems.

```
Header (12 bytes):
  4 bytes: version            (uint32, 1)
  4 bytes: flags              (uint32, reserved — writers MUST emit 0,
                               readers MUST reject non-zero values)
  4 bytes: mime_count         (uint32, number of MIME entries that follow)

MIME Table (sorted ascending by mime_index for binary search):
  Per record (12 bytes):
    2 bytes: mime_index           (uint16, index into MIME_TABLE §3.5)
    2 bytes: reserved             (0)
    4 bytes: posting_list_offset  (uint32, byte offset from section start)
    4 bytes: posting_list_length  (uint32, byte length of the posting list)

Posting Lists:
  Per posting list: a serialized Roaring Bitmap (portable format) whose
  values are content entry IDs (bit 31 clear; redirect IDs MUST NOT appear).
```

**Coverage and tagging.** MIME_INDEX covers content entries only. Redirect entries
(§3.6a) have no MIME type, so their IDs MUST NOT appear in any posting list. Writers
MUST emit untagged content entry IDs (bit 31 clear, as stored in ENTRY_TABLE), and
readers MUST reject archives whose posting lists contain tagged redirect IDs.

**Completeness.** When MIME_INDEX is present, it MUST be exhaustive over content
entries: every content entry's `mime_index` MUST be represented in the table, and the
union of all posting lists MUST equal exactly the set of content entry IDs `0 ..
header.entry_count - 1`. A MIME index with `mime_count == 0` is permitted only in
archives with no content entries. MIME table indices that are referenced by zero
entries MAY be omitted from the table; writers MUST NOT emit an empty posting list.

**Uniqueness.** Each `mime_index` value MUST appear at most once in the MIME table.
Readers MUST reject archives whose MIME index table contains duplicate `mime_index`
values. The ascending-sort requirement enables binary search and makes duplicate
detection a single-pass check.

**Forward compatibility.** The `version` and `flags` fields exist for future
extensions (e.g. alternative posting-list encodings). v1 readers MUST reject
`version != 1` or `flags != 0` rather than guess at unknown semantics; this is
intentionally stricter than the trailing-uvarint pattern used by ENTRY_TABLE fields,
because incorrect posting-list decoding could silently return wrong results.

**Size and cost.** Roaring bitmaps store dense runs efficiently. A Wikipedia-scale
archive whose writer groups entries by MIME (the recommended chunk-packing strategy
in §3.9) yields long consecutive entry-ID runs per MIME type — roaring compresses
each MIME's posting list to a small constant plus run descriptors. Total MIME_INDEX
size for English Wikipedia (~4 M entries across ~10 distinct MIME types) is expected
to be well under 1 MB.

**When to omit.** MIME_INDEX is purely an enumeration accelerator. Readers that never
filter by MIME type, and archives whose `entry_count` is small enough that linear
scans are fast, gain nothing from this section. Writers SHOULD emit it for archives
larger than ~100 K entries or whenever a known consumer (e.g. a UI that lists "all
images") will perform MIME-typed enumeration.

### 3.9 Content Section

```
Chunk Table (at section start):
  4 bytes: chunk_count

  Per chunk descriptor (28 bytes):
    4 bytes:  chunk_id            (uint32)
    8 bytes:  compressed_offset   (uint64, byte offset from start of chunk data area)
    8 bytes:  compressed_size     (uint64)
    4 bytes:  dict_id             (uint32, 0 if not dict-compressed)
    1 byte:   compression         (0=none, 1=zstd, 2=zstd+dict, 3=brotli)
    3 bytes:  entry_count_hint    (uint24, little-endian; 0 = unknown)

[Compressed chunk data follows immediately after the chunk table]
```

**`entry_count_hint`.** The three-byte unsigned integer (little-endian, range 0–16 777 215)
records how many entries are packed into this chunk. A value of `0` means the count is
unknown or was not populated by the writer. Writers SHOULD populate this field; it
enables readers to make informed caching decisions without decompressing the chunk first.
Readers MAY use this hint to select decompression and eviction policy (e.g. a chunk
containing 10 000 small entries is a better cache candidate than one containing a single
large video blob). Readers MUST NOT treat a non-zero `entry_count_hint` as authoritative
— it is advisory only.

Each chunk is independently compressed. Each chunk has its own compression type — HTML
chunks use Zstd level 19, image-only chunks store uncompressed.

Chunk descriptors are sorted by `chunk_id`. Entry records reference chunks by ID;
`compressed_offset` is relative to the start of the chunk data area (immediately after
the chunk table).

Chunk IDs within a CONTENT section MUST be unique. Writers MUST NOT produce duplicate
chunk IDs. Readers MUST reject a CONTENT section containing duplicate chunk IDs.

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
marker for user-visible content. CSS, JS, images, and internal resources SHOULD NOT
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

### 4.5.1 Compression Characteristics

Search indices are compressed as single blobs with plain Zstd at level 19.
Typical compression ratios on real archives:

| Archive type | Body index ratio | Notes |
|---|---|---|
| Long-form text (Gutenberg) | 12-15% | Few documents, large posting lists compress well |
| Video/media (TED) | 18-20% | Sparse index, small posting lists |
| Developer docs | 26-30% | Moderate trigram density |
| Encyclopedias (Wikipedia) | 30-40% | Dense trigram coverage, many documents |
| Dictionaries (Wiktionary) | 45-60% | Very high document count, many short entries |

**Dictionary-trained compression does not help search indices.** Empirical testing
on Tier 1 archives shows that training Zstd dictionaries on search index data either
fails (roaring bitmap structures produce invalid dictionary offsets) or produces
dictionaries larger than the savings they provide (net size increase of 65-224%).
This is because the index is a single large binary blob -- Zstd at level 19 already
finds internal patterns within the stream. Dictionaries are designed for many small
documents with shared structure, not monolithic binary formats.

The dominant factor in search index size is the number of unique trigrams multiplied
by the cardinality of their posting lists. Reducing index size requires algorithmic
changes (fewer indexed trigrams, truncated body text, alternative index structures),
not improved compression.

### 4.5.2 Frequency Pruning

Writers MAY omit trigrams that appear in a high fraction of indexed documents
(default: ≥50%). These trigrams provide negligible selectivity during query
intersection -- a trigram in every document narrows nothing. Pruning removes
0.3-8.7% of trigrams by count but 3-16% of uncompressed index bytes, since
high-frequency trigrams have the largest posting lists.

The threshold is configurable via the writer's `SearchPruneFreq` option
(default `0.5`). Setting it to `0` disables pruning entirely. The decision
is per-trigram: if `posting_count >= threshold * doc_count`, the trigram is
omitted from the serialized index. A minimum absolute count floor (1000
documents) prevents over-pruning in small archives where common substrings
can reach high relative frequencies despite appearing in few documents.

**Search quality impact:** Queries containing only pruned trigrams (e.g.
searching for "the" when "the" appears in >50% of documents) return no
results, but such queries would have returned nearly every document anyway --
not a useful search result. Queries with at least one selective trigram are
unaffected.

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
encountered. Writers MUST NOT set bit 0 unless they use character-aligned grams;
readers MUST NOT assume character-aligned grams unless bit 0 is set.

### 4.7 Trade-offs

- **No stemming.** "running" won't match "run". Intentional -- stemming is
  language-dependent and complex. Users search for stems manually.
- **No BM25 ranking.** Title-match vs body-match tiers provide relevance signal.
  For offline archives, finding the right article matters more than ranking order.
- **High-frequency trigrams pruned.** Common substrings like "the", "ing" that appear
  in ≥50% of documents are omitted from the index by default. This trades a small
  amount of recall (queries using only ubiquitous terms) for 3-16% index size savings.
- **False positives < 5%** for queries longer than 4 characters. Can be eliminated by a
  verification pass against actual content.

### 4.8 Derived-Artifact Semantics

SEARCH_TITLE and SEARCH_BODY are **derived artifacts**: their contents are fully
determined by the ENTRY_TABLE and CONTENT sections. They carry no primary data that
cannot be reconstructed.

Consequences for readers and writers:

**Readers MAY regenerate search indices from CONTENT.** If a search section is absent,
corrupt (section-level checksum fails), or too large for available storage, a reader MAY
build a fresh index from the ENTRY_TABLE and CONTENT sections. The regenerated index is
semantically equivalent to the original; no special handling or downgrade warning is
required.

**Writers of incremental updates SHOULD only re-index changed entries.** When producing
an updated archive where a subset of entries has changed, a writer SHOULD rebuild only
the posting-list contributions of modified entries rather than re-indexing the full
corpus. An entry is considered changed if its `content_hash` differs from the previous
version. Unchanged entries retain their existing posting lists; the writer merges the
delta into the existing index.

**On-device index generation.** A reader on constrained storage MAY omit search sections
entirely when writing a redistributed or stripped archive, and regenerate them locally
on first use. Implementations SHOULD store externally generated indices separately from
the archive (e.g., in a sidecar file or local cache) rather than modifying the original
`.oza` file.

**Repair.** A corrupted SEARCH_TITLE or SEARCH_BODY section does not render an archive
unreadable. Readers SHOULD fall back to linear scan or absent-index behavior and
regenerate the section if storage permits.

---

## 5. Compression Strategy

### 5.1 Zstd (and Brotli)

Two codecs are supported. Legacy formats are deliberately excluded.

- **Zstd** (compression byte `1`, dict-variant `2`) — the primary codec.
  Decompresses 5-10× faster than XZ at comparable ratios; first-class dictionary
  support; pure implementations exist in Go, Rust, JavaScript, Python, Java.
  LZMA's only advantage is ~5-10% better ratio at ultra settings, not worth the
  10× slowdown.

- **Brotli** (compression byte `3`) — used as a complement to Zstd on text
  chunks. The reference writer trial-compresses non-dict text chunks with both
  Zstd and Brotli and keeps whichever produces the smaller output. Brotli often
  beats Zstd on small text chunks where its built-in static dictionary helps.
  Pure-Go decode via `github.com/andybalholm/brotli`.

Excluded: zlib, bzip2, XZ/LZMA.

### 5.2 Recommended Levels

| Content type | Strategy |
|-------------|----------|
| HTML/text | Zstd level 19, with dictionary for small articles |
| CSS/JS | Zstd level 19, with dictionary |
| JPEG/WebP | Store uncompressed (already compressed) |
| GIF | Transcode to WebP via `gif2webp` if available; store uncompressed |
| PNG | Transcode to lossless WebP via `cwebp` if available; store uncompressed |
| SVG | Zstd level 19 |
| Video | Store uncompressed (single-blob chunks) |

### 5.2.1 Image Transcoding

Writers MAY transcode GIF and PNG content to WebP before storage using external
tools from libwebp. This is a pre-storage transform, not a compression step --
the resulting WebP bytes are stored uncompressed in image chunks (WebP is already
a compressed format).

**Tools used:**

| Source format | Tool | Command | Target |
|---|---|---|---|
| GIF (static + animated) | `gif2webp` | `gif2webp -q 75 -m 4 input -o output` | image/webp |
| PNG | `cwebp` | `cwebp -lossless input -o output` | image/webp |

**Install libwebp:**

- macOS: `brew install webp`
- Ubuntu/Debian: `sudo apt install webp`
- Fedora/RHEL: `sudo dnf install libwebp-tools`

**Behaviour:**

- Tool discovery via `PATH` at converter startup. If tools are not found,
  original formats are preserved (no error).
- Per-entry size comparison: if the WebP output is larger than the original,
  the original is kept. This handles edge cases where GIF is already optimal.
- The original entry **path is preserved** (e.g. `Molecule.gif`); only the MIME
  type changes to `image/webp`. Readers serve content by MIME type, not extension.
- Controlled via `--transcode` flag: `auto` (default, use if found), `off`
  (never transcode), `require` (fail if tools missing).

**Observed impact:** On wp_en_chemistry_maxi (586 GIFs, 168 MiB), GIF→WebP
transcoding saves 30-50% of GIF bytes.

### 5.3 Dictionary Training

Writers train dictionaries on 100-1000 representative samples using `zstd --train`.
Dictionaries are most valuable for archives with many small, similar entries (Wiktionary:
millions of 2-5 KB entries). For Wikipedia with its long articles, dictionaries provide
marginal benefit but don't hurt.

### 5.4 Dictionary Trial Compression

A trained dictionary is not always beneficial. For small archives, the dictionary
storage cost (~1 MB) can exceed the compression savings, bloating the output file.
The reference writer (`ozawrite`) uses **trial compression** to make an exact
break-even determination:

1. After training a dictionary for a MIME group, build trial chunks from the
   pending entries (up to 8 chunks at `ChunkTargetSize`).
2. Compress each trial chunk **with** the dictionary and **without** it, using the
   same `compressZstd` function and compression level as production.
3. Compute the total cost: `with_dict = sum(compressed_with) + len(dictionary)`.
4. If `with_dict >= without_dict`, discard the dictionary. The group's chunks
   will be compressed with plain Zstd instead.

This is not a heuristic -- it measures actual compression on representative data.
The trial adds ~2x compression time for the sample chunks (not the whole archive),
which is negligible relative to total conversion time.

**Observed impact:** On the ray_charles test archive (328 entries, 2.7 MiB ZIM),
trial compression discards the dictionary and reduces the OZA/ZIM size ratio from
1.24 (OZA 24% larger) to 0.92 (OZA 8% smaller). On larger archives (50+ MB),
dictionaries are retained because the per-chunk savings exceed storage cost.

**Note:** Dictionary compression is only applied to content chunks, not to search
index sections. See §4.5.1 for why search indices do not benefit from dictionaries.

---

## 6. Integrity and Security

### 6.1 Three-Tier Checksums

**File-level:** SHA-256 of everything before the trailing 32-byte checksum. One pass
over the file for quick verification.

**Section-level:** SHA-256 of each section's on-disk bytes, stored in the section
descriptor. Verify any section independently.

**Entry-level:** Each entry record carries a `hash_algorithm` discriminator and a
`content_hash` field (§3.6). The default algorithm (0) is xxhash64 — fast (5-10× SHA-256)
and sufficient for error-detection against accidental bit corruption. Alternative
algorithms (e.g. blake3-64) are selectable via `hash_algorithm`. All algorithms are
non-cryptographic in this context and provide no tamper-resistance against adversarial
modification; for tamper-resistance, rely on the SHA-256 tiers above.

If the file-level check fails, drill into section-level, then entry-level to localize
the damage. Compare this to ZIM's single MD5: "something's wrong somewhere."

### 6.2 Signatures

Optional Ed25519 signatures live in a **trailer appended after the 32-byte file
checksum**, not as an entry in the section table. This ordering is required: each
signature signs the file-level SHA-256, so the signatures MUST come after it on
disk.

Trailer layout (variable size; only present when `has_signatures` header flag is set):

```
4 bytes: signature_count (uint32, little-endian)

Per signature (128 bytes):
  32 bytes: public_key      (algorithm-specific; for Ed25519, the 32-byte public key)
  64 bytes: signature       (algorithm-specific; for Ed25519, the 64-byte signature over the file-level SHA-256)
  4 bytes:  key_id                    (uint32, little-endian — implementation-defined key identifier)
  1 byte:   key_algorithm             (0 = Ed25519; all other values reserved for future algorithms)
  1 byte:   role                      (0 = creator, 1 = distributor, 2 = verifier; see below)
  2 bytes:  key_uri_length            (uint16, little-endian; 0 = no URI present)
  23 bytes: key_uri                   (UTF-8 URI; key_uri_length bytes used, remainder MUST be zero)
  1 byte:   parent_signature_index    (0xFF = none; otherwise the index of the parent signature in this trailer — see below)
```

The signed payload is the file SHA-256, not the raw file bytes. Signatures can be
verified without re-reading the entire file if the hash is already known.

**`key_algorithm`.** The single-byte algorithm tag provides forward agility without a
format version bump. The only currently defined value is `0` (Ed25519). Writers MUST set
`key_algorithm = 0` when producing Ed25519 records. Readers encountering an unknown
`key_algorithm` value MUST skip that signature record rather than failing; they SHOULD
report that an unrecognised algorithm was skipped.

**`role`.** Declares the signer's relationship to the archive:

| Value | Role         | Meaning                                              |
|-------|--------------|------------------------------------------------------|
| `0`   | `creator`    | Original author or content producer                  |
| `1`   | `distributor`| Mirror or hosting organisation that redistributes    |
| `2`   | `verifier`   | Third-party auditor that endorses the content        |

All other `role` values are reserved; readers MUST NOT treat an unrecognised role as an
error — they SHOULD ignore the role field and process the signature normally.

**`key_uri` / `key_uri_length`.** An optional UTF-8 URI where the public key can be
fetched (e.g. `https://keys.example.org/pub/abc123.pub`). If no URI is present,
`key_uri_length` MUST be 0 and all 23 `key_uri` bytes MUST be zero. When
`key_uri_length > 0`, only the first `key_uri_length` bytes carry the URI; the remainder
MUST be zero-padded. `key_uri_length` MUST NOT exceed 23. Readers MUST NOT fetch the URI
automatically; it is provided as a hint for out-of-band key retrieval.

**`parent_signature_index`.** Encodes an ordered custody chain over the signature
trailer. Each signature record names at most one parent — the signature it directly
endorses — forming a linked list. The reserved value `0xFF` means **no parent**: the
record is a chain root (typically the content creator). Any value `0 ≤ N ≤ 0xFE`
references the signature at trailer index `N`.

Chain invariants:

- `parent_signature_index` of every record MUST be either `0xFF` or strictly less than
  the record's own trailer index. This makes cycles structurally impossible and
  guarantees a topological order: a signature can only endorse a predecessor that was
  already written.
- Because every parent index is strictly less than its child's index, well-formed
  trailers cannot produce dangling references through honest truncation: removing
  signatures from the end of the trailer preserves the chain invariants for every
  surviving record. A `parent_signature_index` that violates the invariants — equal
  to or greater than the record's own index, or `≥ signature_count` while not equal
  to `0xFF` — indicates either a malformed writer or post-publication trailer
  tampering. Readers MUST NOT crash on such records; they SHOULD treat the chain
  link as broken (verify the signature itself in isolation) and SHOULD report the
  invariant violation.
- The trailer's maximum chain length is 255 (the 0xFF sentinel reserves one value).
  This far exceeds any realistic custody chain depth.

The chain expresses **who endorses what**, not whose signature is required for
acceptance. A reader's trust policy decides which roots, which roles, and which path
lengths are acceptable. Common policies include "at least one valid signature from a
trusted creator" (ignore the chain) and "every chain from a trusted creator to the
local mirror must verify" (walk the chain bottom-up).

When a mirror or aggregator appends its own signature, it SHOULD set
`parent_signature_index` to the trailer index of the signature it is endorsing —
typically the previous distributor or the original creator. A root-of-its-own
endorsement (no relationship to existing signatures) sets `parent_signature_index = 0xFF`.

OZA does not define a PKI. Key distribution is out of scope. A reader obtains
trusted public keys externally (config file, well-known URL, TOFU).

### 6.3 Content Sandboxing Guidance

Not a format feature, but the spec recommends:
- Set `Content-Security-Policy: sandbox` on HTML responses
- Disable JavaScript execution by default
- Block external resource loading in offline mode

### 6.4 Signature Security Model

The signature trailer's position after the file SHA-256 has deliberate security
consequences that readers and distribution systems MUST understand.

**Threat model.** OZA signatures authenticate publisher identity; the file SHA-256
handles integrity. Signatures answer "who vouches for this content?" not "has this file
been modified?". The security model holds only when trusted public keys are obtained
through an out-of-band channel.

**Stripping attacks.** An attacker with write access to an OZA file can suppress all
signatures in two ways:

1. *Flag-clear strip:* clear the `has_signatures` header flag and recompute the 32-byte
   file SHA-256 (the header is inside the checksum window). This produces a structurally
   valid, signature-free file. Detection requires an out-of-band known-good hash or a
   policy that refuses unsigned files.

2. *Count-zero strip:* keep `has_signatures` set but write `signature_count = 0` in the
   trailer. The trailer lies outside the checksum window, so no SHA-256 recomputation is
   required. The result is a file that asserts signatures are present but carries none.

To defend against count-zero stripping:

- A reader that requires at least one valid signature MUST NOT treat
  `has_signatures = 1, signature_count = 0` as a signed file.
- Distribution systems that require signatures SHOULD verify that at least one signature
  from a trusted key is present at ingestion.
- Applications enforcing a strict "must be signed" policy SHOULD reject files where
  `has_signatures` is set but no signature verifies against a trusted key.

**Appending signatures.** Because the trailer lies outside the checksum window, a mirror
or aggregator MAY append its own signature without altering the file SHA-256 or any
existing signature. To do so: increment `signature_count` and append one 128-byte
signature record carrying the public key, an Ed25519 signature over the existing
file-level SHA-256, the key ID, the agility fields (`key_algorithm`, `role`,
`key_uri_length`, `key_uri`), and `parent_signature_index` — either `0xFF` for an
independent endorsement or the trailer index of the signature being endorsed (typically
the most recent distributor or the original creator). No change to the header or the
checksum is required. This is intentional: it allows third-party endorsement after
publication and lets each endorser record its position in the custody chain.

**Truncation.** If `has_signatures` is set but fewer than 4 bytes follow the file
SHA-256, the reader MUST treat the file as malformed and refuse to open it.

**Malformed trailer.** If `signature_count` is present but fewer than
`signature_count × 128` bytes follow it, the reader MUST treat the file as malformed and
MUST NOT open it.

If `signature_count = 0` and `has_signatures` is set, the trailer is structurally valid
but carries no signatures; readers MUST NOT treat this state as equivalent to a
positively-verified file.

Invalid Ed25519 data of correct length is a verification failure, not a format error.
Readers SHOULD report which signatures verified and which did not; whether to warn,
refuse, or continue is policy-defined.

---

## 7. Chrome/UI Separation

### 7.1 The Contract

Article content in the CONTENT section is **pure content**. No application shell, no
navigation framework, no search forms, no dead `Special:Search` links, no Vue components
that require MediaWiki's JS. If an article references `style.css`, that file is a content
entry.

### 7.2 Chrome Section

The optional CHROME section contains UI assets that a reader **MAY** use:

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

Writers MUST produce self-contained HTML:
- No references to `Special:*` or any CMS-specific URLs
- No components requiring external JS bundles to render
- All internal links are relative paths
- CSS/JS dependencies included as content entries

---

## 8. Comparison

| Aspect | ZIM v5/v6 | OZA v1 |
|--------|-----------|--------|
| Header | Fixed 80 bytes, no extensibility | 128 bytes + variable section table |
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
    // 0x00415A4F -> OZA  ("OZA\x00" on disk, little-endian)
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

Test vectors are planned but not yet included. The intended reference `test.oza` file
will contain:

- 4 entries: one HTML article, one CSS file, one redirect, one metadata entry
- 2 chunks: one Zstd-compressed HTML chunk, one uncompressed CSS chunk
- Trigram index covering the HTML article
- All checksums pre-computed
- Small enough to include as a hex dump (< 4 KB)
- Accompanied by a JSON file listing expected parse results for every field

Any implementation that correctly parses the reference file and produces the expected
JSON will be considered conformant.

---

*This document is a design proposal, not a ratified standard. It reflects what we would
build if starting from zero with two decades of hindsight about what works, what doesn't,
and what the offline content community actually needs.*

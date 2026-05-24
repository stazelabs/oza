# OZA Format Critique

> A constructive teardown of OZA Draft v0.1. Written against the March 2026 spec with the goal of surfacing structural decisions worth revisiting before the format solidifies at v1.

---

## Overview

OZA is meaningfully better than ZIM across nearly every dimension: extensibility, integrity, compression, and search are all genuine improvements. The critique that follows is not a verdict on whether to ship — it is an attempt to name the places where the design stops short, where stated goals and actual mechanisms don't quite align, and where decisions made for good local reasons may create compounding costs over a longer horizon.

The critique is organized by concern area, not severity. Some items here are straightforward spec wording gaps; others are architectural decisions worth revisiting before v1 freezes. They are not weighted equally.

---

## 1. Versioning and the Extension Model

### 1.1 The magic number encodes the major version

The file magic is `0x01415A4F`, which decodes as the bytes `O`, `Z`, `A`, `0x01` in little-endian order — effectively "OZA version 1" baked into the identity bytes. A v2 file would need a different magic (`0x02415A4F`?), which means every tool doing a magic-byte check (`file(1)`, `xxd | head`, OS association rules, naive format detectors) would misidentify v2 files as "unknown" rather than "OZA, newer version."

The conventional approach is to separate format identity from format version: a fixed 3- or 4-byte signature that is stable forever (`OZA\x00` or a non-printable sentinel), followed by a separate version field. This way `magic == OZA_MAGIC` is always the first test, and `major_version` is the second.

### 1.2 No "required" signal in section flags

The extension model (unknown sections are skipped) is well-designed for optional enhancements. The problem is that it has no way to express the opposite: a section that a reader *must* understand to operate correctly.

Consider a plausible v1.1 design decision: deprecating IDX1 in favor of a compact B-tree index (IDX2) that is smaller and faster. A writer produces an archive with IDX2 only. An old reader sees an unknown section type, skips it, and then operates without any path index — silently degrading. The reader has no way to distinguish "missing optional feature" from "I am missing something critical for correctness."

The 4-byte `flags` field in section descriptors is fully reserved. At minimum, one bit should mean "reader must understand this section type; if it does not, it must reject the file rather than silently skip." This is how EPUB handles required extensions (`required-namespace`), and it is how ZIP64's extra fields work (`requires` bit in the local file header).

### 1.3 Major-version rejection without a fallback path

The spec says readers reject files where `major_version != 1`. This is the right behavior for a reader that cannot interpret the file. But the spec gives no guidance on:

- Whether v2 readers are expected to read v1 files
- What version number a "minor format update" that adds a new section type should use
- What the upgrade path is when v1 limits are hit (e.g., the 2³¹ entry cap)

ZIM went through exactly this: extension attempts were made inside v5 files using overloaded fields, because the major-version bump path was too disruptive. The spec should state its version philosophy before v1 freezes, even if the statement is just "breaking changes require a new major version; minor versions are additive only; v(N+1) readers SHOULD accept vN files."

---

## 2. Entry Identity and Addressing

### 2.1 Entry IDs are array indices with no stability guarantee

Entry IDs in OZA are implicit: an entry's ID is its zero-based index into the offset table in the ENTRY_TABLE section. This gives O(1) access by ID, which is valuable. The cost is that IDs are unstable across archive versions.

A monthly Wikipedia rebuild will reassign every entry ID — entries added, removed, or reordered produce entirely different ID assignments. This makes entry IDs useless for:

- **Stable bookmarks**: a reader saving "entry 4,823,017" as a user's reading position cannot carry that bookmark across the next archive update
- **Cross-archive references**: the `catalog` metadata key allows bundling multiple archives, but there is no mechanism to link from an entry in archive A to an entry in archive B by ID
- **Incremental diffing**: comparing two versions of the same archive requires matching by path, not by ID

The fix does not require changing the on-disk format: defining a canonical "persistent ID" as the NFC-normalized path, and documenting that entry IDs are transient reader-internal identifiers, would be sufficient. Alternatively, an optional STABLE_IDS section could carry UUID-per-entry or content-addressed identifiers for archives that want long-term link stability.

### 2.2 Redirect targets cannot cross archives

The 5-byte redirect record carries a `target_id` that references a content entry in the same archive. Bundled archives (via the `catalog` metadata key) have no way to express a redirect that crosses archive boundaries. For large wiki archives that are split by language or topic and then bundled, cross-archive redirects are a real use case that the current model does not serve.

### 2.3 Path uniqueness is advisory, not required

The spec does not contain a `MUST` requiring path uniqueness within an archive. Duplicate paths would produce undefined behavior in the PATH_INDEX (front-coded indices assume a sorted unique key set) and ambiguous results for path lookups. This should be a `MUST`-level constraint with a corresponding writer-side validation requirement.

### 2.4 No per-entry modification timestamp

The archive-level `date` metadata key captures when the archive was created or last updated. Individual entries carry no timestamp. For archival purposes — particularly for archives derived from sources that change at different rates (a wiki with 6M articles, each last modified on a different date) — the absence of per-entry timestamps is a meaningful gap. An optional `mtime` varint (Unix epoch seconds) in the entry record would cost ~1–4 bytes per entry and enable "show me what changed since date X" queries without full content comparison.

---

## 3. Metadata

### 3.1 The required key set is minimal for archival cataloging

The five required metadata keys (`title`, `language`, `creator`, `date`, `source`) are sufficient for basic display but fall short of what archival catalogs, digital libraries, and content-distribution systems expect. Missing from the well-known key set or required list:

- **`identifier`**: a stable, externally-recognized identifier (DOI, ISBN, ISSN, URN, ISNI). A Wikipedia archive has a clear stable identity that should be expressible.
- **`publisher`**: distinct from `creator` in standard bibliographic models (the Wikimedia Foundation publishes Wikipedia; a scraper tool creates the archive).
- **`relation`**: Dublin Core's relation field (`Is-Version-Of`, `Is-Part-Of`, `Replaces`, `Requires`) is essential for expressing relationships between archive versions and between sub-archives within a bundle.
- **`rights`**: machine-readable rights statement. `license` is listed as optional (SPDX), but rights and license are different: rights covers access restrictions, license covers reuse terms.

None of these need to be *required*, but their absence from the well-known key table means implementations will invent them independently with incompatible spellings.

### 3.2 `date` format is underspecified

"ISO 8601" is not a single format — it is a family. Valid ISO 8601 date representations include: `2024`, `2024-01`, `2024-01-15`, `2024-01-15T00:00:00`, `2024-01-15T00:00:00Z`, `2024-01-15T00:00:00+05:30`. Parsers that expect a full datetime will fail on a year-only string. Parsers that expect a date-only string will fail on a datetime.

The spec must define the required precision and timezone handling. Recommended: `YYYY-MM-DD` for date-only, or `YYYY-MM-DDThh:mm:ssZ` for full datetime, both UTC. Any other representation is a conformance error.

### 3.3 `main_entry` and `favicon_entry` use different referencing mechanisms

`main_entry` is an entry path string; resolving it requires a PATH_INDEX lookup. `favicon_entry` is a uint32 entry ID; resolving it is O(1). These are equivalent concepts — "the entry this archive wants to present as its primary resource" and "the entry for the archive's icon" — but they use different mechanisms with different lookup costs, different stability properties, and different failure modes when the referenced entry is absent.

This inconsistency was recently caught as a real bug (OZA-34 fixed a case where `main_entry` was mistakenly treated as a uint32). The right fix is a consistent referencing convention. Either both use paths (stable across ID renumbering, requires index), or both use IDs with a documented caveat about ID stability. Pick one.

### 3.4 No metadata namespacing

The metadata key space is flat. Two downstream communities independently adding an `origin` key — one meaning "source URL," the other meaning "geospatial origin" — will produce files that appear identical in structure but mean different things. There is no namespace prefix, no reverse-domain convention, no IANA-style registry.

A lightweight convention (`dc:title`, `schema:author`, `x-mykiwix:scrape_version`) costs nothing in the format and prevents silent semantic conflicts. The spec should establish the convention and reserve the unprefixed namespace for spec-defined keys only.

### 3.5 Single language field for multilingual archives

`language` is a single BCP-47 string. A multilingual wiki, a polyglot documentation site, or a bundled archive covering multiple languages cannot express its language coverage in a machine-readable way. There is no per-entry language field. The practical workaround (semicolon-separated list in the value, or multiple archives in a bundle) is not specified, so every tool will do something different.

---

## 4. Integrity and Security

### 4.1 xxhash64 is error-detection, not tamper-detection

The entry-level `content_hash` field uses xxhash64, described as "fast, non-cryptographic" for "per-entry tamper-detection + dedup." The second purpose is the problem: xxhash64 has excellent avalanche properties for random errors, but a determined attacker can trivially construct a byte string with any desired xxhash64 value. Calling this tamper-detection in the spec invites users to trust it against adversarial corruption, which it does not provide.

For an archive format that intends to support signing and distribution through untrusted mirrors, entry-level integrity should use a cryptographic primitive. Blake3 at 32 bytes per entry would cost ~130 MB for a 4M-entry Wikipedia archive — comparable to a section index. Alternatively, the spec could be explicit: "`content_hash` provides error-detection against accidental corruption only; for tamper-resistance, verify the file-level SHA-256 or section-level SHA-256."

### 4.2 The Ed25519 signature model is incomplete where it matters most

The signatures trailer provides the right cryptographic primitive (Ed25519 over the file SHA-256) but punts the non-cryptographic problem: key distribution. "Out-of-spec (config, TOFU, etc.)" is not a distribution story — it is a deferral that ensures every implementation will invent an incompatible solution.

More concretely:

- **No key type agility**: the 128-byte record allocates exactly 32 bytes for the public key and 64 bytes for the signature — both Ed25519-specific. If Ed25519 is ever deprecated (as DSA was), the record format cannot accommodate a different algorithm without a format version bump.
- **No key identifier URI**: readers have no spec-defined way to fetch an unknown public key. `key_id` is uint32 and "implementation-defined."
- **No purpose/role field**: is this signature from the content creator, the format converter, the distribution mirror, or an independent verifier? A 1-byte role field would enable trust-level differentiation.

The 28 reserved bytes per signature record provide ample room for a `key_uri` (max 24 bytes), `key_algorithm` (1 byte), and `role` (1 byte) without changing the record size.

### 4.3 No ordered custody chain

The signatures trailer supports multiple independent signatures but not ordered chains. Archival provenance often requires expressing: "The Wikimedia Foundation created this content, Kiwix converted it to OZA, distributed.kiwix.org mirrored it, and this reader received it from cdn.example.com." A flat array of signatures cannot express this relationship. An optional `parent_signature_index` field (or a linked-list structure) would enable provenance chains without requiring a new section type.

### 4.4 UUID does not establish stable archive identity

UUID v4 is random. The spec provides no guidance on whether archives that are monthly updates of "the same" archive should share a UUID prefix, a deterministic UUID derived from the archive title and language, or something else. In practice, monthly Wikipedia rebuilds will generate a new random UUID each time. The `uuid` field therefore identifies a specific file, not a content identity — which means it cannot be used for "this archive supersedes UUID X" relationships, archive deduplication across mirrors, or user-facing "you already have this archive" detection.

A deterministic UUID (v5, SHA-1 of a namespace + archive identifier) for the content identity, separate from a random UUID for the specific build artifact, would serve both purposes.

### 4.5 The header-flag / trailer interaction is not security-analyzed

The file SHA-256 covers bytes 0 through `checksum_offset`. The signatures trailer appears after `checksum_offset + 32`, and its presence is signaled by `header.flags & has_signatures`. An attacker who strips the signature trailer must also clear `has_signatures` in the header and recompute the file SHA-256 — which requires write access to the beginning of the file. The spec does not analyze this attack, does not state whether stripping signatures is a detectable operation, and does not specify what readers should do if `has_signatures` is set but the trailer is absent or malformed. All three cases need explicit treatment.

---

## 5. Index Design

### 5.1 Title sorting is not Unicode-aware

IDX1 sorts titles by raw UTF-8 byte value. This is deterministic and works correctly for ASCII. For any non-Latin script, the result is semantically wrong:

- **Japanese**: should sort by reading (yomi/furigana), not by Unicode codepoint; `亜` (U+4E9C) and `阿` (U+963F) are both read "a" but sort very differently by codepoint
- **Arabic**: should sort by root form, not surface form; diacritics affect codepoint order but not alphabetical position
- **Chinese**: acceptable orderings include pinyin, stroke count, or radical — none of which correspond to Unicode order

For a format serving offline multilingual content as a primary use case, this is not a minor issue. The spec should either define a collation strategy (Unicode CLDR root collation order with optional locale override) or store precomputed sort keys in the index alongside display titles, leaving the title field unchanged. Storing sort keys is the pragmatic choice: it requires no runtime CLDR library in the reader.

### 5.2 Redirect entries have no self-contained path

The 5-byte redirect record contains `flags` and `target_id`. A redirect entry's own path is only findable via PATH_INDEX lookup. PATH_INDEX is optional ("recommended but not required"). This creates an inconsistency: redirects are first-class entries with their own IDs, they are counted in `header.redirect_count`, they appear in `front_article_count` — but their addresses are not self-contained. A reader without a path index cannot resolve a redirect's own URL, only its target.

Either PATH_INDEX should be required whenever redirects exist, or redirect records should carry enough information to identify themselves without the index.

### 5.3 No secondary indices for large-archive browsing

IDX1 provides forward prefix search (find all paths beginning with `/wiki/A`) and sorted enumeration. There is no mechanism for:

- **Suffix search**: find all `.html` entries, find all entries under a given directory
- **MIME-type filtered enumeration**: "list all images" requires scanning the full ENTRY_TABLE (O(N) for a 4M-entry archive)
- **Date range queries**: impossible without per-entry timestamps, but worth noting

For small archives, full-table scans are acceptable. For a 90 GB Wikipedia archive with 20M entries, the absence of secondary indices pushes readers toward building their own application-layer indices, which fragments the ecosystem. An optional MIME_INDEX section (MIME ID → sorted list of entry IDs) would cost roughly 80 MB for Wikipedia and enable O(log N) MIME-filtered access.

### 5.4 Trigram indices are build-once artifacts with no incremental path

Rebuilding SEARCH_BODY for a full Wikipedia archive produces 200–400 MB of index data. The spec does not acknowledge that trigram indices are derived artifacts (rebuildable from the entry content) rather than primary data. Readers that want to save disk space might want to regenerate them on-device; writers of incremental updates would want to avoid full rebuilds.

The spec should state explicitly that search indices are optional derived artifacts, that readers MAY rebuild them, and that writers of updated archives SHOULD provide them pre-built but are not required to re-index entries that did not change.

---

## 6. Content Section and Chunking

### 6.1 No reverse mapping from chunk to entry

Chunk descriptors contain `chunk_id`, `compressed_offset`, `compressed_size`, and compression metadata — but no list of which entries they contain. To find all entries in chunk N, a reader must scan the entire ENTRY_TABLE. For a 4M-entry archive, this is a linear pass through potentially hundreds of megabytes of entry records to answer "what is in this chunk?"

This matters for:
- **Chunk-level integrity verification** without reading all entry records
- **Progressive rendering**: a browser-style reader wants to know which chunk to prefetch for entries near the one currently displayed
- **Repair**: re-downloading a corrupt chunk requires knowing which entries to re-serve

An optional `entry_count` field in the chunk descriptor (cost: 4 bytes each, zero for "unknown") would at minimum enable readers to choose cache strategies. An optional CHUNK_MANIFEST section that stores (chunk_id → [entry_id, ...]) mappings would enable all three use cases above.

### 6.2 Section ordering is unconstrained and undocumented

Sections may appear at any file offset; the section table contains the offsets. The spec defines no canonical ordering, no RECOMMENDED ordering, and no signal to indicate whether a file is "streaming-friendly" (metadata and indices before content) or "append-built" (content first, indices at the end).

A reader serving an archive over HTTP or from a slow medium needs METADATA before it can answer any request, PATH_INDEX before it can route, and CONTENT last. A writer optimizing for sequential reads would produce this ordering naturally, but nothing in the spec requires or recommends it. A `STREAMING` header flag (bit 3 in `header.flags`) costing zero bytes of new format space could signal "sections appear in streaming-optimal order" and allow readers to make one forward pass rather than seeking back to the section table repeatedly.

### 6.3 Chunk decompression is all-or-nothing

Accessing any entry in a chunk requires decompressing the entire chunk and then slicing out `[blob_offset, blob_offset + blob_size]`. This is a standard tradeoff in block-compressed formats (LevelDB SSTs, Parquet row groups), and the compression ratio benefit is real. But the spec gives no guidance on chunk sizing relative to expected access patterns:

- Text chunks sized at 4 MB uncompressed may contain thousands of small HTML articles; decompressing 4 MB to read a 2 KB article is a 2000x read amplification
- The spec recommends "large media: single-blob chunks, uncompressed" — this is correct but applies only to media; the same reasoning should be applied to very large text articles

Adding an optional `entry_count` hint in chunk descriptors would let readers distinguish "one large blob" from "many small blobs" and apply appropriate caching policy (see also 6.1).

---

## 7. Long-term Sustainability

### 7.1 Fixed MIME indices encode a web-content assumption

The MIME table spec mandates that indices 0, 1, 2 are `text/html`, `text/css`, and `application/javascript` — always, for every archive. A pure dataset archive (CSV files, JSON records, Parquet files), a PDF archive, or a binary asset collection has no use for these three entries but must include them. The fixed indices are an optimization for the dominant use case (web content) imposed on all archives.

A cleaner model: the MIME table is fully dynamic (no fixed entries), and archives that do contain HTML/CSS/JS simply list them. The optimization for common MIME types belongs in the chunk grouping strategy, not in a fixed-index convention.

### 7.2 `content_hash` has no algorithm field

`content_hash` is fixed xxhash64 (8 bytes) with no algorithm identifier. If the entry integrity model is ever upgraded — to Blake3, truncated SHA-256, or a new hash — there is no field in the entry record to carry the algorithm choice. Every entry record would need to be rewritten, which requires a format version bump for what is otherwise a backward-compatible change.

A 1-byte `hash_algorithm` field in the entry record (0 = xxhash64, 1 = Blake3-64, etc.) costs 1 byte per entry and provides indefinite algorithm agility. Since entry records already use varint encoding, an additional varint before `content_hash` would cost 1 byte in the common case.

### 7.3 Compression algorithm list is hard-coded in the spec

Compression codes 0–3 are defined inline in the spec text, mirrored in every implementation. Adding LZ4 (faster decompression), Zstd-seekable (random access within a stream), or a future codec requires a spec revision and corresponding implementation updates across every reader.

A CODEC_TABLE section (analogous to MIME_TABLE: count + length-prefixed strings identifying codec names, e.g., `"zstd/1"`, `"brotli"`) would allow codec discovery from the archive without spec changes. The compression byte in section descriptors and chunk descriptors would then be a CODEC_TABLE index rather than a hardcoded enum. Cost: one additional optional section, one small lookup on open.

### 7.4 Section type `0x0100+` has no registry or namespace

The spec reserves `0x0100+` for extensions. Two organizations independently adding extension section types with the same value will produce archives that are mutually incompatible, with no mechanism for detection or disambiguation. There is no IANA-style registry, no vendor-prefix convention, and no spec-level process for allocating new section types.

A lightweight convention (even just a documented URL where unofficial registrations are tracked) costs nothing and prevents the worst outcomes. The spec should establish this before extensions proliferate.

### 7.5 The format migration story is undefined

The spec does not address:
- Whether v2 readers are expected to read v1 files
- Which fields in a v1 archive are guaranteed to be stable across a major version
- What the conversion path is for archives that hit v1 limits (2³¹ entries)
- What happens to extension sections (`0x0100+`) in a v2 archive

None of this needs to be resolved before v1, but the intent should be stated. "vN+1 readers MUST accept vN files for at least one major version" is a reasonable default. Without a stated policy, the ecosystem will fracture when v2 is eventually needed.

---

## 8. Specification Gaps and Ambiguities

These are not architectural concerns but wording gaps that will produce divergent implementations.

**Section cardinality is unspecified.** The spec does not say whether an archive may contain zero, one, or multiple instances of CONTENT (0x0006) or METADATA (0x0001). Two CONTENT sections with overlapping chunk IDs would be ambiguous. Two METADATA sections with conflicting `title` values would be ambiguous. Required sections should be specified as exactly-once; optional sections should specify whether they may repeat (e.g., ZSTD_DICT is explicitly repeatable; others are not stated).

**Chunk ID uniqueness is not a MUST.** Entry records reference `chunk_id`. If two chunks share an ID, the reader's behavior is undefined. This should be: "Chunk IDs within a CONTENT section MUST be unique. Writers MUST NOT produce duplicate chunk IDs."

**`section_table_offset` is unconstrained.** The header stores the section table offset, allowing it to appear anywhere in the file. In practice, every writer will place it immediately after the 128-byte header (offset 128). The spec should either mandate this (simplifying sequential readers and validators) or explicitly allow arbitrary placement and document why.

**`entry_count` vs `redirect_count` vs `front_article_count` have mixed semantics.** `entry_count` excludes redirects; `redirect_count` is separate; `front_article_count` includes both. Adjacent header fields with subtly different counting semantics are a source of persistent off-by-one bugs. The spec should define all three in one place with a clear table showing what each count includes and excludes.

**RFC 2119 keywords are inconsistently applied.** The spec uses "must," "should," "may," and "warn" in lowercase throughout, mixing normative and advisory language without the formalism that makes specs implementable. "Readers warn on non-zero reserved fields" is advisory; it should be "readers MUST emit a diagnostic warning" to be normative. A one-time declaration at the top of the document that all keywords are used per RFC 2119 would make every conformance requirement unambiguous.

---

## Summary

The table below maps each concern to its impact horizon. Items marked **pre-v1** have low fix cost now and high fix cost later; **v1.x** items can be addressed in a backward-compatible minor revision; **design note** items do not require format changes but should inform documentation or tooling decisions.

| Area | Issue | Horizon |
|------|-------|---------|
| Versioning | Magic bakes in version | pre-v1 |
| Versioning | No required-section bit in flags | pre-v1 |
| Versioning | Major-version rejection without migration intent | pre-v1 |
| Entry identity | Implicit IDs, no stable identity | design note |
| Entry identity | No cross-archive redirect targets | v1.x |
| Entry identity | Path uniqueness not `MUST` | pre-v1 |
| Entry identity | No per-entry timestamp | v1.x |
| Metadata | Insufficient archival identity fields | pre-v1 |
| Metadata | `date` format underspecified | pre-v1 |
| Metadata | `main_entry` / `favicon_entry` inconsistency | pre-v1 |
| Metadata | No namespace for custom keys | pre-v1 |
| Metadata | Single language field | v1.x |
| Integrity | `content_hash` is error-detection, not tamper-detection — doc gap | pre-v1 |
| Integrity | Ed25519 missing key agility, URI, role | v1.x |
| Integrity | No custody chain | v1.x |
| Integrity | UUID not a stable archive identity | pre-v1 |
| Integrity | Header-flag / trailer interaction unanalyzed | pre-v1 |
| Indices | Title sort not Unicode-aware | v1.x |
| Indices | Redirect paths require optional index | pre-v1 |
| Indices | No secondary (MIME, date) indices | v1.x |
| Indices | Search indices not documented as rebuildable | pre-v1 |
| Chunking | No reverse chunk→entry map | v1.x |
| Chunking | No streaming-canonical section ordering | pre-v1 |
| Chunking | No chunk entry-count hint | v1.x |
| Sustainability | Fixed MIME indices encode web bias | pre-v1 |
| Sustainability | `content_hash` no algorithm agility | pre-v1 |
| Sustainability | Compression codes hard-wired in spec | v1.x |
| Sustainability | Extension namespace has no registry | pre-v1 |
| Sustainability | Format migration intent undeclared | pre-v1 |
| Spec gaps | Section cardinality unspecified | pre-v1 |
| Spec gaps | Chunk ID uniqueness not `MUST` | pre-v1 |
| Spec gaps | `section_table_offset` unconstrained | pre-v1 |
| Spec gaps | Mixed semantics in count fields | pre-v1 |
| Spec gaps | RFC 2119 keywords not declared | pre-v1 |

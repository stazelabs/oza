# 王座 OZA -- INDICES.md — Split Search Index Architecture

## Context

OZA uses two separate trigram indices for search: one for titles, one for body content. At query time, title matches rank above body-only matches. The title index is small enough (~3-5 MB for Wikipedia-scale) to stay memory-resident for fast autocomplete, while the body index (~200-400 MB) is loaded lazily for full-text search.

---

## Design Decisions

### 1. Two section types

`SectionSearchTitle (0x000C)` and `SectionSearchBody (0x000D)` are distinct section types.

**Why not flags or multiple instances of the same type?** OZA's extensibility model is "unknown section types are skipped." Two distinct types are self-describing. An old reader skips both gracefully. Multiple instances of the same type create ordering ambiguity. Section-level flags would require every reader to understand the flag scheme to distinguish title from body.

### 2. Title index: front articles only, title text only

The title trigram index includes only entries with `EntryFlagFrontArticle` set. It indexes the `title` field only (not title+path).

**Why front articles only?** The title index exists for user-facing search and autocomplete. CSS files, images, JS bundles, and internal resources should never appear in title search results. Redirects are excluded to avoid duplicates. The `isFrontArticle` flag already exists and is reliably set.

**Why title only, not title+path?** With a dedicated title index, path is the wrong search surface. Path lookup is already served by `SectionPathIndex` (exact binary search). Keeping path out makes the title index smaller and results more predictable.

### 3. Body index: all front articles, title+path+content

The body trigram index includes all non-redirect front article entries (any MIME type with `isFrontArticle` set). It indexes title+path and the visible text content. For `text/html` entries, HTML tags and attributes are stripped and `<script>`/`<style>` element content is removed before indexing, so only user-visible text generates trigrams. Non-HTML content is indexed as-is. This future-proofs for archives with non-HTML article formats (plain text, markdown) while still excluding CSS, JS, images, and other resources.

**Why all front articles, not just HTML?** The `isFrontArticle` flag is the authoritative marker for "user-visible content." Filtering by MIME type is a proxy that may miss future formats. The flag is already reliably set by both `ozawrite` and `zim2oza`.

**Why include titles in the body index?** So that body-only search still finds articles by title. The marginal size overhead of title trigrams in a 200+ MB body index is negligible.

### 4. Wire format

Both section types use the same binary format:

```
Header (16 bytes):
  [0:4]   version         uint32 = 3
  [4:8]   flags           uint32 (bit 0: CJK bigram mode; all other bits reserved 0)
  [8:12]  trigram_count   uint32
  [12:16] doc_count       uint32 (distinct entry IDs indexed)

Trigram Table (trigram_count * 12 bytes, sorted lexicographically):
  per entry:
    [3]byte  trigram/bigram key
    1 byte   reserved (0)
    uint32   posting_offset  (byte offset from section start)
    uint32   posting_length  (byte count of posting list data)

Posting Lists:
  Per trigram: a serialized roaring bitmap (portable format) containing entry IDs.
```

- `doc_count` enables UI display of "N articles indexed" and future ranking normalization
- `posting_length` is byte count (not entry count)

#### CJK Bigram Mode (flags bit 0)

When `flags & 1` is set, the index was built with character-aligned grams for CJK
text rather than a raw 3-byte sliding window. The gram key encoding is:

- **Unigram** (single CJK character `c`): `[c[0], c[1], c[2]]` — the character's
  3 UTF-8 bytes, stored directly as the key.
- **Bigram** (adjacent CJK pair `c1`, `c2`): `[c1[last], c2[0], c2[1]]` — the last
  byte of the first character followed by the first two bytes of the second. This is
  the exact cross-character-boundary byte triplet that a raw sliding window would emit
  at that position.

Non-CJK runs within the same document continue to use the standard 3-byte sliding
window. The reader applies the same character-aligned extraction to the query when
bit 0 is set.

Readers that do not understand bigram mode (e.g., old implementations) will still
return correct results, at reduced precision: they will submit more grams per
query (including raw byte-boundary grams not present in the index) and may return
no results for queries that a bigram-aware reader would satisfy. Readers SHOULD
check bit 0 and apply the appropriate query extraction.

### 5. Reader API: two-tier search with ranking

```go
type SearchResult struct {
    Entry      Entry
    TitleMatch bool
    BodyMatch  bool
}

type SearchOptions struct {
    Limit     int  // max results; 0 = default 20
    TitleOnly bool // search only the title index
}

func (a *Archive) Search(query string, opts SearchOptions) ([]SearchResult, error)
func (a *Archive) SearchTitles(query string, limit int) ([]SearchResult, error)
```

Default `Search()` behavior:
1. Search title index -> collect IDs with `TitleMatch=true`
2. Search body index -> collect IDs with `BodyMatch=true`
3. Merge: entries in both get both flags
4. Sort: title matches first (by entry ID), then body-only matches (by entry ID)
5. Apply limit

### 6. Header flag: keep FlagHasSearch as the umbrella

Keep `FlagHasSearch = 1 << 0` as "this archive has some form of search index." The section table is authoritative for which specific indices are present.

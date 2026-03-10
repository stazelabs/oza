# PLAIN_TEXT (0x0101) Section — Design & Implementation Plan

## Context

LLM.md describes PLAIN_TEXT as "The Foundation" — the section everything else depends on. Today, HTML→markdown conversion happens **at runtime** in three separate places:

1. **MCP `read_entry`** — `htmltomarkdown.ConvertString()` on every read ([mcptools.go:596-605](internal/mcptools/mcptools.go#L596-L605))
2. **Search indexing** — `extractVisibleText()` strips to flat text at build time ([htmltext.go](ozawrite/htmltext.go))
3. **Snippet extraction** — `StripHTML()` strips to plain text at query time ([snippet.go:36-91](internal/snippet/snippet.go#L36-L91))

PLAIN_TEXT does this **once at build time** — clean HTML → structured markdown with passage boundaries. The canonical test case is Wikipedia ZIM (~6M articles, complex HTML with infoboxes, citations, math, tables).

---

## HTML Preprocessor — Generic + Source Hints

The preprocessor is **generic by default** with pluggable **source hint profiles** that activate based on archive metadata (`M/Name`, `M/Source`) or HTML structure detection.

### Architecture

```go
// CleanProfile defines source-specific cleaning rules.
type CleanProfile struct {
    Name           string
    // Elements to strip (by tag, class, id patterns)
    StripSelectors []Selector
    // Elements to extract/transform specially
    Transforms     []Transform
    // Auto-detect: return true if this profile matches the HTML
    Detect         func(html []byte) bool
}

// Built-in profiles
var (
    ProfileGeneric    CleanProfile  // script, style, nav, form — always applied
    ProfileMediaWiki  CleanProfile  // mw-editsection, reflist, navbox, infobox extraction, math→LaTeX
    ProfileStackExchange CleanProfile  // vote counts, user cards, comment noise
    // Future: ProfileGutenberg, ProfileVikidia, etc.
)
```

**Profile selection**: At converter init time, detect source type from ZIM metadata. Apply `ProfileGeneric` always, then layer on the matched source profile. If no source is detected, auto-detect from HTML class patterns in the first few entries.

### Generic strip rules (always applied)

| Element | Treatment | Rationale |
|---------|-----------|-----------|
| `<script>`, `<style>` | Strip entirely | No content value |
| `<nav>`, `role="navigation"` | Strip | Navigation chrome |
| `<form>` | Strip | Interactive elements |
| `<noscript>` | Strip | Fallback content |
| Hidden elements (`display:none`, `aria-hidden="true"`) | Strip | Invisible content |
| Images | Strip tag, preserve `alt` text inline | Alt text carries meaning |
| Bold/italic/links | Preserve text, strip link targets | Structural emphasis matters |

### MediaWiki hint profile (Wikipedia, Wiktionary, Vikidia)

| Element | Treatment | Rationale |
|---------|-----------|-----------|
| `<span class="mw-editsection">` | Strip | Edit UI noise |
| `<sup class="reference">` | Strip | Citation `[1][2]` noise |
| `<div class="reflist">`, `<ol class="references">` | Strip | Citation list at bottom |
| `<div id="toc">`, `<div class="toc">` | Strip | ToC (redundant with headings) |
| `class="navbox"`, `class="navbar"` | Strip | Navigation templates |
| `<div id="catlinks">` | Strip | Categories footer |
| `<span id="coordinates">` | Strip | Geo widget |
| `class="infobox"` | **Extract** key-value pairs → `lead` passage | Structured data for LLMs |
| `<div class="hatnote">` | Preserve as italic lead | Disambiguation notices |
| `<math>` + `<annotation encoding="application/x-tex">` | Convert to `$...$` / `$$...$$` | Preserve math as LaTeX |
| `<span class="IPA">` | Preserve text | LLMs interpret IPA |
| Tables | Convert to markdown tables | Acceptable for Wikipedia tables |

### Special page type detection
- **Disambiguation pages** (`class="dmbox"`): Generate PLAIN_TEXT but flag `EntryFlagExcludeRAG`
- **Stub articles** (<100 words): Generate normally, optionally flag `EntryFlagExcludeRAG`
- **List articles**: Preserve — high-value for structured queries

---

## Passage Segmentation

Passages are the atomic unit for RAG retrieval. Target: 100-500 tokens (~400-2000 chars).

### Algorithm
1. The first paragraph before any heading → `lead` passage
2. Lines starting with `#` → `heading` passage (records heading_level 1-6)
3. Content between headings split on double newlines → `paragraph` passages
4. Fenced code blocks (` ``` `) → `code_block` passage
5. Contiguous `|` lines → `table` passage
6. Contiguous `- ` or `N. ` lines → `list_item` passage
7. Passages >2000 chars → split at sentence boundaries (`. ` + uppercase)

Each passage inherits `heading_level` from the most recent heading above it.

### Passage types
```go
const (
    PassageParagraph  PassageType = 0
    PassageHeading    PassageType = 1
    PassageListItem   PassageType = 2
    PassageTable      PassageType = 3
    PassageCodeBlock  PassageType = 4
    PassageCaption    PassageType = 5
    PassageLead       PassageType = 6
)
```

---

## Binary Format

```
PLAIN_TEXT Section (0x0101):

Section Header:
  version          u32   = 1
  entry_count      u32
  total_passages   u32
  flags            u32   bit 0 = is_markdown, bit 1 = has_token_counts
  chunk_count      u32
  tokenizer_len    u16
  tokenizer_id     [N]byte  (empty in v1, reserved for future token counts)
  [padding to 8-byte alignment]

Entry Directory (12 bytes/entry, sorted by entry_id for binary search):
  entry_id         u32
  chunk_id         u32
  chunk_offset     u32

Chunk Table (20 bytes/chunk):
  offset           u64   from start of chunk data area
  comp_size        u64
  uncomp_size      u32

Chunk Data (independently zstd-compressed):

Per-Entry Text Block (within decompressed chunk):
  text_size        u32   total size of this entry block
  passage_count    u16
  Per passage (12 bytes):
    offset         u32   within concatenated text
    length         u16
    type           u8    PassageType enum
    heading_level  u8    0-6
    token_count    u32   (0 if !has_token_counts)
  [concatenated passage text, UTF-8 markdown]
```

**Why chunked (not monolithic)?** For Wikipedia (~30 GB uncompressed markdown), the section can't be fully decompressed at open time. Independently compressed ~1 MB chunks enable lazy loading — only decompress the chunk containing the requested entry.

---

## Implementation

### New files

| File | Purpose |
|------|---------|
| `oza/plaintext.go` | Types (`Passage`, `PlainTextEntry`, `PlainTextIndex`), reader with lazy chunk loading |
| `ozawrite/htmlclean.go` | Generic HTML preprocessor + `CleanProfile` interface + profile registry |
| `ozawrite/htmlclean_mediawiki.go` | MediaWiki hint profile (Wikipedia, Wiktionary, Vikidia) |
| `ozawrite/passage.go` | Markdown → passage segmentation |
| `ozawrite/plaintextbuild.go` | `PlainTextBuilder` — accumulates entries, serializes section |

### Modified files

| File | Change |
|------|--------|
| [constants.go](oza/constants.go) | Add `SectionPlainText = 0x0101`, `FlagHasPlainText = 1 << 4` |
| [writer.go](ozawrite/writer.go) | Add `BuildPlainText` option, feed `PlainTextBuilder` in `AddEntry`, serialize in `Close` |
| [archive.go](oza/archive.go) | Parse `SectionPlainText` in `loadSection`, add `HasPlainText()` / `PlainText()` |
| [mcptools.go](internal/mcptools/mcptools.go) | Prefer pre-computed PlainText over runtime `htmltomarkdown` in `read_entry` + resource handler |
| [convert.go](cmd/zim2oza/convert.go) | Wire `BuildPlainText` option through |

### Pipeline integration point

In `writer.go` `AddEntry()`, after step 4 (search indexing, line 322):
```go
if w.plainTextBuilder != nil && isFrontArticle {
    w.plainTextBuilder.AddEntry(id, mimeType, content)
}
```

Content is already minified at this point. The PlainTextBuilder runs the HTML preprocessor → markdown converter → passage segmenter pipeline.

### Memory strategy

PlainTextBuilder streams entry text blocks to a temp file during `AddEntry`, keeping only 12-byte directory entries in RAM. For 6M Wikipedia articles: 72 MB directory in memory, ~30 GB streamed to disk. During `Build`, reads back from temp file to assemble and compress chunks.

### Read-time fallback

```go
// mcptools read_entry handler
if format == "markdown" && archive.HasPlainText() {
    if text, err := archive.PlainText().ReadText(entry.ID()); err == nil {
        return header + text  // pre-computed, ~100x faster
    }
}
// fall through to runtime htmltomarkdown.ConvertString()
```

Archives without PLAIN_TEXT (older files, non-Wikipedia) still work via runtime conversion. Zero breaking changes.

---

## Verification

1. **Unit tests**: `htmlclean` with real Wikipedia HTML fixtures (infobox, math, citations, navbox)
2. **Unit tests**: `passage.go` segmenter with various markdown structures
3. **Round-trip test**: AddEntry → Build → Parse → ReadText matches expected markdown
4. **Integration test**: Convert `testdata/small.zim` with `BuildPlainText=true`, verify section present and readable
5. **MCP test**: `read_entry` returns pre-computed text when PLAIN_TEXT exists, falls back when it doesn't
6. **Wikipedia test**: Convert a small Wikipedia ZIM (e.g., Simple English Wikipedia), spot-check 10-20 articles for quality
7. **Benchmark**: Pre-computed read vs runtime `htmltomarkdown.ConvertString()` — expect 10-100x speedup

# ZIM Archive Observations

Working notes on ZIM content conventions, conversion decisions, and open questions
encountered while building OZA and ozaserve.

---

## Browse / Entry Visibility

### `IsRedirect()` vs `IsFrontArticle()` — two orthogonal axes

These are independent questions:

- **`IsRedirect()`** — *what kind of entry is this?*
  Redirects have no content of their own; they are pointers to other entries.
  Stored in a separate section (`SectionRedirectTab`). No MIME type, no blob data.

- **`IsFrontArticle()`** — *is this entry user-visible?*
  A flag bit that can be set on *either* content entries *or* redirects. A redirect can
  be a front article (e.g. a canonical alias users would navigate to directly). A content
  entry can be *not* a front article (CSS, images, auxiliary pages).

Matrix:

|                    | `IsFrontArticle=true`              | `IsFrontArticle=false`                  |
|--------------------|------------------------------------|-----------------------------------------|
| **Content entry**  | Main article (wiki page, book)     | Image, CSS, cover page (source-dependent) |
| **Redirect**       | Browsable alias ("United States")  | Internal namespace alias                |

For browse views, `IsFrontArticle()` is the semantically correct filter and is what
Kiwix uses. Filtering `IsRedirect()` alone is weaker — it leaves non-article content
entries visible while hiding front-article redirects.

### Source-specific auxiliary entry patterns

Beyond redirects, certain content sources produce multiple entries sharing the same
title. `IsFrontArticle()` filtering may or may not help depending on whether the ZIM
creator flagged these as front articles.

| Pattern | Source | Notes |
|---|---|---|
| `*_cover.*` | Gutenberg | Cover page + main text are separate HTML entries, same title |
| Multi-part books (`*_part1*`, `*_part2*`, etc.) | Gutenberg, others | Same title, numbered path suffixes |
| `*(disambiguation)*` | Wikipedia | Distinct page, same root title |
| Tag / category pages | StackExchange | Browse noise alongside Q&A entries |
| Image galleries | Wikipedia | Media entries in content namespace |

**Open question:** For Gutenberg ZIMs, are `_cover` entries marked as front articles?
If yes, `IsFrontArticle()` filtering does not eliminate them and a secondary mechanism
is needed (see Configurable Exclusions below).

### Configurable browse exclusions

Hardcoding source-specific path patterns is fragile. A better approach is a per-archive
`browse_exclude` metadata key (glob patterns or regexes) that can be set at conversion
time or via a sidecar config. The `zim2oza` converter could auto-detect ZIM source type
from `M/Name` or `M/Source` metadata and suggest appropriate defaults.

---

## ZIM-as-PDF-Container

Some ZIM archives are effectively just containers for PDFs — the "article" is a PDF
blob, and the ZIM provides metadata and navigation wrappers around it. Common sources:

- Wikisource (scanned books, legal texts)
- RACHEL educational content
- Some Gutenberg variants (where the canonical form is PDF, not HTML)

### Preference: extract text, discard PDF wrapper

Serving raw PDFs requires a PDF reader in the client and keeps content opaque to
search, compression, and transformation. The preferred approach is to extract textual
content from the PDF at conversion time and store it in a more useful format.

**Extraction caveats:**
- Born-digital PDFs (text layer present): high-fidelity extraction with pdftotext,
  pdfminer, or similar
- Scanned PDFs (image-only): requires OCR (Tesseract etc.); quality varies significantly
- Complex layouts (multi-column, tables, math): extraction may lose structure
- Image-only content (diagrams, plates): must be preserved as images regardless

### Preferred output format: Markdown

Markdown is the preferred format for extracted textual content:

- **Compression**: Plain text/markdown compresses significantly better than HTML under
  zstd — HTML has substantial tag and attribute overhead that markdown eliminates
- **Ecosystem fit**: Goldmark renders markdown in Go natively; widely supported elsewhere
- **Transparency**: Content is readable without any renderer; easier to inspect, diff, index
- **Search**: Body text is directly accessible to the trigram search indexer without
  HTML parsing
- **Interoperability**: Markdown is a stable, broadly understood exchange format

MIME type: `text/markdown` (RFC 7763). ozaserve would need to pipe `text/markdown`
entries through Goldmark before serving them as HTML, or serve raw markdown with a
client-side renderer.

### Other notable considerations

- **Metadata mapping**: PDF metadata (Title, Author, Subject, CreationDate) maps cleanly
  to OZA metadata keys. Should be captured at extraction time.
- **Image preservation**: Figures, plates, and diagrams embedded in PDFs should be
  extracted and stored as separate image entries, referenced from the markdown via
  relative paths — consistent with how ZIM HTML content already references images.
- **Math**: PDFs from academic or educational sources often contain equations. Consider
  whether to preserve as images or convert to LaTeX/MathML for rendering.
- **Tables**: Markdown table syntax handles simple tables; complex PDF tables may
  require HTML fragments embedded in the markdown.
- **Multi-file PDFs**: Some ZIMs bundle entire book collections as a single PDF.
  Chapter/section detection (via PDF bookmarks or heading heuristics) would allow
  splitting into per-chapter entries for finer-grained browse and search.
- **Lossy vs. lossless**: Text extraction is inherently lossy for formatted PDFs.
  Consider whether to retain the original PDF blob as a secondary entry (e.g.
  `{path}_original.pdf`) for fidelity, or discard it to save space.

---

## General ZIM Namespace Notes

| Namespace | Treatment in zim2oza | Notes |
|-----------|---------------------|-------|
| `C` | Content | Main articles and resources |
| `C/_mw_/` | Skipped (chrome) | MediaWiki UI assets; deferred to Phase 8 |
| `M` | Extracted to archive metadata | Key-value pairs, not entries |
| `X` | Skipped | Xapian full-text search index (replaced by OZA trigram index) |
| `W` | Rewritten to `_well_known/` | Well-known resources |
| `-` | Content | New-format ZIM uses `-` instead of `C` |
| other | Rewritten to `_other/{ns}/` | Unrecognized namespaces preserved |

The `isFrontArticle` flag in OZA is set at conversion time as:
`ns == 'C' && MIME == "text/html"` — mirroring ZIM's own front-article convention.
This may not always match what the original ZIM creator intended if they used
non-standard namespace or MIME assignments.

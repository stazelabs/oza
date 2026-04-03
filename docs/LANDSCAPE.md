# OZA Converter Landscape Analysis — Backlog 3.9 Deep Dive

## Context

OZA has two converters today: `zim2oza` (ZIM archives) and `epub2oza` (EPUB books). Backlog item 3.9 lists "static site, PDF collection, Markdown corpus" as future sources and mentions a pluggable ingest pipeline. This analysis maps the full landscape of potential converter sources, identifies functionality gaps, and recommends a prioritized path to breakout adoption.

---

## 1. User Segments & What They Need Converted

| Audience | Content gap (not covered by ZIM pipeline) |
|----------|------------------------------------------|
| **Offline/humanitarian** (Kiwix inheritors) | Government PDFs, local-language educational material, institutional docs |
| **Researchers/archivists** | WARC web archives, PDF paper collections (arXiv, PubMed), institutional wiki exports |
| **AI/LLM developers** | Markdown docs, API specs (OpenAPI), code docs (godoc/rustdoc), curated datasets |
| **Privacy-conscious / local-first** | Personal Obsidian vaults, saved articles, email archives (niche) |

**Key insight:** The largest unserved content categories are (a) documentation that exists as HTML/Markdown directories and (b) PDF collections. ZIM already covers Wikipedia, StackExchange, Gutenberg, etc. — OZA's growth comes from content ZIM never packaged.

---

## 2. Content Sources Ranked by Impact

### Tier A — High volume, high feasibility, high strategic value

| Source | Feasibility | Why it matters | ZIM overlap |
|--------|------------|----------------|-------------|
| **Static site / HTML directory** | HIGH — walk dir, detect MIME, call AddEntry | Universal adapter: subsumes Hugo, MkDocs, Docusaurus, Jekyll, godoc, rustdoc, javadoc, wget output | Low (Kiwix covers popular sites, not the long tail) |
| **Markdown corpus** | HIGH — goldmark already a dependency | Direct bridge to AI/LLM audience; every OSS project has MD docs | Low |
| **PDF collections** | MEDIUM — text extraction quality varies; `pdftotext` (external) or `pdfcpu` (pure Go) | #1 format people want searchable but can't; academics, legal, government | None |

### Tier B — Significant value, moderate complexity

| Source | Feasibility | Why it matters | ZIM overlap |
|--------|------------|----------------|-------------|
| **WARC archives** | MEDIUM — parsing straightforward, URL rewriting complex | ISO standard for web archives; bridges archival community | Moderate (Zimit) |
| **API documentation (OpenAPI)** | HIGH — JSON/YAML parse + render | LLMs + tool use: API docs as OZA = instant MCP-accessible reference | None |
| **Wiki exports (Confluence, DokuWiki)** | MEDIUM — Confluence exports HTML, DokuWiki is flat text | Corporate knowledge bases going offline | Low |

### Tier C — Niche or better served by existing tools

| Source | Assessment |
|--------|-----------|
| **Knowledge bases (Notion, Obsidian)** | Obsidian = Markdown dir, use `site2oza`. Notion exports as MD. Not worth a dedicated converter. |
| **RSS/Atom feeds** | Better served by crawling the linked content with wget, then `site2oza` |
| **Chat/forum archives** | StackExchange in ZIM. Reddit has licensing issues. Discourse exports as JSON — niche. |
| **Datasets (CSV/JSON)** | Requires substantial UX design for tabular browsing. Low priority. |
| **Email archives** | MIME parsing complexity. Very niche. |
| **Man pages** | Partially covered by DevDocs in ZIM. Pre-rendered HTML via `site2oza`. |
| **MediaWiki XML dumps** | ZIM already handles this; `zim2oza` is the bridge. Skip. |

---

## 3. Developer / Integration Opportunities

### 3.1 CI/CD Documentation Pipeline (highest leverage)

```yaml
# GitHub Action concept
- uses: stazelabs/oza-action@v1
  with:
    input: ./docs/build/
    output: docs.oza
```

Every project that builds docs in CI could also produce an OZA artifact. Creates a flywheel: more OZA files, more tool support, more builders. **Requires `site2oza` to exist first.**

### 3.2 LLM Tool Ecosystem

The workflow: `site2oza ./docs/ → ozamcp docs.oza → Claude/GPT has instant searchable access`. No vector DB, no ingestion script, no configuration. This is the "killer app" — see §5.

### 3.3 Offline-First App Embedding

Medical app ships drug reference .oza. Legal app ships statute .oza. Educational app ships textbook .oza. The pure-Go reader with zero CGo makes embedding trivial.

---

## 4. Functionality Gaps

### True Blockers

| Gap | Impact |
|-----|--------|
| **No general-purpose ingest tool** | Cannot create OZA from the most common content format (HTML/MD directories). 90% of potential content is locked out. |
| **No platform distribution** | No Homebrew, no apt, no Docker, no pre-built binaries. Non-Go users cannot install. |

### Blockers at Scale

| Gap | Impact |
|-----|--------|
| **No incremental updates (`ozaupdate`)** | Monthly Wikipedia rebuilds are 2-3h vs ~35min with chunk-level copy. Impractical for regularly-updated archives. |

### Important but Not Blocking

| Gap | Impact |
|-----|--------|
| **PLAIN_TEXT section not implemented** | Foundation for AI story. Currently `ozamcp` does runtime `htmltomarkdown.ConvertString()` per read — works but slow. |
| **No content registry/discovery** | Users can't find what OZA files exist. Even a JSON file listing available archives would help. |
| **No cross-archive unified search** | `ozaserve` handles multiple archives but search results are per-archive, not globally ranked. |

### Nice-to-Have

| Gap | Impact |
|-----|--------|
| Browser WASM reader | Impressive demo, not critical |
| Python/JS writer bindings | CLI-first approach is sufficient |
| Streaming/progressive loading | Format supports range requests already |

---

## 5. Strategic Recommendations

### The Three Converters to Build (in order)

**1. `site2oza` — Static Site / Directory Converter**

- Takes `--input-dir ./site/` and produces an OZA file
- Subsumes: Markdown docs, pre-built static sites, pre-rendered code docs, crawled websites
- If input is `.md` with no `.html`, render via goldmark (already a dep)
- Follows `epub2oza/convert.go` pattern (~150 lines core logic)
- Integrates with `classify` for auto-detected content profiles
- **Effort: ~1-2 weeks. Unlocks 10x the content that can become OZA.**

**2. `pdf2oza` — PDF Collection Converter**

- Takes a directory of PDFs and produces a searchable OZA archive
- Extract text via `pdftotext` (external, high quality) with `pdfcpu` pure-Go fallback
- HTML wrapper per document with extracted text for search; optionally embed raw PDF
- Generate TOC from filenames/PDF metadata
- **Effort: ~2-3 weeks. Fills a gap ZIM never addressed.**

**3. `warc2oza` — WARC Archive Converter**

- Parse WARC records, reconstruct browsable site from HTTP responses
- URL rewriting and resource resolution are the hard parts
- **Effort: ~3-4 weeks. Bridges the web archiving community.**

### The Killer App: "One-Command Documentation RAG"

```bash
site2oza --title "My Project Docs" ./docs/build/ project-docs.oza
ozamcp project-docs.oza
# → LLM has instant offline searchable access via MCP
```

**Why this wins:**

1. **No existing tool does this.** RAG today = choose vector DB + write ingest script + chunk + embed + configure. OZA = one command.
2. **Reproducible** — same input, same file, same results. No index drift.
3. **Distributable** — share the .oza file, recipient runs `ozamcp`, identical access.
4. **Composable with CI** — every project already builds docs, adding OZA is one line.

### What NOT to Build

- Email archives (MIME parsing complexity, niche)
- Dataset browsers (substantial UX design needed for tabular data)
- Chat/forum scrapers (licensing issues, StackExchange already in ZIM)
- Dedicated Notion/Obsidian importers (they export Markdown — use `site2oza`)
- MediaWiki XML parser (ZIM handles this; `zim2oza` is the bridge)

### Enabling Infrastructure (alongside converters)

1. **Platform packages** — Homebrew formula, Docker image, GitHub Release binaries. Prerequisite for non-Go adoption.
2. **GitHub Action** — `stazelabs/oza-action` wrapping `site2oza`. Makes CI integration trivial.
3. **PLAIN_TEXT section** — Ship alongside or shortly after `site2oza`. Makes MCP reads 100x faster.
4. **Content registry** — Even a `registry.json` on GitHub listing available .oza files with URLs.

### Pluggable Ingest Pipeline

The backlog mentions a generic pluggable pipeline. Both existing converters already follow the same implicit pattern: scan, classify, iterate entries, `AddEntry`. A shared `Source` interface in `cmd/internal/ingest/` would reduce per-converter boilerplate to ~50 lines. **Build `site2oza` first, then extract the pattern** — don't design the abstraction before the second concrete instance (`epub2oza` is one, `site2oza` would be two, enough to generalize).

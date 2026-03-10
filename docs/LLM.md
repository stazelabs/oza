# 王座 OZA -- AI Ecosystem Native

> *OZA is the world's first AI ecosystem native content distribution format.*

ZIM served the offline web for 20 years. OZA serves the offline web *and* the AI ecosystem simultaneously — same file, same format, dual purpose.

An OZA file on disk **is** a complete, self-contained knowledge base for LLMs. No ingestion pipeline, no vector database, no configuration. Drop the file, point `ozamcp` at it, and an LLM has instant RAG.

---

## What "AI Ecosystem Native" Means

1. **Zero-setup RAG.** An OZA file is a complete retrieval-augmented generation source. Pre-computed embeddings, pre-extracted clean text, pre-built search indexes — all in one file.
2. **MCP-native.** Ships with an MCP server (`ozamcp`) that makes any OZA file instantly available as tools and resources for Claude, GPT, and other LLM agents.
3. **Context-window aware.** Pre-computed token counts and multi-tier summaries let RAG pipelines make instant budget decisions without tokenizing at query time.
4. **Citation-ready.** Provenance metadata per entry enables LLMs to cite sources properly.
5. **Hybrid search.** Trigram (keyword) + vector (semantic) search fused together, all from sections in one file.
6. **Self-describing tools.** Domain-specific archives can declare custom MCP tools that activate automatically.

---

## Why OZA, Not a Vector Database

| | Vector DB (Pinecone, Chroma, etc.) | OZA File |
|--|-----|-----|
| Setup | Install, configure, ingest, index | Drop file on disk |
| Running process | Required (server) | None needed |
| Ingestion | Parse → chunk → embed → upsert | Pre-computed at build time |
| Distribution | Export/import, migration scripts | Copy a file |
| Reproducibility | Index drift, schema changes | Immutable. Same file → same results |
| Air-gapped | Requires local server setup | File + `ozamcp` binary |
| Auditability | Opaque internal state | Single file, verifiable checksums |
| Licensing | Content mixed into opaque index | Discrete unit with per-entry provenance |

**The file IS the knowledge base.**

### Deployment scenarios

- **Air-gapped.** Military, submarine, space station, remote clinic. Drop file, get RAG.
- **Edge/laptop.** 10 GB file on disk = instant local RAG. No internet, no API calls.
- **Reproducible AI.** Pin an OZA file version. Every query returns identical retrieval results.
- **Content licensing.** Archive is a discrete auditable unit. Know exactly what the LLM can access.
- **Offline-first AI apps.** Ship an OZA file with your app. Instant domain knowledge.

---

## Capability Map

OZA's extensible section table enables LLM capabilities as new section types. All are optional — readers skip unknown sections. An archive can ship with any subset.

| Section Type | Name | What It Enables |
|-------------|------|-----------------|
| 0x0100 | VECTOR_EMBEDDINGS | Semantic search, similarity, RAG retrieval |
| 0x0101 | PLAIN_TEXT | LLM-ready markdown without HTML noise |
| 0x0102 | KNOWLEDGE_GRAPH | Entity search, topic filtering, graph-augmented RAG |
| 0x0103 | CONTEXT_HINTS | Token budgets, multi-tier summaries, instant context packing |
| 0x0104 | MULTIMODAL_EMBED | Cross-modal search (text query → relevant images) |
| 0x0105 | PROVENANCE | Proper attribution in LLM responses |
| 0x0106 | TOOL_MANIFEST | Domain-specific tools auto-registered via MCP |

Additionally:

| Capability | Mechanism |
|-----------|-----------|
| Hybrid search | Trigram (0x000C/0x000D) + vector (0x0100) fusion |
| Overlay embeddings | `.oza.embeddings` companion file for model updates |
| MCP server | `ozamcp` CLI tool exposing archives as MCP resources/tools |

---

## Section Designs

### PLAIN_TEXT (0x0101) — The Foundation

LLMs don't want HTML. Every RAG pipeline starts by stripping tags. OZA does it once at build time, not on every query.

**Format**: Markdown. Preserves structural information (headings, lists, tables, emphasis) that LLMs use effectively, at 30-50% the size of HTML.

```
Header:
  version (u32), entry_count (u32), total_passage_count (u32)
  flags (u32): bit 0 = is_markdown, bit 1 = has_token_counts
  tokenizer_id (length-prefixed string, e.g. "cl100k_base")

Entry Directory (sorted by entry_id for binary search):
  Per entry: entry_id (u32), text_offset (u32), text_size (u32)

Per Entry Text Block:
  passage_count (u16)
  Per passage:
    passage_offset (u32), passage_length (u16)
    passage_type (u8): paragraph | heading | list_item | table |
                       code_block | caption | lead
    heading_level (u8): 0-6
    token_count (u32)
  [concatenated passage text, UTF-8 markdown]
```

Passages are segmented at natural boundaries (paragraph breaks, heading changes). They are the atomic unit for RAG retrieval — small enough to fit context, large enough to be meaningful. Passage boundaries are stable across the archive and align with embedding vectors in the VECTOR_EMBEDDINGS section.

**Build**: HTML parser strips scripts/styles/navigation, converts structural elements to markdown, segments into passages, counts tokens with the declared tokenizer. No LLM needed.

**Storage**: ~25-35% of content size (compressed). For Wikipedia: ~20-30 GB additional.

**Priority**: Critical — everything else depends on clean text with stable passage boundaries.

---

### CONTEXT_HINTS (0x0103) — Context Window Intelligence

LLMs have finite context windows. "Can I fit this article? Should I use the summary? How many articles fit in 128K tokens?" These decisions must be instant.

```
Header:
  version (u32), entry_count (u32)
  flags (u32): bit 0 = has_summaries, bit 1 = has_key_facts
  tokenizer_id (length-prefixed)

Entry Directory (sorted by entry_id):
  Per entry: entry_id (u32), data_offset (u32), data_size (u32),
             full_token_count (u32)

Per Entry:
  tier_count (u8)
  Per tier:
    tier_type (u8): one_sentence | one_paragraph | section_leads | key_facts
    token_count (u32), text_length (u32)
    [text bytes, UTF-8 markdown]

  Key facts (optional, if has_key_facts):
    fact_count (u16)
    Per fact: text_length (u16), [text bytes]
```

**Summary tiers:**

| Tier | ~Tokens | Use Case |
|------|---------|----------|
| one_sentence | 20-50 | Citation snippet, search result |
| one_paragraph | 100-200 | Multi-source RAG, brief context |
| section_leads | 500-1500 | Single-source deep dive |
| key_facts | 5-15 per fact | Structured knowledge injection |

Key facts are atomic, structured facts suitable for direct inclusion in system prompts. Example for "Albert Einstein": `"Albert Einstein (1879-1955) was a German-born theoretical physicist"`, `"Developed the theory of relativity"`, `"Won the 1921 Nobel Prize in Physics"`.

**Build**: Extractive by default (first sentence, first paragraph, first sentence of each section). Optional abstractive generation via LLM for higher quality. Key facts extracted from infoboxes and lead paragraphs.

**Storage**: ~3-5% of content size. Cheap.

---

### VECTOR_EMBEDDINGS (0x0100) — Semantic Search

Trigram search finds "quantum entanglement" literally. Vector search finds articles *about* quantum entanglement that never use those exact words.

```
Header:
  version (u32), model_count (u32), total_vector_count (u32)
  flags (u32): bit 0 = has_ann_index

Per Model Space:
  Model Descriptor (128 bytes):
    model_name (length-prefixed, e.g. "all-MiniLM-L6-v2")
    model_version (length-prefixed)
    dimensions (u16), precision (u8), distance_metric (u8)
    normalization (u8), granularity (u8): entry_level | passage_level
    vector_count (u32)
    vector_table_offset (u64), ann_index_offset (u64), ann_index_size (u64)

  Vector Table:
    Per vector:
      entry_id (u32), passage_id (u32)
      text_offset (u32), text_length (u16)  — into PLAIN_TEXT section
      [vector_data: dimensions * bytes_per_element]

  ANN Index (HNSW):
    M (u32), ef_construction (u32), max_level (u32), entry_point (u32)
    Per level: node_count + neighbor lists
```

**Precision options:**

| Precision | Bytes/vec (dim=384) | Wikipedia (30M passages) | Quality loss |
|-----------|-------------------|--------------------------|-------------|
| float32 | 1,536 | 46 GB | None |
| float16 | 768 | 23 GB | Negligible |
| int8 | 384 | 11.5 GB | ~1% recall |
| binary | 48 | 1.4 GB | ~5% recall |

**Recommended default**: int8 at 384 dimensions. ~780 MB total for Wikipedia passage-level vectors (including per-vector metadata). HNSW index adds ~15%.

**Multiple models**: The section supports N model spaces. Readers that don't recognize a model name skip to the next, or fall back to trigram search. This handles model obsolescence gracefully — rebuild with a newer model, old readers still function.

**Why HNSW**: Best choice for read-only pre-built indexes. O(log N) query time with high recall. Serializes cleanly. IVF adds complexity for marginal benefit at this scale. Flat brute-force is too slow beyond ~100K vectors.

**Overlay files**: Embeddings become stale as models improve. Rather than rebuilding the entire archive:

```
archive.oza                 — original archive
archive.oza.embeddings      — overlay with newer model's vectors
```

The overlay references the original by UUID. `ozamcp` prefers the overlay if present.

---

### KNOWLEDGE_GRAPH (0x0102) — Structured Intelligence

"Find everything about Marie Curie" is an entity query, not a text query. Categories enable topic filtering before search. Relations enable graph-augmented RAG.

```
Entity Table:
  Per entity: entity_id (u32), name (len-prefixed),
              entity_type (u8: person|place|org|event|concept|work|date|quantity|other),
              wikidata_qid (u32), mention_count (u16)

Entity-Entry Mapping:
  Per mapping: entity_id (u32), entry_id (u32),
               mention_count (u16), is_primary_topic (u8)

Category Hierarchy:
  Per category: category_id (u32), name (len-prefixed),
                parent_id (u32), entry_count (u32)

Category-Entry Mapping:
  Per mapping: category_id (u32), entry_id (u32)

Relations:
  Per relation: source_entry_id (u32), target_entry_id (u32),
                relation_type (u16), weight (f32)

Relation types:
  0x0000 = links_to        0x0001 = see_also
  0x0002 = subtopic_of     0x0003 = prerequisite_for
  0x0004 = contradicts     0x0005 = extends
  0x0006 = cites           0x0100+ = archive-specific
```

**Storage**: ~1-2% of content size. For Wikipedia: ~800 MB compressed.

---

### PROVENANCE (0x0105) — Citation Ready

```
Per entry:
  entry_id (u32), source_url (len-prefixed), author (len-prefixed),
  publication_date (u32 unix), retrieval_date (u32 unix),
  license (len-prefixed SPDX), revision_id (u32)
```

Enables LLM responses like: *"According to the Wikipedia article on Quantum Entanglement (retrieved 2026-01-15, CC BY-SA 4.0)..."*

**Storage**: < 1% of content size.

---

### MULTIMODAL_EMBED (0x0104) — Cross-Modal Search

Same layout as VECTOR_EMBEDDINGS but for image/media entries. A vision encoder (e.g. CLIP) produces embeddings in a shared space with text embeddings, enabling text-query → image results.

**Storage**: ~512 bytes/image at int8/512-dim. Wikipedia's ~10M images = ~5 GB. Defer until the text pipeline is solid.

---

### TOOL_MANIFEST (0x0106) — Self-Describing Archives

Archives can declare domain-specific MCP tools that `ozamcp` auto-registers:

```
Per tool:
  definition (JSON, MCP-compatible schema)
  handler_type (u32): search_based | lookup_based | computed
  handler_config (JSON)
```

A medical encyclopedia declares `drug_interaction_check`. A legal archive declares `statute_lookup`. The archive teaches the LLM how to use it.

---

## `ozamcp` — MCP Server

The tool that makes everything real. Exposes OZA archives as MCP resources and tools over stdio (Claude Desktop, VS Code). For networked access with browsable URLs, use `ozaserve --mcp` instead.

```bash
ozamcp [file.oza ...] [--dir <path>]
  --transport stdio    # for Claude Desktop, VS Code (default)
```

### Tools

| Tool | Input | What It Does |
|------|-------|-------------|
| `search_text` | query, limit | Trigram substring search |
| `search_semantic` | query, limit, min_score | Embed query → HNSW search → passages |
| `search_hybrid` | query, limit, weights | Reciprocal Rank Fusion of both |
| `read_entry` | entry_id, format | Read content as markdown, html, or summary |
| `read_passage` | entry_id, passage_id | Read specific passage with heading context |
| `budget_context` | entry_ids[], max_tokens | Greedy-pack entries into token budget using best-fit summary tiers |
| `browse_category` | category, limit | List entries in a category |
| `find_entity` | name | Entity lookup → entry list |
| `get_related` | entry_id, relation_type | Follow knowledge graph edges |
| `list_archives` | | Available archives and capabilities |

### `budget_context` — The Killer Feature

The LLM says: "I found 20 relevant articles. I have 128K tokens. Give me as much context as possible."

1. Sort entries by relevance score
2. For each entry in order:
   - Full text fits in remaining budget → include it
   - Else section_leads tier fits → use that
   - Else one_paragraph tier fits → use that
   - Else one_sentence tier fits → use that
   - Else skip
3. Return packed context with exact token counts and citations

All lookups O(1) from pre-computed data. No tokenization at query time.

### Resources

```
oza://{uuid}/entry/{id}                  → clean markdown
oza://{uuid}/entry/{id}/summary/{tier}   → pre-computed summary
oza://{uuid}/metadata                    → archive metadata
oza://{uuid}/entity/{id}                 → entity details + mentions
```

---

## Hybrid Search Pipeline

```
Query: "What is quantum entanglement?"
  │
  ├──→ Trigram Search (0x000C/0x000D) ──→ entry_ids + scores
  │    Binary search posting lists
  │    < 10ms
  │
  ├──→ Vector Search (0x0100)        ──→ entry_ids + passage_ids + scores
  │    Embed query → HNSW traverse
  │    < 50ms (local model)
  │
  └──→ Reciprocal Rank Fusion        ──→ merged ranked results
       Knowledge graph boost:
         primary_topic entities → 1.5x
       │
       └──→ budget_context            ──→ packed context with citations
            Greedy-fill from CONTEXT_HINTS
            < 5ms
```

**Fallback behavior**: No embeddings → pure trigram. No trigram index → pure vector. Neither → path/title match. Graceful degradation based on which sections exist.

**When trigram wins**: Exact terms, proper nouns, code, technical jargon. "Python GIL" should match literally.

**When vector wins**: Conceptual queries, paraphrases, cross-lingual similarity. "How does memory management work" finds articles that never use those exact words.

**Hybrid (most real queries)**: RRF fusion combines both, with knowledge graph boosting for entity-aware ranking.

---

## Format-Level Early Provisions

Things to put in the base format now to avoid painting into a corner later.

### New Section Type Constants

```go
// LLM extension section types (0x0100+ range)
SectionVectorEmbeddings SectionType = 0x0100
SectionPlainText        SectionType = 0x0101
SectionKnowledgeGraph   SectionType = 0x0102
SectionContextHints     SectionType = 0x0103
SectionMultimodalEmbed  SectionType = 0x0104
SectionProvenance       SectionType = 0x0105
SectionToolManifest     SectionType = 0x0106
```

### New Header Flags

```go
FlagHasEmbeddings     = 1 << 3  // vector embeddings present
FlagHasPlainText      = 1 << 4  // LLM-ready text present
FlagHasKnowledgeGraph = 1 << 5  // knowledge graph present
FlagHasContextHints   = 1 << 6  // context hints present
```

Technically redundant (scan section table), but enables O(1) capability detection from the 128-byte header alone.

### New Entry Flags

The entry record's `flags` byte (currently only bit 0 = `is_front_article`):

```go
EntryFlagFrontArticle = 1 << 0  // existing
EntryFlagHasEmbedding = 1 << 1  // vectors exist for this entry
EntryFlagHasPlainText = 1 << 2  // clean text exists for this entry
EntryFlagHasSummary   = 1 << 3  // summaries exist for this entry
EntryFlagExcludeRAG   = 1 << 4  // exclude from RAG (disambig pages, stubs)
```

Per-entry flags allow fast filtering without scanning auxiliary sections.

### New Metadata Keys

| Key | Value | Purpose |
|-----|-------|---------|
| `embedding_model` | Model name string | Primary embedding model used |
| `embedding_dimensions` | Integer as string | Embedding dimensions |
| `plaintext_format` | `"markdown"` or `"plaintext"` | Format of PLAIN_TEXT section |
| `tokenizer` | Tokenizer ID string | Tokenizer used for token counts |
| `summary_method` | `"extractive"` or `"abstractive"` | How summaries were generated |
| `llm_ready` | `"true"` | Archive has at least PLAIN_TEXT + CONTEXT_HINTS |
| `mcp_tools` | Comma-separated tool names | Supported MCP tool names |

### Section Table Ordering Convention

LLM extension sections (0x0100+) appear in the section table AFTER all core sections (0x0001-0x00FF). Defense-in-depth for readers that might stop at the first unknown section type.

---

## Other Opportunities

### Interactive Archive Browsing (Agentic)

Beyond search-and-retrieve, an LLM can browse an OZA archive as an agent:

```
table_of_contents(entry_id)     → heading structure
read_section(entry_id, heading) → text under a specific heading
compare_entries(entry_ids)      → side-by-side key facts
random_article(category)        → exploration and serendipity
```

The LLM navigates like a researcher: search → overview → drill into section → follow related articles → compare sources.

### Fine-Tuning Data Export

An OZA archive with PLAIN_TEXT and KNOWLEDGE_GRAPH sections contains high-quality, structured training data. An `oza2jsonl` tool could export:

- **Instruction tuning pairs**: Question (from headings) + answer (from passage text)
- **Knowledge distillation**: Entity-fact pairs for knowledge injection
- **Domain adaptation**: Category-filtered text for domain-specific fine-tuning

### Differential Privacy / Content Filtering

For enterprise deployments, `EntryFlagExcludeRAG` controls per-entry RAG availability. An archive-level `rag_policy` metadata key declares the default policy:

```
"rag_policy": "all"            — all front articles available for RAG
"rag_policy": "flagged_only"   — only entries with EntryFlagHasEmbedding
"rag_policy": "exclude_stubs"  — exclude articles under N tokens
```

---

## Storage Overhead Summary

| Section | % of Content Size | Wikipedia Estimate | Notes |
|---------|------------------|-------------------|-------|
| PLAIN_TEXT | 25-35% | 20-30 GB | Compressed markdown |
| CONTEXT_HINTS | 3-5% | 2-3 GB | Multi-tier summaries |
| VECTOR_EMBEDDINGS | 8-15% | 780 MB - 12 GB | Depends on precision/dimensions |
| KNOWLEDGE_GRAPH | 1-2% | ~800 MB | Entities + categories + relations |
| PROVENANCE | < 1% | ~200 MB | Per-entry citation metadata |
| MULTIMODAL_EMBED | 5-10% | ~5 GB | Image embeddings (deferred) |
| TOOL_MANIFEST | negligible | < 1 MB | JSON tool definitions |

A "fully AI-ready" OZA file is roughly 1.4-1.7x the size of a content-only archive. For most deployments, PLAIN_TEXT + CONTEXT_HINTS alone (1.3-1.4x) provide the majority of the value.

---

## Implementation Priority

| # | What | Build Cost | Value |
|---|------|-----------|-------|
| 1 | PLAIN_TEXT section | Medium | **Critical** — foundation |
| 2 | CONTEXT_HINTS section | Medium | High — instant budgeting |
| 3 | VECTOR_EMBEDDINGS section | High (GPU) | High — semantic search |
| 4 | `ozamcp` MCP server | Medium | **Critical** — integration point |
| 5 | PROVENANCE section | Low | Medium — citations |
| 6 | Hybrid search fusion | Low | High — best retrieval |
| 7 | KNOWLEDGE_GRAPH section | Medium | Medium — enriched RAG |
| 8 | TOOL_MANIFEST section | Low | Low — domain-specific |
| 9 | MULTIMODAL_EMBED section | High | Low — defer |
| 10 | Overlay embeddings | Low | Medium — model updates |

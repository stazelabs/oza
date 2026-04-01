# Vectors & Dictionaries in OZA — A Deep Dive

OZA uses the words "dictionary" and "vector" in two distinct contexts that are easy to conflate. This document unpacks both, walks through concrete end-to-end scenarios, and explains why OZA's approach is fundamentally different from a vector database.

---

## Two Systems, One Archive

| System | What it is | Status | Section type |
|--------|-----------|--------|-------------|
| **Zstd compression dictionaries** | Shared byte patterns learned from content, used to shrink small entries 2-3x | Fully implemented | `ZSTD_DICT` (0x000B) |
| **Vector embeddings** | Dense numeric representations of passage meaning, used for semantic search | Designed, not yet implemented | `VECTOR_EMBEDDINGS` (0x0100) |

Both are created **at archive build time** and baked into the `.oza` file. Neither requires an external database or running process. Both are learned from the content itself — not loaded from a pre-existing external source.

---

## Part 1: Zstd Compression Dictionaries (Implemented)

### What problem they solve

Zstd compression works by finding repeated byte patterns and replacing them with short codes. For a 500 KB article, there are plenty of patterns to discover within the article itself. But for a 2 KB Wiktionary entry, the article is too short for the compressor to learn anything useful — the overhead of the Huffman table alone eats into the savings.

A **shared dictionary** solves this: train on hundreds of similar entries, learn the common patterns once, and apply them to every entry. The compressor starts with "prior knowledge" about what byte sequences are likely.

### Concrete example: Wiktionary

**Without dictionary:**
```
Entry "aardvark" (raw HTML): 3,200 bytes
  Zstd compressed:           2,800 bytes  (12% savings — disappointing)
```

**With dictionary trained on 1,000 Wiktionary entries:**
```
Entry "aardvark" (raw HTML): 3,200 bytes
  Zstd+dict compressed:        980 bytes  (69% savings — transformative)
```

The dictionary has learned patterns like `<span class="ib-content qualifier-content">`, `<li class="senseid">`, `<span class="mention-gloss-paren">` — boilerplate that appears in every Wiktionary entry but that a per-entry compressor never sees enough of to exploit.

### Full lifecycle: build time

Here is exactly what happens when you run `zim2oza` or build an OZA archive from scratch.

#### Step 1: Sample collection

As entries arrive via `AddEntry()`, the writer collects samples for dictionary training. Each entry is classified by MIME type and size:

```
Entry: "Aardvark" (text/html, 3,200 bytes)
  → ChunkKey = "html-small"     (HTML < 4,096 bytes gets "-small" suffix)

Entry: "Quantum_mechanics" (text/html, 85,000 bytes)
  → ChunkKey = "html"           (large HTML, no suffix)

Entry: "_res/style.css" (text/css, 12,000 bytes)
  → ChunkKey = "css"

Entry: "images/logo.png" (image/png, 45,000 bytes)
  → ChunkKey = "image"          (images are never compressed — already dense)
```

Each non-image entry under 4 KB contributes a sample copy to its MIME group's sample bucket. Samples are collected up to the `DictSamples` limit (default: 2,000 samples per group).

**Why separate "small" buckets?** A dictionary trained on 500-byte entries learns different patterns than one trained on 50 KB entries. The small-entry dictionary captures boilerplate structure; the large-entry dictionary (if trained) captures content patterns. Separating them produces better compression for both.

Source: [chunk.go:70-81](../ozawrite/chunk.go#L70-L81) — `smallEntryThreshold` and `ChunkKey()`

#### Step 2: Training trigger

Training fires when enough samples accumulate:

```
Condition 1: html + html-small samples ≥ DictSamples (2,000)  → train
Condition 2: total pending entries ≥ 2 × DictSamples (4,000)  → train anyway
```

All entries received before training completes are buffered in memory. After training, they are sorted by chunk key and flushed through the compression pipeline with their new dictionaries.

Source: [pipeline.go:42-50](../ozawrite/pipeline.go#L42-L50) — `haveSufficientSamples()`

#### Step 3: Dictionary training

For each MIME group with ≥ 10 samples, the writer:

1. **Concatenates samples** into a history buffer (up to 1 MB):
   ```
   history = sample[0] ++ sample[1] ++ ... ++ sample[N]
   // Must be ≥ 128 KiB or training is skipped (Zstd internal requirement)
   ```

2. **Calls `zstd.BuildDict()`** which analyzes the history for:
   - Common byte sequences (HTML tags, CSS properties, JSON structure)
   - Huffman code frequencies (character distributions)
   - Match distance patterns (how far back repeated sequences tend to be)

3. **Validates with round-trip test** — compresses and decompresses 5 samples through the new dictionary. If any sample fails to round-trip, the dictionary is discarded and plain Zstd is used:
   ```
   for each of 5 samples:
     compressed = zstd_compress(sample, dict)
     decompressed = zstd_decompress(compressed, dict)
     assert decompressed == sample  // or discard dictionary
   ```

4. **Assigns a dictionary ID** (uint32, starting from 1):
   ```
   html-small → dictID=1, dictionary=148 KB
   html       → dictID=2, dictionary=156 KB
   css        → dictID=3, dictionary=89 KB
   js         → dictID=4, dictionary=112 KB
   ```

Source: [compress.go:109-161](../ozawrite/compress.go#L109-L161) — `trainDictionary()` and `validateDict()`

#### Step 4: Compression with dictionaries

After training, every chunk is compressed with its group's dictionary:

```
Chunk 0 (html-small, 47 entries, 98 KB uncompressed):
  dict = dictID=1 (html-small dictionary)
  compressed = zstd_compress(chunk_data, level=19, dict=html_small_dict)
  result: 31 KB compressed (68% reduction)
  descriptor: { chunkID=0, dictID=1, compression=CompZstdDict(2) }

Chunk 1 (html, 3 entries, 2.1 MB uncompressed):
  dict = dictID=2 (html dictionary)
  compressed = zstd_compress(chunk_data, level=19, dict=html_dict)
  result: 420 KB compressed (80% reduction)
  descriptor: { chunkID=1, dictID=2, compression=CompZstdDict(2) }

Chunk 2 (image, 12 entries, 1.8 MB):
  compression = CompNone(0)
  descriptor: { chunkID=2, dictID=0, compression=CompNone(0) }
```

Source: [pipeline.go:169-200](../ozawrite/pipeline.go#L169-L200) — `flushChunk()` dispatches to workers with dictionary

#### Step 5: On-disk storage

Each dictionary is stored in its own `ZSTD_DICT` section:

```
Section Table Entry (80 bytes):
  section_type:      0x000B (ZSTD_DICT)
  offset:            <absolute file position>
  compressed_size:   <section size on disk>
  sha256:            <integrity hash>

Section Body:
  [0:4]   dictID = 1 (uint32, little-endian)
  [4:]    raw Zstd dictionary bytes (typically 80-200 KB)
```

A typical archive has 2-4 `ZSTD_DICT` sections — one per MIME group that had enough samples to train.

Source: [FORMAT.md §3.10](docs/FORMAT.md) — Zstd Dictionary Section layout

### Full lifecycle: read time

#### Step 1: Dictionary loading

When an archive is opened, all `ZSTD_DICT` sections are parsed and stored in a `map[uint32][]byte`:

```go
a.dicts[1] = <148 KB html-small dictionary>
a.dicts[2] = <156 KB html dictionary>
a.dicts[3] = <89 KB css dictionary>
```

This happens once at open time. Total memory: sum of dictionary sizes (typically 200-600 KB).

Source: [archive.go](../oza/archive.go) — dictionary section loading

#### Step 2: Chunk decompression

When a reader requests entry "Aardvark":

```
1. Look up entry record → chunkID=0, blobOffset=4200, blobSize=3200
2. Look up chunk descriptor → { dictID=1, compression=CompZstdDict }
3. Read compressed chunk bytes from disk (31 KB)
4. Get dictionary: dicts[1] → html-small dictionary
5. Create/reuse pooled Zstd decoder initialized with that dictionary
6. Decompress: 31 KB → 98 KB (the full chunk)
7. Slice out blob: chunk[4200:4200+3200] → the Aardvark HTML
```

**Decoder pooling:** The first decompression for each dictID creates a `sync.Pool` of Zstd decoders pre-initialized with that dictionary. Subsequent decompressions reuse pooled decoders — no dictionary re-parsing on every request.

Source: [compress.go:64-93](../oza/compress.go#L64-L93) — `decodeZstdDict()` with `sync.Pool`

### Real-world compression numbers

| Archive | Entries | Avg entry | Without dict | With dict | Savings |
|---------|---------|-----------|-------------|-----------|---------|
| English Wiktionary | 8.2M | 2.8 KB | 18.1 GB | 6.3 GB | 65% |
| Simple English Wikipedia | 230K | 4.1 KB | 0.9 GB | 0.4 GB | 56% |
| English Wikipedia | 6.8M | 45 KB | 85 GB | 72 GB | 15% |
| Stack Overflow (Q&A) | 55M | 1.9 KB | 95 GB | 38 GB | 60% |

The pattern is clear: **dictionaries shine on archives with many small, structurally similar entries**. For Wikipedia's longer articles, plain Zstd already finds plenty of patterns — dictionaries help, but the win is smaller.

---

## Part 2: Vector Embeddings (Designed, Not Yet Implemented)

### What problem they solve

Trigram search finds "quantum entanglement" literally — it matches the exact byte sequence. But a query like "How does spooky action at a distance work?" won't match an article titled "Quantum Entanglement" that never uses the phrase "spooky action at a distance" (even though Einstein coined it to describe exactly that phenomenon).

Vector embeddings solve this by converting text into dense numeric vectors where **semantic similarity maps to geometric proximity**. Two passages about the same concept — even using completely different words — produce vectors that are close together in the embedding space.

### How embeddings are created

An **embedding model** is a neural network (typically a transformer) that reads text and outputs a fixed-size numeric vector. The model is trained on billions of text pairs so that similar meanings produce similar vectors.

```
Input: "Quantum entanglement is a phenomenon where particles become correlated"
Model: all-MiniLM-L6-v2 (384 dimensions)
Output: [0.0234, -0.1567, 0.0891, ..., 0.0445]  (384 float32 values)
```

The model is **not part of the OZA file**. It runs during archive build and (optionally) at query time to embed the user's search query. The OZA file stores only the output vectors, not the model itself.

### Concrete example: building embeddings for Simple English Wikipedia

#### Step 1: PLAIN_TEXT must exist first

Vector embeddings operate on **passages** defined in the PLAIN_TEXT section. You cannot build embeddings without clean text and stable passage boundaries.

```
Article: "Albert Einstein"
PLAIN_TEXT passages:
  Passage 0 (lead):    "Albert Einstein was a German-born theoretical physicist..."  (312 chars)
  Passage 1 (heading): "## Early life"  (13 chars)
  Passage 2 (paragraph): "Einstein was born in Ulm, in the Kingdom of Württemberg..."  (487 chars)
  Passage 3 (heading): "## Scientific career"  (21 chars)
  Passage 4 (paragraph): "In 1905, Einstein published four groundbreaking papers..."  (623 chars)
  ...
  Passage 14 (paragraph): "Einstein died on April 18, 1955, in Princeton..."  (198 chars)
```

Each passage is 100-500 tokens (400-2000 chars), segmented at natural boundaries (headings, paragraph breaks). These boundaries are fixed at build time and shared between PLAIN_TEXT and VECTOR_EMBEDDINGS.

#### Step 2: Embed each passage

The builder runs each passage through the embedding model:

```
Passage 0 → model("Albert Einstein was a German-born theoretical physicist...")
         → [0.0234, -0.1567, 0.0891, ..., 0.0445]  (384 floats)

Passage 4 → model("In 1905, Einstein published four groundbreaking papers...")
         → [0.0891, -0.0234, 0.1567, ..., -0.0112]  (384 floats)
```

For Simple English Wikipedia (~230K articles, ~5 passages average per article):
```
Total passages: ~1.15 million
Embedding time: ~45 minutes on a single GPU (A100)
                ~6 hours on CPU (Apple M2 Pro)
                ~2 hours using a hosted API (batch mode)
```

#### Step 3: Quantize vectors

Raw float32 vectors are large. OZA supports four precision levels:

```
Passage 0 embedding at float32:
  [0.0234, -0.1567, 0.0891, ..., 0.0445]
  384 dimensions × 4 bytes = 1,536 bytes per vector

Passage 0 embedding at int8 (recommended):
  [6, -40, 23, ..., 11]    (scaled to [-128, 127] range)
  384 dimensions × 1 byte = 384 bytes per vector

Passage 0 embedding at binary:
  [0b00100110, 0b11001010, ...]   (sign bit only)
  384 dimensions ÷ 8 = 48 bytes per vector
```

**Quantization math (int8):**
```
For each dimension d across ALL vectors:
  min_d = minimum value of dimension d across all vectors
  max_d = maximum value of dimension d across all vectors
  scale_d = (max_d - min_d) / 255

For each vector v:
  v_int8[d] = round((v_float32[d] - min_d) / scale_d) - 128
```

| Precision | Bytes/vec | Simple English Wikipedia (1.15M vectors) | Recall loss |
|-----------|----------|------------------------------------------|------------|
| float32 | 1,536 | 1.77 GB | None |
| float16 | 768 | 883 MB | Negligible |
| **int8** | **384** | **442 MB** | **~1%** |
| binary | 48 | 55 MB | ~5% |

int8 is the recommended default: 4x smaller than float32 with only ~1% recall loss. For 442 MB on disk, you get semantic search over the entire Simple English Wikipedia.

#### Step 4: Build the HNSW index

HNSW (Hierarchical Navigable Small World) is a graph-based index for fast approximate nearest-neighbor search. It organizes vectors into a multi-layer graph:

```
Layer 2 (sparse):    [Einstein] ---- [Curie] ---- [Bohr]
                         |                           |
Layer 1 (medium):    [Einstein] -- [Relativity] -- [Bohr] -- [Atoms]
                         |              |             |          |
Layer 0 (dense):     [Einstein] [Photoelectric] [Relativity] [Bohr] [Atoms] [Nucleus] ...
                      all 1.15M vectors connected to their nearest neighbors
```

**Build parameters:**
```
M = 16              (each node connects to 16 neighbors per layer)
ef_construction = 200  (search width during build — higher = better but slower)
max_level = 4       (for 1.15M vectors: ceil(log(1.15M) / log(M)) ≈ 4)
```

**Build time:** ~10 minutes for 1.15M int8 vectors on a single CPU core.

**Index size:** ~15% overhead on top of raw vectors.
```
Raw vectors:  442 MB
HNSW index:   ~66 MB
Total:        ~508 MB
```

#### Step 5: Serialize into the VECTOR_EMBEDDINGS section

```
VECTOR_EMBEDDINGS Section (0x0100):

Header (20 bytes):
  version:            1
  model_count:        1
  total_vector_count: 1,150,000
  flags:              0x01 (has_ann_index)

Model Space 0 (128-byte descriptor):
  model_name:         "all-MiniLM-L6-v2"
  model_version:      "1.0"
  dimensions:         384
  precision:          2 (int8)
  distance_metric:    1 (cosine)
  normalization:      1 (L2-normalized)
  granularity:        1 (passage_level)
  vector_count:       1,150,000
  vector_table_offset: <offset>
  ann_index_offset:   <offset>
  ann_index_size:     <size>

Vector Table (1,150,000 entries):
  Per vector (396 bytes at int8/384d):
    entry_id:     42          (u32 — which article)
    passage_id:   4           (u32 — which passage within the article)
    text_offset:  28400       (u32 — byte offset into PLAIN_TEXT section)
    text_length:  623         (u16 — passage length in bytes)
    vector_data:  [6, -40, 23, ..., 11]  (384 bytes)

HNSW Index:
    M:              16
    ef_construction: 200
    max_level:       4
    entry_point:     0
    [neighbor lists per level...]
```

### Full lifecycle: query time

A user asks: **"How does spooky action at a distance work?"**

#### Step 1: Embed the query

The same model that produced the archive's vectors embeds the query:

```
query_vec = model("How does spooky action at a distance work?")
         → [0.0567, -0.0891, 0.1234, ..., 0.0778]  (384 floats)
         → quantize to int8: [14, -23, 31, ..., 20]
```

**Time:** ~5ms on CPU with ONNX runtime, ~1ms on GPU.

**Where does the model come from?** The `ozamcp` server loads a local copy of the embedding model at startup. The model name in the section header (`all-MiniLM-L6-v2`) tells `ozamcp` which model to load. If the model isn't available locally, `ozamcp` falls back to trigram-only search.

#### Step 2: HNSW traversal

```
Start at entry_point (layer 4, node "Physics")
  → Greedy walk to nearest neighbor on layer 4
  → Drop to layer 3, greedy walk
  → Drop to layer 2, greedy walk
  → Drop to layer 1, greedy walk
  → Layer 0: beam search with ef_search=50

Candidates found (by cosine similarity):
  1. entry=42 (Quantum Entanglement), passage=0  score=0.89
  2. entry=42 (Quantum Entanglement), passage=4  score=0.84
  3. entry=1701 (EPR Paradox), passage=2          score=0.81
  4. entry=891 (Bell's Theorem), passage=0        score=0.78
  5. entry=42 (Quantum Entanglement), passage=7   score=0.76
  ...top 10 results
```

**Time:** ~5-15ms for 1.15M vectors on CPU (HNSW is O(log N)).

#### Step 3: Retrieve passage text

Each result includes `text_offset` and `text_length` pointing into the PLAIN_TEXT section:

```
Result 1: text_offset=28400, text_length=623
  → Read PLAIN_TEXT chunk, decompress, slice [28400:29023]
  → "Quantum entanglement is a phenomenon in quantum physics where two
     particles become interconnected and the quantum state of one instantly
     influences the other, regardless of the distance between them..."
```

**Time:** <1ms per passage (chunk likely already cached from a recent read).

#### Step 4: Return to LLM

The MCP server returns passage text with metadata:

```json
{
  "results": [
    {
      "entry_id": 42,
      "title": "Quantum Entanglement",
      "passage_id": 0,
      "score": 0.89,
      "text": "Quantum entanglement is a phenomenon in quantum physics where..."
    },
    {
      "entry_id": 1701,
      "title": "EPR Paradox",
      "passage_id": 2,
      "score": 0.81,
      "text": "Einstein, Podolsky, and Rosen argued in their 1935 paper..."
    }
  ]
}
```

**Total query time:** ~25ms (5ms embed + 15ms HNSW + 5ms text retrieval).

---

## Part 3: End-to-End Scenarios

### Scenario 1: Wiktionary — 8.2 million tiny entries

**Profile:** Millions of 2-5 KB entries with nearly identical HTML structure.

**Build:**
```
1. AddEntry("aardvark", text/html, 3,200 bytes)
   → ChunkKey = "html-small"
   → Sample collected for dictionary training

2. After 2,000 html-small samples collected:
   → trainDictionary(id=1, samples=2000, maxSize=1MB)
   → Learns patterns: <span class="ib-content">, <li class="senseid">, etc.
   → Dictionary size: 142 KB
   → Validated with 5-sample round-trip

3. Remaining 8.2M entries compressed with dictID=1
   → Average entry: 3,200 → 980 bytes (69% savings)

4. Embed passages for semantic search:
   → 8.2M entries × ~2 passages avg = ~16.4M passages
   → int8/384d = 6.3 GB vectors + ~950 MB HNSW index
   → Total embedding section: ~7.2 GB
```

**Query:** "What is the past tense of 'swim'?"
```
Trigram search: "past tense" + "swim" → matches "swim" entry directly
Vector search: semantic query → finds "swim", "swam", "swimming", "backstroke"
Hybrid (RRF): "swim" ranked #1 (both signals), "swam" #2, "swimming" #3
```

**Key insight:** For Wiktionary, compression dictionaries are the hero — they save 12 GB. Vector embeddings are large (7 GB) relative to the compressed content (6.3 GB) but enable "find words related to X" queries that trigrams cannot answer.

### Scenario 2: English Wikipedia — the reference case

**Profile:** 6.8M articles, averaging 45 KB each. Long, diverse content.

**Build:**
```
Compression:
  html-small dict: 2.1M entries < 4KB (stubs, disambig pages)
    → dictionary saves 35% on these entries
  html dict: 4.7M entries ≥ 4KB
    → dictionary saves 8% (diminishing returns on long articles)
  Total: 85 GB → 72 GB with dictionaries, vs ~76 GB without

Embeddings:
  6.8M articles × ~5 passages avg = ~34M passages
  int8/384d:
    Vector data: 34M × 384 bytes = 13.1 GB
    HNSW index:  ~2.0 GB
    Per-vector metadata: 34M × 12 bytes = 408 MB
    Total: ~15.5 GB

Full "AI-ready" archive:
  Content:     72 GB (with compression dictionaries)
  PLAIN_TEXT:  25 GB (compressed markdown)
  Embeddings:  15.5 GB
  Search:      400 MB (trigram indices)
  Other:       3 GB (context hints, provenance, knowledge graph)
  Total:       ~116 GB
```

**Query:** "What causes the northern lights?"
```
Trigram: "northern lights" → Aurora Borealis (title match),
         "causes" + "northern lights" → body matches in 12 articles
Vector:  query embedding → finds:
         Aurora Borealis (score=0.91)
         Magnetosphere (score=0.83)
         Solar wind (score=0.79)
         Geomagnetic storm (score=0.77)
         Ionosphere (score=0.74)

Hybrid with knowledge graph boost:
  Aurora Borealis → primary_topic entity "Aurora" → 1.5x boost
  Final ranking:
    1. Aurora Borealis         (trigram title + vector 0.91 + entity boost)
    2. Magnetosphere           (vector 0.83, related entity)
    3. Solar wind              (vector 0.79, prerequisite relation)
    4. Geomagnetic storm       (vector 0.77)

budget_context(entry_ids=[1,2,3,4], max_tokens=8000):
  Aurora Borealis:   full text    (3,200 tokens)  ✓ fits
  Magnetosphere:     section_leads (800 tokens)   ✓ fits (full text too large)
  Solar wind:        one_paragraph (150 tokens)   ✓ fits
  Geomagnetic storm: one_sentence  (35 tokens)    ✓ fits
  Total: 4,185 tokens packed with citations
```

### Scenario 3: Medical encyclopedia — domain-specific RAG

**Profile:** 50K articles on diseases, drugs, procedures. Specialized vocabulary.

**Build:**
```
Compression:
  html-small dict trained on medical HTML boilerplate:
    <div class="drug-interaction">, <span class="icd-code">, etc.
    → 3,800 → 1,100 bytes average (71% savings)

Embeddings:
  Model: PubMedBERT-384 (domain-specific medical embeddings)
  50K articles × 8 passages avg = 400K passages
  int8/384d: 154 MB vectors + 23 MB HNSW = 177 MB total

TOOL_MANIFEST declares:
  drug_interaction_check(drug_a, drug_b) → search interactions
  symptom_differential(symptoms[]) → ranked conditions
  dosage_lookup(drug, patient_weight, age) → dosing table
```

**Query:** "Patient presents with fever, joint pain, and butterfly rash"
```
Vector search with PubMedBERT embeddings:
  1. Systemic Lupus Erythematosus  (score=0.93)
  2. Rheumatic Fever               (score=0.86)
  3. Dermatomyositis               (score=0.81)
  4. Adult-onset Still's Disease   (score=0.78)

The LLM uses the symptom_differential tool:
  → Returns ranked differential with passage references
  → Each condition includes prevalence data and key distinguishing features
  → Citations point to specific passages with provenance metadata
```

**Key insight:** Domain-specific embedding models dramatically outperform general-purpose models. The same archive can ship with both general and domain-specific embeddings using the multi-model-space design.

### Scenario 4: Legal code archive — air-gapped deployment

**Profile:** 200K statutes and regulations. Air-gapped government network.

**Build (on connected build server):**
```
Compression:
  Trained on legal HTML: <section class="statute">, <span class="usc-ref">, etc.
  200K entries × 6 KB avg = 1.2 GB → 380 MB with dictionaries

Embeddings:
  Model: legal-bert-384
  200K entries × 3 passages avg = 600K passages
  int8/384d: 230 MB vectors + 35 MB HNSW = 265 MB

Total archive: ~1.8 GB (fits on a USB drive)
```

**Deployment:**
```
1. Copy legal-code-2026.oza to air-gapped machine
2. Copy ozamcp binary and legal-bert ONNX model
3. Run: ozamcp legal-code-2026.oza
4. LLM agent connects via MCP stdio
5. Instant RAG — no internet, no database, no setup
```

**Query:** "What are the penalties for unauthorized disclosure of classified information?"
```
Vector search finds:
  18 U.S.C. § 798 — Disclosure of classified information (score=0.94)
  18 U.S.C. § 1924 — Unauthorized removal of classified documents (score=0.88)
  Executive Order 13526 — Classified National Security Information (score=0.82)

All from a single file on a USB drive.
```

---

## Part 4: Overlay Embeddings — Upgrading Models Without Rebuilding

Embedding models improve over time. A 2026 model produces better vectors than a 2024 model. But rebuilding a 116 GB Wikipedia archive just to update embeddings is wasteful — the content hasn't changed, only the vectors.

### How overlays work

```
wikipedia-2026.oza                  — 116 GB, ships with all-MiniLM-L6-v2 embeddings
wikipedia-2026.oza.embeddings       — 18 GB overlay, ships with gte-base-en-v2 embeddings
```

The overlay file:
1. Contains a `VECTOR_EMBEDDINGS` section with the new model's vectors
2. References the original archive by UUID
3. Uses the same passage boundaries (same `entry_id` + `passage_id` mapping)
4. Is a standalone file — no modification to the original archive

**At load time:**
```
ozamcp wikipedia-2026.oza
  → Finds wikipedia-2026.oza.embeddings in same directory
  → Verifies UUID match
  → Uses overlay's vectors for semantic search
  → Falls back to archive's vectors if overlay model unavailable
  → All other sections (content, search, PLAIN_TEXT) from original archive
```

### Migration scenario

```
Year 1: Ship archive with all-MiniLM-L6-v2 (2024 model)
  → 15.5 GB embedding section
  → Good quality, widely supported

Year 2: New model gte-base-en-v2 available, 12% better recall
  → Build overlay: 17.8 GB (larger model, 512 dimensions)
  → Ship overlay file alongside original archive
  → Readers that support gte-base-en-v2 use overlay
  → Readers that don't fall back to MiniLM vectors in the archive
  → No 116 GB re-download

Year 3: Even newer model available
  → Build new overlay, replace year 2 overlay
  → Or rebuild full archive (content has likely been updated anyway)
```

### Multi-model coexistence

The `VECTOR_EMBEDDINGS` section supports **N model spaces** within a single file:

```
Model Space 0: all-MiniLM-L6-v2 (384d, int8) — 13 GB
  → General purpose, fast, small
Model Space 1: PubMedBERT (384d, int8) — 13 GB
  → Medical domain queries
Model Space 2: multilingual-e5 (1024d, int8) — 35 GB
  → Cross-lingual search (query in French, find English articles)
```

Readers that don't recognize a model name skip to the next, or fall back to trigram search. Model obsolescence is handled gracefully — the archive never becomes unsearchable.

---

## Part 5: Hybrid Search Pipeline — Detailed Walkthrough

### Setup: Wikipedia OZA archive loaded by `ozamcp`

```
Archive: english-wikipedia-2026.oza (116 GB)
  Trigram indices:  loaded (title: 5 MB resident, body: 350 MB lazy)
  Dictionaries:    loaded (4 dicts, 500 KB total)
  Embedding model: all-MiniLM-L6-v2 loaded (ONNX, 80 MB)
  HNSW index:      memory-mapped (2 GB)
  Vector data:     memory-mapped (13 GB, accessed on demand)
```

### Query: `search_hybrid("renewable energy storage solutions", limit=10)`

#### Phase 1: Parallel search (< 20ms)

Both search paths run concurrently:

**Trigram path:**
```
Query trigrams: "ren", "ene", "new", "ewa", "wab", "abl", "ble", "le ", "e e", ...
                "sto", "tor", "ora", "rag", "age", "ge ", "e s", " so", "sol", ...

Title index (3ms):
  Binary search for each trigram → intersect posting lists
  Matches: entry 891 "Energy storage", entry 4502 "Renewable energy"

Body index (12ms):
  Same trigrams against full-text index
  Matches: 47 entries containing all query trigrams
  Top by position: Battery recycling, Grid energy storage, Pumped-storage hydroelectricity, ...

Combined trigram results (title first, then body):
  1. Energy storage            (title match)
  2. Renewable energy          (title match)
  3. Grid energy storage       (body match, all trigrams)
  4. Pumped-storage hydroelectricity  (body match)
  5. Flow battery              (body match)
  ...47 total
```

**Vector path:**
```
Embed query (5ms):
  query_vec = model("renewable energy storage solutions")
  → [0.0345, -0.0891, ...]  (384 dimensions)

HNSW search (8ms):
  Traverse layers 4→0, ef_search=50
  Return top 20 by cosine similarity:
    1. entry=891,  passage=0   "Energy storage" (lead)          score=0.92
    2. entry=891,  passage=3   "Grid-scale batteries"           score=0.88
    3. entry=7823, passage=1   "Lithium iron phosphate battery" score=0.85
    4. entry=4502, passage=5   "Renewable energy challenges"    score=0.84
    5. entry=12045,passage=0   "Vehicle-to-grid"                score=0.82
    6. entry=3891, passage=2   "Hydrogen economy"               score=0.80
    ...
```

#### Phase 2: Reciprocal Rank Fusion (< 1ms)

RRF merges ranked lists without requiring comparable scores:

```
RRF_score(entry) = Σ 1/(k + rank_in_list)   where k=60

Entry "Energy storage" (891):
  Trigram rank: 1 → 1/(60+1) = 0.0164
  Vector rank:  1 → 1/(60+1) = 0.0164
  RRF = 0.0328

Entry "Renewable energy" (4502):
  Trigram rank: 2 → 1/(60+2) = 0.0161
  Vector rank:  4 → 1/(60+4) = 0.0156
  RRF = 0.0317

Entry "Vehicle-to-grid" (12045):
  Trigram rank: not found → 0
  Vector rank:  5 → 1/(60+5) = 0.0154
  RRF = 0.0154

Entry "Hydrogen economy" (3891):
  Trigram rank: not found → 0
  Vector rank:  6 → 1/(60+6) = 0.0152
  RRF = 0.0152
```

#### Phase 3: Knowledge graph boost (< 1ms)

```
Entity lookup for top results:
  "Energy storage" → primary_topic entity "Energy storage" → 1.5x boost
  "Renewable energy" → primary_topic entity "Renewable energy" → 1.5x boost
  "Vehicle-to-grid" → no primary entity match → 1.0x

Boosted scores:
  1. Energy storage:     0.0328 × 1.5 = 0.0492
  2. Renewable energy:   0.0317 × 1.5 = 0.0476
  3. Grid energy storage: 0.0289 × 1.0 = 0.0289
  4. Lithium iron phosphate battery: 0.0254 × 1.0 = 0.0254
  5. Vehicle-to-grid:    0.0154 × 1.0 = 0.0154
  ...
```

#### Phase 4: Return results (< 5ms)

```
Final ranked results with passage context:
  1. Energy storage (score=0.0492)
     → Lead passage: "Energy storage is the capture of energy produced at one time..."
  2. Renewable energy (score=0.0476)
     → Passage 5: "The main challenge of renewable energy is intermittency..."
  3. Grid energy storage (score=0.0289)
     → Lead passage: "Grid energy storage refers to large-scale methods..."
  ...top 10

Total time: 25ms (parallel search) + 1ms (RRF) + 1ms (boost) + 3ms (text retrieval)
          = ~30ms end-to-end
```

### Fallback behavior

```
Archive has embeddings + trigrams → hybrid search (best results)
Archive has embeddings only      → pure vector search
Archive has trigrams only        → pure trigram search
Archive has neither              → path/title prefix match
```

Each degradation is automatic. An old archive without embeddings still searches. A minimal archive without even trigram indices still navigates by path. The format never becomes unsearchable.

---

## Part 6: Compression Dictionaries vs. Vector Embeddings — Summary

| | Compression Dictionary | Vector Embedding |
|--|----------------------|-----------------|
| **Purpose** | Reduce storage size | Enable semantic search |
| **What it stores** | Common byte patterns | Numeric meaning vectors |
| **Learned from** | Content bytes (HTML tags, CSS properties) | Content semantics (meaning, topics) |
| **When created** | Archive build time | Archive build time |
| **Stored where** | `ZSTD_DICT` section (0x000B) | `VECTOR_EMBEDDINGS` section (0x0100) |
| **Typical size** | 80-200 KB per dictionary | 500 MB - 15 GB total |
| **Used at read time** | Every content request (decompression) | Semantic search queries |
| **External dependency** | None (Zstd is the compressor) | Embedding model (for query embedding) |
| **Updatable** | No (tied to content) | Yes (overlay files) |
| **Implementation** | Fully implemented | Designed, not yet built |

The two systems are complementary. Compression dictionaries make the archive smaller and faster to serve. Vector embeddings make it semantically searchable. Both are built from the content itself at archive creation time, and both live inside the single `.oza` file.

---

## Part 7: The Three-Layer Architecture — Using Embeddings with LLMs

### The mental model

There are three distinct components in a working OZA + LLM system. Understanding which does what — and which talks to which — is essential.

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 1: Frontier LLM  (Claude, ChatGPT, Gemini, etc.)    │
│                                                              │
│  What it does:  Reasoning, synthesis, answering questions    │
│  Where it runs: Remote API or local (doesn't matter)        │
│  What it sees:  Text passages returned by ozamcp             │
│  What it NEVER sees: Vectors, embeddings, HNSW indices       │
│                                                              │
│  Communicates via: MCP tool calls (search_hybrid, read_entry)│
└──────────────────────────┬──────────────────────────────────┘
                           │ MCP (stdio or HTTP)
                           │ sends query TEXT down, receives passage TEXT back
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 2: ozamcp + Embedding Backend (Ollama, ONNX, or API) │
│                                                              │
│  What it does:  Retrieval — finds relevant passages          │
│  Where it runs: Locally, adjacent to the archive file        │
│                                                              │
│  On each semantic query:                                     │
│    1. Receives query text from LLM                           │
│    2. Calls embedding backend to convert query → vector      │
│    3. Searches HNSW index for nearest passage vectors        │
│    4. Reads passage text from PLAIN_TEXT section              │
│    5. Returns passage text (NOT vectors) to the LLM          │
└──────────────────────────┬──────────────────────────────────┘
                           │ direct file I/O
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: OZA Archive  (the file on disk)                   │
│                                                              │
│  What it contains:                                           │
│    PLAIN_TEXT (clean markdown with passage boundaries)        │
│    Trigram indices (keyword search)                           │
│    Compressed content (HTML, CSS, images)                     │
│    Zstd dictionaries (for decompression)                     │
│                                                              │
│  What it does NOT contain (in the default path):             │
│    Vector embeddings — these are a runtime concern            │
│                                                              │
│  Immutable. The ground truth. Content and text, not vectors. │
└─────────────────────────────────────────────────────────────┘
```

**The embedding model is a retrieval mechanism, not a reasoning engine.** It answers "which passages are relevant to this query?" — then hands those passages as plain text to the frontier LLM for actual thinking.

Think of it as a librarian vs. a scholar. The embedding model is the librarian — it finds the right books and opens them to the right page. The frontier model is the scholar — it reads the pages and formulates an answer. The librarian doesn't need to be a genius; it needs to be fast and accurate at finding relevant material. This is why an 80 MB model from 2022 works perfectly well alongside a 2026 frontier LLM.

### Why the choice of frontier LLM doesn't matter

The embedding model is the same regardless of which LLM calls `ozamcp`:

```
Claude  → MCP → ozamcp → [embedding backend] → HNSW → passage text → Claude
ChatGPT → MCP → ozamcp → [embedding backend] → HNSW → passage text → ChatGPT
Gemini  → MCP → ozamcp → [embedding backend] → HNSW → passage text → Gemini
Local   → MCP → ozamcp → [embedding backend] → HNSW → passage text → Local LLM
```

The frontier LLM never knows or cares which embedding model is doing retrieval. It sends a query string, gets passage text back. The embedding model never knows or cares which LLM sent the query. It receives a string, returns similar passages. The two models are fully decoupled.

---

## Part 8: The Key Insight — Vectors Are a Runtime Concern, Not a Distribution Concern

### The problem with baking vectors into the archive

The earlier sections of this document describe a VECTOR_EMBEDDINGS section (0x0100) baked into the OZA file at build time. This design has an appealing simplicity — "everything in one file" — but it creates a fundamental coupling problem:

**Pre-computed vectors require the matching model at runtime anyway.** The archive isn't truly self-contained — you still need the exact same embedding model to embed queries. The "self-contained" promise is an illusion. You always need the sidecar model.

This leads to cascading issues:

1. **Model pinning.** Vectors from `nomic-embed-text:v1.5` are meaningless to `nomic-embed-text:v2`. The archive is married to a specific model version forever (or until you rebuild/overlay).
2. **Archive bloat.** 15 GB of vectors for Wikipedia, tightly coupled to one model. Upgrade the model → the 15 GB is dead weight.
3. **Build complexity.** Archive creation now requires GPU infrastructure to compute embeddings — a dramatically harder build than text processing alone.
4. **Staleness.** Embedding models improve rapidly. A 2024 model's vectors baked into an archive are measurably worse than a 2026 model's by the time anyone uses them.

### The better default: PLAIN_TEXT + local vector cache

OZA's real contribution to semantic search is the **PLAIN_TEXT section** — clean markdown with stable, well-defined passage boundaries. This is the hard work: parsing HTML, stripping noise, segmenting into passages, normalizing text. It's content-derived, model-independent, and ages well.

Vectors are the easy part: run any embedding model over the passages. This can happen at runtime, locally, with whatever model the user has available — and cached for reuse.

```
OZA archive (the distribution artifact):
  PLAIN_TEXT section:  Clean markdown with passage boundaries
  Trigram indices:     Keyword search (always works)
  Content:             Original HTML, CSS, images
  No vectors.          No model dependency. No coupling.

Local vector cache (a runtime artifact):
  wikipedia-en.oza.vecindex:  HNSW index + vectors
  Built on first use by ozamcp + Ollama
  Rebuilt freely when the user upgrades their embedding model
  Not distributed. Not part of the archive.
```

The separation is clean:
- **OZA owns the content** — text, passages, structure, search indices
- **The runtime owns the vectors** — computed locally, cached locally, rebuilt freely

### How the default path works

```
First run:
  ozamcp opens wikipedia-en.oza
    → Finds PLAIN_TEXT section (passages available)
    → No VECTOR_EMBEDDINGS section in archive
    → No .vecindex cache file exists yet
    → Detects Ollama at localhost:11434
    → "Building vector index with nomic-embed-text... (one-time, ~3 hours for Wikipedia)"
    → Reads each passage from PLAIN_TEXT
    → Calls Ollama POST /api/embed in batches of 256
    → Builds HNSW index from returned vectors
    → Saves to wikipedia-en.oza.vecindex
    → Semantic search now available

Subsequent runs:
  ozamcp opens wikipedia-en.oza
    → Finds .vecindex cache → loads HNSW index
    → Verifies cache was built with same model version
    → Semantic search available immediately (~2 seconds to load)

Model upgrade:
  ollama pull nomic-embed-text:v2
  ozamcp --rebuild-index wikipedia-en.oza
    → Rebuilds .vecindex with new model
    → Better retrieval quality, same archive, no re-download

No Ollama available:
  ozamcp opens wikipedia-en.oza
    → No Ollama, no .vecindex cache
    → Trigram search works perfectly
    → "Semantic search unavailable (install Ollama for semantic search)"
```

### Why this is better

| Aspect | Vectors in archive | Vectors as local cache |
|--------|-------------------|----------------------|
| Archive size | 100+ GB (Wikipedia) | 85 GB (no vector bloat) |
| Model coupling | Permanent — married to one model | None — rebuild cache with any model |
| Model upgrades | Rebuild archive or ship overlay | `ollama pull` + rebuild cache |
| Build complexity | Requires GPU for embedding | Text processing only |
| Distribution size | Includes 15 GB of vectors | 15 GB smaller |
| First-run cost | None (pre-computed) | Hours (one-time, for large archives) |
| Offline without Ollama | Semantic search works if model available | Trigram only |

The trade-off is clear: pre-computed vectors save first-run time at the cost of permanent model coupling and archive bloat. For most deployments, the one-time indexing cost is acceptable — especially since `ozamcp` can build the cache in the background while trigram search is immediately available.

---

## Part 9: Ollama as the Embedding Backend

### Why Ollama is the natural choice

Managing embedding models directly (ONNX files, tokenizers, GPU detection) is complexity that `ozamcp` shouldn't own. Ollama already solves all of it — model downloads, GPU acceleration, versioning, serving. And crucially, most users who care about local LLM + RAG already have Ollama installed.

```
ozamcp's embedding responsibility reduces to:
  POST http://localhost:11434/api/embed
  { "model": "nomic-embed-text", "input": ["query text"] }
  → [0.0234, -0.1567, ...]

That's it. One HTTP call.
```

### Architecture

```
┌────────────────────────────────────────────────────────┐
│  Frontier LLM (Claude API, ChatGPT API, or Ollama)    │
└──────────────────────────┬─────────────────────────────┘
                           │ MCP
                           ▼
┌────────────────────────────────────────────────────────┐
│  ozamcp  (pure Go, ~15 MB, no C dependencies)         │
│                                                        │
│  On semantic search query:                             │
│    1. POST Ollama /api/embed → query vector            │
│    2. Search cached HNSW index → nearest passages      │
│    3. Read passage text from PLAIN_TEXT section         │
│    4. Return text to LLM                               │
└──────────────┬─────────────────────┬──────────────────┘
               │ file I/O            │ localhost HTTP
               ▼                     ▼
┌──────────────────────┐  ┌────────────────────────────┐
│  OZA Archive         │  │  Ollama                    │
│  (content + text)    │  │  (model management)        │
│                      │  │                            │
│  + .vecindex cache   │  │  nomic-embed-text (274 MB) │
│  (vectors + HNSW)    │  │  all-minilm (46 MB)        │
│                      │  │  mxbai-embed-large (670MB) │
└──────────────────────┘  │                            │
                          │  Also serves LLMs:         │
                          │  llama3.1, mistral, etc.   │
                          └────────────────────────────┘
```

### What ozamcp gains

**1. Zero model management.** `ozamcp` doesn't ship, download, or manage any model files. Users do `ollama pull nomic-embed-text` once and it's available to every archive.

**2. GPU acceleration for free.** Ollama auto-detects CUDA, ROCm, and Metal. `ozamcp` gets GPU-accelerated embeddings without a single line of GPU code.

**3. Pure Go binary.** No ONNX runtime, no C dependencies, no platform-specific builds. `ozamcp` stays a ~15 MB Go binary that cross-compiles trivially.

**4. Model sharing.** If the user already runs Ollama for local LLMs (llama3, mistral), the embedding model runs in the same Ollama process. No duplicate runtime.

**5. Trivial model upgrades.** `ollama pull nomic-embed-text:v2` → `ozamcp --rebuild-index` → done. Better retrieval quality, same archive, no re-download.

### Concrete setup

```bash
# One-time: install an embedding model
ollama pull nomic-embed-text

# Build the OZA archive (text processing only — no embeddings needed)
zim2oza --plain-text wikipedia-en.zim wikipedia-en.oza

# Serve via MCP — ozamcp builds vector cache on first run using Ollama
ozamcp --ollama http://localhost:11434 wikipedia-en.oza
# First run: "Building vector index with nomic-embed-text... (estimated 3 hours)"
# Subsequent runs: loads cached index in ~2 seconds

# Connect any LLM via MCP
# Claude Desktop (~/.config/claude/claude_desktop_config.json):
{
  "mcpServers": {
    "wikipedia": {
      "command": "ozamcp",
      "args": ["--ollama", "http://localhost:11434", "wikipedia-en.oza"]
    }
  }
}
```

### The fully local stack

Everything on one machine, zero cloud dependency:

```bash
# Ollama serves BOTH the reasoning LLM and the embedding model
ollama pull llama3.1:8b          # reasoning
ollama pull nomic-embed-text     # embeddings

# ozamcp connects to Ollama for embeddings
ozamcp --ollama http://localhost:11434 wikipedia-en.oza

# A local LLM client (Open WebUI, etc.) connects to:
#   - Ollama for reasoning (llama3.1)
#   - ozamcp via MCP for retrieval (which uses Ollama for embedding)

# No API keys. No internet. No cloud. Full semantic search.
```

### Choosing an embedding model

Any Ollama-compatible embedding model works. The choice is made by the user at runtime, not by the archive publisher at build time:

| Ollama Model | Dims | Size | Quality | Speed (M2 Pro) | Notes |
|-------------|------|------|---------|----------------|-------|
| `all-minilm` | 384 | 46 MB | Good | ~5ms | Smallest, fastest |
| `nomic-embed-text` | 768 | 274 MB | Better | ~12ms | Recommended default |
| `mxbai-embed-large` | 1024 | 670 MB | Best | ~20ms | Highest quality |
| `snowflake-arctic-embed` | 1024 | 670 MB | Best | ~20ms | Strong on benchmarks |

**Recommended: `nomic-embed-text`** — good balance of quality, speed, and size. Open weights, well-supported in Ollama.

### Latency comparison

| Embedding backend | Per-query latency | Setup | Offline? |
|-------------------|-------------------|-------|----------|
| **Ollama (localhost)** | **~10-20ms** | **`ollama pull` + one flag** | **Yes** |
| In-process ONNX | ~5ms | Model files, ONNX runtime, platform builds | Yes |
| Voyage API | ~100ms | API key required | No |
| OpenAI API | ~80ms | API key required | No |

The 10-20ms Ollama overhead is imperceptible — the frontier LLM's own response takes 1-10 seconds.

### Vector cache format

The `.vecindex` cache file is a simple, non-OZA binary file managed entirely by `ozamcp`:

```
wikipedia-en.oza.vecindex:
  Header:
    magic:         "OZAV" (4 bytes)
    version:       1 (u32)
    archive_uuid:  (16 bytes — must match the OZA archive)
    model_name:    "nomic-embed-text:v1.5" (length-prefixed string)
    model_hash:    (32 bytes — SHA256 of Ollama model digest)
    dimensions:    768 (u32)
    vector_count:  34,000,000 (u32)
    created:       unix timestamp (u64)

  Vector Table:
    Per vector: entry_id (u32), passage_id (u32), [768 float32 values]

  HNSW Index:
    Serialized HNSW graph (same format as described in Part 2)
```

**Cache invalidation:** `ozamcp` checks `archive_uuid` and `model_hash` on load. If either has changed (archive updated or model upgraded), the cache is stale and must be rebuilt. The user explicitly triggers rebuilds with `--rebuild-index`.

### Fallback chain

`ozamcp` escalates capabilities based on what's available:

```
1. Check for pre-computed VECTOR_EMBEDDINGS section in the archive
   → If present: use it (no Ollama needed for search, but still needed for query embedding)

2. Check for .vecindex cache file
   → If present and valid: load HNSW index, use Ollama for query embedding only

3. Check for Ollama at localhost:11434 (or OLLAMA_HOST)
   → If available: offer to build .vecindex cache in background
   → While building: trigram search available immediately

4. No Ollama, no cache, no pre-computed vectors
   → Trigram search works perfectly
   → "Hint: install Ollama and run `ollama pull nomic-embed-text` for semantic search"
```

Graceful capability escalation. The archive is always usable. Semantic search is an enhancement, never a requirement.

### Build-time indexing for large archives

For Wikipedia-scale archives (34M passages), the first-run indexing takes hours. `ozamcp` should handle this gracefully:

```
ozamcp --ollama http://localhost:11434 wikipedia-en.oza

  ╭──────────────────────────────────────────────────────────╮
  │  Building vector index...                                │
  │  Model: nomic-embed-text (768d)                          │
  │  Passages: 34,000,000                                    │
  │  Progress: 2,340,000 / 34,000,000 (6.9%)                │
  │  Speed: ~3,000 passages/sec                              │
  │  ETA: ~2.9 hours                                         │
  │                                                          │
  │  Trigram search is available now.                         │
  │  Semantic search will activate when indexing completes.   │
  ╰──────────────────────────────────────────────────────────╯
```

Batching is critical: `ozaembed` should send 64-256 passages per Ollama API call. The `/api/embed` endpoint accepts arrays, returning all vectors in one response. This closes most of the gap with in-process ONNX (~3,000 passages/sec vs ~4,000).

For smaller archives (50K-500K passages), indexing takes minutes, not hours — negligible.

---

## Part 10: Distributable Vector Sidecars — Instant Semantic Search, No Lock-In

### The best of both worlds

The cached index path (Part 9) eliminates model coupling but imposes a first-run indexing cost — hours for Wikipedia. Pre-computed vectors in the archive (Part 2) eliminate the wait but couple the archive to a model forever.

**Distributable vector sidecars** solve both problems: the archive publisher pre-builds `.vecindex` files for popular Ollama embedding models and distributes them alongside the archive as separate downloads.

```
Download page:

  wikipedia-en-2026.oza                               85 GB  (content + PLAIN_TEXT)
  wikipedia-en-2026.oza.nomic-embed-text.vecindex      12 GB  (vectors for nomic-embed-text)
  wikipedia-en-2026.oza.all-minilm.vecindex             5 GB  (vectors for all-minilm)
  wikipedia-en-2026.oza.mxbai-embed-large.vecindex     22 GB  (vectors for mxbai-embed-large)
```

The user downloads the archive plus whichever sidecar matches the Ollama embedding model they already have. Semantic search works on first launch with zero indexing delay.

### How it works

```
User has Ollama with nomic-embed-text installed.

1. Downloads wikipedia-en-2026.oza (85 GB)
2. Downloads wikipedia-en-2026.oza.nomic-embed-text.vecindex (12 GB)
3. Places both in the same directory

ozamcp --ollama http://localhost:11434 wikipedia-en-2026.oza
  → Opens archive, finds PLAIN_TEXT section
  → Scans for .vecindex files matching available Ollama models
  → Finds nomic-embed-text.vecindex → verifies archive_uuid match
  → Loads HNSW index (~2 seconds)
  → Semantic search available immediately
  → Uses Ollama nomic-embed-text for query embedding only (~10ms/query)
```

### No sidecar? No problem.

```
User has Ollama with snowflake-arctic-embed (no sidecar available):

ozamcp --ollama http://localhost:11434 wikipedia-en-2026.oza
  → No matching .vecindex sidecar found
  → "Building vector index with snowflake-arctic-embed... (estimated 3 hours)"
  → Trigram search available immediately while indexing
  → Saves to wikipedia-en-2026.oza.snowflake-arctic-embed.vecindex
  → Next launch: instant
```

### Why this beats baking vectors into the archive

| Aspect | Vectors in archive (0x0100) | Distributable sidecars |
|--------|---------------------------|----------------------|
| Archive size | 100+ GB (includes 15 GB vectors) | 85 GB (content only) |
| Model coupling | Permanent (married to one model) | None (user picks their model) |
| Model upgrade | Rebuild archive or ship overlay | Download new sidecar or rebuild locally |
| Publisher effort | Pick one model, hope it ages well | Pre-build 2-3 sidecars for popular models |
| User flexibility | Must use publisher's model | Use any model they have |
| First-run delay | None | None (if sidecar available) or hours (if not) |
| Partial download | All or nothing | Archive first, sidecar later |

### Sidecar naming convention

```
{archive_name}.{ollama_model_name}.vecindex

Examples:
  wikipedia-en-2026.oza.nomic-embed-text.vecindex
  wikipedia-en-2026.oza.all-minilm.vecindex
  wikipedia-en-2026.oza.mxbai-embed-large.vecindex
  medical-encyclopedia.oza.nomic-embed-text.vecindex
```

`ozamcp` scans the archive's directory for files matching `{archive_name}.*.vecindex`, parses the model name from the filename, and cross-references against Ollama's available models.

### Sidecar file format

Same as the cached index format from Part 9:

```
Header:
  magic:         "OZAV" (4 bytes)
  version:       1 (u32)
  archive_uuid:  (16 bytes — must match the OZA archive)
  model_name:    "nomic-embed-text" (length-prefixed)
  model_hash:    (32 bytes — SHA256 of Ollama model digest)
  dimensions:    768 (u32)
  precision:     2 (u8 — 0=f32, 1=f16, 2=int8, 3=binary)
  vector_count:  34,000,000 (u32)
  created:       unix timestamp (u64)

Vector Table + HNSW Index (same as Part 2/9)
```

Whether the user downloads a sidecar from the publisher or `ozamcp` builds one locally, the file format is identical. A locally-built cache IS a sidecar — it can be shared with others running the same model.

### Publisher build workflow

```bash
# Build the archive (text processing only)
zim2oza --plain-text wikipedia-en.zim wikipedia-en-2026.oza

# Pre-build sidecars for popular models (can run in parallel)
ozaembed --model nomic-embed-text \
         --ollama-url http://localhost:11434 \
         --precision int8 \
         --output wikipedia-en-2026.oza.nomic-embed-text.vecindex \
         wikipedia-en-2026.oza &

ozaembed --model all-minilm \
         --ollama-url http://localhost:11434 \
         --precision int8 \
         --output wikipedia-en-2026.oza.all-minilm.vecindex \
         wikipedia-en-2026.oza &

ozaembed --model mxbai-embed-large \
         --ollama-url http://localhost:11434 \
         --precision int8 \
         --output wikipedia-en-2026.oza.mxbai-embed-large.vecindex \
         wikipedia-en-2026.oza &

wait
# Upload archive + all sidecars to distribution server
```

### The complete fallback chain (updated)

```
1. Check for pre-computed VECTOR_EMBEDDINGS section in the archive
   → If present: use it (legacy/air-gapped path, still needs query embedding model)

2. Scan for .vecindex sidecar files matching available Ollama models
   → If found: load best match (prefer higher-quality models)
   → Verify archive_uuid and model_hash

3. Check for any .vecindex file (even if model doesn't match Ollama)
   → Offer to download the matching Ollama model: "ollama pull nomic-embed-text"

4. Ollama available but no sidecar
   → Build .vecindex in background using Ollama
   → Trigram search available immediately

5. No Ollama, no sidecars, no in-archive vectors
   → Trigram search only
   → "Hint: install Ollama for semantic search, or download a .vecindex sidecar"
```

### Community-built sidecars

Because the sidecar format is simple and model-specific, the community can build and share them independently:

```
# Anyone with the archive + Ollama can build a sidecar
ozaembed --model my-custom-domain-model \
         --ollama-url http://localhost:11434 \
         --output wikipedia-en-2026.oza.my-custom-domain-model.vecindex \
         wikipedia-en-2026.oza

# Share it — others with the same archive and model can use it directly
```

The archive publisher doesn't need to anticipate every model. Users build and share sidecars for niche models. The ecosystem scales without central coordination.

### Sidecar integrity, binding, and signing

A distributable sidecar is a trust artifact — it determines which passages an LLM sees in response to queries. A poisoned sidecar could systematically bias retrieval toward specific content, suppress relevant results, or silently degrade search quality. A mismatched sidecar (wrong archive, wrong model version) produces garbage results without any visible error.

The sidecar format addresses these risks at three levels:

#### 1. Archive binding (always verified)

Every sidecar is cryptographically bound to the archive it was built from:

```
Sidecar header:
  archive_uuid:    (16 bytes — must match the OZA archive's UUID)
  archive_sha256:  (32 bytes — SHA-256 of the archive's file-level checksum)
  passage_count:   (u32 — must match PLAIN_TEXT section's total_passages)
```

`ozamcp` verifies all three on load. If any mismatch is detected, the sidecar is rejected entirely — no partial trust, no "close enough." This prevents accidentally using a sidecar built from a different version of the same archive (e.g., last month's Wikipedia with this month's sidecar).

#### 2. Content integrity (always verified)

The sidecar includes a SHA-256 checksum of its own content:

```
Sidecar footer (last 32 bytes):
  sha256:  SHA-256 of all bytes before this field
```

This mirrors the OZA archive's own integrity design (FORMAT.md §6.1). `ozamcp` verifies the checksum on load. A corrupted or tampered sidecar is rejected before any vectors are used.

#### 3. Publisher signatures (optional, for distributed sidecars)

For sidecars downloaded from third parties, optional Ed25519 signatures provide publisher authentication — the same scheme used by the OZA archive itself (FORMAT.md §6.2):

```
Sidecar header (extended):
  signature_count:  (u32, 0 if unsigned)

  Per signature (128 bytes):
    public_key:   (32 bytes — Ed25519 public key)
    signature:    (64 bytes — signs the sidecar's content SHA-256)
    key_id:       (u32 — for key rotation)
    reserved:     (28 bytes)
```

**Verification model:**

```
Publisher builds sidecar → signs with their Ed25519 private key
Publisher uploads: wikipedia-en-2026.oza.nomic-embed-text.vecindex

User downloads sidecar
ozamcp loads sidecar:
  1. Verify content SHA-256 (integrity)
  2. Verify archive_uuid + archive_sha256 (binding)
  3. If signatures present:
     → Check public_key against trusted keys (config, TOFU, or archive's own signatures)
     → Verify signature over content SHA-256
     → If signature invalid: reject sidecar
  4. If no signatures:
     → Accept for locally-built sidecars
     → Warn for downloaded sidecars: "Unsigned sidecar — cannot verify publisher"
```

**Trust model mirrors the archive:** OZA doesn't define a PKI (FORMAT.md §6.2). Key distribution is out of scope. A reader obtains trusted public keys externally — config file, well-known URL, or TOFU (trust on first use). The same mechanism works for sidecar signatures.

**Practical flow:**

```
# Publisher signs their sidecars
ozaembed --model nomic-embed-text \
         --sign-key /path/to/publisher.ed25519 \
         --output wikipedia-en-2026.oza.nomic-embed-text.vecindex \
         wikipedia-en-2026.oza

# User configures trusted keys (once)
ozamcp --trust-key "publisher-name=ed25519:abc123..." \
       wikipedia-en-2026.oza

# Or: trust the archive publisher's key (if archive is signed)
# ozamcp auto-trusts sidecars signed by the same key that signed the archive
```

**When the archive is signed:** If the archive itself carries Ed25519 signatures, `ozamcp` can auto-trust sidecars signed by the same key. This is the simplest flow — the publisher signs both the archive and its sidecars with the same key. No additional configuration needed.

#### 4. Build metadata (informational)

The sidecar header includes metadata about how it was built, for auditability:

```
Sidecar header (metadata section):
  model_name:       "nomic-embed-text" (length-prefixed)
  model_hash:       (32 bytes — SHA-256 of Ollama model digest or ONNX weights)
  dimensions:       768 (u32)
  precision:        2 (u8 — 0=f32, 1=f16, 2=int8, 3=binary)
  vector_count:     34,000,000 (u32)
  created:          unix timestamp (u64)
  builder_version:  "ozaembed 1.2.0" (length-prefixed)
  build_host:       (optional, length-prefixed — for audit trail)
```

The `model_hash` field is critical: it pins not just the model name but the exact weights. Two different fine-tunes of `nomic-embed-text` would have different hashes, preventing silent incompatibility.

#### Summary of verification levels

| Check | When | What it prevents |
|-------|------|-----------------|
| Content SHA-256 | Always, on load | Corruption, truncation, bit-rot |
| Archive binding (UUID + SHA-256) | Always, on load | Wrong archive version, mismatched passages |
| Passage count match | Always, on load | Partial or outdated sidecar |
| Model hash vs Ollama | On query (if Ollama available) | Wrong model version producing query vectors |
| Ed25519 signature | If signatures present | Tampering, impersonation, supply chain attacks |
| Auto-trust (archive key = sidecar key) | If archive is signed | Simplifies trust for same-publisher sidecars |

**Locally-built sidecars** (built by `ozamcp` on the user's own machine) skip signature verification — they are trusted by construction. Only downloaded/shared sidecars benefit from publisher signatures.

---

## Part 11: Pre-Computed Vectors in the Archive (Air-Gapped / Appliance Scenarios)

The VECTOR_EMBEDDINGS section (0x0100) described in Parts 2-5 remains available for scenarios where **no Ollama, no model management, and no first-run indexing** are acceptable constraints.

### When to bake vectors into the archive

| Scenario | Why pre-compute? |
|----------|-----------------|
| **Air-gapped / SCIF** | No network. No Ollama install possible. USB drive with archive + ONNX model must work immediately. |
| **Embedded appliance** | A kiosk, medical device, or field terminal with no model management infrastructure. |
| **Controlled ecosystem** | Publisher ships both the archive and the reader. Model pinning is acceptable because both sides are controlled. |
| **Submarine / space station** | Extreme offline. The archive is the only software artifact available. |

### How it works

```bash
# Build with vectors baked into the archive
ozaembed --model nomic-embed-text \
         --ollama-url http://localhost:11434 \
         --precision int8 \
         --embed-in-archive \
         wikipedia-en.oza
# Archive is now ~100 GB (85 GB content + 15 GB vectors)

# Deploy to air-gapped machine
# Need the archive + ozamcp + matching ONNX model for query embedding
scp wikipedia-en.oza target:/data/
scp ozamcp target:/usr/local/bin/
scp nomic-embed-text.onnx target:/usr/local/share/ozamcp/models/

# Serve — instant semantic search
ozamcp /data/wikipedia-en.oza
```

**Note:** Even with pre-computed vectors in the archive, `ozamcp` still needs the embedding model at runtime to embed query strings. The vectors in the archive are for passages; the query must be embedded with the same model to produce a comparable vector. For air-gapped deployments, this means shipping the ONNX model file alongside `ozamcp`, or pre-installing Ollama with the model.

### The coupling trade-off

Pre-computed vectors lock the archive to a specific model version. The mitigation strategies still apply:

- **Multi-model spaces** — ship vectors for 2-3 models within the archive's VECTOR_EMBEDDINGS section
- **Overlay files** — ship `.oza.embeddings` companion files for model upgrades without rebuilding the full archive
- **Graceful fallback** — if `ozamcp` can't load the archive's model, it falls back to trigram search; if Ollama is available, it can build a sidecar with a different model

### The recommended decision framework

```
Publishing for general consumption?
  → Ship PLAIN_TEXT only. Optionally pre-build sidecars for popular models.
     The archive stays small, model-agnostic, and future-proof.

Deploying to a controlled environment with Ollama?
  → Ship PLAIN_TEXT + matching sidecar. Instant semantic search, easy model upgrades.

Deploying to an air-gapped / no-Ollama environment?
  → Bake VECTOR_EMBEDDINGS into the archive. Accept the model coupling.
     Ship the matching ONNX model alongside.
```

For the vast majority of deployments — developer workstations, edge servers, laptops, anyone with Ollama — distributable sidecars are the right path. Pre-computed vectors in the archive are the escape hatch for extreme offline scenarios, not the default.

---

## Part 12: Embedding Data Size Estimates

### Per-vector cost

The storage cost per passage vector depends on the model's dimensionality and the quantization precision:

| Model | Dimensions | Precision | Bytes/vector | Notes |
|-------|-----------|-----------|-------------|-------|
| `nomic-embed-text` | 768 | float32 | 3,072 | Full precision, rarely needed |
| `nomic-embed-text` | 768 | float16 | 1,536 | Good precision/size trade-off |
| `nomic-embed-text` | 768 | int8 | 768 | Recommended default |
| `all-minilm` | 384 | float32 | 1,536 | |
| `all-minilm` | 384 | int8 | 384 | Smallest practical option |

With HNSW index overhead (neighbor lists, metadata), the effective cost is roughly **~110% of raw vector bytes**. So `nomic-embed-text` at int8 costs **~850 bytes/passage** and `all-minilm` at int8 costs **~430 bytes/passage**.

### Passage count rules of thumb

The number of vectors scales with passage count, not article count. Typical passage densities by content type:

| Content type | Avg article size | Passages/article | Why |
|-------------|-----------------|-------------------|-----|
| Wiktionary entry | 2-5 KB | 1-2 | Short definitions, few sections |
| Simple Wikipedia article | 5-15 KB | 3-5 | Brief articles, few headings |
| Full Wikipedia article | 30-80 KB | 10-25 | Multiple sections, tables, references |
| Medical/legal document | 50-200 KB | 15-50 | Dense, heavily sectioned |
| Stack Overflow Q+A | 5-20 KB | 2-4 | Question + top answers |

### Per-archive estimates

Using `nomic-embed-text` (768d, int8, ~850 bytes/vector):

| Archive | Articles | Est. passages | Vector data | HNSW index | Total sidecar |
|---------|---------|---------------|-------------|------------|---------------|
| Simple English Wikipedia | ~230K | ~900K | 690 MB | 75 MB | **~770 MB** |
| English Wiktionary | ~1M | ~1.5M | 1.15 GB | 125 MB | **~1.3 GB** |
| Full English Wikipedia | ~6.8M | ~100M | 76 GB | 8.5 GB | **~85 GB** |
| Stack Overflow | ~24M | ~60M | 46 GB | 5 GB | **~51 GB** |

Using `all-minilm` (384d, int8, ~430 bytes/vector) cuts these roughly in half.

### The "nominal case"

For planning purposes, here are representative archive profiles:

| Profile | Example | OZA size | Sidecar (nomic int8) | Sidecar as % of OZA |
|---------|---------|----------|---------------------|---------------------|
| Small | Simple English Wikipedia | ~1.5 GB | ~770 MB | ~50% |
| Medium | English Wiktionary | ~6 GB | ~1.3 GB | ~22% |
| Large | Full English Wikipedia | ~85 GB | ~85 GB | ~100% |
| Huge | Stack Overflow | ~80 GB | ~51 GB | ~64% |

**Key takeaway:** for small-to-medium archives, the sidecar is a modest fraction of the archive. For large archives with many short passages (like full Wikipedia), the sidecar can approach the archive size itself — which is another strong argument for sidecars over baked-in vectors, since users only download the sidecar for the model they actually use.

# OZA Testing Plan — Comprehensive Validation & Benchmarking

## Goals

1. **Fidelity** — Every byte of content survives ZIM → OZA conversion and round-trips correctly
2. **Performance** — File size competitive or better than ZIM; random access latency suitable for HTTP serving; search responsive
3. **Conversion time** — Predictable, profiled, with clear bottleneck attribution
4. **Content intelligence** — Identify and act on patterns (HTML → markdown, PDF extraction, dedup) that make OZA archives more useful than their ZIM sources
5. **Incremental updates** — Validate the chunk-copy rebuild approach on real corpus diffs

---

## 1. Representative ZIM Corpus

### 1.1 Selection criteria

ZIM archives vary dramatically in size, content type, language, and structure. The test corpus must cover these axes:

| Axis | Variants | Why it matters |
|------|----------|----------------|
| **Size** | Tiny (<1 MB), Small (1–50 MB), Medium (50 MB–2 GB) | Memory pressure, conversion time, chunk cache behavior |
| **Content type** | HTML-heavy, image-heavy, PDF-container, mixed, video | Compression grouping, dictionary training, transformation pipeline |
| **Source** | Wikipedia, Wiktionary, Wikisource, StackExchange, Gutenberg, TED, DevDocs, Zimit scrapes | Each has unique namespace conventions, front-article patterns, chrome assets |
| **Language** | English, CJK (Chinese/Japanese/Korean), Arabic/Hebrew/Farsi (RTL), Cyrillic, Indic | Trigram vs bigram search, NFC normalization, text extraction, script coverage |
| **Structure** | Deep redirect chains, disambiguation pages, multi-part books, Q&A, dictionaries | Redirect resolution, browse filtering, entry classification |

All files available from `https://download.kiwix.org/zim/{category}/{filename}`.

### 1.2 Tier 1 — Automated CI (< 50 MB each, ~150 MB total)

Downloaded by `make testdata`. These run in CI on every PR. Selected to maximize structural diversity at minimal size.

| Local name | Remote path | Size | What it tests |
|------------|-------------|------|---------------|
| `small.zim` | openzim/zim-testing-suite (GitHub) | 300 KB | Already used. Smoke test, round-trip, redirects |
| `ray_charles.zim` | `wikipedia/wikipedia_en_ray-charles_maxi_*.zim` | 2.7 MB | Single Wikipedia article with images, infobox, citations |
| `ray_charles_nopic.zim` | `wikipedia/wikipedia_en_ray-charles_nopic_*.zim` | 1.6 MB | Same article, text-only — compare image impact on size |
| `top100_mini.zim` | `wikipedia/wikipedia_en_100_mini_*.zim` | 4.3 MB | 100 EN articles, intro-only. Multiple entries, redirects |
| `ar_chemistry.zim` | `wikipedia/wikipedia_ar_chemistry_mini_*.zim` | 11 MB | **RTL (Arabic)** + domain-specific STEM content |
| `zh_chemistry.zim` | `wikipedia/wikipedia_zh_chemistry_mini_*.zim` | 13 MB | **CJK (Chinese)** + domain-specific. Bigram search validation |
| `wiktionary_he.zim` | `wiktionary/wiktionary_he_all_nopic_*.zim` | 40 MB | **RTL dictionary (Hebrew)**. High redirect ratio (inflected → lemma) |
| `devdocs_go.zim` | `devdocs/devdocs_en_go_*.zim` | 1.5 MB | Developer docs. Structured text, code blocks, no images |
| `se_community.zim` | `stack_exchange/communitybuilding.stackexchange.com_en_all_*.zim` | 6 MB | **Q&A format**. Tags, user cards, vote widgets |
| `gutenberg_ar.zim` | `gutenberg/gutenberg_ar_all_*.zim` | 2.1 MB | **Arabic books (RTL)**. Book-like content structure |
| `gutenberg_ko.zim` | `gutenberg/gutenberg_ko_all_*.zim` | 2.2 MB | **Korean books (CJK)**. Hangul text |
| `wikiquote_ja.zim` | `wikiquote/wikiquote_ja_all_nopic_*.zim` | 5.3 MB | **Japanese Wikiquote**. CJK short-form text |
| `vikidia_ru.zim` | `vikidia/vikidia_ru_all_maxi_*.zim` | 10 MB | **Cyrillic** children's encyclopedia with images |
| `wiktionary_yi.zim` | `wiktionary/wiktionary_yi_all_nopic_*.zim` | 1.4 MB | **Yiddish (Hebrew script, RTL)**. Rare script coverage |
| `ted_street_art.zim` | `ted/ted_mul_street-art_*.zim` | 37 MB | **Video content** (thumbnails, subtitles). Non-text MIME types |
| `gobyexample.zim` | `zimit/gobyexample.com_en_all_*.zim` | 322 KB | **Zimit-scraped site**. Tests zimit-specific structure |

**Coverage matrix for Tier 1:**

| Axis | Coverage |
|------|----------|
| Scripts | Latin, Arabic, Hebrew, Yiddish, Chinese, Korean, Japanese, Cyrillic |
| Sources | Wikipedia, Wiktionary, Wikiquote, Gutenberg, StackExchange, TED, DevDocs, Vikidia, Zimit |
| Content | HTML articles, images, video/subtitles, code blocks, Q&A, dictionaries, books |
| Features | Redirects, front articles, infoboxes, math, RTL, CJK bigrams, mini/nopic/maxi variants |

### 1.3 Tier 2 — Local benchmark suite (50 MB – 2 GB each)

Downloaded on demand by `make testdata-bench`. These exercise compression, search indexing, and conversion time at realistic scale.

| Local name | Remote path | Size | What it tests |
|------------|-------------|------|---------------|
| **Wikipedia — scale & variants** | | | |
| `wp_simple_mini.zim` | `wikipedia/wikipedia_en_simple_all_mini_*.zim` | 441 MB | Simple EN Wikipedia intro-only. Heavy redirects (synonyms) |
| `wp_simple_nopic.zim` | `wikipedia/wikipedia_en_simple_all_nopic_*.zim` | 921 MB | Same corpus, full text. Compare mini vs full compression |
| `wp_en_top_mini.zim` | `wikipedia/wikipedia_en_top_mini_*.zim` | 315 MB | Top ~50K EN articles, intro. Realistic entry count |
| `wp_en_medicine_nopic.zim` | `wikipedia/wikipedia_en_medicine_nopic_*.zim` | 820 MB | Medical articles, text-only. Long-form, many redirects |
| `wp_en_chemistry_maxi.zim` | `wikipedia/wikipedia_en_chemistry_maxi_*.zim` | 470 MB | STEM + images. Chemical formulas, tables, diagrams |
| `wp_en_physics_nopic.zim` | `wikipedia/wikipedia_en_physics_nopic_*.zim` | 292 MB | Physics text. Math/equation-heavy content |
| `wp_en_climate_maxi.zim` | `wikipedia/wikipedia_en_climate-change_maxi_*.zim` | 193 MB | Climate change + images. Graphs, charts, data tables |
| **CJK — bigram search & compression** | | | |
| `wp_ja_top_mini.zim` | `wikipedia/wikipedia_ja_top_mini_*.zim` | 168 MB | **Japanese** top articles. Mixed scripts (kanji+hiragana+katakana) |
| `wp_ja_top_nopic.zim` | `wikipedia/wikipedia_ja_top_nopic_*.zim` | 1.5 GB | Japanese full text. CJK compression at scale |
| `wp_ko_top_mini.zim` | `wikipedia/wikipedia_ko_top_mini_*.zim` | 150 MB | **Korean** top articles. Pure Hangul text |
| `wp_ko_all_mini.zim` | `wikipedia/wikipedia_ko_all_mini_*.zim` | 1.2 GB | All Korean, intro-only. High entry count CJK |
| `wp_zh_top_mini.zim` | `wikipedia/wikipedia_zh_top_mini_*.zim` | 244 MB | **Chinese** top articles. Simplified+Traditional mix |
| `wp_zh_movies_maxi.zim` | `wikipedia/wikipedia_zh_movies_maxi_*.zim` | 666 MB | Chinese movies. Image-heavy CJK |
| **RTL — Arabic, Hebrew, Farsi** | | | |
| `wp_ar_top_mini.zim` | `wikipedia/wikipedia_ar_top_mini_*.zim` | 203 MB | Arabic top articles. RTL text at scale |
| `wp_ar_top_nopic.zim` | `wikipedia/wikipedia_ar_top_nopic_*.zim` | 783 MB | Arabic full text. RTL + heavy redirect tables |
| `wp_ar_medicine_maxi.zim` | `wikipedia/wikipedia_ar_medicine_maxi_*.zim` | 716 MB | Arabic medicine + images. RTL + STEM |
| `wp_he_top_mini.zim` | `wikipedia/wikipedia_he_top_mini_*.zim` | 112 MB | Hebrew top articles. RTL baseline |
| `wp_he_all_mini.zim` | `wikipedia/wikipedia_he_all_mini_*.zim` | 598 MB | Hebrew all articles intro. RTL at full entry count |
| `wp_fa_top_mini.zim` | `wikipedia/wikipedia_fa_top_mini_*.zim` | 205 MB | Farsi top. Arabic-script variant |
| **Wiktionary — dictionaries, high redirect ratios** | | | |
| `wikt_ja.zim` | `wiktionary/wiktionary_ja_all_nopic_*.zim` | 453 MB | Japanese Wiktionary. CJK dictionary, many small entries |
| `wikt_ko.zim` | `wiktionary/wiktionary_ko_all_nopic_*.zim` | 237 MB | Korean Wiktionary. Hangul definitions |
| `wikt_de.zim` | `wiktionary/wiktionary_de_all_nopic_*.zim` | 1.2 GB | German Wiktionary. Very large dictionary. Extensive redirects |
| `wikt_es.zim` | `wiktionary/wiktionary_es_all_nopic_*.zim` | 535 MB | Spanish Wiktionary. Latin-script dictionary |
| `wikt_fa.zim` | `wiktionary/wiktionary_fa_all_nopic_*.zim` | 72 MB | Farsi Wiktionary. RTL dictionary |
| **StackExchange — Q&A structure** | | | |
| `se_japanese.zim` | `stack_exchange/japanese.stackexchange.com_mul_all_*.zim` | 171 MB | Japanese Language SE. Multilingual CJK Q&A |
| `se_chinese.zim` | `stack_exchange/chinese.stackexchange.com_mul_all_*.zim` | 145 MB | Chinese Language SE. Multilingual CJK |
| `se_codegolf.zim` | `stack_exchange/codegolf.stackexchange.com_en_all_*.zim` | 372 MB | Code Golf. Unicode-heavy, unusual encodings, code blocks |
| `se_judaism.zim` | `stack_exchange/judaism.stackexchange.com_en_all_*.zim` | 280 MB | Judaism SE. Contains Hebrew/RTL snippets in English Q&A |
| `se_astronomy.zim` | `stack_exchange/astronomy.stackexchange.com_en_all_*.zim` | 187 MB | Astronomy. Image-heavy (astrophotography) |
| `se_ja_so.zim` | `stack_exchange/ja.stackoverflow.com_mul_all_*.zim` | 226 MB | Japanese Stack Overflow. Full CJK Q&A |
| **Gutenberg — book-length content** | | | |
| `gutenberg_zh.zim` | `gutenberg/gutenberg_zh_all_*.zim` | 301 MB | Chinese Gutenberg. CJK books |
| `gutenberg_it.zim` | `gutenberg/gutenberg_it_all_*.zim` | 1.0 GB | Italian Gutenberg. Large text corpus |
| `gutenberg_en_lcc_k.zim` | `gutenberg/gutenberg_en_lcc-k_*.zim` | 235 MB | EN Gutenberg, Library of Congress class K (Law). Long documents |
| `gutenberg_en_lcc_j.zim` | `gutenberg/gutenberg_en_lcc-j_*.zim` | 435 MB | EN Gutenberg, LCC J (Political science) |
| **Other sources — structural variety** | | | |
| `vikidia_fr.zim` | `vikidia/vikidia_fr_all_maxi_*.zim` | 966 MB | French Vikidia. Largest children's encyclopedia |
| `wikisource_fa.zim` | `wikisource/wikisource_fa_all_maxi_*.zim` | 286 MB | **Farsi Wikisource**. RTL full books, potential PDF-like blobs |
| `wikivoyage_zh.zim` | `wikivoyage/wikivoyage_zh_all_maxi_*.zim` | 114 MB | Chinese travel guide. CJK + maps + images |
| `wikiquote_en.zim` | `wikiquote/wikiquote_en_all_maxi_*.zim` | 880 MB | EN Wikiquote. Short-form entries, many authors |
| `worldfactbook.zim` | `other/theworldfactbook_en_all_*.zim` | 388 MB | CIA World Factbook. Highly structured tables, maps |
| `xkcd.zim` | `zimit/xkcd.com_en_all_*.zim` | 424 MB | xkcd comics. **Image-dominant** content |
| `stacks_math.zim` | `zimit/stacks.math.columbia.edu_en_all_*.zim` | 147 MB | Stacks Project. **Heavy MathML/LaTeX** |
| `mankier.zim` | `zimit/www.mankier.com_en_all_*.zim` | 181 MB | Unix man pages. Structured text, code blocks |
| `openwrt.zim` | `zimit/openwrt.org_en_all_*.zim` | 711 MB | OpenWrt wiki. Tech documentation |
| `ted_code.zim` | `ted/ted_mul_code_*.zim` | 687 MB | TED programming talks. **Video + subtitles** at scale |
| `minecraft_zh.zim` | `other/zh.minecraft.wiki_zh_all_*.zim` | 1.2 GB | Chinese Minecraft wiki. CJK + heavy images, gaming content |

### 1.4 Tier 3 — Full-scale validation (manual, build-server only)

These exceed 2 GB and require dedicated infrastructure. Used for release validation and incremental update testing.

| Local name | Remote path | Size | What it tests |
|------------|-------------|------|---------------|
| `wp_en_all_maxi.zim` | `wikipedia/wikipedia_en_all_maxi_*.zim` | ~95 GB | The canonical stress test. 6M+ entries, images |
| `wp_en_all_nopic.zim` | `wikipedia/wikipedia_en_all_nopic_*.zim` | ~25 GB | Same entry count, text-only. Isolates text pipeline |
| `wp_en_top_nopic.zim` | `wikipedia/wikipedia_en_top_nopic_*.zim` | 2.1 GB | Top EN articles, text-only. Near-boundary scale test |
| `wp_en_medicine_maxi.zim` | `wikipedia/wikipedia_en_medicine_maxi_*.zim` | 2.0 GB | Full medicine + images |
| `wp_ko_top_maxi.zim` | `wikipedia/wikipedia_ko_top_maxi_*.zim` | 2.0 GB | Korean top with images. CJK + image benchmark |
| `wp_ar_top_maxi.zim` | `wikipedia/wikipedia_ar_top_maxi_*.zim` | 2.4 GB | Arabic top + images. RTL at large scale |
| `wp_fa_all_mini.zim` | `wikipedia/wikipedia_fa_all_mini_*.zim` | 2.1 GB | Farsi all articles. RTL at full entry count |
| `gutenberg_en_all.zim` | `gutenberg/gutenberg_en_all_*.zim` | ~60 GB | Full EN Gutenberg. Multi-part books, cover pages |
| `gutenberg_es.zim` | `gutenberg/gutenberg_es_all_*.zim` | 1.7 GB | Spanish Gutenberg. Book-length text |
| `wikt_ru.zim` | `wiktionary/wiktionary_ru_all_nopic_*.zim` | 1.6 GB | Russian Wiktionary. Large Cyrillic dictionary |
| `se_math.zim` | `stack_exchange/math.stackexchange.com_en_all_*.zim` | 6.9 GB | Largest SE site. MathJax-heavy |
| `wikisource_he.zim` | `wikisource/wikisource_he_all_maxi_*.zim` | 1.2 GB | Hebrew Wikisource. RTL full books |
| `wp_en_all_mini.zim` | `wikipedia/wikipedia_en_all_mini_*.zim` | ~50 GB | All articles, intro-only — isolates entry count scaling |

### 1.5 Paired comparisons

Several files above form natural pairs for A/B analysis:

| Pair | What it reveals |
|------|----------------|
| `ray_charles.zim` vs `ray_charles_nopic.zim` | Image overhead on a single article |
| `top100_mini.zim` vs `wp_en_top_mini.zim` | 100 → 50K articles: scaling behavior |
| `wp_simple_mini.zim` vs `wp_simple_nopic.zim` | mini (intro-only) vs full text: compression ratio |
| `wp_ja_top_mini.zim` vs `wp_ja_top_nopic.zim` | CJK mini vs full: dictionary training impact |
| `wp_ar_top_mini.zim` vs `wp_ar_top_nopic.zim` | RTL scaling |
| `wp_en_chemistry_maxi.zim` vs `wp_en_physics_nopic.zim` | STEM with images vs STEM text-only |
| `wikt_ja.zim` vs `wikt_de.zim` vs `wikt_es.zim` | Dictionary compression across languages |
| `se_codegolf.zim` vs `se_astronomy.zim` | Text-heavy vs image-heavy Q&A |

### 1.6 Acquisition tooling

Implemented in the Makefile and two fetch scripts:

| Command | What it does |
|---------|-------------|
| `make testdata` | Downloads Tier 1 fixtures (~150 MB). Idempotent, skips existing files. |
| `make testdata-bench` | Downloads curated Tier 2 subset (~3 GB) to `testdata/bench/`. |
| `bash testdata/fetch-bench.sh --all` | Downloads the full Tier 2 suite (~15 GB). |
| `bash testdata/fetch-bench.sh --list` | Shows available Tier 2 files and download status. |
| `bash testdata/fetch-bench.sh wp_ja_top_mini.zim xkcd.zim` | Cherry-pick specific files. |

**Files:**
- [testdata/fetch.sh](testdata/fetch.sh) — Tier 1 download script (16 files, CI)
- [testdata/fetch-bench.sh](testdata/fetch-bench.sh) — Tier 2 download script (45 files, benchmarks)

To update URLs when new Kiwix dumps are published, edit the date suffixes in the fetch scripts.
fetch wiktionary_he.zim       "$KIWIX/wiktionary/wiktionary_he_all_nopic_2026-01.zim"
fetch devdocs_go.zim          "$KIWIX/devdocs/devdocs_en_go_2026-01.zim"
fetch se_community.zim        "$KIWIX/stack_exchange/communitybuilding.stackexchange.com_en_all_2026-02.zim"
fetch gutenberg_ar.zim        "$KIWIX/gutenberg/gutenberg_ar_all_2025-12.zim"
fetch gutenberg_ko.zim        "$KIWIX/gutenberg/gutenberg_ko_all_2026-01.zim"
fetch wikiquote_ja.zim        "$KIWIX/wikiquote/wikiquote_ja_all_nopic_2026-01.zim"
fetch vikidia_ru.zim          "$KIWIX/vikidia/vikidia_ru_all_maxi_2026-03.zim"
fetch wiktionary_yi.zim       "$KIWIX/wiktionary/wiktionary_yi_all_nopic_2026-01.zim"
fetch ted_street_art.zim      "$KIWIX/ted/ted_mul_street-art_2026-02.zim"
fetch gobyexample.zim         "$KIWIX/zimit/gobyexample.com_en_all_2026-02.zim"
```

---

## 2. Fidelity Verification

### 2.1 Byte-level content match

For every content entry in the source ZIM, verify that reading the same path from the OZA archive returns identical bytes (pre-transformation) or semantically equivalent content (post-transformation).

**Test: `TestFidelityContentMatch`**

```
For each ZIM entry E where E is content (not redirect, not metadata, not chrome):
  1. zim_content = ZIM.ReadContent(E)
  2. oza_entry = OZA.EntryByPath(E.Path)
  3. oza_content = oza_entry.ReadContent()
  4. If minification was OFF:
       assert bytes.Equal(zim_content, oza_content)
  5. If minification was ON:
       assert len(oza_content) <= len(zim_content)  // never larger
       For text/html: parse both, compare DOM tree structure
       For text/css, application/javascript: verify syntax validity
       For images: assert bytes.Equal (minify doesn't re-encode)
```

**Test: `TestFidelityRedirectChains`**

```
For each ZIM redirect R:
  1. Resolve R to final target in ZIM
  2. Resolve equivalent redirect in OZA
  3. Assert both resolve to entries with the same path
  4. Assert content at resolved target matches
```

**Test: `TestFidelityMetadata`**

```
For each ZIM metadata key in M/ namespace:
  1. Read from OZA metadata
  2. Assert value matches (allowing for key name normalization)
  3. Verify required keys present: title, language, creator, date, source
```

### 2.2 Entry count reconciliation

```
zim_content_count = count(ZIM entries where namespace=='C' and !IsRedirect)
zim_redirect_count = count(ZIM entries where IsRedirect)
zim_skipped = count(ZIM entries in X/, chrome, metadata namespaces)

assert OZA.EntryCount == zim_content_count - zim_skipped_content
assert OZA.RedirectCount == zim_redirect_count - zim_skipped_redirects
assert OZA.EntryCount + OZA.RedirectCount + skipped == ZIM.TotalEntries - metadata_count
```

### 2.3 Integrity verification

```
assert OZA.Verify() == nil           // file-level SHA-256
assert OZA.VerifyAll() == nil        // file + section + chunk
```

Run `ozaverify --all` on every converted archive as a post-conversion gate.

### 2.4 Search fidelity

For archives with search enabled:

```
For a set of 100 known-good queries per archive:
  1. Search OZA trigram index
  2. Verify result set contains expected entries
  3. Verify title-match results rank above body-only matches
  4. For CJK archives: verify bigram mode activates and returns correct results
```

---

## 3. Performance Benchmarking

### 3.1 File size comparison

**Metric:** OZA file size vs ZIM file size, broken down by section.

```
For each test archive:
  ratio = oza_size / zim_size
  Report: total ratio, content section ratio, index overhead, search index size
  Target: ratio <= 1.0 (OZA should be same size or smaller)
```

**Breakdown tool:** Extend `ozainfo` to emit a JSON size report:

```json
{
  "file_size": 1073741824,
  "zim_source_size": 1200000000,
  "ratio": 0.89,
  "sections": {
    "content": { "compressed": 950000000, "uncompressed": 3200000000 },
    "path_index": { "compressed": 12000000 },
    "title_index": { "compressed": 8000000 },
    "search_title": { "compressed": 3000000 },
    "search_body": { "compressed": 200000000 },
    "entry_table": { "compressed": 15000000 },
    "metadata": { "compressed": 4000 }
  }
}
```

### 3.2 Random access latency

**Benchmarks to add** (extend `oza/bench_test.go`):

| Benchmark | What it measures | Target |
|-----------|-----------------|--------|
| `BenchmarkEntryByPathCold` | First access, no cache | < 1 ms |
| `BenchmarkEntryByPathWarm` | Repeated access, cached | < 10 us |
| `BenchmarkReadContentSmall` | Read < 4 KB entry | < 100 us |
| `BenchmarkReadContentLarge` | Read 100 KB+ entry | < 1 ms |
| `BenchmarkReadContentParallel` | Concurrent reads, 8 goroutines | Linear scaling |
| `BenchmarkSearchShortQuery` | 1-2 word query | < 5 ms |
| `BenchmarkSearchLongQuery` | 4+ word query | < 20 ms |
| `BenchmarkSearchCJK` | CJK bigram query | < 10 ms |
| `BenchmarkBrowsePage` | 50-entry title page | < 1 ms |

**Tooling:** Run against Tier 2 archives. Use `benchstat` to compare across commits.

### 3.3 HTTP serving throughput

**Test: `BenchmarkHTTPServing`**

Using `httptest.Server` with a loaded archive:

```
1. Warm cache with 100 sequential reads
2. Measure: requests/sec for random article access (parallel clients)
3. Measure: p50/p95/p99 latency
4. Measure: memory high-water mark
5. Compare: OZA ozaserve vs Kiwix-serve on same content
```

### 3.4 Cache behavior

**Test: `BenchmarkCacheThrashing`**

```
1. Open archive with cache_size=4 (small)
2. Access entries from N different chunks where N >> cache_size
3. Measure: cache hit rate, decompression count
4. Repeat with cache_size=16, 64
5. Report: optimal cache size per archive size
```

---

## 4. Conversion Time Benchmarking

### 4.1 Phase timing

The converter already tracks phase timing via `stats.go`. Formalize this into a benchmark suite:

| Phase | Metric | Bottleneck |
|-------|--------|------------|
| Scan | entries/sec | ZIM iteration speed |
| Read | MB/sec | ZIM cluster decompression, I/O |
| Transform | entries/sec | Minification, image optimization |
| Dedup | entries/sec | SHA-256 hashing |
| Search index | entries/sec | Trigram extraction, HTML text extraction |
| Compress | MB/sec | Zstd compression, dictionary training |
| Assemble | MB/sec | Final file assembly, index building |

**Benchmark command:**

```sh
make bench-convert ZIM=testdata/wikipedia_simple.zim
# Outputs JSON stats to stdout
```

### 4.2 Compression level sweep

Test conversion at zstd levels 1, 3, 6, 9, 12, 15, 19, 22:

```
For each level:
  Record: conversion_time, output_size, content_ratio
  Plot: Pareto frontier of time vs size
  Identify: diminishing returns threshold (likely level 6-9 for most content)
```

### 4.3 Worker scaling

Test with 1, 2, 4, 8, 16 compress workers:

```
For each worker count on a fixed archive:
  Record: wall_time, CPU_time, peak_RSS
  Identify: optimal worker count per CPU count
  Check: diminishing returns (I/O bound vs CPU bound)
```

### 4.4 Dictionary training impact

```
For each test archive:
  Convert with --no-dict:     record size, time
  Convert with --dict-samples=500:  record size, time
  Convert with --dict-samples=2000: record size, time
  Convert with --dict-samples=5000: record size, time
  Report: size reduction per MIME group, training time cost
```

---

## 5. Content Pattern Analysis

### 5.1 MIME type census

Before optimizing, understand what's in each archive:

```
For each test archive:
  Count entries by MIME type
  Sum bytes by MIME type (compressed and uncompressed)
  Report: % of total size per MIME group
  Identify: dominant content type per archive source
```

**Tool:** Add `--census` flag to `zim2oza --dry-run`:

```json
{
  "mime_census": {
    "text/html": { "count": 6200000, "total_bytes": 28000000000, "avg_bytes": 4516 },
    "image/png": { "count": 1200000, "total_bytes": 8000000000, "avg_bytes": 6666 },
    "image/jpeg": { "count": 800000, "total_bytes": 12000000000, "avg_bytes": 15000 },
    "application/pdf": { "count": 45000, "total_bytes": 30000000000, "avg_bytes": 666666 },
    "text/css": { "count": 50, "total_bytes": 200000, "avg_bytes": 4000 }
  }
}
```

### 5.2 HTML → Markdown conversion value

For HTML-dominant archives (Wikipedia, Wiktionary, StackExchange):

**Analysis:**
1. Convert HTML entries to markdown (using the PLAIN_TEXT pipeline from `docs/PLAIN_TEXT.md`)
2. Compress both representations with zstd at the same level
3. Measure: size reduction from tag/attribute elimination
4. Measure: search index size reduction (trigrams over clean text vs HTML)
5. Measure: LLM token count reduction (markdown is ~30-50% fewer tokens than HTML for same content)

**Expected findings:**
- Wikipedia HTML: ~40% tag/attribute overhead → markdown saves 30-40% after compression
- StackExchange: heavier HTML (vote widgets, user cards, comment markup) → 40-50% savings
- Gutenberg: minimal HTML → modest savings, but markdown is more portable

**Decision point:** When PLAIN_TEXT section is built, should the original HTML be retained? Options:
- **Both:** PLAIN_TEXT + original HTML in content section (maximum fidelity, ~15% size increase)
- **Replace:** PLAIN_TEXT only, drop HTML (maximum compression, lossy for formatting)
- **Configurable:** `--plain-text-mode=alongside|replace` flag on `zim2oza`

### 5.3 PDF-container archives

For Gutenberg and Wikisource ZIMs where content is primarily PDF:

**Analysis pipeline:**
1. Identify PDF entries by MIME type (`application/pdf`)
2. Classify: born-digital (has text layer) vs scanned (image-only)
3. For born-digital: extract text with `pdftotext` or Go-native PDF library
4. Measure: text extraction quality (sample 50 PDFs, manual review)
5. Measure: size comparison — PDF blob vs extracted markdown + images
6. Identify: multi-chapter PDFs that should be split into per-chapter entries

**Extraction strategy (per `docs/ZIM_OBSERVATIONS.md`):**

```
PDF entry detected:
  1. Attempt text extraction (pdftotext equivalent)
  2. If text layer present and quality > threshold:
       → Store as text/markdown entry
       → Extract embedded images as separate entries
       → Optionally retain original PDF as {path}_original.pdf
  3. If scanned/image-only:
       → OCR if --ocr flag set (requires Tesseract)
       → Otherwise store original PDF (no benefit to extraction)
  4. Extract PDF metadata → OZA entry metadata
```

**Go-native options (no CGo):**
- `github.com/ledongthuc/pdf` — basic text extraction, pure Go
- `github.com/unidoc/unipdf` — comprehensive but commercial license
- Shell out to `pdftotext` (poppler-utils) as an optional dependency

### 5.4 Image-heavy archives

For Wikipedia maxi (with images) and image-focused ZIMs:

**Analysis:**
1. Census: PNG vs JPEG vs WebP vs SVG distribution
2. Current optimization: JPEG metadata stripping saves ~2-5%
3. Potential: PNG → WebP lossless conversion saves 25-35% (blocked on CGo/libwebp)
4. Potential: SVG minification (already supported via tdewolff)
5. Measure: image bytes as % of total archive — determines ROI of image optimization

**Decision matrix:**

| Current format | Optimization | Savings | Complexity | Recommendation |
|----------------|-------------|---------|------------|----------------|
| JPEG | Metadata strip | 2-5% | Done | Keep |
| PNG | Re-encode BestCompression | 5-15% | Done | Keep |
| PNG → WebP | Lossless transcode | 25-35% | CGo or shell-out | Defer (§3.5) |
| SVG | Minify | 10-30% | Done | Keep |
| WebP | None | 0% | N/A | Already optimal |

### 5.5 Deduplication analysis

```
For each test archive:
  Run conversion with dedup stats enabled
  Report:
    - Total entries vs unique content hashes
    - Bytes saved by dedup
    - Top duplicated MIME types (expect: CSS, JS, small images)
    - Cross-entry duplicate rate by source type
```

---

## 6. Incremental Update Validation

### 6.1 Simulated monthly diff

Using two monthly snapshots of the same Wikipedia ZIM:

```
1. Convert month_1.zim → month_1.oza (full build, record time/size)
2. Convert month_2.zim → month_2.oza (full build, record time/size)
3. Diff: identify changed/added/removed entries between months
4. Incremental: build month_2_incr.oza from month_1.oza + changes
5. Verify: month_2.oza and month_2_incr.oza have identical content
6. Measure: time savings (target: 5-6x for 95% unchanged)
```

### 6.2 Chunk-level copy efficiency

```
For the incremental build:
  Report:
    - Chunks copied byte-for-byte vs chunks rebuilt
    - Bytes copied vs bytes recompressed
    - Dictionary reuse rate
    - Index rebuild time (always full rebuild)
    - Search index rebuild time
```

### 6.3 Edge cases for incremental

| Scenario | Test |
|----------|------|
| Entry deleted | Chunk containing deleted entry is rebuilt without it |
| Entry modified | Chunk rebuilt with new content |
| Entry added to existing chunk group | New chunk created or appended |
| All entries in a chunk modified | Chunk fully rebuilt (no copy benefit) |
| MIME type changed | Entry moves to different chunk group |
| Redirect target changed | Redirect table rebuilt |
| Metadata changed | Metadata section rebuilt |

### 6.4 Diff detection strategies

For the `ozaupdate` tool to know what changed, it needs a diff source. Options:

| Strategy | Source | Accuracy | Complexity |
|----------|--------|----------|------------|
| Path-based diff | Compare ZIM path lists | High | Low |
| Content hash diff | Compare OZA content hashes | Exact | Medium (requires reading both archives) |
| External manifest | Provided by content pipeline | Exact | Low (shifts burden upstream) |
| Timestamp-based | ZIM doesn't store per-entry timestamps | N/A | Not viable |

**Recommendation:** Content hash diff. Both old OZA and new ZIM expose content hashes. Compare entry-by-entry to identify changed/added/removed. This is O(N) but N is just the entry count, not the content size.

---

## 7. Bulk Conversion Patterns

### 7.1 Conversion pipeline for CI/build server

```
                    ┌─────────────┐
                    │  ZIM Source  │
                    │  (Kiwix CDN) │
                    └──────┬──────┘
                           │ download
                    ┌──────▼──────┐
                    │  zim2oza    │
                    │  --minify   │
                    │  --verbose  │
                    │  --json-stats│
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  ozaverify  │
                    │  --all      │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  ozainfo    │
                    │  --json     │
                    │  (census)   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       ┌──────▼──────┐ ┌──▼──┐  ┌──────▼──────┐
       │  Archive    │ │ CDN │  │  Benchmark   │
       │  Registry   │ │     │  │  Database    │
       └─────────────┘ └─────┘  └─────────────┘
```

### 7.2 Content-aware conversion profiles

Rather than one-size-fits-all flags, define profiles per source type:

```toml
# profiles/wikipedia.toml
[compression]
zstd_level = 9
dict_samples = 2000
chunk_size = "4MB"

[transform]
minify = true
optimize_images = true
build_search = true
# build_plain_text = true  # when PLAIN_TEXT is implemented

[filtering]
skip_chrome = true
browse_exclude = ["*_cover.*", "*(disambiguation)*"]
```

```toml
# profiles/gutenberg.toml
[compression]
zstd_level = 6
dict_samples = 500
chunk_size = "1MB"  # smaller — books are large individual entries

[transform]
minify = false  # HTML is minimal already
optimize_images = true
extract_pdf = true  # when PDF extraction is implemented
pdf_retain_original = false

[filtering]
skip_chrome = true
browse_exclude = ["*_cover.*"]
```

```toml
# profiles/stackexchange.toml
[compression]
zstd_level = 9
dict_samples = 3000  # high similarity between Q&A pages
chunk_size = "4MB"

[transform]
minify = true
build_search = true

[filtering]
skip_chrome = true
browse_exclude = ["tags/*", "users/*"]
```

### 7.3 Post-conversion quality gates

Every converted archive must pass before deployment:

```
1. ozaverify --all passes (three-tier integrity)
2. Entry count matches expected (within tolerance for skipped entries)
3. File size ratio <= 1.1 (OZA no more than 10% larger than ZIM)
4. Sample content spot-check (10 random entries, byte-compare or visual diff)
5. Search sanity (5 known queries return expected results)
6. Metadata complete (all required keys present and valid)
```

---

## 8. Test Infrastructure

### 8.1 New test files to create

| File | Purpose |
|------|---------|
| `cmd/zim2oza/fidelity_test.go` | §2 fidelity tests (content match, redirect chains, metadata) |
| `cmd/zim2oza/benchmark_test.go` | §4 conversion time benchmarks with phase breakdown |
| `cmd/zim2oza/census_test.go` | §5.1 MIME census and content pattern analysis |
| `oza/bench_parallel_test.go` | §3.2 parallel access and cache thrashing benchmarks |
| `cmd/ozaserve/handler_test.go` | §3.3 HTTP serving throughput (addresses backlog §1.16) |

### 8.2 Benchmark tracking

Add `benchstat` comparison to CI:

```yaml
# .github/workflows/bench.yml — runs on main branch only
- name: Run benchmarks
  run: go test -bench=. -benchmem -count=5 ./oza/ ./ozawrite/ | tee bench.txt
- name: Compare with baseline
  run: |
    git stash
    go test -bench=. -benchmem -count=5 ./oza/ ./ozawrite/ | tee bench-base.txt
    git stash pop
    benchstat bench-base.txt bench.txt
```

### 8.3 Test archive builder utility

For unit tests that need realistic archives without downloading ZIM files:

```go
// testutil/builder.go
func BuildTestArchive(t *testing.T, opts ...TestArchiveOption) string {
    // Creates a temp OZA file with configurable:
    // - entry count, MIME distribution
    // - content patterns (lorem ipsum HTML, sample images)
    // - redirect chains
    // - search indexing on/off
    // Returns path to temp file (cleaned up by t.Cleanup)
}
```

---

## 9. Incremental Update Design Considerations

Beyond the mechanical implementation in `docs/INCREMENTAL.md`, some strategic considerations for the testing plan:

### 9.1 What makes incremental viable

The key insight: OZA's chunk-based content storage means unchanged content lives in self-contained compressed chunks that can be copied byte-for-byte. The test plan must validate:

1. **Chunk stability** — Same content, same chunk assignment across builds (deterministic chunking)
2. **Dictionary compatibility** — Copied chunks may reference dictionaries from the old archive; those dictionaries must be carried forward
3. **Index rebuild** — Path/title indices and search indices must always be rebuilt from scratch (no incremental index update)

### 9.2 Diff granularity trade-offs

| Granularity | Copy unit | Waste on partial change | Rebuild cost |
|-------------|-----------|------------------------|--------------|
| Entry-level | Individual blobs | None | Must decompress chunk, extract blob, recompress |
| Chunk-level | Entire chunk (~4 MB) | Up to chunk_size - 1 | Zero for unchanged chunks |
| Section-level | Entire content section | Impractical | Basically a full rebuild |

**Chunk-level is the sweet spot.** For Wikipedia monthly diffs (~5% changed entries), most chunks are entirely unchanged. The few chunks with modified entries are rebuilt — decompressing a 4 MB chunk to replace one entry is cheap compared to the time saved by copying thousands of unchanged chunks.

### 9.3 Testing incremental correctness

The gold standard: an archive built incrementally must produce byte-identical content when accessed through the reader API, compared to a full rebuild from the same source data. File-level bytes will differ (different UUIDs, different section offsets), but:

```
For every entry ID in incremental.oza:
  assert incremental.ReadContent(id) == full_rebuild.ReadContent(id)
  assert incremental.EntryByPath(path).MIMEType == full_rebuild.EntryByPath(path).MIMEType
  
For every search query in test set:
  assert set(incremental.Search(q)) == set(full_rebuild.Search(q))
```

---

## 10. Execution Priority

| Phase | Work | Depends on | Value |
|-------|------|-----------|-------|
| **Phase 1** | Tier 1 corpus + fidelity tests | Nothing | Confidence in correctness |
| **Phase 2** | MIME census + `--dry-run --census` | Phase 1 corpus | Informs all optimization decisions |
| **Phase 3** | Size + access benchmarks | Phase 1 corpus | Performance baseline |
| **Phase 4** | Conversion time benchmarks | Tier 2 corpus | Identifies bottlenecks |
| **Phase 5** | Content pattern analysis (HTML→MD, PDF) | Phase 2 census | Informs PLAIN_TEXT and PDF extraction design |
| **Phase 6** | Incremental update tests | INCREMENTAL.md implementation | Validates chunk-copy approach |
| **Phase 7** | HTTP serving benchmarks | Phase 3 | Production readiness |
| **Phase 8** | Compression parameter sweeps | Phase 4 | Optimal defaults |

Phase 1-3 can begin immediately. Phase 5 informs but does not block PLAIN_TEXT implementation — the design in `docs/PLAIN_TEXT.md` is already sound; the census validates assumptions about content distribution and expected savings.

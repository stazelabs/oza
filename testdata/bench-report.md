# OZA Benchmark Report

| | |
|---|---|
| **Generated** | 2026-04-02 01:30:53 UTC |
| **System** | Darwin 25.3.0 arm64 |
| **Go** | go1.26.1 |
| **CPU** | Apple M4 Pro |
| **Files tested** | 15 |
| **Flags** | bench=0 deep=0 min-size=100000 |

---

## Executive Summary

| File | ZIM Size | OZA Size | Ratio | Δ% | Profile | Conf | Entries | Time (s) | Verify |
|------|----------|----------|------:|---:|---------|-----:|--------:|---------:|:------:|
| ar_chemistry | 10.7 MB | 8.0 MB | 0.753 | -24.7 | encyclopedia | 83% | 10594 | 2.4 | **FAIL** |
| devdocs_go | 1.5 MB | 1.4 MB | 0.939 | -6.1 | docs | 82% | 191 | 0.3 | PASS |
| gobyexample | 321.5 KB | 308.6 KB | 0.960 | -4.0 | docs | 83% | 95 | 0.1 | PASS |
| gutenberg_ar | 2.1 MB | 1.2 MB | 0.600 | -40.0 | mixed-scrape | 50% | 93 | 0.3 | PASS |
| gutenberg_ko | 2.2 MB | 1.3 MB | 0.603 | -39.7 | mixed-scrape | 50% | 93 | 0.3 | PASS |
| ray_charles | 2.7 MB | 2.5 MB | 0.915 | -8.5 | encyclopedia | 80% | 518 | 0.4 | PASS |
| ray_charles_nopic | 1.6 MB | 1.3 MB | 0.865 | -13.5 | encyclopedia | 83% | 363 | 0.4 | PASS |
| se_community | 6.0 MB | 5.5 MB | 0.912 | -8.8 | qa-forum | 90% | 3695 | 2.5 | PASS |
| ted_street_art | 36.6 MB | 33.3 MB | 0.911 | -8.9 | media-heavy | 89% | 590 | 1.4 | PASS |
| top100_mini | 4.3 MB | 3.2 MB | 0.736 | -26.4 | encyclopedia | 69% | 5094 | 0.9 | PASS |
| vikidia_ru | 10.4 MB | 9.7 MB | 0.935 | -6.5 | media-heavy | 90% | 848 | 3.4 | PASS |
| wikiquote_ja | 5.3 MB | 2.7 MB | 0.519 | -48.1 | encyclopedia | 84% | 1634 | 1.5 | PASS |
| wiktionary_he | 39.9 MB | 30.1 MB | 0.755 | -24.5 | encyclopedia | 85% | 31128 | 6.8 | PASS |
| wiktionary_yi | 1.4 MB | 567.7 KB | 0.392 | -60.8 | encyclopedia | 84% | 1196 | 0.7 | PASS |
| zh_chemistry | 13.4 MB | 10.6 MB | 0.791 | -20.9 | encyclopedia | 82% | 12549 | 2.6 | **FAIL** |

---

## Aggregate Statistics

| Metric | Value |
|--------|-------|
| Total ZIM size | 138.4 MB |
| Total OZA size | 111.9 MB |
| Overall size ratio | 0.809 |
| Savings | 19.1% |
| Files converted | 15 |
| Conversion failures | 0 |
| Verification | 13/15 pass |
| Mean conversion time | 1.6 s |
| Min conversion time | 0.1 s |
| Max conversion time | 6.8 s |

### Profile Distribution

| Profile | Count |
|---------|------:|
| encyclopedia | 8 |
| mixed-scrape | 2 |
| media-heavy | 2 |
| docs | 2 |
| qa-forum | 1 |

---

## Per-File Details

### ar_chemistry

#### File Info

| Key | Value |
|-----|-------|
| title | <binary 35 bytes> |
| language | ara |
| creator | Wikipedia |
| date | 2026-01-15 |
| source | ar.wikipedia.org |
| description | <binary 84 bytes> |
| UUID | `c1811eec-2b10-c5aa-f7f8-8ded05412c6c` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 10.7 MB |
| OZA size | 8.0 MB |
| Size ratio | 0.7535 |
| Size delta | -24.7% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 6501999 | 77.1% | content |
| SEARCH_BODY | 1559004 | 18.5% | overhead |
| PATH_INDEX | 102847 | 1.2% | overhead |
| TITLE_INDEX | 99206 | 1.2% | overhead |
| SEARCH_TITLE | 84205 | 1% | overhead |
| ENTRY_TABLE | 63165 | 0.7% | overhead |
| REDIRECT_TABLE | 12183 | 0.1% | overhead |
| METADATA | 5940 | 0.1% | overhead |
| MIME_TABLE | 179 | 0% | overhead |

Content: 77.1% — Overhead: 22.9%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 83% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 93.6% |
| html_bytes_ratio | 92.9% |
| image_bytes_ratio | 6.3% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 66.9% |
| avg_entry_bytes | 13.3 KB |
| small_entry_ratio | 9.5% |
| entry_count | 3505 |
| mime_type_count | 7 |
| compression_ratio | 78.1% |
| source_hint | wikipedia |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 3505 |
| redirects | 0 |
| front_articles | 3066 |
| metadata_refs | 0 |
| total_blob_bytes | 47765440 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 3066 | 44370477 | 14471 | 148 | 307801 |
| image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0" | 327 | 1253308 | 3832 | 436 | 24147 |
| image/webp | 100 | 1752052 | 17520 | 2034 | 94340 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 16 |
| avg_entries_per_chunk | 219.1 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 428 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 3066 |
| body_doc_count | 3066 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 11 |
| read | 39 |
| transform | 280 |
| dedup | 14 |
| search_index | 24 |
| chunk_build | 1647 |
| dict_train | 0 |
| compress | 0 |
| assemble | 152 |
| close | 158 |
| total | 2395 |

| Metric | Value |
|--------|------:|
| bytes_read | 52129409 |
| cache_hits | 3489 |
| cache_misses | 30 |
| entry_content | 3505 |
| entry_redirect | 7089 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;font/woff2=4;image/gif=1;image/png=143;image/svg+xml=15;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=327;image/webp=99;text/css=25;text/html=3066;text/html; charset=iso-8859-1=1;text/javascript=3 | application/javascript=4;font/woff2=4;image/gif=1;image/png=143;image/svg+xml=15;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=327;image/webp=99;text/css=25;text/html=3066;text/html; charset=iso-8859-1=1;text/javascript=3 | ✓ |
| creator | Wikipedia | Wikipedia | ✓ |
| date | 2026-01-15 | 2026-01-15 | ✓ |
| description | — | <binary 84 bytes> | **✗** |
| flavour | mini | mini | ✓ |
| language | ara | ara | ✓ |
| main_entry | — | index | **✗** |
| name | wikipedia_ar_chemistry | wikipedia_ar_chemistry | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.4 | mwoffliner 1.17.4 | ✓ |
| source | ar.wikipedia.org | ar.wikipedia.org | ✓ |
| tags | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:no;_ftindex:yes | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:no;_ftindex:yes | ✓ |
| title | — | <binary 35 bytes> | **✗** |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 99 | 100 | 1 |
| image/png | 1 | 0 | -1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 5940 | 5940 | none |
| 1 | MIME_TABLE | 179 | 179 | none |
| 2 | ENTRY_TABLE | 63165 | 72427 | zstd |
| 3 | PATH_INDEX | 102847 | 272447 | zstd |
| 4 | TITLE_INDEX | 99206 | 251634 | zstd |
| 5 | CONTENT | 6501999 | 6501999 | none |
| 6 | REDIRECT_TABLE | 12183 | 35449 | zstd |
| 7 | SEARCH_TITLE | 84205 | 168427 | zstd |
| 8 | SEARCH_BODY | 1559004 | 3488950 | zstd |

Overall compression ratio: 0.781

#### Verification

**FAILED** (exit code 1)

#### Notes

- :warning: 3 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### devdocs_go

#### File Info

| Key | Value |
|-----|-------|
| title | Go Docs |
| language | eng |
| creator | DevDocs |
| date | 2026-01-04 |
| source | /Users/cstaszak/projects/oza/testdata/devdocs_go.zim |
| description | Go documentation, by DevDocs |
| UUID | `85720e7e-5c62-7a58-92f8-82c18490f540` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 1.5 MB |
| OZA size | 1.4 MB |
| Size ratio | 0.9392 |
| Size delta | -6.1% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 1030915 | 68.3% | content |
| SEARCH_BODY | 461120 | 30.6% | overhead |
| SEARCH_TITLE | 5700 | 0.4% | overhead |
| ENTRY_TABLE | 3452 | 0.2% | overhead |
| METADATA | 2730 | 0.2% | overhead |
| PATH_INDEX | 1874 | 0.1% | overhead |
| TITLE_INDEX | 1553 | 0.1% | overhead |
| MIME_TABLE | 103 | 0% | overhead |
| REDIRECT_TABLE | 9 | 0% | overhead |

Content: 68.4% — Overhead: 31.6%

#### Classification

| | |
|---|---|
| **Profile** | docs |
| **Confidence** | 82% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 87.4% |
| html_bytes_ratio | 84.7% |
| image_bytes_ratio | 0.0% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 0.5% |
| avg_entry_bytes | 26.1 KB |
| small_entry_ratio | 0.0% |
| entry_count | 190 |
| mime_type_count | 5 |
| compression_ratio | 55.8% |
| source_hint | devdocs |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 2097152 |
| zstd_level | 6 |
| dict_samples | 3000 |
| minify | true |
| search_prune_freq | 0.5 |
| notes | smaller chunks for small-medium entries; more dict samples for similar structure |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 190 |
| redirects | 0 |
| front_articles | 185 |
| metadata_refs | 0 |
| total_blob_bytes | 5075441 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 185 | 4297579 | 23230 | 772 | 178005 |
| application/octet-stream | 2 | 219247 | 109623 | 152 | 219095 |
| application/json | 1 | 422556 | 422556 | 422556 | 422556 |
| text/css | 1 | 117339 | 117339 | 117339 | 117339 |
| text/plain | 1 | 18720 | 18720 | 18720 | 18720 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 7 |
| avg_entries_per_chunk | 27.1 |
| min_entries_per_chunk | 1 |
| max_entries_per_chunk | 89 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 185 |
| body_doc_count | 185 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 4 |
| read | 2 |
| transform | 16 |
| dedup | 1 |
| search_index | 14 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 53 |
| close | 265 |
| total | 343 |

| Metric | Value |
|--------|------:|
| bytes_read | 5355958 |
| cache_hits | 197 |
| cache_misses | 4 |
| entry_content | 190 |
| entry_redirect | 1 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/json=1;text/css=1;text/html=185;text/plain=1 | application/json=1;text/css=1;text/html=185;text/plain=1 | ✓ |
| creator | DevDocs | DevDocs | ✓ |
| date | 2026-01-04 | 2026-01-04 | ✓ |
| description | Go documentation, by DevDocs | Go documentation, by DevDocs | ✓ |
| language | eng | eng | ✓ |
| main_entry | — | index | **✗** |
| name | devdocs_en_go | devdocs_en_go | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | devdocs2zim v0.2.0 | devdocs2zim v0.2.0 | ✓ |
| source | — | /Users/cstaszak/projects/oza/testdata/devdocs_go.zim | **✗** |
| tags | devdocs;go | devdocs;go | ✓ |
| title | Go Docs | Go Docs | ✓ |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=2097152 dict=3000 search=all search-prune=0.50 minify transcode |
| Chunk Target Size | 2097152 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 2730 | 2730 | none |
| 1 | MIME_TABLE | 103 | 103 | none |
| 2 | ENTRY_TABLE | 3452 | 3859 | zstd |
| 3 | PATH_INDEX | 1874 | 4211 | zstd |
| 4 | TITLE_INDEX | 1553 | 3185 | zstd |
| 5 | CONTENT | 1030915 | 1030915 | none |
| 6 | REDIRECT_TABLE | 9 | 9 | none |
| 7 | SEARCH_TITLE | 5700 | 21517 | zstd |
| 8 | SEARCH_BODY | 461120 | 1635654 | zstd |

Overall compression ratio: 0.558

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA

---

### gobyexample

#### File Info

| Key | Value |
|-----|-------|
| title | Go by example |
| language | eng |
| creator | - |
| date | 2026-02-02 |
| source | /Users/cstaszak/projects/oza/testdata/gobyexample.zim |
| description | a hands-on introduction to Go using annotated example programs |
| UUID | `ee3d450d-847c-2dda-786c-e91374e0dc24` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 321.5 KB |
| OZA size | 308.6 KB |
| Size ratio | 0.9597 |
| Size delta | -4.0% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 203757 | 64.5% | content |
| SEARCH_BODY | 100171 | 31.7% | overhead |
| SEARCH_TITLE | 4597 | 1.5% | overhead |
| METADATA | 2405 | 0.8% | overhead |
| ENTRY_TABLE | 1703 | 0.5% | overhead |
| TITLE_INDEX | 1247 | 0.4% | overhead |
| PATH_INDEX | 1103 | 0.3% | overhead |
| MIME_TABLE | 90 | 0% | overhead |
| REDIRECT_TABLE | 9 | 0% | overhead |

Content: 64.7% — Overhead: 35.3%

#### Classification

| | |
|---|---|
| **Profile** | docs |
| **Confidence** | 83% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 93.4% |
| html_bytes_ratio | 82.1% |
| image_bytes_ratio | 6.6% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 1.1% |
| avg_entry_bytes | 11.4 KB |
| small_entry_ratio | 3.2% |
| entry_count | 94 |
| mime_type_count | 5 |
| compression_ratio | 55.1% |
| source_hint | zimit |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 2097152 |
| zstd_level | 6 |
| dict_samples | 3000 |
| minify | true |
| search_prune_freq | 0.5 |
| notes | smaller chunks for small-medium entries; more dict samples for similar structure |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 94 |
| redirects | 0 |
| front_articles | 87 |
| metadata_refs | 0 |
| total_blob_bytes | 1101408 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 87 | 903933 | 10390 | 4449 | 23984 |
| text/javascript | 3 | 122036 | 40678 | 532 | 101360 |
| image/webp | 2 | 826 | 413 | 294 | 532 |
| image/x-icon | 1 | 71518 | 71518 | 71518 | 71518 |
| text/css | 1 | 3095 | 3095 | 3095 | 3095 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 5 |
| avg_entries_per_chunk | 18.8 |
| min_entries_per_chunk | 1 |
| max_entries_per_chunk | 87 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 87 |
| body_doc_count | 87 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 1 |
| read | 0 |
| transform | 17 |
| dedup | 0 |
| search_index | 3 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 14 |
| close | 87 |
| total | 114 |

| Metric | Value |
|--------|------:|
| bytes_read | 1375764 |
| cache_hits | 105 |
| cache_misses | 2 |
| entry_content | 94 |
| entry_redirect | 1 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | image/png=2;image/x-icon=1;text/css=1;text/html=87;text/javascript=3 | image/png=2;image/x-icon=1;text/css=1;text/html=87;text/javascript=3 | ✓ |
| creator | - | - | ✓ |
| date | 2026-02-02 | 2026-02-02 | ✓ |
| description | a hands-on introduction to Go using annotated example programs | a hands-on introduction to Go using annotated example programs | ✓ |
| language | eng | eng | ✓ |
| main_entry | — | gobyexample.com/ | **✗** |
| name | gobyexample.com_en_all | gobyexample.com_en_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | warc2zim 2.3.0,Browsertrix-Crawler 1.11.1 (with warcio.js 2.4.7),zimit 3.1.1 | warc2zim 2.3.0,Browsertrix-Crawler 1.11.1 (with warcio.js 2.4.7),zimit 3.1.1 | ✓ |
| source | — | /Users/cstaszak/projects/oza/testdata/gobyexample.zim | **✗** |
| tags | _ftindex:yes;_category:other | _ftindex:yes;_category:other | ✓ |
| title | Go by example | Go by example | ✓ |
| x-contentdate | 2026-02-02 | 2026-02-02 | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 0 | 2 | 2 |
| image/png | 2 | 0 | -2 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 2405 | 2405 | none |
| 1 | MIME_TABLE | 90 | 90 | none |
| 2 | ENTRY_TABLE | 1703 | 1888 | zstd |
| 3 | PATH_INDEX | 1103 | 2076 | zstd |
| 4 | TITLE_INDEX | 1247 | 2275 | zstd |
| 5 | CONTENT | 203757 | 203757 | none |
| 6 | REDIRECT_TABLE | 9 | 9 | none |
| 7 | SEARCH_TITLE | 4597 | 17807 | zstd |
| 8 | SEARCH_BODY | 100171 | 341012 | zstd |

Overall compression ratio: 0.551

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### gutenberg_ar

#### File Info

| Key | Value |
|-----|-------|
| title | <binary 36 bytes> |
| language | ara |
| creator | gutenberg.org |
| date | 2025-12-08 |
| source | /Users/cstaszak/projects/oza/testdata/gutenberg_ar.zim |
| description | <binary 119 bytes> |
| UUID | `b6bdd5a4-e65c-ab49-3a58-c995ddf1d816` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 2.1 MB |
| OZA size | 1.2 MB |
| Size ratio | 0.5998 |
| Size delta | -40.0% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 1293301 | 98.7% | content |
| SEARCH_BODY | 7220 | 0.6% | overhead |
| METADATA | 3742 | 0.3% | overhead |
| ENTRY_TABLE | 1441 | 0.1% | overhead |
| PATH_INDEX | 1227 | 0.1% | overhead |
| TITLE_INDEX | 1173 | 0.1% | overhead |
| SEARCH_TITLE | 449 | 0% | overhead |
| MIME_TABLE | 245 | 0% | overhead |
| REDIRECT_TABLE | 9 | 0% | overhead |

Content: 98.8% — Overhead: 1.2%

#### Classification

| | |
|---|---|
| **Profile** | mixed-scrape |
| **Confidence** | 50% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 26.1% |
| html_bytes_ratio | 19.3% |
| image_bytes_ratio | 8.7% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 1.1% |
| avg_entry_bytes | 37.2 KB |
| small_entry_ratio | 53.3% |
| entry_count | 92 |
| mime_type_count | 14 |
| compression_ratio | 96.6% |
| source_hint | gutenberg |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | conservative defaults for unknown content |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 92 |
| redirects | 0 |
| front_articles | 8 |
| metadata_refs | 0 |
| total_blob_bytes | 3507538 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/javascript | 29 | 27028 | 932 | 16 | 15613 |
| image/webp | 19 | 52918 | 2785 | 44 | 23580 |
| text/css | 12 | 213048 | 17754 | 7608 | 27682 |
| font/sfnt | 10 | 1284032 | 128403 | 125332 | 141564 |
| text/html | 8 | 675658 | 84457 | 24650 | 93841 |
| application/javascript | 6 | 686913 | 114485 | 960 | 246870 |
| image/vnd.microsoft.icon | 1 | 15086 | 15086 | 15086 | 15086 |
| application/epub+zip | 1 | 83878 | 83878 | 83878 | 83878 |
| image/jpeg | 1 | 13054 | 13054 | 13054 | 13054 |
| application/vnd.ms-opentype | 1 | 75188 | 75188 | 75188 | 75188 |
| font/woff | 1 | 83760 | 83760 | 83760 | 83760 |
| text/plain | 1 | 2 | 2 | 2 | 2 |
| application/vnd.ms-fontobject | 1 | 72449 | 72449 | 72449 | 72449 |
| image/svg+xml | 1 | 224524 | 224524 | 224524 | 224524 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 7 |
| avg_entries_per_chunk | 13.1 |
| min_entries_per_chunk | 1 |
| max_entries_per_chunk | 29 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 8 |
| body_doc_count | 8 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 2 |
| read | 0 |
| transform | 164 |
| dedup | 1 |
| search_index | 0 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 4 |
| close | 81 |
| total | 252 |

| Metric | Value |
|--------|------:|
| bytes_read | 3892580 |
| cache_hits | 101 |
| cache_misses | 3 |
| entry_content | 92 |
| entry_redirect | 1 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/epub+zip=1;application/javascript=6;application/vnd.ms-fontobject=1;application/vnd.ms-opentype=1;font/sfnt=10;font/woff=1;image/gif=2;image/jpeg=1;image/png=17;image/svg+xml=1;image/vnd.microsoft.icon=1;text/css=12;text/html=8;text/javascript=29;text/plain=1 | application/epub+zip=1;application/javascript=6;application/vnd.ms-fontobject=1;application/vnd.ms-opentype=1;font/sfnt=10;font/woff=1;image/gif=2;image/jpeg=1;image/png=17;image/svg+xml=1;image/vnd.microsoft.icon=1;text/css=12;text/html=8;text/javascript=29;text/plain=1 | ✓ |
| creator | gutenberg.org | gutenberg.org | ✓ |
| date | 2025-12-08 | 2025-12-08 | ✓ |
| description | — | <binary 119 bytes> | **✗** |
| language | ara | ara | ✓ |
| main_entry | — | Home | **✗** |
| name | gutenberg_ar_all | gutenberg_ar_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | gutenberg2zim-3.0.1 | gutenberg2zim-3.0.1 | ✓ |
| source | — | /Users/cstaszak/projects/oza/testdata/gutenberg_ar.zim | **✗** |
| tags | _category:gutenberg;gutenberg | _category:gutenberg;gutenberg | ✓ |
| title | — | <binary 36 bytes> | **✗** |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 0 | 19 | 19 |
| image/png | 17 | 0 | -17 |
| image/gif | 2 | 0 | -2 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=8388608 dict=1000 search=all search-prune=0.70 minify transcode |
| Chunk Target Size | 8388608 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 3742 | 3742 | none |
| 1 | MIME_TABLE | 245 | 245 | none |
| 2 | ENTRY_TABLE | 1441 | 1796 | zstd |
| 3 | PATH_INDEX | 1227 | 2490 | zstd |
| 4 | TITLE_INDEX | 1173 | 2406 | zstd |
| 5 | CONTENT | 1293301 | 1293301 | none |
| 6 | REDIRECT_TABLE | 9 | 9 | none |
| 7 | SEARCH_TITLE | 449 | 1760 | zstd |
| 8 | SEARCH_BODY | 7220 | 49213 | zstd |

Overall compression ratio: 0.966

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 4 metadata key(s) differ between ZIM and OZA
- :warning: 3 MIME type count(s) differ between ZIM and OZA

---

### gutenberg_ko

#### File Info

| Key | Value |
|-----|-------|
| title | metadata_defaults.title |
| language | kor |
| creator | gutenberg.org |
| date | 2026-01-05 |
| source | /Users/cstaszak/projects/oza/testdata/gutenberg_ko.zim |
| description | All books in "kor" language from the first producer of free Ebooks |
| UUID | `a83df828-234b-5f29-2287-904b43f24324` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 2.2 MB |
| OZA size | 1.3 MB |
| Size ratio | 0.6033 |
| Size delta | -39.7% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 1340693 | 97.7% | content |
| SEARCH_BODY | 22451 | 1.6% | overhead |
| METADATA | 3676 | 0.3% | overhead |
| ENTRY_TABLE | 1446 | 0.1% | overhead |
| PATH_INDEX | 1238 | 0.1% | overhead |
| TITLE_INDEX | 1171 | 0.1% | overhead |
| SEARCH_TITLE | 482 | 0% | overhead |
| MIME_TABLE | 245 | 0% | overhead |
| REDIRECT_TABLE | 9 | 0% | overhead |

Content: 97.8% — Overhead: 2.2%

#### Classification

| | |
|---|---|
| **Profile** | mixed-scrape |
| **Confidence** | 50% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 26.7% |
| html_bytes_ratio | 20.0% |
| image_bytes_ratio | 8.7% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 1.1% |
| avg_entry_bytes | 38.0 KB |
| small_entry_ratio | 53.3% |
| entry_count | 92 |
| mime_type_count | 14 |
| compression_ratio | 89.3% |
| source_hint | gutenberg |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | conservative defaults for unknown content |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 92 |
| redirects | 0 |
| front_articles | 8 |
| metadata_refs | 0 |
| total_blob_bytes | 3577364 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/javascript | 29 | 27150 | 936 | 16 | 15613 |
| image/webp | 19 | 56712 | 2984 | 44 | 27374 |
| text/css | 12 | 213048 | 17754 | 7608 | 27682 |
| font/sfnt | 10 | 1284032 | 128403 | 125332 | 141564 |
| text/html | 8 | 715686 | 89460 | 59265 | 94714 |
| application/javascript | 6 | 686913 | 114485 | 960 | 246870 |
| image/jpeg | 1 | 15065 | 15065 | 15065 | 15065 |
| image/vnd.microsoft.icon | 1 | 15086 | 15086 | 15086 | 15086 |
| application/epub+zip | 1 | 107749 | 107749 | 107749 | 107749 |
| application/vnd.ms-fontobject | 1 | 72449 | 72449 | 72449 | 72449 |
| font/woff | 1 | 83760 | 83760 | 83760 | 83760 |
| text/plain | 1 | 2 | 2 | 2 | 2 |
| image/svg+xml | 1 | 224524 | 224524 | 224524 | 224524 |
| application/vnd.ms-opentype | 1 | 75188 | 75188 | 75188 | 75188 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 7 |
| avg_entries_per_chunk | 13.1 |
| min_entries_per_chunk | 1 |
| max_entries_per_chunk | 29 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 8 |
| body_doc_count | 8 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 3 |
| read | 0 |
| transform | 166 |
| dedup | 1 |
| search_index | 1 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 7 |
| close | 84 |
| total | 259 |

| Metric | Value |
|--------|------:|
| bytes_read | 3990383 |
| cache_hits | 101 |
| cache_misses | 3 |
| entry_content | 92 |
| entry_redirect | 1 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/epub+zip=1;application/javascript=6;application/vnd.ms-fontobject=1;application/vnd.ms-opentype=1;font/sfnt=10;font/woff=1;image/gif=2;image/jpeg=1;image/png=17;image/svg+xml=1;image/vnd.microsoft.icon=1;text/css=12;text/html=8;text/javascript=29;text/plain=1 | application/epub+zip=1;application/javascript=6;application/vnd.ms-fontobject=1;application/vnd.ms-opentype=1;font/sfnt=10;font/woff=1;image/gif=2;image/jpeg=1;image/png=17;image/svg+xml=1;image/vnd.microsoft.icon=1;text/css=12;text/html=8;text/javascript=29;text/plain=1 | ✓ |
| creator | gutenberg.org | gutenberg.org | ✓ |
| date | 2026-01-05 | 2026-01-05 | ✓ |
| description | All books in "kor" language from the first producer of free Ebooks | All books in "kor" language from the first producer of free Ebooks | ✓ |
| language | kor | kor | ✓ |
| main_entry | — | Home | **✗** |
| name | gutenberg_ko_all | gutenberg_ko_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | gutenberg2zim-3.0.1 | gutenberg2zim-3.0.1 | ✓ |
| source | — | /Users/cstaszak/projects/oza/testdata/gutenberg_ko.zim | **✗** |
| tags | _category:gutenberg;gutenberg | _category:gutenberg;gutenberg | ✓ |
| title | metadata_defaults.title | metadata_defaults.title | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 0 | 19 | 19 |
| image/png | 17 | 0 | -17 |
| image/gif | 2 | 0 | -2 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=8388608 dict=1000 search=all search-prune=0.70 minify transcode |
| Chunk Target Size | 8388608 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 3676 | 3676 | none |
| 1 | MIME_TABLE | 245 | 245 | none |
| 2 | ENTRY_TABLE | 1446 | 1799 | zstd |
| 3 | PATH_INDEX | 1238 | 2487 | zstd |
| 4 | TITLE_INDEX | 1171 | 2393 | zstd |
| 5 | CONTENT | 1340693 | 1340693 | none |
| 6 | REDIRECT_TABLE | 9 | 9 | none |
| 7 | SEARCH_TITLE | 482 | 1918 | zstd |
| 8 | SEARCH_BODY | 22451 | 182369 | zstd |

Overall compression ratio: 0.893

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA
- :warning: 3 MIME type count(s) differ between ZIM and OZA

---

### ray_charles

#### File Info

| Key | Value |
|-----|-------|
| title | Ray Charles |
| language | eng |
| creator | Wikipedia |
| date | 2026-02-01 |
| source | en.wikipedia.org |
| description | Wikipedia articles about Ray Charles |
| UUID | `c86a04dd-9007-fa4a-c506-58f763f77965` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 2.7 MB |
| OZA size | 2.5 MB |
| Size ratio | 0.9150 |
| Size delta | -8.5% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 2262205 | 87.8% | content |
| SEARCH_BODY | 275840 | 10.7% | overhead |
| SEARCH_TITLE | 10796 | 0.4% | overhead |
| PATH_INDEX | 7351 | 0.3% | overhead |
| METADATA | 6876 | 0.3% | overhead |
| ENTRY_TABLE | 5943 | 0.2% | overhead |
| TITLE_INDEX | 5833 | 0.2% | overhead |
| REDIRECT_TABLE | 286 | 0% | overhead |
| MIME_TABLE | 91 | 0% | overhead |

Content: 87.8% — Overhead: 12.2%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 80% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 84.7% |
| html_bytes_ratio | 80.8% |
| image_bytes_ratio | 15.0% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 36.7% |
| avg_entry_bytes | 27.9 KB |
| small_entry_ratio | 1.5% |
| entry_count | 328 |
| mime_type_count | 6 |
| compression_ratio | 79.5% |
| source_hint | wikipedia |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 328 |
| redirects | 0 |
| front_articles | 151 |
| metadata_refs | 0 |
| total_blob_bytes | 9385414 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| image/webp | 165 | 1408860 | 8538 | 240 | 50866 |
| text/html | 151 | 7586951 | 50244 | 168 | 362780 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 7 |
| avg_entries_per_chunk | 46.9 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 166 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 151 |
| body_doc_count | 151 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 3 |
| read | 5 |
| transform | 61 |
| dedup | 2 |
| search_index | 8 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 35 |
| close | 242 |
| total | 359 |

| Metric | Value |
|--------|------:|
| bytes_read | 10062981 |
| cache_hits | 336 |
| cache_misses | 6 |
| entry_content | 328 |
| entry_redirect | 190 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/png=3;image/svg+xml=11;image/webp=164;text/css=12;text/html=151;text/javascript=3 | application/javascript=4;image/png=3;image/svg+xml=11;image/webp=164;text/css=12;text/html=151;text/javascript=3 | ✓ |
| creator | Wikipedia | Wikipedia | ✓ |
| date | 2026-02-01 | 2026-02-01 | ✓ |
| description | Wikipedia articles about Ray Charles | Wikipedia articles about Ray Charles | ✓ |
| flavour | maxi | maxi | ✓ |
| language | eng | eng | ✓ |
| main_entry | — | index | **✗** |
| name | wikipedia_en_ray-charles | wikipedia_en_ray-charles | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.4 | mwoffliner 1.17.4 | ✓ |
| source | en.wikipedia.org | en.wikipedia.org | ✓ |
| tags | wikipedia;_category:wikipedia;_pictures:yes;_videos:no;_details:yes;_ftindex:yes | wikipedia;_category:wikipedia;_pictures:yes;_videos:no;_details:yes;_ftindex:yes | ✓ |
| title | Ray Charles | Ray Charles | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 164 | 165 | 1 |
| image/png | 1 | 0 | -1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 6876 | 6876 | none |
| 1 | MIME_TABLE | 91 | 91 | none |
| 2 | ENTRY_TABLE | 5943 | 6750 | zstd |
| 3 | PATH_INDEX | 7351 | 16362 | zstd |
| 4 | TITLE_INDEX | 5833 | 12551 | zstd |
| 5 | CONTENT | 2262205 | 2262205 | none |
| 6 | REDIRECT_TABLE | 286 | 954 | zstd |
| 7 | SEARCH_TITLE | 10796 | 40494 | zstd |
| 8 | SEARCH_BODY | 275840 | 891260 | zstd |

Overall compression ratio: 0.795

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 1 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### ray_charles_nopic

#### File Info

| Key | Value |
|-----|-------|
| title | Ray Charles |
| language | eng |
| creator | Wikipedia |
| date | 2026-02-01 |
| source | en.wikipedia.org |
| description | Wikipedia articles about Ray Charles |
| UUID | `79820bba-d83f-9534-bd9e-bf4df7976bd6` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 1.6 MB |
| OZA size | 1.3 MB |
| Size ratio | 0.8647 |
| Size delta | -13.5% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 1098092 | 78.1% | content |
| SEARCH_BODY | 275753 | 19.6% | overhead |
| SEARCH_TITLE | 10796 | 0.8% | overhead |
| METADATA | 6874 | 0.5% | overhead |
| TITLE_INDEX | 5459 | 0.4% | overhead |
| PATH_INDEX | 4966 | 0.4% | overhead |
| ENTRY_TABLE | 3234 | 0.2% | overhead |
| REDIRECT_TABLE | 286 | 0% | overhead |
| MIME_TABLE | 91 | 0% | overhead |

Content: 78.1% — Overhead: 21.9%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 83% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 96.5% |
| html_bytes_ratio | 92.0% |
| image_bytes_ratio | 3.2% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 52.3% |
| avg_entry_bytes | 45.3 KB |
| small_entry_ratio | 2.9% |
| entry_count | 173 |
| mime_type_count | 6 |
| compression_ratio | 68.1% |
| source_hint | wikipedia |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 173 |
| redirects | 0 |
| front_articles | 151 |
| metadata_refs | 0 |
| total_blob_bytes | 8027951 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 151 | 7385620 | 48911 | 168 | 359454 |
| image/webp | 10 | 252728 | 25272 | 2378 | 50866 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 7 |
| avg_entries_per_chunk | 24.7 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 76 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 151 |
| body_doc_count | 151 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 2 |
| read | 4 |
| transform | 61 |
| dedup | 2 |
| search_index | 8 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 34 |
| close | 239 |
| total | 354 |

| Metric | Value |
|--------|------:|
| bytes_read | 8687815 |
| cache_hits | 181 |
| cache_misses | 6 |
| entry_content | 173 |
| entry_redirect | 190 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/png=3;image/svg+xml=11;image/webp=9;text/css=12;text/html=151;text/javascript=3 | application/javascript=4;image/png=3;image/svg+xml=11;image/webp=9;text/css=12;text/html=151;text/javascript=3 | ✓ |
| creator | Wikipedia | Wikipedia | ✓ |
| date | 2026-02-01 | 2026-02-01 | ✓ |
| description | Wikipedia articles about Ray Charles | Wikipedia articles about Ray Charles | ✓ |
| flavour | nopic | nopic | ✓ |
| language | eng | eng | ✓ |
| main_entry | — | index | **✗** |
| name | wikipedia_en_ray-charles | wikipedia_en_ray-charles | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.4 | mwoffliner 1.17.4 | ✓ |
| source | en.wikipedia.org | en.wikipedia.org | ✓ |
| tags | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:yes;_ftindex:yes | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:yes;_ftindex:yes | ✓ |
| title | Ray Charles | Ray Charles | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 9 | 10 | 1 |
| image/png | 1 | 0 | -1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 6874 | 6874 | none |
| 1 | MIME_TABLE | 91 | 91 | none |
| 2 | ENTRY_TABLE | 3234 | 3641 | zstd |
| 3 | PATH_INDEX | 4966 | 10567 | zstd |
| 4 | TITLE_INDEX | 5459 | 11448 | zstd |
| 5 | CONTENT | 1098092 | 1098092 | none |
| 6 | REDIRECT_TABLE | 286 | 954 | zstd |
| 7 | SEARCH_TITLE | 10796 | 40494 | zstd |
| 8 | SEARCH_BODY | 275753 | 891042 | zstd |

Overall compression ratio: 0.681

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 1 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### se_community

#### File Info

| Key | Value |
|-----|-------|
| title | Community Building Q&A |
| language | eng |
| creator | Stack Exchange |
| date | 2026-02-05 |
| source | /Users/cstaszak/projects/oza/testdata/se_community.zim |
| description | Stack Exchange Q&A for community managers, administrators, and moderators |
| UUID | `abd8f2be-6640-388f-8b1b-a983045dab5e` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 6.0 MB |
| OZA size | 5.5 MB |
| Size ratio | 0.9124 |
| Size delta | -8.8% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 4549449 | 78.6% | content |
| SEARCH_BODY | 1029919 | 17.8% | overhead |
| SEARCH_TITLE | 93455 | 1.6% | overhead |
| TITLE_INDEX | 40868 | 0.7% | overhead |
| PATH_INDEX | 37149 | 0.6% | overhead |
| ENTRY_TABLE | 30559 | 0.5% | overhead |
| REDIRECT_TABLE | 2846 | 0% | overhead |
| METADATA | 2388 | 0% | overhead |
| MIME_TABLE | 152 | 0% | overhead |

Content: 78.6% — Overhead: 21.4%

#### Classification

| | |
|---|---|
| **Profile** | qa-forum |
| **Confidence** | 90% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 86.0% |
| html_bytes_ratio | 80.4% |
| image_bytes_ratio | 3.9% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 54.2% |
| avg_entry_bytes | 15.6 KB |
| small_entry_ratio | 0.1% |
| entry_count | 1693 |
| mime_type_count | 10 |
| compression_ratio | 75.8% |
| source_hint | stackexchange |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 3000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.4 |
| notes | more dict samples for repetitive Q&A templates |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 1693 |
| redirects | 0 |
| front_articles | 1503 |
| metadata_refs | 0 |
| total_blob_bytes | 27127780 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 1503 | 21811580 | 14512 | 4902 | 67296 |
| WEBP | 79 | 726630 | 9197 | 974 | 60676 |
| image/webp | 45 | 830826 | 18462 | 88 | 187412 |
| image/svg+xml | 34 | 226862 | 6672 | 163 | 27802 |
| application/javascript | 15 | 2013567 | 134237 | 810 | 782786 |
| text/javascript | 9 | 312931 | 34770 | 19 | 302394 |
| text/css | 5 | 1198209 | 239641 | 1131 | 818505 |
| image/vnd.microsoft.icon | 1 | 5430 | 5430 | 5430 | 5430 |
| image/gif | 1 | 265 | 265 | 265 | 265 |
| application/json | 1 | 1480 | 1480 | 1480 | 1480 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 13 |
| avg_entries_per_chunk | 130.2 |
| min_entries_per_chunk | 2 |
| max_entries_per_chunk | 617 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 1503 |
| body_doc_count | 1503 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 5 |
| read | 21 |
| transform | 855 |
| dedup | 8 |
| search_index | 21 |
| chunk_build | 2 |
| dict_train | 0 |
| compress | 0 |
| assemble | 106 |
| close | 1505 |
| total | 2548 |

| Metric | Value |
|--------|------:|
| bytes_read | 37778180 |
| cache_hits | 1684 |
| cache_misses | 22 |
| entry_content | 1693 |
| entry_redirect | 2002 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | WEBP=79;application/javascript=15;application/json=1;image/gif=1;image/png=14;image/svg+xml=34;image/vnd.microsoft.icon=1;image/webp=31;text/css=5;text/html=1503;text/javascript=9 | WEBP=79;application/javascript=15;application/json=1;image/gif=1;image/png=14;image/svg+xml=34;image/vnd.microsoft.icon=1;image/webp=31;text/css=5;text/html=1503;text/javascript=9 | ✓ |
| creator | Stack Exchange | Stack Exchange | ✓ |
| date | 2026-02-05 | 2026-02-05 | ✓ |
| description | Stack Exchange Q&A for community managers, administrators, and moderators | Stack Exchange Q&A for community managers, administrators, and moderators | ✓ |
| language | eng | eng | ✓ |
| license | CC-BY-SA | CC-BY-SA | ✓ |
| main_entry | — | questions | **✗** |
| name | communitybuilding.stackexchange.com_en_all | communitybuilding.stackexchange.com_en_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | sotoki v3.0.2 | sotoki v3.0.2 | ✓ |
| source | — | /Users/cstaszak/projects/oza/testdata/se_community.zim | **✗** |
| tags | _category:stack_exchange;_videos:no;_details:no;stack_exchange | _category:stack_exchange;_videos:no;_details:no;stack_exchange | ✓ |
| title | Community Building Q&A | Community Building Q&A | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 31 | 45 | 14 |
| image/png | 14 | 0 | -14 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=3000 search=all search-prune=0.40 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 2388 | 2388 | none |
| 1 | MIME_TABLE | 152 | 152 | none |
| 2 | ENTRY_TABLE | 30559 | 35111 | zstd |
| 3 | PATH_INDEX | 37149 | 99188 | zstd |
| 4 | TITLE_INDEX | 40868 | 96308 | zstd |
| 5 | CONTENT | 4549449 | 4549449 | none |
| 6 | REDIRECT_TABLE | 2846 | 10014 | zstd |
| 7 | SEARCH_TITLE | 93455 | 236516 | zstd |
| 8 | SEARCH_BODY | 1029919 | 2607541 | zstd |

Overall compression ratio: 0.758

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### ted_street_art

#### File Info

| Key | Value |
|-----|-------|
| title | TED street art |
| language | eng,ara,spa,fra,por,rus,tur,vie,zho,ell,heb,hun,ita,jpn,kor,mya,nld,pol,por,srp,zho,deu,fas,ind,lav,ron,swe,ukr |
| creator | TED |
| date | 2026-02-01 |
| source | /Users/cstaszak/projects/oza/testdata/ted_street_art.zim |
| description | A collection of TED videos about street art |
| UUID | `1377ac95-0ece-7326-96de-87912c6d47a8` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 36.6 MB |
| OZA size | 33.3 MB |
| Size ratio | 0.9105 |
| Size delta | -8.9% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 34797092 | 99.7% | content |
| SEARCH_BODY | 88871 | 0.3% | overhead |
| ENTRY_TABLE | 10536 | 0% | overhead |
| PATH_INDEX | 4343 | 0% | overhead |
| TITLE_INDEX | 4076 | 0% | overhead |
| METADATA | 1612 | 0% | overhead |
| SEARCH_TITLE | 1090 | 0% | overhead |
| MIME_TABLE | 197 | 0% | overhead |
| REDIRECT_TABLE | 9 | 0% | overhead |

Content: 99.7% — Overhead: 0.3%

#### Classification

| | |
|---|---|
| **Profile** | media-heavy |
| **Confidence** | 89% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 4.7% |
| html_bytes_ratio | 0.3% |
| image_bytes_ratio | 0.3% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 77.4% |
| redirect_density | 0.2% |
| avg_entry_bytes | 70.6 KB |
| small_entry_ratio | 29.5% |
| entry_count | 589 |
| mime_type_count | 13 |
| compression_ratio | 98.8% |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 8388608 |
| zstd_level | 3 |
| dict_samples | 500 |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | low zstd level (images are incompressible); image optimization is the real win |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 589 |
| redirects | 0 |
| front_articles | 6 |
| metadata_refs | 0 |
| total_blob_bytes | 42595661 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| application/json | 174 | 284559 | 1635 | 25 | 7744 |
| application/javascript | 157 | 4475523 | 28506 | 51 | 980170 |
| text/vtt | 150 | 1396899 | 9312 | 5967 | 31069 |
| text/javascript | 55 | 328434 | 5971 | 45 | 113195 |
| image/webp | 14 | 79054 | 5646 | 294 | 23248 |
| application/wasm | 11 | 2025047 | 184095 | 39066 | 419205 |
| text/html | 6 | 142289 | 23714 | 4137 | 39463 |
| font/sfnt | 6 | 718696 | 119782 | 8848 | 145932 |
| text/css | 6 | 120875 | 20145 | 1523 | 47581 |
| video/webm | 5 | 32965386 | 6593077 | 4685064 | 10394663 |
| application/octet-stream | 3 | 14526 | 4842 | 2621 | 7895 |
| image/svg+xml | 1 | 39017 | 39017 | 39017 | 39017 |
| font/woff | 1 | 5356 | 5356 | 5356 | 5356 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 12 |
| avg_entries_per_chunk | 49.1 |
| min_entries_per_chunk | 2 |
| max_entries_per_chunk | 145 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 6 |
| body_doc_count | 6 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 4 |
| read | 20 |
| transform | 143 |
| dedup | 12 |
| search_index | 3 |
| chunk_build | 2 |
| dict_train | 0 |
| compress | 0 |
| assemble | 44 |
| close | 1205 |
| total | 1403 |

| Metric | Value |
|--------|------:|
| bytes_read | 47121237 |
| cache_hits | 579 |
| cache_misses | 22 |
| entry_content | 589 |
| entry_redirect | 1 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=157;application/json=174;application/octet-stream=3;application/wasm=11;font/sfnt=6;font/woff=1;image/png=5;image/svg+xml=1;image/webp=9;text/css=6;text/html=6;text/javascript=55;text/vtt=150;video/webm=5 | application/javascript=157;application/json=174;application/octet-stream=3;application/wasm=11;font/sfnt=6;font/woff=1;image/png=5;image/svg+xml=1;image/webp=9;text/css=6;text/html=6;text/javascript=55;text/vtt=150;video/webm=5 | ✓ |
| creator | TED | TED | ✓ |
| date | 2026-02-01 | 2026-02-01 | ✓ |
| description | A collection of TED videos about street art | A collection of TED videos about street art | ✓ |
| language | eng,ara,spa,fra,por,rus,tur,vie,zho,ell,heb,hun,ita,jpn,kor,mya,nld,pol,por,srp,zho,deu,fas,ind,lav,ron,swe,ukr | eng,ara,spa,fra,por,rus,tur,vie,zho,ell,heb,hun,ita,jpn,kor,mya,nld,pol,por,srp,zho,deu,fas,ind,lav,ron,swe,ukr | ✓ |
| main_entry | — | index | **✗** |
| name | ted_mul_street-art | ted_mul_street-art | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | ted2zim 3.1.0 | ted2zim 3.1.0 | ✓ |
| source | — | /Users/cstaszak/projects/oza/testdata/ted_street_art.zim | **✗** |
| tags | _category:ted;ted;_videos:yes | _category:ted;ted;_videos:yes | ✓ |
| title | TED street art | TED street art | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 9 | 14 | 5 |
| image/png | 5 | 0 | -5 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 1612 | 1612 | none |
| 1 | MIME_TABLE | 197 | 197 | none |
| 2 | ENTRY_TABLE | 10536 | 11837 | zstd |
| 3 | PATH_INDEX | 4343 | 12084 | zstd |
| 4 | TITLE_INDEX | 4076 | 11717 | zstd |
| 5 | CONTENT | 34797092 | 34797092 | none |
| 6 | REDIRECT_TABLE | 9 | 9 | none |
| 7 | SEARCH_TITLE | 1090 | 5546 | zstd |
| 8 | SEARCH_BODY | 88871 | 484591 | zstd |

Overall compression ratio: 0.988

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### top100_mini

#### File Info

| Key | Value |
|-----|-------|
| title | Wikipedia 100 |
| language | eng |
| creator | Wikipedia |
| date | 2026-01-15 |
| source | en.wikipedia.org |
| description | Top hundred Wikipedia articles |
| UUID | `847e07fa-16d3-8013-ddcc-934d3d75155a` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 4.3 MB |
| OZA size | 3.2 MB |
| Size ratio | 0.7365 |
| Size delta | -26.4% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 2956773 | 88.5% | content |
| SEARCH_BODY | 217339 | 6.5% | overhead |
| TITLE_INDEX | 47360 | 1.4% | overhead |
| PATH_INDEX | 44704 | 1.3% | overhead |
| SEARCH_TITLE | 44140 | 1.3% | overhead |
| ENTRY_TABLE | 23455 | 0.7% | overhead |
| METADATA | 5203 | 0.2% | overhead |
| REDIRECT_TABLE | 1992 | 0.1% | overhead |
| MIME_TABLE | 91 | 0% | overhead |

Content: 88.5% — Overhead: 11.5%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 69% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 43.0% |
| html_bytes_ratio | 35.3% |
| image_bytes_ratio | 56.4% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 72.3% |
| avg_entry_bytes | 3.2 KB |
| small_entry_ratio | 92.7% |
| entry_count | 1409 |
| mime_type_count | 6 |
| compression_ratio | 83.7% |
| source_hint | wikipedia |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 1409 |
| redirects | 0 |
| front_articles | 1301 |
| metadata_refs | 0 |
| total_blob_bytes | 4647876 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 1301 | 1638527 | 1259 | 110 | 32632 |
| image/webp | 96 | 2619746 | 27289 | 582 | 91700 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 6 |
| avg_entries_per_chunk | 234.8 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 1202 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 1301 |
| body_doc_count | 1301 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 4 |
| read | 0 |
| transform | 31 |
| dedup | 1 |
| search_index | 6 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 41 |
| close | 842 |
| total | 902 |

| Metric | Value |
|--------|------:|
| bytes_read | 4934501 |
| cache_hits | 1419 |
| cache_misses | 4 |
| entry_content | 1409 |
| entry_redirect | 3685 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/png=5;image/svg+xml=8;image/webp=95;text/css=16;text/html=1301;text/javascript=3 | application/javascript=4;image/png=5;image/svg+xml=8;image/webp=95;text/css=16;text/html=1301;text/javascript=3 | ✓ |
| creator | Wikipedia | Wikipedia | ✓ |
| date | 2026-01-15 | 2026-01-15 | ✓ |
| description | Top hundred Wikipedia articles | Top hundred Wikipedia articles | ✓ |
| flavour | mini | mini | ✓ |
| language | eng | eng | ✓ |
| main_entry | — | index | **✗** |
| name | wikipedia_en_100 | wikipedia_en_100 | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.4 | mwoffliner 1.17.4 | ✓ |
| source | en.wikipedia.org | en.wikipedia.org | ✓ |
| tags | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:no;_ftindex:yes | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:no;_ftindex:yes | ✓ |
| title | Wikipedia 100 | Wikipedia 100 | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 95 | 96 | 1 |
| image/png | 1 | 0 | -1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 5203 | 5203 | none |
| 1 | MIME_TABLE | 91 | 91 | none |
| 2 | ENTRY_TABLE | 23455 | 28160 | zstd |
| 3 | PATH_INDEX | 44704 | 113859 | zstd |
| 4 | TITLE_INDEX | 47360 | 106971 | zstd |
| 5 | CONTENT | 2956773 | 2956773 | none |
| 6 | REDIRECT_TABLE | 1992 | 18429 | zstd |
| 7 | SEARCH_TITLE | 44140 | 116205 | zstd |
| 8 | SEARCH_BODY | 217339 | 645000 | zstd |

Overall compression ratio: 0.837

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 1 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### vikidia_ru

#### File Info

| Key | Value |
|-----|-------|
| title | Vikidia |
| language | rus |
| creator | Vikidia |
| date | 2026-03-12 |
| source | ru.vikidia.org |
| description | <binary 29 bytes> |
| UUID | `b687febc-d42c-79ba-93e7-818ce747eb74` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 10.4 MB |
| OZA size | 9.7 MB |
| Size ratio | 0.9353 |
| Size delta | -6.5% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 10013778 | 98% | content |
| SEARCH_BODY | 149112 | 1.5% | overhead |
| SEARCH_TITLE | 16681 | 0.2% | overhead |
| PATH_INDEX | 15260 | 0.1% | overhead |
| ENTRY_TABLE | 14127 | 0.1% | overhead |
| TITLE_INDEX | 6664 | 0.1% | overhead |
| METADATA | 3598 | 0% | overhead |
| REDIRECT_TABLE | 234 | 0% | overhead |
| MIME_TABLE | 102 | 0% | overhead |

Content: 98.0% — Overhead: 2.0%

#### Classification

| | |
|---|---|
| **Profile** | media-heavy |
| **Confidence** | 90% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 19.3% |
| html_bytes_ratio | 16.3% |
| image_bytes_ratio | 80.5% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 5.4% |
| avg_entry_bytes | 14.6 KB |
| small_entry_ratio | 0.6% |
| entry_count | 802 |
| mime_type_count | 7 |
| compression_ratio | 96.2% |
| source_hint | vikidia |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 8388608 |
| zstd_level | 3 |
| dict_samples | 500 |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | low zstd level (images are incompressible); image optimization is the real win |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 802 |
| redirects | 0 |
| front_articles | 338 |
| metadata_refs | 0 |
| total_blob_bytes | 11977040 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| image/webp | 451 | 9619986 | 21330 | 236 | 4099928 |
| text/html | 338 | 1951144 | 5772 | 2004 | 95120 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/css | 4 | 3887 | 971 | 26 | 2145 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |
| image/gif | 1 | 16325 | 16325 | 16325 | 16325 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 8 |
| avg_entries_per_chunk | 100.3 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 197 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 338 |
| body_doc_count | 338 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 3 |
| read | 1 |
| transform | 3037 |
| dedup | 4 |
| search_index | 4 |
| chunk_build | 1 |
| dict_train | 0 |
| compress | 0 |
| assemble | 31 |
| close | 286 |
| total | 3358 |

| Metric | Value |
|--------|------:|
| bytes_read | 12573092 |
| cache_hits | 808 |
| cache_misses | 8 |
| entry_content | 802 |
| entry_redirect | 46 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/gif=7;image/png=2;image/svg+xml=14;image/webp=444;text/css=10;text/html=338;text/javascript=3 | application/javascript=4;image/gif=7;image/png=2;image/svg+xml=14;image/webp=444;text/css=10;text/html=338;text/javascript=3 | ✓ |
| creator | Vikidia | Vikidia | ✓ |
| date | 2026-03-12 | 2026-03-12 | ✓ |
| description | — | <binary 29 bytes> | **✗** |
| flavour | maxi | maxi | ✓ |
| language | rus | rus | ✓ |
| main_entry | — | <binary 35 bytes> | **✗** |
| name | vikidia_ru_all | vikidia_ru_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.5 | mwoffliner 1.17.5 | ✓ |
| source | ru.vikidia.org | ru.vikidia.org | ✓ |
| tags | vikidia;_category:vikidia;_pictures:yes;_videos:no;_details:yes;_ftindex:yes | vikidia;_category:vikidia;_pictures:yes;_videos:no;_details:yes;_ftindex:yes | ✓ |
| title | Vikidia | Vikidia | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 444 | 451 | 7 |
| image/gif | 7 | 1 | -6 |
| image/png | 1 | 0 | -1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 3598 | 3598 | none |
| 1 | MIME_TABLE | 102 | 102 | none |
| 2 | ENTRY_TABLE | 14127 | 16246 | zstd |
| 3 | PATH_INDEX | 15260 | 31277 | zstd |
| 4 | TITLE_INDEX | 6664 | 15634 | zstd |
| 5 | CONTENT | 10013778 | 10013778 | none |
| 6 | REDIRECT_TABLE | 234 | 234 | none |
| 7 | SEARCH_TITLE | 16681 | 45496 | zstd |
| 8 | SEARCH_BODY | 149112 | 491596 | zstd |

Overall compression ratio: 0.962

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA
- :warning: 3 MIME type count(s) differ between ZIM and OZA

---

### wikiquote_ja

#### File Info

| Key | Value |
|-----|-------|
| title | Wikiquote |
| language | jpn |
| creator | Wikiquote |
| date | 2026-01-15 |
| source | ja.wikiquote.org |
| description | An Offline Version of Wikiquote in Japanese Language |
| UUID | `7a6e5c10-ac52-8351-f1ea-521c3d2ede03` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 5.3 MB |
| OZA size | 2.7 MB |
| Size ratio | 0.5193 |
| Size delta | -48.1% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 1815545 | 63.4% | content |
| SEARCH_BODY | 953881 | 33.3% | overhead |
| SEARCH_TITLE | 30491 | 1.1% | overhead |
| ENTRY_TABLE | 24206 | 0.8% | overhead |
| PATH_INDEX | 15767 | 0.6% | overhead |
| TITLE_INDEX | 14979 | 0.5% | overhead |
| METADATA | 4881 | 0.2% | overhead |
| REDIRECT_TABLE | 580 | 0% | overhead |
| MIME_TABLE | 179 | 0% | overhead |

Content: 63.5% — Overhead: 36.5%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 84% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 99.4% |
| html_bytes_ratio | 95.7% |
| image_bytes_ratio | 0.3% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 17.1% |
| avg_entry_bytes | 6.9 KB |
| small_entry_ratio | 1.3% |
| entry_count | 1355 |
| mime_type_count | 7 |
| compression_ratio | 65.3% |
| source_hint | wikiquote |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 1355 |
| redirects | 0 |
| front_articles | 1331 |
| metadata_refs | 0 |
| total_blob_bytes | 9608825 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 1331 | 9193368 | 6907 | 2827 | 178986 |
| image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0" | 11 | 23486 | 2135 | 953 | 4727 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |
| image/webp | 1 | 2368 | 2368 | 2368 | 2368 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 14 |
| avg_entries_per_chunk | 96.8 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 288 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 1331 |
| body_doc_count | 1331 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 4 |
| read | 10 |
| transform | 53 |
| dedup | 3 |
| search_index | 17 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 87 |
| close | 1346 |
| total | 1503 |

| Metric | Value |
|--------|------:|
| bytes_read | 10702723 |
| cache_hits | 1361 |
| cache_misses | 8 |
| entry_content | 1355 |
| entry_redirect | 279 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/png=1;image/svg+xml=12;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=11;text/css=15;text/html=1331;text/html; charset=iso-8859-1=1;text/javascript=3 | application/javascript=4;image/png=1;image/svg+xml=12;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=11;text/css=15;text/html=1331;text/html; charset=iso-8859-1=1;text/javascript=3 | ✓ |
| creator | Wikiquote | Wikiquote | ✓ |
| date | 2026-01-15 | 2026-01-15 | ✓ |
| description | An Offline Version of Wikiquote in Japanese Language | An Offline Version of Wikiquote in Japanese Language | ✓ |
| flavour | nopic | nopic | ✓ |
| language | jpn | jpn | ✓ |
| main_entry | — | <binary 18 bytes> | **✗** |
| name | wikiquote_ja_all | wikiquote_ja_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.4 | mwoffliner 1.17.4 | ✓ |
| source | ja.wikiquote.org | ja.wikiquote.org | ✓ |
| tags | wikiquote;_category:wikiquote;_pictures:no;_videos:no;_details:yes;_ftindex:yes | wikiquote;_category:wikiquote;_pictures:no;_videos:no;_details:yes;_ftindex:yes | ✓ |
| title | Wikiquote | Wikiquote | ✓ |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/png | 1 | 0 | -1 |
| image/webp | 0 | 1 | 1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=9 chunk=1048576 dict=4000 search=all search-prune=0.30 minify transcode |
| Chunk Target Size | 1048576 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 4881 | 4881 | none |
| 1 | MIME_TABLE | 179 | 179 | none |
| 2 | ENTRY_TABLE | 24206 | 27096 | zstd |
| 3 | PATH_INDEX | 15767 | 34216 | zstd |
| 4 | TITLE_INDEX | 14979 | 33444 | zstd |
| 5 | CONTENT | 1815545 | 1815545 | none |
| 6 | REDIRECT_TABLE | 580 | 1399 | zstd |
| 7 | SEARCH_TITLE | 30491 | 93580 | zstd |
| 8 | SEARCH_BODY | 953881 | 2370863 | zstd |

Overall compression ratio: 0.653

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 1 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### wiktionary_he

#### File Info

| Key | Value |
|-----|-------|
| title | <binary 18 bytes> |
| language | heb |
| creator | Wiktionary |
| date | 2026-01-15 |
| source | he.wiktionary.org |
| description | The biggest Wiki Dictionary in Hebrew language |
| UUID | `20372fa5-57b3-2ccd-b1cf-5777da0130a8` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 39.9 MB |
| OZA size | 30.1 MB |
| Size ratio | 0.7549 |
| Size delta | -24.5% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 23957617 | 75.8% | content |
| SEARCH_BODY | 6418121 | 20.3% | overhead |
| ENTRY_TABLE | 480335 | 1.5% | overhead |
| SEARCH_TITLE | 314632 | 1% | overhead |
| TITLE_INDEX | 208177 | 0.7% | overhead |
| PATH_INDEX | 198371 | 0.6% | overhead |
| REDIRECT_TABLE | 7260 | 0% | overhead |
| METADATA | 6929 | 0% | overhead |
| MIME_TABLE | 179 | 0% | overhead |

Content: 75.8% — Overhead: 24.2%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 85% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 99.9% |
| html_bytes_ratio | 99.8% |
| image_bytes_ratio | 0.0% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 13.5% |
| avg_entry_bytes | 9.0 KB |
| small_entry_ratio | 0.2% |
| entry_count | 26927 |
| mime_type_count | 7 |
| compression_ratio | 83.2% |
| source_hint | wiktionary |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 26927 |
| redirects | 0 |
| front_articles | 26860 |
| metadata_refs | 0 |
| total_blob_bytes | 247365478 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 26860 | 246857165 | 9190 | 110 | 103123 |
| image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0" | 54 | 115990 | 2147 | 700 | 5132 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |
| image/webp | 1 | 2720 | 2720 | 2720 | 2720 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 239 |
| avg_entries_per_chunk | 112.7 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 416 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 26860 |
| body_doc_count | 26860 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 13 |
| read | 191 |
| transform | 1280 |
| dedup | 84 |
| search_index | 86 |
| chunk_build | 2913 |
| dict_train | 0 |
| compress | 0 |
| assemble | 568 |
| close | 581 |
| total | 6780 |

| Metric | Value |
|--------|------:|
| bytes_read | 280436508 |
| cache_hits | 26802 |
| cache_misses | 139 |
| entry_content | 26927 |
| entry_redirect | 4201 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/gif=1;image/png=2;image/svg+xml=10;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=54;text/css=23;text/html=26860;text/html; charset=iso-8859-1=1;text/javascript=3 | application/javascript=4;image/gif=1;image/png=2;image/svg+xml=10;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=54;text/css=23;text/html=26860;text/html; charset=iso-8859-1=1;text/javascript=3 | ✓ |
| creator | Wiktionary | Wiktionary | ✓ |
| date | 2026-01-15 | 2026-01-15 | ✓ |
| description | The biggest Wiki Dictionary in Hebrew language | The biggest Wiki Dictionary in Hebrew language | ✓ |
| flavour | nopic | nopic | ✓ |
| language | heb | heb | ✓ |
| main_entry | — | <binary 36 bytes> | **✗** |
| name | wiktionary_he_all | wiktionary_he_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.4 | mwoffliner 1.17.4 | ✓ |
| source | he.wiktionary.org | he.wiktionary.org | ✓ |
| tags | wiktionary;_category:wiktionary;_pictures:no;_videos:no;_details:yes;_ftindex:yes | wiktionary;_category:wiktionary;_pictures:no;_videos:no;_details:yes;_ftindex:yes | ✓ |
| title | — | <binary 18 bytes> | **✗** |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/png | 1 | 0 | -1 |
| image/webp | 0 | 1 | 1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=9 chunk=1048576 dict=4000 search=all search-prune=0.30 minify transcode |
| Chunk Target Size | 1048576 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 6929 | 6929 | none |
| 1 | MIME_TABLE | 179 | 179 | none |
| 2 | ENTRY_TABLE | 480335 | 552979 | zstd |
| 3 | PATH_INDEX | 198371 | 527879 | zstd |
| 4 | TITLE_INDEX | 208177 | 541037 | zstd |
| 5 | CONTENT | 23957617 | 23957617 | none |
| 6 | REDIRECT_TABLE | 7260 | 21009 | zstd |
| 7 | SEARCH_TITLE | 314632 | 525180 | zstd |
| 8 | SEARCH_BODY | 6418121 | 11823385 | zstd |

Overall compression ratio: 0.832

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### wiktionary_yi

#### File Info

| Key | Value |
|-----|-------|
| title | <binary 26 bytes> |
| language | yid |
| creator | Wiktionary |
| date | 2026-01-15 |
| source | yi.wiktionary.org |
| description | Wiktionary in Yiddish Language |
| UUID | `1e7b2a76-f94a-c461-a237-8ace2375f981` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 1.4 MB |
| OZA size | 567.7 KB |
| Size ratio | 0.3918 |
| Size delta | -60.8% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 399151 | 68.7% | content |
| SEARCH_BODY | 122627 | 21.1% | overhead |
| SEARCH_TITLE | 19301 | 3.3% | overhead |
| ENTRY_TABLE | 14356 | 2.5% | overhead |
| PATH_INDEX | 8703 | 1.5% | overhead |
| TITLE_INDEX | 8624 | 1.5% | overhead |
| METADATA | 7151 | 1.2% | overhead |
| REDIRECT_TABLE | 429 | 0.1% | overhead |
| MIME_TABLE | 91 | 0% | overhead |

Content: 68.8% — Overhead: 31.2%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 84% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 99.6% |
| html_bytes_ratio | 95.3% |
| image_bytes_ratio | 0.1% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 32.4% |
| avg_entry_bytes | 10.1 KB |
| small_entry_ratio | 0.6% |
| entry_count | 809 |
| mime_type_count | 6 |
| compression_ratio | 57.6% |
| source_hint | wiktionary |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 809 |
| redirects | 0 |
| front_articles | 796 |
| metadata_refs | 0 |
| total_blob_bytes | 8332444 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 796 | 7938587 | 9973 | 2794 | 19917 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/webp | 1 | 4254 | 4254 | 4254 | 4254 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 12 |
| avg_entries_per_chunk | 67.4 |
| min_entries_per_chunk | 2 |
| max_entries_per_chunk | 168 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 796 |
| body_doc_count | 796 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 3 |
| read | 4 |
| transform | 74 |
| dedup | 2 |
| search_index | 6 |
| chunk_build | 0 |
| dict_train | 0 |
| compress | 0 |
| assemble | 21 |
| close | 580 |
| total | 719 |

| Metric | Value |
|--------|------:|
| bytes_read | 9916185 |
| cache_hits | 817 |
| cache_misses | 6 |
| entry_content | 809 |
| entry_redirect | 387 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/png=1;image/svg+xml=7;text/css=12;text/html=796;text/javascript=3 | application/javascript=4;image/png=1;image/svg+xml=7;text/css=12;text/html=796;text/javascript=3 | ✓ |
| creator | Wiktionary | Wiktionary | ✓ |
| date | 2026-01-15 | 2026-01-15 | ✓ |
| description | Wiktionary in Yiddish Language | Wiktionary in Yiddish Language | ✓ |
| flavour | nopic | nopic | ✓ |
| language | yid | yid | ✓ |
| main_entry | — | <binary 19 bytes> | **✗** |
| name | wiktionary_yi_all | wiktionary_yi_all | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.4 | mwoffliner 1.17.4 | ✓ |
| source | yi.wiktionary.org | yi.wiktionary.org | ✓ |
| tags | wiktionary;_category:wiktionary;_pictures:no;_videos:no;_details:yes;_ftindex:yes | wiktionary;_category:wiktionary;_pictures:no;_videos:no;_details:yes;_ftindex:yes | ✓ |
| title | — | <binary 26 bytes> | **✗** |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/png | 1 | 0 | -1 |
| image/webp | 0 | 1 | 1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=9 chunk=1048576 dict=4000 search=all search-prune=0.30 minify transcode |
| Chunk Target Size | 1048576 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 7151 | 7151 | none |
| 1 | MIME_TABLE | 91 | 91 | none |
| 2 | ENTRY_TABLE | 14356 | 16188 | zstd |
| 3 | PATH_INDEX | 8703 | 19845 | zstd |
| 4 | TITLE_INDEX | 8624 | 19744 | zstd |
| 5 | CONTENT | 399151 | 399151 | none |
| 6 | REDIRECT_TABLE | 429 | 1939 | zstd |
| 7 | SEARCH_TITLE | 19301 | 63313 | zstd |
| 8 | SEARCH_BODY | 122627 | 479930 | zstd |

Overall compression ratio: 0.576

#### Verification

All integrity checks **passed** (file, section, chunk).

#### Notes

- :warning: 2 metadata key(s) differ between ZIM and OZA
- :warning: 2 MIME type count(s) differ between ZIM and OZA

---

### zh_chemistry

#### File Info

| Key | Value |
|-----|-------|
| title | <binary 18 bytes> |
| language | zho |
| creator | Wikipedia |
| date | 2026-03-18 |
| source | zh.wikipedia.org |
| description | <binary 36 bytes> |
| UUID | `e3c82e5c-8173-31b6-f65b-c22a4dfe9887` |
| Flags | has-search |

#### Size Comparison

| Metric | Value |
|--------|------:|
| ZIM size | 13.4 MB |
| OZA size | 10.6 MB |
| Size ratio | 0.7914 |
| Size delta | -20.9% |

#### Size Budget

| Section | Size | % of File | Category |
|---------|-----:|----------:|----------|
| CONTENT | 8168883 | 73.4% | content |
| SEARCH_BODY | 2551686 | 22.9% | overhead |
| PATH_INDEX | 115285 | 1% | overhead |
| TITLE_INDEX | 95162 | 0.9% | overhead |
| SEARCH_TITLE | 91166 | 0.8% | overhead |
| ENTRY_TABLE | 84606 | 0.8% | overhead |
| REDIRECT_TABLE | 12377 | 0.1% | overhead |
| METADATA | 5860 | 0.1% | overhead |
| MIME_TABLE | 179 | 0% | overhead |

Content: 73.4% — Overhead: 26.6%

#### Classification

| | |
|---|---|
| **Profile** | encyclopedia |
| **Confidence** | 82% |

**Features:**

| Feature | Value |
|---------|------:|
| text_bytes_ratio | 90.5% |
| html_bytes_ratio | 89.7% |
| image_bytes_ratio | 9.4% |
| pdf_bytes_ratio | 0.0% |
| video_bytes_ratio | 0.0% |
| redirect_density | 62.2% |
| avg_entry_bytes | 9.1 KB |
| small_entry_ratio | 14.1% |
| entry_count | 4744 |
| mime_type_count | 7 |
| compression_ratio | 77.0% |
| source_hint | wikipedia |

**Recommendations:**

| Setting | Value |
|---------|-------|
| chunk_size | 4194304 |
| zstd_level | 6 |
| dict_samples | 2000 |
| minify | true |
| optimize_images | true |
| search_prune_freq | 0.5 |
| notes | balanced defaults for text+image articles |

#### Entry Statistics

| Metric | Count |
|--------|------:|
| content_entries | 4744 |
| redirects | 0 |
| front_articles | 3968 |
| metadata_refs | 0 |
| total_blob_bytes | 44412413 |

#### MIME Census

| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |
|-----------|------:|------------:|----------:|----:|----:|
| text/html | 3968 | 39840124 | 10040 | 100 | 311981 |
| image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0" | 666 | 2405812 | 3612 | 599 | 20156 |
| image/webp | 98 | 1776874 | 18131 | 1792 | 156696 |
| application/javascript | 4 | 30767 | 7691 | 348 | 23816 |
| text/css | 4 | 3905 | 976 | 44 | 2145 |
| text/javascript | 3 | 354687 | 118229 | 2649 | 344910 |
| image/svg+xml | 1 | 244 | 244 | 244 | 244 |

#### Chunk Statistics

| Metric | Value |
|--------|------:|
| chunk_count | 15 |
| avg_entries_per_chunk | 316.3 |
| min_entries_per_chunk | 3 |
| max_entries_per_chunk | 803 |

#### Search Index

| Metric | Value |
|--------|-------|
| has_title_search | true |
| has_body_search | true |
| title_doc_count | 3968 |
| body_doc_count | 3968 |

#### Conversion Performance

| Phase | Time (ms) |
|-------|----------:|
| scan | 7 |
| read | 42 |
| transform | 421 |
| dedup | 14 |
| search_index | 40 |
| chunk_build | 1561 |
| dict_train | 0 |
| compress | 0 |
| assemble | 255 |
| close | 261 |
| total | 2559 |

| Metric | Value |
|--------|------:|
| bytes_read | 49735343 |
| cache_hits | 4730 |
| cache_misses | 28 |
| entry_content | 4744 |
| entry_redirect | 7805 |

#### Metadata Comparison

| Key | ZIM | OZA | Match |
|-----|-----|-----|:-----:|
| counter | application/javascript=4;image/gif=2;image/png=7;image/svg+xml=12;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=666;image/webp=95;text/css=19;text/html=3968;text/html; charset=iso-8859-1=1;text/javascript=3 | application/javascript=4;image/gif=2;image/png=7;image/svg+xml=12;image/svg+xml; charset=utf-8; profile="https://www.mediawiki.org/wiki/Specs/SVG/1.0.0"=666;image/webp=95;text/css=19;text/html=3968;text/html; charset=iso-8859-1=1;text/javascript=3 | ✓ |
| creator | Wikipedia | Wikipedia | ✓ |
| date | 2026-03-18 | 2026-03-18 | ✓ |
| description | — | <binary 36 bytes> | **✗** |
| flavour | mini | mini | ✓ |
| language | zho | zho | ✓ |
| main_entry | — | index | **✗** |
| name | wikipedia_zh_chemistry | wikipedia_zh_chemistry | ✓ |
| publisher | openZIM | openZIM | ✓ |
| scraper | mwoffliner 1.17.5 | mwoffliner 1.17.5 | ✓ |
| source | zh.wikipedia.org | zh.wikipedia.org | ✓ |
| tags | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:no;_ftindex:yes | wikipedia;_category:wikipedia;_pictures:no;_videos:no;_details:no;_ftindex:yes | ✓ |
| title | — | <binary 18 bytes> | **✗** |

#### MIME Count Discrepancies

| MIME Type | ZIM Count | OZA Count | Delta |
|-----------|----------:|----------:|------:|
| image/webp | 95 | 98 | 3 |
| image/gif | 2 | 0 | -2 |
| image/png | 1 | 0 | -1 |

#### Conversion Settings

| Setting | Value |
|---------|-------|
| Converter | zim2oza |
| Version | dev |
| Flags | zstd=6 chunk=4194304 dict=2000 search=all search-prune=0.50 minify optimize-images transcode |
| Chunk Target Size | 4194304 |

#### Section Breakdown

| # | Type | Compressed | Uncompressed | Compression |
|--:|------|------------|--------------|-------------|
| 0 | METADATA | 5860 | 5860 | none |
| 1 | MIME_TABLE | 179 | 179 | none |
| 2 | ENTRY_TABLE | 84606 | 97529 | zstd |
| 3 | PATH_INDEX | 115285 | 264048 | zstd |
| 4 | TITLE_INDEX | 95162 | 230981 | zstd |
| 5 | CONTENT | 8168883 | 8168883 | none |
| 6 | REDIRECT_TABLE | 12377 | 39029 | zstd |
| 7 | SEARCH_TITLE | 91166 | 256024 | zstd |
| 8 | SEARCH_BODY | 2551686 | 5391320 | zstd |

Overall compression ratio: 0.770

#### Verification

**FAILED** (exit code 1)

#### Notes

- :warning: 3 metadata key(s) differ between ZIM and OZA
- :warning: 3 MIME type count(s) differ between ZIM and OZA

---

## Errors & Warnings

### Conversion Failures

_None._

### Verification Failures

- **ar_chemistry**: exit code 1
- **zh_chemistry**: exit code 1

### Metadata Mismatches

| File | Key | ZIM Value | OZA Value |
|------|-----|-----------|-----------|
| ar_chemistry | description | — | <binary 84 bytes> |
| ar_chemistry | main_entry | — | index |
| ar_chemistry | title | — | <binary 35 bytes> |
| devdocs_go | main_entry | — | index |
| devdocs_go | source | — | /Users/cstaszak/projects/oza/testdata/devdocs_go.zim |
| gobyexample | main_entry | — | gobyexample.com/ |
| gobyexample | source | — | /Users/cstaszak/projects/oza/testdata/gobyexample.zim |
| gutenberg_ar | description | — | <binary 119 bytes> |
| gutenberg_ar | main_entry | — | Home |
| gutenberg_ar | source | — | /Users/cstaszak/projects/oza/testdata/gutenberg_ar.zim |
| gutenberg_ar | title | — | <binary 36 bytes> |
| gutenberg_ko | main_entry | — | Home |
| gutenberg_ko | source | — | /Users/cstaszak/projects/oza/testdata/gutenberg_ko.zim |
| ray_charles | main_entry | — | index |
| ray_charles_nopic | main_entry | — | index |
| se_community | main_entry | — | questions |
| se_community | source | — | /Users/cstaszak/projects/oza/testdata/se_community.zim |
| ted_street_art | main_entry | — | index |
| ted_street_art | source | — | /Users/cstaszak/projects/oza/testdata/ted_street_art.zim |
| top100_mini | main_entry | — | index |
| vikidia_ru | description | — | <binary 29 bytes> |
| vikidia_ru | main_entry | — | <binary 35 bytes> |
| wikiquote_ja | main_entry | — | <binary 18 bytes> |
| wiktionary_he | main_entry | — | <binary 36 bytes> |
| wiktionary_he | title | — | <binary 18 bytes> |
| wiktionary_yi | main_entry | — | <binary 19 bytes> |
| wiktionary_yi | title | — | <binary 26 bytes> |
| zh_chemistry | description | — | <binary 36 bytes> |
| zh_chemistry | main_entry | — | index |
| zh_chemistry | title | — | <binary 18 bytes> |

### MIME Count Discrepancies

| File | MIME Type | ZIM | OZA | Delta |
|------|----------|----:|----:|------:|
| ar_chemistry | image/webp | 99 | 100 | 1 |
| ar_chemistry | image/png | 1 | 0 | -1 |
| gobyexample | image/webp | 0 | 2 | 2 |
| gobyexample | image/png | 2 | 0 | -2 |
| gutenberg_ar | image/webp | 0 | 19 | 19 |
| gutenberg_ar | image/png | 17 | 0 | -17 |
| gutenberg_ar | image/gif | 2 | 0 | -2 |
| gutenberg_ko | image/webp | 0 | 19 | 19 |
| gutenberg_ko | image/png | 17 | 0 | -17 |
| gutenberg_ko | image/gif | 2 | 0 | -2 |
| ray_charles | image/webp | 164 | 165 | 1 |
| ray_charles | image/png | 1 | 0 | -1 |
| ray_charles_nopic | image/webp | 9 | 10 | 1 |
| ray_charles_nopic | image/png | 1 | 0 | -1 |
| se_community | image/webp | 31 | 45 | 14 |
| se_community | image/png | 14 | 0 | -14 |
| ted_street_art | image/webp | 9 | 14 | 5 |
| ted_street_art | image/png | 5 | 0 | -5 |
| top100_mini | image/webp | 95 | 96 | 1 |
| top100_mini | image/png | 1 | 0 | -1 |
| vikidia_ru | image/webp | 444 | 451 | 7 |
| vikidia_ru | image/gif | 7 | 1 | -6 |
| vikidia_ru | image/png | 1 | 0 | -1 |
| wikiquote_ja | image/png | 1 | 0 | -1 |
| wikiquote_ja | image/webp | 0 | 1 | 1 |
| wiktionary_he | image/png | 1 | 0 | -1 |
| wiktionary_he | image/webp | 0 | 1 | 1 |
| wiktionary_yi | image/png | 1 | 0 | -1 |
| wiktionary_yi | image/webp | 0 | 1 | 1 |
| zh_chemistry | image/webp | 95 | 98 | 3 |
| zh_chemistry | image/gif | 2 | 0 | -2 |
| zh_chemistry | image/png | 1 | 0 | -1 |

### Skipped Files

Below 100000-byte threshold:

- small.zim (41155 bytes)

### Conversion Warnings

_None._

---

_Report generated by `scripts/bench-report.sh`._

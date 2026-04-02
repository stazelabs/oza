#!/usr/bin/env bash
# fetch.sh — download Tier 1 ZIM test fixtures for CI (~150 MB total).
# Idempotent: skips files that already exist. Safe to re-run.
#
# Update the date suffixes below when new Kiwix dumps are published.
# See docs/TESTING_PLAN.md §1.2 for what each file tests.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$REPO/testdata"
mkdir -p "$DIR"
KIWIX="https://download.kiwix.org/zim"
ZIM_SUITE="https://github.com/openzim/zim-testing-suite/raw/main/data"
FAILED=0

fetch() {
  local name="$1" url="$2"
  local dest="$DIR/$name"
  if [ -f "$dest" ]; then
    printf "EXISTS %s\n" "$name"
    return
  fi
  printf "FETCH  %s\n" "$name"
  if curl -fSL --retry 3 --connect-timeout 30 -o "$dest.tmp" "$url"; then
    mv "$dest.tmp" "$dest"
  else
    rm -f "$dest.tmp"
    printf "WARN   %s download failed, skipping\n" "$name" >&2
    FAILED=$((FAILED + 1))
  fi
}

# --- openzim test suite (smoke test) ---
fetch small.zim "$ZIM_SUITE/nons/small.zim"

# --- Wikipedia ---
# Single article with images, infobox, citations (2.7 MB)
fetch ray_charles.zim "$KIWIX/wikipedia/wikipedia_en_ray-charles_maxi_2026-02.zim"
# Same article, text-only — compare image impact on size (1.6 MB)
fetch ray_charles_nopic.zim "$KIWIX/wikipedia/wikipedia_en_ray-charles_nopic_2026-02.zim"
# 100 EN articles, intro-only — multiple entries, redirects (4.3 MB)
fetch top100_mini.zim "$KIWIX/wikipedia/wikipedia_en_100_mini_2026-01.zim"
# RTL Arabic STEM content (11 MB)
fetch ar_chemistry.zim "$KIWIX/wikipedia/wikipedia_ar_chemistry_mini_2026-01.zim"
# CJK Chinese — bigram search validation (13 MB)
fetch zh_chemistry.zim "$KIWIX/wikipedia/wikipedia_zh_chemistry_mini_2026-03.zim"

# --- Wiktionary ---
# RTL dictionary, Hebrew — high redirect ratio, inflected→lemma (40 MB)
fetch wiktionary_he.zim "$KIWIX/wiktionary/wiktionary_he_all_nopic_2026-01.zim"
# Yiddish, Hebrew script RTL — rare script coverage (1.4 MB)
fetch wiktionary_yi.zim "$KIWIX/wiktionary/wiktionary_yi_all_nopic_2026-01.zim"

# --- Wikiquote ---
# Japanese CJK short-form text (5.3 MB)
fetch wikiquote_ja.zim "$KIWIX/wikiquote/wikiquote_ja_all_nopic_2026-01.zim"

# --- Gutenberg ---
# Arabic books, RTL (2.1 MB)
fetch gutenberg_ar.zim "$KIWIX/gutenberg/gutenberg_ar_all_2025-12.zim"
# Korean books, CJK Hangul (2.2 MB)
fetch gutenberg_ko.zim "$KIWIX/gutenberg/gutenberg_ko_all_2026-01.zim"

# --- StackExchange ---
# Q&A format — tags, user cards, vote widgets (6 MB)
fetch se_community.zim "$KIWIX/stack_exchange/communitybuilding.stackexchange.com_en_all_2026-02.zim"

# --- DevDocs ---
# Structured developer docs, code blocks, no images (1.5 MB)
fetch devdocs_go.zim "$KIWIX/devdocs/devdocs_en_go_2026-01.zim"

# --- Vikidia ---
# Cyrillic children's encyclopedia with images (10 MB)
fetch vikidia_ru.zim "$KIWIX/vikidia/vikidia_ru_all_maxi_2026-03.zim"

# --- TED ---
# Video content — thumbnails, subtitles, non-text MIME types (37 MB)
fetch ted_street_art.zim "$KIWIX/ted/ted_mul_street-art_2026-02.zim"

# --- Zimit ---
# Zimit-scraped site — tests zimit-specific structure (322 KB)
fetch gobyexample.zim "$KIWIX/zimit/gobyexample.com_en_all_2026-02.zim"

echo ""
if [ "$FAILED" -gt 0 ]; then
  echo "Done. $FAILED file(s) failed to download — tests using them will be skipped."
  exit 0  # non-fatal: tests skip missing fixtures
else
  echo "Done. All Tier 1 fixtures downloaded."
fi

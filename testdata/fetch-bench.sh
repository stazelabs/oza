#!/usr/bin/env bash
# fetch-bench.sh — download Tier 2 ZIM benchmark fixtures (50 MB – 2 GB each).
# These are too large for CI. Run manually: make testdata-bench
#
# By default downloads a curated subset (~3 GB). Pass --all for the full
# suite (~15 GB). Pass individual names to cherry-pick:
#   bash testdata/fetch-bench.sh wp_simple_mini.zim wp_ja_top_mini.zim se_codegolf.zim
#
# See docs/TESTING_PLAN.md §1.3 for what each file tests.
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)/bench"
mkdir -p "$DIR"
KIWIX="https://download.kiwix.org/zim"
FAILED=0

fetch() {
  local name="$1" url="$2"
  local dest="$DIR/$name"
  if [ -f "$dest" ]; then
    printf "EXISTS %s\n" "$name"
    return
  fi
  printf "FETCH  %s  (this may take a while)\n" "$name"
  if curl -fSL --retry 3 --connect-timeout 30 -o "$dest.tmp" "$url"; then
    mv "$dest.tmp" "$dest"
  else
    rm -f "$dest.tmp"
    printf "WARN   %s download failed, skipping\n" "$name" >&2
    FAILED=$((FAILED + 1))
  fi
}

# ---------------------------------------------------------------------------
# Registry: local_name -> URL
# Update dates when new Kiwix dumps are published.
#
# Uses a lookup function instead of associative arrays for bash 3.2
# compatibility (macOS ships bash 3.2 which lacks declare -A).
# ---------------------------------------------------------------------------

# ALL_NAMES is the complete list of available files.
ALL_NAMES=(
  # Wikipedia — scale & variants
  wp_simple_mini.zim
  wp_simple_nopic.zim
  wp_en_top_mini.zim
  wp_en_medicine_nopic.zim
  wp_en_chemistry_maxi.zim
  wp_en_physics_nopic.zim
  wp_en_climate_maxi.zim
  # CJK — bigram search & compression
  wp_ja_top_mini.zim
  wp_ja_top_nopic.zim
  wp_ko_top_mini.zim
  wp_ko_all_mini.zim
  wp_zh_top_mini.zim
  wp_zh_movies_maxi.zim
  # RTL — Arabic, Hebrew, Farsi
  wp_ar_top_mini.zim
  wp_ar_top_nopic.zim
  wp_ar_medicine_maxi.zim
  wp_he_top_mini.zim
  wp_he_all_mini.zim
  wp_fa_top_mini.zim
  # Wiktionary — dictionaries, high redirect ratios
  wikt_ja.zim
  wikt_ko.zim
  wikt_de.zim
  wikt_es.zim
  wikt_fa.zim
  # StackExchange — Q&A structure
  se_japanese.zim
  se_chinese.zim
  se_codegolf.zim
  se_judaism.zim
  se_astronomy.zim
  se_ja_so.zim
  # Gutenberg — book-length content
  gutenberg_zh.zim
  gutenberg_it.zim
  gutenberg_en_lcc_k.zim
  gutenberg_en_lcc_j.zim
  # Other sources — structural variety
  vikidia_fr.zim
  wikisource_fa.zim
  wikivoyage_zh.zim
  wikiquote_en.zim
  worldfactbook.zim
  xkcd.zim
  stacks_math.zim
  mankier.zim
  openwrt.zim
  ted_code.zim
  minecraft_zh.zim
)

# registry_url returns the download URL for a given file name, or empty if unknown.
registry_url() {
  case "$1" in
    # Wikipedia — scale & variants
    wp_simple_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_en_simple_all_mini_2026-02.zim" ;;
    wp_simple_nopic.zim)      echo "$KIWIX/wikipedia/wikipedia_en_simple_all_nopic_2026-02.zim" ;;
    wp_en_top_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_en_top_mini_2026-03.zim" ;;
    wp_en_medicine_nopic.zim) echo "$KIWIX/wikipedia/wikipedia_en_medicine_nopic_2026-01.zim" ;;
    wp_en_chemistry_maxi.zim) echo "$KIWIX/wikipedia/wikipedia_en_chemistry_maxi_2026-01.zim" ;;
    wp_en_physics_nopic.zim)  echo "$KIWIX/wikipedia/wikipedia_en_physics_nopic_2026-01.zim" ;;
    wp_en_climate_maxi.zim)   echo "$KIWIX/wikipedia/wikipedia_en_climate-change_maxi_2026-01.zim" ;;
    # CJK — bigram search & compression
    wp_ja_top_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_ja_top_mini_2026-01.zim" ;;
    wp_ja_top_nopic.zim)      echo "$KIWIX/wikipedia/wikipedia_ja_top_nopic_2026-01.zim" ;;
    wp_ko_top_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_ko_top_mini_2026-01.zim" ;;
    wp_ko_all_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_ko_all_mini_2026-01.zim" ;;
    wp_zh_top_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_zh_top_mini_2026-03.zim" ;;
    wp_zh_movies_maxi.zim)    echo "$KIWIX/wikipedia/wikipedia_zh_movies_maxi_2026-03.zim" ;;
    # RTL — Arabic, Hebrew, Farsi
    wp_ar_top_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_ar_top_mini_2026-01.zim" ;;
    wp_ar_top_nopic.zim)      echo "$KIWIX/wikipedia/wikipedia_ar_top_nopic_2026-01.zim" ;;
    wp_ar_medicine_maxi.zim)  echo "$KIWIX/wikipedia/wikipedia_ar_medicine_maxi_2026-01.zim" ;;
    wp_he_top_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_he_top_mini_2026-01.zim" ;;
    wp_he_all_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_he_all_mini_2026-01.zim" ;;
    wp_fa_top_mini.zim)       echo "$KIWIX/wikipedia/wikipedia_fa_top_mini_2026-01.zim" ;;
    # Wiktionary — dictionaries, high redirect ratios
    wikt_ja.zim)              echo "$KIWIX/wiktionary/wiktionary_ja_all_nopic_2026-01.zim" ;;
    wikt_ko.zim)              echo "$KIWIX/wiktionary/wiktionary_ko_all_nopic_2026-03.zim" ;;
    wikt_de.zim)              echo "$KIWIX/wiktionary/wiktionary_de_all_nopic_2026-01.zim" ;;
    wikt_es.zim)              echo "$KIWIX/wiktionary/wiktionary_es_all_nopic_2026-03.zim" ;;
    wikt_fa.zim)              echo "$KIWIX/wiktionary/wiktionary_fa_all_nopic_2026-01.zim" ;;
    # StackExchange — Q&A structure
    se_japanese.zim)          echo "$KIWIX/stack_exchange/japanese.stackexchange.com_mul_all_2026-02.zim" ;;
    se_chinese.zim)           echo "$KIWIX/stack_exchange/chinese.stackexchange.com_mul_all_2026-02.zim" ;;
    se_codegolf.zim)          echo "$KIWIX/stack_exchange/codegolf.stackexchange.com_en_all_2026-02.zim" ;;
    se_judaism.zim)           echo "$KIWIX/stack_exchange/judaism.stackexchange.com_en_all_2026-02.zim" ;;
    se_astronomy.zim)         echo "$KIWIX/stack_exchange/astronomy.stackexchange.com_en_all_2026-02.zim" ;;
    se_ja_so.zim)             echo "$KIWIX/stack_exchange/ja.stackoverflow.com_mul_all_2026-02.zim" ;;
    # Gutenberg — book-length content
    gutenberg_zh.zim)         echo "$KIWIX/gutenberg/gutenberg_zh_all_2025-12.zim" ;;
    gutenberg_it.zim)         echo "$KIWIX/gutenberg/gutenberg_it_all_2026-01.zim" ;;
    gutenberg_en_lcc_k.zim)   echo "$KIWIX/gutenberg/gutenberg_en_lcc-k_2026-03.zim" ;;
    gutenberg_en_lcc_j.zim)   echo "$KIWIX/gutenberg/gutenberg_en_lcc-j_2026-03.zim" ;;
    # Other sources — structural variety
    vikidia_fr.zim)           echo "$KIWIX/vikidia/vikidia_fr_all_maxi_2025-12.zim" ;;
    wikisource_fa.zim)        echo "$KIWIX/wikisource/wikisource_fa_all_maxi_2026-01.zim" ;;
    wikivoyage_zh.zim)        echo "$KIWIX/wikivoyage/wikivoyage_zh_all_maxi_2026-03.zim" ;;
    wikiquote_en.zim)         echo "$KIWIX/wikiquote/wikiquote_en_all_maxi_2026-01.zim" ;;
    worldfactbook.zim)        echo "$KIWIX/other/theworldfactbook_en_all_2026-02.zim" ;;
    xkcd.zim)                 echo "$KIWIX/zimit/xkcd.com_en_all_2026-02.zim" ;;
    stacks_math.zim)          echo "$KIWIX/zimit/stacks.math.columbia.edu_en_all_2025-06.zim" ;;
    mankier.zim)              echo "$KIWIX/zimit/www.mankier.com_en_all_2026-01.zim" ;;
    openwrt.zim)              echo "$KIWIX/zimit/openwrt.org_en_all_2026-03.zim" ;;
    ted_code.zim)             echo "$KIWIX/ted/ted_mul_code_2026-01.zim" ;;
    minecraft_zh.zim)         echo "$KIWIX/other/zh.minecraft.wiki_zh_all_2026-03.zim" ;;
    *) ;;
  esac
}

# Curated default subset — one representative per category (~3 GB)
DEFAULT_SET=(
  wp_simple_mini.zim        # 441 MB — EN Wikipedia at scale
  wp_ja_top_mini.zim        # 168 MB — CJK Japanese
  wp_ar_top_mini.zim        # 203 MB — RTL Arabic
  wikt_fa.zim               # 72 MB  — RTL dictionary
  se_codegolf.zim           # 372 MB — Q&A, Unicode-heavy
  gutenberg_zh.zim          # 301 MB — CJK books
  wikisource_fa.zim         # 286 MB — RTL full books
  worldfactbook.zim         # 388 MB — structured tables
  wp_en_chemistry_maxi.zim  # 470 MB — STEM + images
  xkcd.zim                  # 424 MB — image-dominant
)

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------

if [ $# -eq 0 ]; then
  TARGETS=("${DEFAULT_SET[@]}")
  echo "Downloading curated Tier 2 subset (~3 GB)."
  echo "Use --all for the full suite, or pass names to cherry-pick."
  echo ""
elif [ "$1" = "--all" ]; then
  TARGETS=("${ALL_NAMES[@]}")
  echo "Downloading full Tier 2 suite (~15 GB). This will take a while."
  echo ""
elif [ "$1" = "--list" ]; then
  echo "Available Tier 2 files:"
  for name in "${ALL_NAMES[@]}"; do
    if [ -f "$DIR/$name" ]; then
      printf "  [x] %s\n" "$name"
    else
      printf "  [ ] %s\n" "$name"
    fi
  done
  exit 0
else
  TARGETS=("$@")
fi

for name in "${TARGETS[@]}"; do
  url="$(registry_url "$name")"
  if [ -z "$url" ]; then
    echo "ERROR: unknown file '$name'. Run with --list to see available files." >&2
    exit 1
  fi
  fetch "$name" "$url"
done

echo ""
if [ "$FAILED" -gt 0 ]; then
  echo "Done. $FAILED file(s) failed to download."
else
  echo "Done. All requested Tier 2 fixtures downloaded to testdata/bench/."
fi

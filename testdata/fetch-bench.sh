#!/usr/bin/env bash
# fetch-bench.sh — download Tier 2 ZIM benchmark fixtures (50 MB – 2 GB each).
# These are too large for CI. Run manually: make testdata-bench
#
# By default downloads a curated subset (~3 GB). Pass --all for the full
# suite (~15 GB). Pass individual names to cherry-pick:
#   bash testdata/fetch-bench.sh wp_simple_mini wp_ja_top_mini se_codegolf
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
# ---------------------------------------------------------------------------

declare -A REGISTRY=(
  # Wikipedia — scale & variants
  [wp_simple_mini.zim]="$KIWIX/wikipedia/wikipedia_en_simple_all_mini_2026-02.zim"
  [wp_simple_nopic.zim]="$KIWIX/wikipedia/wikipedia_en_simple_all_nopic_2026-02.zim"
  [wp_en_top_mini.zim]="$KIWIX/wikipedia/wikipedia_en_top_mini_2026-03.zim"
  [wp_en_medicine_nopic.zim]="$KIWIX/wikipedia/wikipedia_en_medicine_nopic_2026-01.zim"
  [wp_en_chemistry_maxi.zim]="$KIWIX/wikipedia/wikipedia_en_chemistry_maxi_2026-01.zim"
  [wp_en_physics_nopic.zim]="$KIWIX/wikipedia/wikipedia_en_physics_nopic_2026-01.zim"
  [wp_en_climate_maxi.zim]="$KIWIX/wikipedia/wikipedia_en_climate-change_maxi_2026-01.zim"

  # CJK — bigram search & compression
  [wp_ja_top_mini.zim]="$KIWIX/wikipedia/wikipedia_ja_top_mini_2026-01.zim"
  [wp_ja_top_nopic.zim]="$KIWIX/wikipedia/wikipedia_ja_top_nopic_2026-01.zim"
  [wp_ko_top_mini.zim]="$KIWIX/wikipedia/wikipedia_ko_top_mini_2026-01.zim"
  [wp_ko_all_mini.zim]="$KIWIX/wikipedia/wikipedia_ko_all_mini_2026-01.zim"
  [wp_zh_top_mini.zim]="$KIWIX/wikipedia/wikipedia_zh_top_mini_2026-03.zim"
  [wp_zh_movies_maxi.zim]="$KIWIX/wikipedia/wikipedia_zh_movies_maxi_2026-03.zim"

  # RTL — Arabic, Hebrew, Farsi
  [wp_ar_top_mini.zim]="$KIWIX/wikipedia/wikipedia_ar_top_mini_2026-01.zim"
  [wp_ar_top_nopic.zim]="$KIWIX/wikipedia/wikipedia_ar_top_nopic_2026-01.zim"
  [wp_ar_medicine_maxi.zim]="$KIWIX/wikipedia/wikipedia_ar_medicine_maxi_2026-01.zim"
  [wp_he_top_mini.zim]="$KIWIX/wikipedia/wikipedia_he_top_mini_2026-01.zim"
  [wp_he_all_mini.zim]="$KIWIX/wikipedia/wikipedia_he_all_mini_2026-01.zim"
  [wp_fa_top_mini.zim]="$KIWIX/wikipedia/wikipedia_fa_top_mini_2026-01.zim"

  # Wiktionary — dictionaries, high redirect ratios
  [wikt_ja.zim]="$KIWIX/wiktionary/wiktionary_ja_all_nopic_2026-01.zim"
  [wikt_ko.zim]="$KIWIX/wiktionary/wiktionary_ko_all_nopic_2026-03.zim"
  [wikt_de.zim]="$KIWIX/wiktionary/wiktionary_de_all_nopic_2026-01.zim"
  [wikt_es.zim]="$KIWIX/wiktionary/wiktionary_es_all_nopic_2026-03.zim"
  [wikt_fa.zim]="$KIWIX/wiktionary/wiktionary_fa_all_nopic_2026-01.zim"

  # StackExchange — Q&A structure
  [se_japanese.zim]="$KIWIX/stack_exchange/japanese.stackexchange.com_mul_all_2026-02.zim"
  [se_chinese.zim]="$KIWIX/stack_exchange/chinese.stackexchange.com_mul_all_2026-02.zim"
  [se_codegolf.zim]="$KIWIX/stack_exchange/codegolf.stackexchange.com_en_all_2026-02.zim"
  [se_judaism.zim]="$KIWIX/stack_exchange/judaism.stackexchange.com_en_all_2026-02.zim"
  [se_astronomy.zim]="$KIWIX/stack_exchange/astronomy.stackexchange.com_en_all_2026-02.zim"
  [se_ja_so.zim]="$KIWIX/stack_exchange/ja.stackoverflow.com_mul_all_2026-02.zim"

  # Gutenberg — book-length content
  [gutenberg_zh.zim]="$KIWIX/gutenberg/gutenberg_zh_all_2025-12.zim"
  [gutenberg_it.zim]="$KIWIX/gutenberg/gutenberg_it_all_2026-01.zim"
  [gutenberg_en_lcc_k.zim]="$KIWIX/gutenberg/gutenberg_en_lcc-k_2026-03.zim"
  [gutenberg_en_lcc_j.zim]="$KIWIX/gutenberg/gutenberg_en_lcc-j_2026-03.zim"

  # Other sources — structural variety
  [vikidia_fr.zim]="$KIWIX/vikidia/vikidia_fr_all_maxi_2025-12.zim"
  [wikisource_fa.zim]="$KIWIX/wikisource/wikisource_fa_all_maxi_2026-01.zim"
  [wikivoyage_zh.zim]="$KIWIX/wikivoyage/wikivoyage_zh_all_maxi_2026-03.zim"
  [wikiquote_en.zim]="$KIWIX/wikiquote/wikiquote_en_all_maxi_2026-01.zim"
  [worldfactbook.zim]="$KIWIX/other/theworldfactbook_en_all_2026-02.zim"
  [xkcd.zim]="$KIWIX/zimit/xkcd.com_en_all_2026-02.zim"
  [stacks_math.zim]="$KIWIX/zimit/stacks.math.columbia.edu_en_all_2025-06.zim"
  [mankier.zim]="$KIWIX/zimit/www.mankier.com_en_all_2026-01.zim"
  [openwrt.zim]="$KIWIX/zimit/openwrt.org_en_all_2026-03.zim"
  [ted_code.zim]="$KIWIX/ted/ted_mul_code_2026-01.zim"
  [minecraft_zh.zim]="$KIWIX/other/zh.minecraft.wiki_zh_all_2026-03.zim"
)

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
  # Default: curated subset
  TARGETS=("${DEFAULT_SET[@]}")
  echo "Downloading curated Tier 2 subset (~3 GB)."
  echo "Use --all for the full suite, or pass names to cherry-pick."
  echo ""
elif [ "$1" = "--all" ]; then
  TARGETS=("${!REGISTRY[@]}")
  echo "Downloading full Tier 2 suite (~15 GB). This will take a while."
  echo ""
elif [ "$1" = "--list" ]; then
  echo "Available Tier 2 files:"
  for name in $(printf '%s\n' "${!REGISTRY[@]}" | sort); do
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
  if [ -z "${REGISTRY[$name]+x}" ]; then
    echo "ERROR: unknown file '$name'. Run with --list to see available files." >&2
    exit 1
  fi
  fetch "$name" "${REGISTRY[$name]}"
done

echo ""
if [ "$FAILED" -gt 0 ]; then
  echo "Done. $FAILED file(s) failed to download."
else
  echo "Done. All requested Tier 2 fixtures downloaded to testdata/bench/."
fi

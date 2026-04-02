#!/usr/bin/env bash
# bench-report.sh — Generate a comprehensive ZIM→OZA benchmark report (Markdown).
#
# Converts every ZIM testdata file to OZA, collects statistics from all CLI
# tools (zim2oza, ozainfo, ozacmp, ozaverify), and emits a self-contained
# markdown report.
#
# Usage:
#   scripts/bench-report.sh [OPTIONS]
#
# Options:
#   --bench           Include testdata/bench/*.zim (large files, slow)
#   --deep            Pass --deep to ozacmp for per-entry byte sizes
#   --min-size N      Skip ZIMs smaller than N bytes (default: 100000)
#   --output FILE     Write report to FILE instead of stdout
#   -h, --help        Show this help
#
# Requirements: jq, go, make
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$ROOT/bin"
TESTDATA="$ROOT/testdata"

MIN_SIZE=100000
INCLUDE_BENCH=0
DEEP=0
OUTPUT=""

# ---------- argument parsing ----------

usage() {
  sed -n '2,/^[^#]/s/^# \?//p' "$0"
  exit 0
}

die() { echo "error: $*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --bench)       INCLUDE_BENCH=1; shift ;;
    --deep)        DEEP=1; shift ;;
    --min-size)    MIN_SIZE="$2"; shift 2 ;;
    --output)      OUTPUT="$2"; shift 2 ;;
    -h|--help)     usage ;;
    *)             die "unknown option: $1" ;;
  esac
done

# ---------- dependency checks ----------

check_deps() {
  for cmd in jq go make; do
    command -v "$cmd" >/dev/null 2>&1 || die "$cmd is required but not found"
  done
}
check_deps

# ---------- helpers ----------

portable_size() {
  stat -f%z "$1" 2>/dev/null || stat -c%s "$1" 2>/dev/null
}

# fmt_bytes <bytes> — human-readable size string.
fmt_bytes() {
  awk "BEGIN {
    s = $1
    if      (s >= 1073741824) printf \"%.1f GB\", s/1073741824
    else if (s >= 1048576)    printf \"%.1f MB\", s/1048576
    else if (s >= 1024)       printf \"%.1f KB\", s/1024
    else                      printf \"%d B\", s
  }"
}

# ---------- setup ----------

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

if [ -n "$OUTPUT" ]; then
  exec > "$OUTPUT"
fi

echo "Building binaries..." >&2
(cd "$ROOT" && make build) >/dev/null 2>&1 || die "make build failed"

# ---------- phase 1: discover ZIM files ----------

ZIM_LIST="$WORK/zim_list.txt"
SKIPPED="$WORK/skipped.txt"
: > "$ZIM_LIST"
: > "$SKIPPED"

for zim in "$TESTDATA"/*.zim; do
  [ -f "$zim" ] || continue
  sz=$(portable_size "$zim")
  if [ "$sz" -lt "$MIN_SIZE" ]; then
    echo "$(basename "$zim") ($sz bytes)" >> "$SKIPPED"
    continue
  fi
  echo "$zim" >> "$ZIM_LIST"
done

if [ "$INCLUDE_BENCH" -eq 1 ] && [ -d "$TESTDATA/bench" ]; then
  for zim in "$TESTDATA/bench"/*.zim; do
    [ -f "$zim" ] || continue
    sz=$(portable_size "$zim")
    if [ "$sz" -lt "$MIN_SIZE" ]; then
      echo "$(basename "$zim") ($sz bytes)" >> "$SKIPPED"
      continue
    fi
    echo "$zim" >> "$ZIM_LIST"
  done
fi

FILE_COUNT=$(wc -l < "$ZIM_LIST" | tr -d ' ')
[ "$FILE_COUNT" -gt 0 ] || die "no ZIM files found above ${MIN_SIZE}-byte threshold"

# ---------- phase 2: process each ZIM ----------

ERRORS="$WORK/errors.txt"
: > "$ERRORS"

# Ordered list of basenames (without .zim) for report generation.
NAMES="$WORK/names.txt"
: > "$NAMES"

process_zim() {
  local zim="$1"
  local name
  name=$(basename "$zim" .zim)
  echo "$name" >> "$NAMES"

  echo "  Processing $name..." >&2

  # 1. Convert.
  local deep_flag=""
  if "$BIN/zim2oza" --auto --json-stats "$zim" "$WORK/${name}.oza" \
       > "$WORK/${name}.convert.json" 2> "$WORK/${name}.convert.stderr"; then
    echo 0 > "$WORK/${name}.convert.exit"
  else
    echo $? > "$WORK/${name}.convert.exit"
    echo "$name" >> "$ERRORS"
    return 0  # continue with other files
  fi

  # 2. Inspect.
  "$BIN/ozainfo" --json --classify "$WORK/${name}.oza" \
    > "$WORK/${name}.info.json" 2> "$WORK/${name}.info.stderr" || true

  # 3. Compare.
  if [ "$DEEP" -eq 1 ]; then deep_flag="--deep"; fi
  "$BIN/ozacmp" "$zim" "$WORK/${name}.oza" --format json $deep_flag \
    > "$WORK/${name}.cmp.json" 2> "$WORK/${name}.cmp.stderr" || true

  # 4. Verify.
  if "$BIN/ozaverify" --all --quiet "$WORK/${name}.oza" \
       > /dev/null 2> "$WORK/${name}.verify.stderr"; then
    echo 0 > "$WORK/${name}.verify.exit"
  else
    echo $? > "$WORK/${name}.verify.exit"
  fi
}

while IFS= read -r zim; do
  process_zim "$zim"
done < "$ZIM_LIST"

# ---------- phase 3: emit report ----------

echo "Generating report..." >&2

# ---- header ----

cat <<EOF
# OZA Benchmark Report

| | |
|---|---|
| **Generated** | $(date -u +"%Y-%m-%d %H:%M:%S UTC") |
| **System** | $(uname -srm) |
| **Go** | $(go version | awk '{print $3}') |
| **CPU** | $(sysctl -n machdep.cpu.brand_string 2>/dev/null || (lscpu 2>/dev/null | grep 'Model name' | sed 's/.*: //') || echo "unknown") |
| **Files tested** | ${FILE_COUNT} |
| **Flags** | bench=$INCLUDE_BENCH deep=$DEEP min-size=$MIN_SIZE |

---

EOF

# ---- executive summary ----

cat <<'EOF'
## Executive Summary

| File | ZIM Size | OZA Size | Ratio | Δ% | Profile | Conf | Entries | Time (s) | Verify |
|------|----------|----------|------:|---:|---------|-----:|--------:|---------:|:------:|
EOF

while IFS= read -r name; do
  conv="$WORK/${name}.convert.json"
  info="$WORK/${name}.info.json"
  conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)

  if [ "$conv_exit" -ne 0 ]; then
    zim_sz=$(jq -r '.input_size_bytes // empty' "$conv" 2>/dev/null || echo "")
    zim_hr="—"
    [ -n "$zim_sz" ] && zim_hr=$(fmt_bytes "$zim_sz")
    echo "| $name | $zim_hr | — | — | — | — | — | — | — | **FAIL** |"
    continue
  fi

  zim_sz=$(jq -r '.input_size_bytes' "$conv")
  oza_sz=$(jq -r '.output_size_bytes' "$conv")
  ratio=$(jq -r '.size_ratio // empty' "$conv")
  entries=$(jq -r '.entry_total' "$conv")
  time_ms=$(jq -r '.time_total_ms' "$conv")
  time_s=$(awk "BEGIN { printf \"%.1f\", $time_ms / 1000 }")

  zim_hr=$(fmt_bytes "$zim_sz")
  oza_hr=$(fmt_bytes "$oza_sz")

  if [ -n "$ratio" ] && [ "$ratio" != "null" ]; then
    ratio_fmt=$(awk "BEGIN { printf \"%.3f\", $ratio }")
    delta_pct=$(awk "BEGIN { printf \"%+.1f\", ($ratio - 1) * 100 }")
  else
    ratio_fmt="—"
    delta_pct="—"
  fi

  profile=$(jq -r '.classification.profile // "—"' "$info" 2>/dev/null || echo "—")
  conf=$(jq -r '.classification.confidence // empty' "$info" 2>/dev/null || echo "—")
  if [ -n "$conf" ] && [ "$conf" != "null" ]; then
    conf=$(awk "BEGIN { printf \"%.0f%%\", $conf * 100 }")
  else
    conf="—"
  fi

  verify_exit=$(cat "$WORK/${name}.verify.exit" 2>/dev/null || echo 1)
  verify="PASS"
  [ "$verify_exit" -ne 0 ] && verify="**FAIL**"

  echo "| $name | $zim_hr | $oza_hr | $ratio_fmt | $delta_pct | $profile | $conf | $entries | $time_s | $verify |"
done < "$NAMES"

echo ""

# ---- aggregate statistics ----

cat <<'HEADER'
---

## Aggregate Statistics

HEADER

# Compute aggregates from convert JSONs.
{
  total_zim=0; total_oza=0
  count=0; pass=0; fail=0
  sum_time=0; min_time=999999999; max_time=0
  while IFS= read -r name; do
    conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)
    if [ "$conv_exit" -ne 0 ]; then
      fail=$((fail + 1))
      continue
    fi
    count=$((count + 1))
    zim_sz=$(jq -r '.input_size_bytes' "$WORK/${name}.convert.json")
    oza_sz=$(jq -r '.output_size_bytes' "$WORK/${name}.convert.json")
    time_ms=$(jq -r '.time_total_ms' "$WORK/${name}.convert.json")
    total_zim=$((total_zim + zim_sz))
    total_oza=$((total_oza + oza_sz))
    sum_time=$((sum_time + time_ms))
    [ "$time_ms" -lt "$min_time" ] && min_time=$time_ms
    [ "$time_ms" -gt "$max_time" ] && max_time=$time_ms

    verify_exit=$(cat "$WORK/${name}.verify.exit" 2>/dev/null || echo 1)
    [ "$verify_exit" -eq 0 ] && pass=$((pass + 1))
  done < "$NAMES"

  overall_ratio=""
  if [ "$total_zim" -gt 0 ]; then
    overall_ratio=$(awk "BEGIN { printf \"%.3f\", $total_oza / $total_zim }")
  fi
  mean_time=0
  [ "$count" -gt 0 ] && mean_time=$((sum_time / count))

  echo "| Metric | Value |"
  echo "|--------|-------|"
  echo "| Total ZIM size | $(fmt_bytes $total_zim) |"
  echo "| Total OZA size | $(fmt_bytes $total_oza) |"
  echo "| Overall size ratio | ${overall_ratio:-—} |"
  echo "| Savings | $(awk "BEGIN { printf \"%.1f%%\", (1 - $total_oza / ($total_zim + 0.001)) * 100 }") |"
  echo "| Files converted | $count |"
  echo "| Conversion failures | $fail |"
  echo "| Verification | $pass/$count pass |"
  echo "| Mean conversion time | $(awk "BEGIN { printf \"%.1f s\", $mean_time / 1000 }") |"
  echo "| Min conversion time | $(awk "BEGIN { printf \"%.1f s\", $min_time / 1000 }") |"
  echo "| Max conversion time | $(awk "BEGIN { printf \"%.1f s\", $max_time / 1000 }") |"
}

echo ""

# Profile distribution.
echo "### Profile Distribution"
echo ""
echo "| Profile | Count |"
echo "|---------|------:|"

while IFS= read -r name; do
  conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)
  [ "$conv_exit" -ne 0 ] && continue
  jq -r '.classification.profile // "unknown"' "$WORK/${name}.info.json" 2>/dev/null || echo "unknown"
done < "$NAMES" | sort | uniq -c | sort -rn | while read -r cnt prof; do
  echo "| $prof | $cnt |"
done

echo ""
echo "---"
echo ""

# ---- per-file details ----

echo "## Per-File Details"
echo ""

while IFS= read -r name; do
  conv="$WORK/${name}.convert.json"
  info="$WORK/${name}.info.json"
  cmp="$WORK/${name}.cmp.json"
  conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)

  echo "### $name"
  echo ""

  if [ "$conv_exit" -ne 0 ]; then
    echo "> **CONVERSION FAILED** — exit code $conv_exit"
    echo ""
    if [ -f "$WORK/${name}.convert.stderr" ] && [ -s "$WORK/${name}.convert.stderr" ]; then
      echo '```'
      cat "$WORK/${name}.convert.stderr"
      echo '```'
    fi
    echo ""
    echo "---"
    echo ""
    continue
  fi

  # ---- file info ----
  echo "#### File Info"
  echo ""
  echo "| Key | Value |"
  echo "|-----|-------|"
  for key in title language creator date source description; do
    val=$(jq -r ".metadata.\"$key\" // empty" "$info" 2>/dev/null || true)
    [ -n "$val" ] && echo "| $key | $val |"
  done
  uuid=$(jq -r '.header.uuid // empty' "$info" 2>/dev/null || true)
  [ -n "$uuid" ] && echo "| UUID | \`$uuid\` |"
  flags=$(jq -r '(.header.flag_names // []) | join(", ")' "$info" 2>/dev/null || true)
  [ -n "$flags" ] && echo "| Flags | $flags |"
  echo ""

  # ---- size comparison ----
  echo "#### Size Comparison"
  echo ""
  zim_sz=$(jq -r '.input_size_bytes' "$conv")
  oza_sz=$(jq -r '.output_size_bytes' "$conv")
  ratio=$(jq -r '.size_ratio // empty' "$conv")
  echo "| Metric | Value |"
  echo "|--------|------:|"
  echo "| ZIM size | $(fmt_bytes "$zim_sz") |"
  echo "| OZA size | $(fmt_bytes "$oza_sz") |"
  if [ -n "$ratio" ] && [ "$ratio" != "null" ]; then
    echo "| Size ratio | $(awk "BEGIN { printf \"%.4f\", $ratio }") |"
    echo "| Size delta | $(awk "BEGIN { printf \"%+.1f%%\", ($ratio - 1) * 100 }") |"
    if [ "$(awk "BEGIN { print ($ratio > 1) }")" = "1" ]; then
      echo "| | :warning: OZA is larger than ZIM |"
    fi
  fi
  echo ""

  # ---- content budget from cmp ----
  if [ -f "$cmp" ] && [ -s "$cmp" ]; then
    content_pct=$(jq -r '.size_budget.content_pct // empty' "$cmp" 2>/dev/null || true)
    overhead_pct=$(jq -r '.size_budget.overhead_pct // empty' "$cmp" 2>/dev/null || true)
    budget_entries=$(jq -r '.size_budget.entries // empty' "$cmp" 2>/dev/null || true)
    if [ -n "$content_pct" ] && [ "$content_pct" != "null" ]; then
      echo "#### Size Budget"
      echo ""
      echo "| Section | Size | % of File | Category |"
      echo "|---------|-----:|----------:|----------|"
      jq -r '.size_budget.entries[]? | "| \(.section) | \(.size) | \(.pct_of_file | .*10|round/10)% | \(.category) |"' "$cmp" 2>/dev/null || true
      echo ""
      echo "Content: $(awk "BEGIN { printf \"%.1f%%\", $content_pct }") — Overhead: $(awk "BEGIN { printf \"%.1f%%\", $overhead_pct }")"
      echo ""
    fi
  fi

  # ---- classification ----
  echo "#### Classification"
  echo ""
  profile=$(jq -r '.classification.profile // "—"' "$info" 2>/dev/null || echo "—")
  conf=$(jq -r '.classification.confidence // empty' "$info" 2>/dev/null || echo "")
  echo "| | |"
  echo "|---|---|"
  echo "| **Profile** | $profile |"
  if [ -n "$conf" ] && [ "$conf" != "null" ]; then
    echo "| **Confidence** | $(awk "BEGIN { printf \"%.0f%%\", $conf * 100 }") |"
  fi
  echo ""

  # Features.
  echo "**Features:**"
  echo ""
  echo "| Feature | Value |"
  echo "|---------|------:|"
  for feat in text_bytes_ratio html_bytes_ratio image_bytes_ratio pdf_bytes_ratio video_bytes_ratio \
              redirect_density avg_entry_bytes small_entry_ratio entry_count mime_type_count \
              compression_ratio source_hint; do
    val=$(jq -r ".classification.features.${feat} // empty" "$info" 2>/dev/null || true)
    if [ -n "$val" ] && [ "$val" != "null" ]; then
      # Format ratios as percentages, skip -1 (unavailable).
      case "$feat" in
        *_ratio|*_density)
          if [ "$val" = "-1" ]; then
            val="n/a"
          else
            val=$(awk "BEGIN { printf \"%.1f%%\", $val * 100 }")
          fi
          ;;
        avg_entry_bytes)
          val=$(awk "BEGIN { printf \"%.0f\", $val }")
          val="$(fmt_bytes "$val")"
          ;;
      esac
      echo "| $feat | $val |"
    fi
  done
  echo ""

  # Recommendations.
  recs=$(jq -r '.classification.recommendations // empty' "$info" 2>/dev/null || true)
  if [ -n "$recs" ] && [ "$recs" != "null" ]; then
    echo "**Recommendations:**"
    echo ""
    echo "| Setting | Value |"
    echo "|---------|-------|"
    for rkey in chunk_size zstd_level dict_samples minify optimize_images search_prune_freq notes; do
      rval=$(jq -r ".classification.recommendations.${rkey} // empty" "$info" 2>/dev/null || true)
      [ -n "$rval" ] && [ "$rval" != "null" ] && echo "| $rkey | $rval |"
    done
    echo ""
  fi

  # ---- entry statistics ----
  echo "#### Entry Statistics"
  echo ""
  echo "| Metric | Count |"
  echo "|--------|------:|"
  jq -r '.entry_stats | to_entries[] | "| \(.key) | \(.value) |"' "$info" 2>/dev/null || true
  echo ""

  # ---- MIME census ----
  echo "#### MIME Census"
  echo ""
  mime_count=$(jq -r '.mime_census | length' "$info" 2>/dev/null || echo 0)
  if [ "$mime_count" -gt 0 ]; then
    echo "| MIME Type | Count | Total Bytes | Avg Bytes | Min | Max |"
    echo "|-----------|------:|------------:|----------:|----:|----:|"
    jq -r '.mime_census[] | "| \(.mime_type) | \(.count) | \(.total_bytes) | \(.avg_bytes | floor) | \(.min_bytes) | \(.max_bytes) |"' "$info" 2>/dev/null || true
  else
    echo "_No MIME census data._"
  fi
  echo ""

  # ---- chunk statistics ----
  echo "#### Chunk Statistics"
  echo ""
  echo "| Metric | Value |"
  echo "|--------|------:|"
  jq -r '.chunk_stats | to_entries[] | "| \(.key) | \(if (.value | type) == "number" and ((.value | floor) != .value) then (.value * 10 | round / 10) else .value end) |"' "$info" 2>/dev/null || true
  echo ""

  # ---- search index ----
  echo "#### Search Index"
  echo ""
  echo "| Metric | Value |"
  echo "|--------|-------|"
  jq -r '.search_stats | to_entries[] | "| \(.key) | \(.value) |"' "$info" 2>/dev/null || true
  echo ""

  # ---- conversion performance ----
  echo "#### Conversion Performance"
  echo ""
  echo "| Phase | Time (ms) |"
  echo "|-------|----------:|"
  for phase in time_scan_ms time_read_ms time_transform_ms time_dedup_ms time_search_index_ms \
               time_chunk_build_ms time_dict_train_ms time_compress_ms time_assemble_ms \
               time_close_ms time_total_ms; do
    val=$(jq -r ".${phase} // empty" "$conv" 2>/dev/null || true)
    label="${phase%_ms}"
    label="${label#time_}"
    [ -n "$val" ] && echo "| $label | $val |"
  done
  echo ""
  echo "| Metric | Value |"
  echo "|--------|------:|"
  for metric in bytes_read cache_hits cache_misses entry_content entry_redirect; do
    val=$(jq -r ".${metric} // empty" "$conv" 2>/dev/null || true)
    [ -n "$val" ] && echo "| $metric | $val |"
  done
  echo ""

  # ---- metadata comparison ----
  if [ -f "$cmp" ] && [ -s "$cmp" ]; then
    meta_len=$(jq -r '.metadata_match | length' "$cmp" 2>/dev/null || echo 0)
    if [ "$meta_len" -gt 0 ]; then
      echo "#### Metadata Comparison"
      echo ""
      echo "| Key | ZIM | OZA | Match |"
      echo "|-----|-----|-----|:-----:|"
      jq -r '.metadata_match[]? | "| \(.key) | \(.zim_value // "—") | \(.oza_value // "—") | \(if .match then "✓" else "**✗**" end) |"' "$cmp" 2>/dev/null || true
      echo ""
    fi

    # ---- MIME count comparison ----
    mime_delta=$(jq -r '[.mime_count_match[]? | select(.delta != 0)] | length' "$cmp" 2>/dev/null || echo 0)
    if [ "$mime_delta" -gt 0 ]; then
      echo "#### MIME Count Discrepancies"
      echo ""
      echo "| MIME Type | ZIM Count | OZA Count | Delta |"
      echo "|-----------|----------:|----------:|------:|"
      jq -r '.mime_count_match[]? | select(.delta != 0) | "| \(.mime_type) | \(.zim_count) | \(.oza_count) | \(.delta) |"' "$cmp" 2>/dev/null || true
      echo ""
    fi

    # ---- conversion settings ----
    settings_len=$(jq -r '.conversion_settings | length' "$cmp" 2>/dev/null || echo 0)
    if [ "$settings_len" -gt 0 ]; then
      echo "#### Conversion Settings"
      echo ""
      echo "| Setting | Value |"
      echo "|---------|-------|"
      jq -r '.conversion_settings[]? | "| \(.key) | \(.value) |"' "$cmp" 2>/dev/null || true
      echo ""
    fi
  fi

  # ---- section breakdown ----
  section_count=$(jq -r '.sections | length' "$info" 2>/dev/null || echo 0)
  if [ "$section_count" -gt 0 ]; then
    echo "#### Section Breakdown"
    echo ""
    echo "| # | Type | Compressed | Uncompressed | Compression |"
    echo "|--:|------|------------|--------------|-------------|"
    jq -r '.sections[]? | "| \(.index) | \(.type) | \(.compressed_size) | \(.uncompressed_size) | \(.compression) |"' "$info" 2>/dev/null || true
    echo ""
    comp_ratio=$(jq -r '.section_summary.ratio // empty' "$info" 2>/dev/null || true)
    if [ -n "$comp_ratio" ] && [ "$comp_ratio" != "null" ]; then
      echo "Overall compression ratio: $(awk "BEGIN { printf \"%.3f\", $comp_ratio }")"
      echo ""
    fi
  fi

  # ---- verification ----
  echo "#### Verification"
  echo ""
  verify_exit=$(cat "$WORK/${name}.verify.exit" 2>/dev/null || echo 1)
  if [ "$verify_exit" -eq 0 ]; then
    echo "All integrity checks **passed** (file, section, chunk)."
  else
    echo "**FAILED** (exit code $verify_exit)"
    if [ -f "$WORK/${name}.verify.stderr" ] && [ -s "$WORK/${name}.verify.stderr" ]; then
      echo ""
      echo '```'
      cat "$WORK/${name}.verify.stderr"
      echo '```'
    fi
  fi
  echo ""

  # ---- notes / warnings ----
  warnings=""
  # Check for suspect ratio.
  if [ -n "$ratio" ] && [ "$ratio" != "null" ]; then
    is_large=$(awk "BEGIN { print ($ratio > 1.5) }")
    [ "$is_large" = "1" ] && warnings="${warnings}- :warning: OZA is >50% larger than ZIM (ratio $ratio)\n"
  fi
  # Check stderr for warnings.
  if [ -f "$WORK/${name}.convert.stderr" ] && [ -s "$WORK/${name}.convert.stderr" ]; then
    stderr_warns=$(grep -iE 'warn|error|unsupported|corrupt|invalid|failed|panic' "$WORK/${name}.convert.stderr" 2>/dev/null || true)
    if [ -n "$stderr_warns" ]; then
      warnings="${warnings}- Conversion stderr:\n\`\`\`\n${stderr_warns}\n\`\`\`\n"
    fi
  fi
  # Check metadata mismatches.
  if [ -f "$cmp" ] && [ -s "$cmp" ]; then
    mismatch_count=$(jq -r '[.metadata_match[]? | select(.match == false)] | length' "$cmp" 2>/dev/null || echo 0)
    [ "$mismatch_count" -gt 0 ] && warnings="${warnings}- :warning: $mismatch_count metadata key(s) differ between ZIM and OZA\n"
    mime_delta_count=$(jq -r '[.mime_count_match[]? | select(.delta != 0)] | length' "$cmp" 2>/dev/null || echo 0)
    [ "$mime_delta_count" -gt 0 ] && warnings="${warnings}- :warning: $mime_delta_count MIME type count(s) differ between ZIM and OZA\n"
  fi

  if [ -n "$warnings" ]; then
    echo "#### Notes"
    echo ""
    printf '%b' "$warnings"
    echo ""
  fi

  echo "---"
  echo ""
done < "$NAMES"

# ---- errors & warnings (aggregate) ----

echo "## Errors & Warnings"
echo ""

# Conversion failures.
echo "### Conversion Failures"
echo ""
if [ -s "$ERRORS" ]; then
  while IFS= read -r name; do
    echo "- **$name**: exit code $(cat "$WORK/${name}.convert.exit")"
    if [ -f "$WORK/${name}.convert.stderr" ] && [ -s "$WORK/${name}.convert.stderr" ]; then
      echo '  ```'
      sed 's/^/  /' "$WORK/${name}.convert.stderr"
      echo '  ```'
    fi
  done < "$ERRORS"
else
  echo "_None._"
fi
echo ""

# Verification failures.
echo "### Verification Failures"
echo ""
vfy_fail=0
while IFS= read -r name; do
  verify_exit=$(cat "$WORK/${name}.verify.exit" 2>/dev/null || echo 1)
  conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)
  [ "$conv_exit" -ne 0 ] && continue
  if [ "$verify_exit" -ne 0 ]; then
    vfy_fail=1
    echo "- **$name**: exit code $verify_exit"
  fi
done < "$NAMES"
[ "$vfy_fail" -eq 0 ] && echo "_None._"
echo ""

# All metadata mismatches.
echo "### Metadata Mismatches"
echo ""
meta_found=0
while IFS= read -r name; do
  conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)
  [ "$conv_exit" -ne 0 ] && continue
  cmp="$WORK/${name}.cmp.json"
  [ -f "$cmp" ] && [ -s "$cmp" ] || continue
  mismatches=$(jq -r '.metadata_match[]? | select(.match == false) | "\(.key)\t\(.zim_value // "—")\t\(.oza_value // "—")"' "$cmp" 2>/dev/null || true)
  if [ -n "$mismatches" ]; then
    if [ "$meta_found" -eq 0 ]; then
      echo "| File | Key | ZIM Value | OZA Value |"
      echo "|------|-----|-----------|-----------|"
      meta_found=1
    fi
    while IFS=$'\t' read -r key zv ov; do
      echo "| $name | $key | $zv | $ov |"
    done <<< "$mismatches"
  fi
done < "$NAMES"
[ "$meta_found" -eq 0 ] && echo "_None._"
echo ""

# All MIME discrepancies.
echo "### MIME Count Discrepancies"
echo ""
mime_found=0
while IFS= read -r name; do
  conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)
  [ "$conv_exit" -ne 0 ] && continue
  cmp="$WORK/${name}.cmp.json"
  [ -f "$cmp" ] && [ -s "$cmp" ] || continue
  deltas=$(jq -r '.mime_count_match[]? | select(.delta != 0) | "\(.mime_type)\t\(.zim_count)\t\(.oza_count)\t\(.delta)"' "$cmp" 2>/dev/null || true)
  if [ -n "$deltas" ]; then
    if [ "$mime_found" -eq 0 ]; then
      echo "| File | MIME Type | ZIM | OZA | Delta |"
      echo "|------|----------|----:|----:|------:|"
      mime_found=1
    fi
    while IFS=$'\t' read -r mt zc oc d; do
      echo "| $name | $mt | $zc | $oc | $d |"
    done <<< "$deltas"
  fi
done < "$NAMES"
[ "$mime_found" -eq 0 ] && echo "_None._"
echo ""

# Skipped files.
echo "### Skipped Files"
echo ""
if [ -s "$SKIPPED" ]; then
  echo "Below ${MIN_SIZE}-byte threshold:"
  echo ""
  while IFS= read -r line; do
    echo "- $line"
  done < "$SKIPPED"
else
  echo "_None._"
fi
echo ""

# Conversion warnings (stderr lines).
echo "### Conversion Warnings"
echo ""
warn_found=0
while IFS= read -r name; do
  conv_exit=$(cat "$WORK/${name}.convert.exit" 2>/dev/null || echo 1)
  [ "$conv_exit" -ne 0 ] && continue
  if [ -f "$WORK/${name}.convert.stderr" ] && [ -s "$WORK/${name}.convert.stderr" ]; then
    warns=$(grep -iE 'warn|error|unsupported|corrupt|invalid|failed|panic' "$WORK/${name}.convert.stderr" 2>/dev/null || true)
    if [ -n "$warns" ]; then
      warn_found=1
      echo "**$name:**"
      echo '```'
      echo "$warns"
      echo '```'
      echo ""
    fi
  fi
done < "$NAMES"
[ "$warn_found" -eq 0 ] && echo "_None._"
echo ""

echo "---"
echo ""
echo "_Report generated by \`scripts/bench-report.sh\`._"

echo "Done." >&2

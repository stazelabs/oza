#!/usr/bin/env bash
# bench-classify.sh — Compare default vs --auto classifier conversion results.
# Converts each Tier 1 ZIM twice, compares output size, and measures verify-all time.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN="$ROOT/bin"
TESTDATA="$ROOT/testdata"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Build fresh binaries.
echo "Building binaries..."
cd "$ROOT" && make build 2>/dev/null

# Skip tiny files that don't give meaningful results.
MIN_SIZE=500000  # 500KB

echo
printf "%-22s %7s | %9s %9s %7s | %8s %8s %7s\n" \
  "FILE" "PROFILE" "DEF_SIZE" "AUTO_SIZE" "DELTA" "DEF_VFY" "AUTO_VFY" "DELTA"
printf "%s\n" "$(printf '=%.0s' {1..105})"

for zim in "$TESTDATA"/*.zim; do
  name=$(basename "$zim" .zim)
  size=$(stat -f%z "$zim" 2>/dev/null || stat -c%s "$zim" 2>/dev/null)
  if [ "$size" -lt "$MIN_SIZE" ]; then
    continue
  fi

  def_oza="$TMPDIR/${name}_default.oza"
  auto_oza="$TMPDIR/${name}_auto.oza"

  # Get the auto-detected profile.
  profile=$("$BIN/zim2oza" --auto --dry-run --verbose "$zim" /dev/null 2>&1 \
    | grep -oP 'Auto-detected profile: \K\S+' || echo "n/a")

  # Convert with defaults.
  "$BIN/zim2oza" "$zim" "$def_oza" 2>/dev/null

  # Convert with --auto.
  "$BIN/zim2oza" --auto "$zim" "$auto_oza" 2>/dev/null

  # Measure output sizes.
  def_size=$(stat -f%z "$def_oza" 2>/dev/null || stat -c%s "$def_oza" 2>/dev/null)
  auto_size=$(stat -f%z "$auto_oza" 2>/dev/null || stat -c%s "$auto_oza" 2>/dev/null)
  if [ "$def_size" -gt 0 ]; then
    size_delta=$(python3 -c "print('%.1f%%' % (($auto_size - $def_size) / $def_size * 100))")
  else
    size_delta="n/a"
  fi

  # Measure verify-all time (3 runs, take median).
  measure_verify() {
    local oza=$1
    local times=()
    for i in 1 2 3; do
      t=$( { time "$BIN/ozaverify" --all "$oza" >/dev/null 2>&1; } 2>&1 \
        | grep real | sed 's/real[[:space:]]*//' )
      # Convert to milliseconds.
      ms=$(python3 -c "
import re
m = re.match(r'(\d+)m([\d.]+)s', '$t')
if m:
    print(int(float(m.group(1))*60000 + float(m.group(2))*1000))
else:
    print(0)
")
      times+=("$ms")
    done
    # Sort and take median.
    IFS=$'\n' sorted=($(sort -n <<<"${times[*]}")); unset IFS
    echo "${sorted[1]}"
  }

  def_ms=$(measure_verify "$def_oza")
  auto_ms=$(measure_verify "$auto_oza")
  if [ "$def_ms" -gt 0 ]; then
    vfy_delta=$(python3 -c "print('%.1f%%' % (($auto_ms - $def_ms) / $def_ms * 100))")
  else
    vfy_delta="n/a"
  fi

  # Format sizes.
  fmt_size() {
    python3 -c "
s=$1
if s >= 1048576: print('%.1fM' % (s/1048576))
elif s >= 1024: print('%.1fK' % (s/1024))
else: print('%dB' % s)
"
  }

  printf "%-22s %7s | %9s %9s %7s | %6dms %6dms %7s\n" \
    "$name" "$profile" \
    "$(fmt_size $def_size)" "$(fmt_size $auto_size)" "$size_delta" \
    "$def_ms" "$auto_ms" "$vfy_delta"
done

echo
echo "Negative DELTA = auto is smaller/faster. Positive = auto is larger/slower."

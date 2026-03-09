#!/usr/bin/env bash
set -euo pipefail

MIRROR=/Users/cstaszak/Stazelabs/projects/open-access-kit/mirror
TESTDATA=$(dirname "$0")/../testdata
ZIM2OZA=$(dirname "$0")/../zim2oza

mkdir -p "$TESTDATA"

find "$MIRROR" -name "*.zim" | while read -r zim; do
    base=$(basename "$zim" .zim)
    out="$TESTDATA/${base}.oza"
    if [[ -f "$out" ]]; then
        echo "SKIP $base (already exists)"
        continue
    fi
    echo "Converting $base..."
    "$ZIM2OZA" --verbose "$zim" "$out"
done

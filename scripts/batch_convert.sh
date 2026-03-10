#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
ZIM2OZA="${ZIM2OZA:-$SCRIPT_DIR/../bin/zim2oza}"

usage() {
    echo "Usage: $0 [OPTIONS] <input-dir> <output-dir>"
    echo ""
    echo "Convert all .zim files in <input-dir> to .oza files in <output-dir>."
    echo ""
    echo "Options:"
    echo "  -f, --force     Overwrite existing .oza files"
    echo "  -v, --verbose   Pass --verbose to zim2oza"
    echo "  -h, --help      Show this help"
    echo ""
    echo "Environment:"
    echo "  ZIM2OZA   Path to the zim2oza binary (default: ../zim2oza relative to this script)"
    exit 1
}

FORCE=0
VERBOSE=0

while [[ $# -gt 0 ]]; do
    case "$1" in
        -f|--force)   FORCE=1; shift ;;
        -v|--verbose) VERBOSE=1; shift ;;
        -h|--help)    usage ;;
        -*)           echo "Unknown option: $1" >&2; usage ;;
        *)            break ;;
    esac
done

if [[ $# -ne 2 ]]; then
    echo "Error: expected <input-dir> and <output-dir>" >&2
    usage
fi

INPUT_DIR="$1"
OUTPUT_DIR="$2"

if [[ ! -d "$INPUT_DIR" ]]; then
    echo "Error: input directory does not exist: $INPUT_DIR" >&2
    exit 1
fi

if [[ ! -x "$ZIM2OZA" ]]; then
    echo "Error: zim2oza binary not found or not executable: $ZIM2OZA" >&2
    exit 1
fi

mkdir -p "$OUTPUT_DIR"

ZIM2OZA_ARGS=()
[[ $VERBOSE -eq 1 ]] && ZIM2OZA_ARGS+=(--verbose)

CONVERTED=0
SKIPPED=0
FAILED=0

while IFS= read -r -d '' zim; do
    base=$(basename "$zim" .zim)
    out="$OUTPUT_DIR/${base}.oza"

    if [[ -f "$out" && $FORCE -eq 0 ]]; then
        echo "SKIP $base (already exists)"
        ((SKIPPED++)) || true
        continue
    fi

    echo "Converting $base..."
    if "$ZIM2OZA" ${ZIM2OZA_ARGS[@]+"${ZIM2OZA_ARGS[@]}"} "$zim" "$out"; then
        ((CONVERTED++)) || true
    else
        echo "FAIL $base" >&2
        ((FAILED++)) || true
    fi
done < <(find "$INPUT_DIR" -name "*.zim" -print0 | sort -z)

echo ""
echo "Done: $CONVERTED converted, $SKIPPED skipped, $FAILED failed."
[[ $FAILED -eq 0 ]]

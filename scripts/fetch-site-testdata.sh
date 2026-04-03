#!/usr/bin/env bash
# fetch-site-testdata.sh — download Tier 1 static site test fixtures (~30 MB).
# Idempotent: skips repos that already exist. Safe to re-run.
#
# These are small, well-known documentation repos used for site2oza
# integration tests. Each has an MCP use case story:
#
#   react.dev       — "When should I use useMemo vs useCallback?"
#   gobyexample     — "Show me goroutine syntax"
#   go-doc          — "What does sync.Pool do?"
#
# See docs/LANDSCAPE.md for the full MCP-driven converter rationale.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
DIR="$REPO/testdata/sites"
mkdir -p "$DIR"
FAILED=0

clone_sparse() {
  local name="$1" url="$2" subdir="$3"
  local dest="$DIR/$name"
  if [ -d "$dest" ]; then
    printf "EXISTS %s\n" "$name"
    return
  fi
  printf "FETCH  %s (sparse: %s)\n" "$name" "$subdir"
  if git clone --depth 1 --filter=blob:none --sparse "$url" "$dest.tmp" 2>/dev/null; then
    cd "$dest.tmp"
    git sparse-checkout set "$subdir" 2>/dev/null
    cd "$REPO"
    # Move the subdirectory content up to the target.
    if [ -d "$dest.tmp/$subdir" ]; then
      mv "$dest.tmp/$subdir" "$dest"
      rm -rf "$dest.tmp"
    else
      mv "$dest.tmp" "$dest"
    fi
  else
    rm -rf "$dest.tmp"
    printf "WARN   %s clone failed, skipping\n" "$name" >&2
    FAILED=$((FAILED + 1))
  fi
}

clone_full() {
  local name="$1" url="$2"
  local dest="$DIR/$name"
  if [ -d "$dest" ]; then
    printf "EXISTS %s\n" "$name"
    return
  fi
  printf "FETCH  %s\n" "$name"
  if git clone --depth 1 "$url" "$dest.tmp" 2>/dev/null; then
    mv "$dest.tmp" "$dest"
  else
    rm -rf "$dest.tmp"
    printf "WARN   %s clone failed, skipping\n" "$name" >&2
    FAILED=$((FAILED + 1))
  fi
}

# --- React docs (~15 MB of Markdown) ---
# MCP story: "When should I use useMemo vs useCallback?"
clone_sparse react.dev https://github.com/reactjs/react.dev.git src/content

# --- Go by Example (~2 MB of HTML) ---
# MCP story: "Show me goroutine channel syntax"
clone_sparse gobyexample https://github.com/mmcgrana/gobyexample.git public

# --- Go documentation (~5 MB mixed HTML+MD) ---
# MCP story: "What does sync.Pool do?"
clone_sparse go-doc https://github.com/golang/go.git doc

echo ""
if [ "$FAILED" -gt 0 ]; then
  echo "Done. $FAILED repo(s) failed to download — tests using them will be skipped."
  exit 0  # non-fatal: tests skip missing fixtures
else
  echo "Done. All Tier 1 site fixtures downloaded to testdata/sites/."
fi

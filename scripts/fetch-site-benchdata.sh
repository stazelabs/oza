#!/usr/bin/env bash
# fetch-site-benchdata.sh — download Tier 2 documentation repos for benchmarks (~800 MB).
# Idempotent: skips repos that already exist. Safe to re-run.
#
# These are large, real-world documentation corpora used for:
#   1. Performance benchmarking (entries/sec, compression ratio, memory)
#   2. MCP round-trip validation (search quality on real content)
#
# MCP use case stories:
#   mdn-content       — "What arguments does fetch() take?" (web reference)
#   kubernetes-docs    — "Write a CronJob manifest" (K8s reference)
#   terraform-aws      — "Configure an RDS with read replicas" (infra reference)
#
# Run with: make site-benchdata
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
  printf "FETCH  %s (sparse: %s) — this may take a while...\n" "$name" "$subdir"
  if git clone --depth 1 --filter=blob:none --sparse "$url" "$dest.tmp" 2>/dev/null; then
    cd "$dest.tmp"
    git sparse-checkout set "$subdir" 2>/dev/null
    cd "$REPO"
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

# --- MDN Web Docs (~500 MB, ~12k MD files) ---
# The definitive web reference. Largest corpus — stress tests everything.
# MCP story: "What arguments does fetch() take?"
clone_sparse mdn-content https://github.com/mdn/content.git files/en-us

# --- Kubernetes website (~150 MB, ~1.5k MD files) ---
# Infrastructure documentation with YAML examples.
# MCP story: "Write a CronJob manifest"
clone_sparse kubernetes-docs https://github.com/kubernetes/website.git content/en/docs

# --- Terraform AWS Provider (~100 MB, ~3k MD files) ---
# API-style reference docs — every AWS resource as a page.
# MCP story: "Configure an RDS instance with read replicas"
clone_sparse terraform-aws https://github.com/hashicorp/terraform-provider-aws.git website/docs

echo ""
if [ "$FAILED" -gt 0 ]; then
  echo "Done. $FAILED repo(s) failed to download — benchmark tests using them will be skipped."
  exit 0
else
  echo "Done. All Tier 2 site benchdata downloaded to testdata/sites/."
fi

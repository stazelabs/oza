# Archived design docs

This directory holds forward-looking design documents and supplementary
narrative content that were once in `docs/` but no longer reflect the
canonical state of the project. They are preserved for context — most
are precursor designs for workstreams now tracked in [Linear](https://linear.app/) (team: **OZA**).

The canonical format specification is [`../SPEC.md`](../SPEC.md). The
public API reference is the package documentation on
[pkg.go.dev](https://pkg.go.dev/github.com/stazelabs/oza/oza) and the
writer-specific [`../OZAWRITE.md`](../OZAWRITE.md).

## Contents

| Doc | Why archived | Successor |
|-----|--------------|-----------|
| [INCREMENTAL.md](INCREMENTAL.md) | Multi-phase design for chunk-level reuse during archive rebuilds. Not implemented in v0.1; the design is sound and broken into a 4-ticket epic. | Linear tickets (OZA team, "Incremental update" theme) |
| [PLAIN_TEXT.md](PLAIN_TEXT.md) | Design for the `PLAIN_TEXT` (0x0101) section: HTML→markdown extraction with passage segmentation. Not implemented in v0.1. | Linear tickets (OZA team, "PLAIN_TEXT & AI sections" theme) |
| [LLM.md](LLM.md) | Vision document positioning OZA as "AI ecosystem native": reserved section types 0x0100–0x0106, MCP tool roadmap, header/entry-flag reservations. ~95% aspirational; describes capabilities OZA does not yet have. | Linear tickets (OZA team, multiple themes); selected reservations may land in SPEC.md once implemented |
| [INDICES.md](INDICES.md) | Design rationale for the split title/body search architecture. The wire format described here is now canonical in SPEC.md §4; this doc kept the design "why" for future readers. | SPEC.md §4 (wire format); this doc for rationale |
| [EMBEDDINGS.md](EMBEDDINGS.md) | Deep-dive narrative on Zstd compression dictionaries (Part 1, implemented) and vector embeddings (Parts 2-12, designed but not implemented). Mixed-status content; archived whole pending a focused rewrite. | SPEC.md §3.10 + OZAWRITE.md dictionaries section (Part 1); Linear tickets (Part 2-12, "Vector embeddings & semantic search" theme) |

## Status

These files are read-only by convention — they document past intent, not
future commitments. When a workstream from one of them starts, the
implementer should:

1. File or claim the corresponding Linear ticket.
2. If the design has changed, capture the new design in SPEC.md or the
   relevant active doc — do not edit the archived file.
3. Once shipped, leave the archived doc in place as a historical
   record; add a one-line "Implemented in: <ticket>" note at the top.

The archive is not a draft folder; it is a graveyard with names.

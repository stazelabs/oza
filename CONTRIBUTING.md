# Contributing to OZA

**Status:** Pre-v1 prototype. Breaking changes are free — no backward-compatibility
obligation yet. The format spec and API are still being locked down.

---

## Development setup

**Requirements:** Go 1.24+, golangci-lint, pre-commit (optional but recommended).

```sh
# Clone and verify
git clone https://github.com/stazelabs/oza
cd oza

# Set up Go workspace (enables cross-module navigation and single go test ./...)
go work init . ./cmd

# Run tests for both modules
make test

# Install golangci-lint (required for make lint)
brew install golangci-lint          # macOS
# or: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Install pre-commit hooks (optional, runs gofmt + go vet + lint on commit)
brew install pre-commit
pre-commit install
```

## Common tasks

| Command | What it does |
|---------|--------------|
| `make test` | Run all tests |
| `make test-race` | Run tests with race detector |
| `make cover` | Coverage report (per-function summary) |
| `make cover-html` | Coverage report (browser HTML) |
| `make lint` | Run golangci-lint |
| `make lint-fix` | Run golangci-lint with auto-fix |
| `make vet` | Run go vet |
| `make build` | Build all binaries into `bin/` |
| `make bench` | Run benchmarks (`oza` and `ozawrite` packages) |
| `make fuzz` | Run all fuzz targets (30s each) |
| `make bench-convert ZIM=...` | Convert a ZIM file and verify the result |
| `make snapshot` | Build release archives locally (requires goreleaser) |

## Releasing

The repository contains two Go modules:

| Module | Path | Contents |
|--------|------|----------|
| `github.com/stazelabs/oza` | root | Library packages (`oza/`, `ozawrite/`) |
| `github.com/stazelabs/oza/cmd` | `cmd/` | CLI tools and internal packages |

**Library releases** are triggered by pushing a `v*` tag. **CLI binary releases** are triggered by pushing a `cmd/v*` tag.

```sh
# Library release
git tag v0.x.0
git push origin v0.x.0

# CLI binary release (after updating cmd/go.mod to point at the library tag)
git tag cmd/v0.x.0
git push origin cmd/v0.x.0
```

The GitHub Actions release workflow runs tests for both modules, then builds cross-compiled binaries for Linux, macOS, and Windows (amd64 + arm64) and publishes a GitHub Release with archives and a `checksums.txt`.

**Local snapshot build** (no tag required, no GitHub push):

```sh
# Install goreleaser (required once)
brew install goreleaser

make snapshot   # builds into dist/
```

Prereleases are auto-detected from the tag: tags matching `v*-*` (e.g. `v0.1.0-rc1`) are published as pre-releases.

## Code layout

```
oza/              — reader library (Archive, Entry, Index, Search)
ozawrite/         — writer library (Writer, pipeline, assembly)
cmd/              — CLI module (separate go.mod)
  internal/       — shared packages not part of the public API
    mcptools/     — MCP tool/resource registration shared by ozamcp and ozaserve
    snippet/      — snippet extraction utilities
  ozacat/         — cat/list/inspect entries from the CLI
  ozainfo/        — print archive metadata and section table
  ozasearch/      — full-text search from the CLI
  ozaserve/       — HTTP + MCP server
  ozaverify/      — verify file/section/chunk checksums
  ozamcp/         — standalone MCP server (stdio)
  ozakeygen/      — generate Ed25519 signing key pairs
  zim2oza/        — ZIM → OZA converter
docs/
  FORMAT.md       — binary format specification
  BACKLOG.md      — known issues, design observations, future work
```

## Before submitting a PR

1. `make test-race` must pass.
2. `make lint` must exit 0 (or new findings must be addressed with a rationale).
3. Format changes that touch the on-disk binary layout must update `docs/FORMAT.md`.
4. New public API must have godoc comments.

There is no CLA. Contributions are accepted under the same license as the project
(see `LICENSE`).

## Spec changes

The format spec lives in `docs/FORMAT.md`. Any change that affects the wire format
must be discussed before implementation — even in pre-v1, byte-level compatibility
between the spec and the Go implementation is a hard requirement. Open an issue or
start a discussion before sending a spec PR.

## Reporting issues

Use [GitHub Issues](https://github.com/stazelabs/oza/issues). For security issues,
email the maintainers directly rather than filing a public issue.

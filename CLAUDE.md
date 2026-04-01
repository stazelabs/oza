# CLAUDE.md -- OZA

## What is this?

OZA is a Go library and CLI toolset for reading/writing OZA (Open Zipped Archive) files -- a modern replacement for the ZIM format. Pure Go, no CGo.

- Module: `github.com/stazelabs/oza`
- Go 1.24+
- Pre-v1: breaking changes are free

## Repo structure

Multi-module repo with a `replace` directive:

| Module | Path | What |
|--------|------|------|
| `github.com/stazelabs/oza` | root (`go.mod`) | Library: `oza/` (reader), `ozawrite/` (writer) |
| `github.com/stazelabs/oza/cmd` | `cmd/go.mod` | CLI tools + internal packages |

Use `go work init . ./cmd` for cross-module navigation in an IDE.

## Build & test

```sh
make test          # both modules, -count=1
make test-race     # with race detector
make lint          # golangci-lint (both modules)
make build         # all binaries -> bin/
make bench         # benchmarks for oza/ and ozawrite/
make fuzz          # all fuzz targets, 30s each
make cover         # coverage report
```

Tests must pass in both modules -- Makefile targets `cd cmd &&` for the second module.

## Key conventions

- **Errors:** return `error`, use sentinel errors from `oza/errors.go`, prefix `oza:`. No panics in library code.
- **Binary format:** all integers little-endian, all strings UTF-8 NFC-normalized.
- **Test naming:** `Test<FunctionName>` or `Test<FunctionName><Scenario>`.
- **CLI banner:** `王座 <toolname> v<version>`.
- **Dependencies are minimal.** Reader has zero CLI deps. Writer imports reader, never the reverse.

## Format spec

The binary format spec lives in `docs/FORMAT.md`. Any change to on-disk layout **must** update that file. Byte-level spec/implementation agreement is a hard requirement even pre-v1.

## Releasing

- Library: push `v*` tag
- CLI binaries: push `cmd/v*` tag (after updating `cmd/go.mod` to point at the library tag)
- Prereleases: `v*-*` tags (e.g. `v0.1.0-rc1`)

## PR checklist

1. `make test-race` passes
2. `make lint` exits 0
3. Format changes update `docs/FORMAT.md`
4. New public API has godoc comments

# 王座 OZA -- Backlog

Outstanding tasks and known issues for future work.

## CLI

### ozaclean

A CLI tool to "clean" a OZA file. Offers the opportunity to optimize further, update to a new format spec (whenever that arrives), etc.

## Compression

### Dictionary training can panic

`zstd.BuildDict()` can panic on certain inputs ("can only encode up to 64K
sequences"). This is caught by a `defer/recover` in `trainDictionary()`, but
upstream fixes would be better. Track klauspost/compress issues.

## Features

### Split Archives

Evalute and consider along the lines of https://wiki.openzim.org/wiki/ZIM_file_format#Split_ZIM_archives_in_chunks

### Chrome section

Implement the optional CHROME section (§7.2 in FORMAT.md):

- `ozawrite/chrome.go` — `AddChromeAsset(role, name, data)` method on Writer.
- `oza/chrome.go` — parse chrome section, enumerate assets by role.
- `cmd/zim2oza/chrome.go` — extract `C/_mw_/` CSS/JS from ZIM into chrome
  section instead of skipping them.

Currently `categoryChrome` exists in the converter but is treated as skip.
ZIM entries under `C/_mw_/` are MediaWiki chrome — these could be stored in
the dedicated section or stripped entirely depending on use case.

Wire format: `uint32 asset_count`, per asset: `uint16 role` (0=stylesheet,
1=script, 2=template, 3=icon, 4=font), `uint16 name_length`, name bytes,
`uint32 data_length`, data bytes.

### Image format conversion (PNG→WebP)

Lossless PNG→WebP conversion yields 25-35% savings per image. Requires CGo
(`libwebp`). Lossy JPEG→WebP at quality 80 saves 25-35% more. This is the
single highest-impact size optimization available but blocked on CGo decision.

Could be exposed as `--recompress-images` (lossless) and `--lossy-images`
(with quality parameter) flags on `zim2oza`.

### Incremental / append mode

Currently the Writer must produce a complete archive in one shot. An incremental
mode that can append entries to an existing archive would be useful for large
corpus updates.

## Correctness

### Minification can change semantics

HTML minification (tdewolff) removes whitespace that may be significant in
`<pre>` blocks or inline formatting. CSS/JS minification is generally safe but
may break code that relies on `toString()` or source-level introspection. There
is no per-entry opt-out — only global `--no-minify-html` etc.

### PNG re-encode can change pixel data

The Go `image/png` decoder + re-encoder round-trip is not guaranteed to be
bit-exact for all valid PNGs (e.g., 16-bit color depth, unusual chunk ordering).
The optimization is skipped if the re-encoded file is larger, but there's no
pixel-level verification.

## Testing

### No integration test for large archives

The test suite uses small synthetic archives. There's no automated test that
converts a real ZIM file and verifies the output with `ozaverify --all`. The
Makefile has `bench-convert-large` for manual runs.


## CI / CD

### GitHub Actions

Set up CI with:

- `go test -race ./...` on Linux, macOS, Windows
- `go vet ./...`
- Fuzz tests (short duration, e.g. 30s per target)
- Build all binaries (`zim2oza`, `ozainfo`, `ozaserve`, etc.)

### Release

- Update README with final API docs and performance numbers
- Tag v0.1.0
- Create GitHub release with prebuilt binaries for Linux/macOS/Windows

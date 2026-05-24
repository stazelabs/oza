# Plan: Homebrew Distribution for OZA CLI Tools

## Context

OZA has 10 CLI tools built with GoReleaser but no Homebrew distribution. GoReleaser already produces cross-platform archives (darwin/linux/windows x amd64/arm64). The goal is to let users install all tools with `brew tap stazelabs/oza && brew install oza`.

## Approach: GoReleaser `brews` section + Homebrew Tap repo

GoReleaser has built-in Homebrew support — it auto-generates a formula and pushes it to a tap repo on each release. This is the standard approach for Go CLI projects and requires minimal ongoing maintenance.

## Status

Steps 1-4 are done. Step 5 (manual GitHub setup) remains before the first release.

## Implementation Steps

### 1. Add missing builds to `.goreleaser.yaml` -- DONE

Two tools were missing from goreleaser config: `ozacmp` and `epub2oza`. Added following the existing pattern.

### 2. Add ldflags for version stamping -- DONE

`zim2oza` and `epub2oza` have `var Version = "dev"` in `version.go`. Added ldflags to their goreleaser build entries so the release version is injected:

```yaml
ldflags:
  - -s -w -X main.Version={{.Version}}
```

### 3. Add `brews` section to `.goreleaser.yaml` -- DONE

```yaml
brews:
  - name: oza
    repository:
      owner: stazelabs
      name: homebrew-oza
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    commit_author:
      name: stazelabs-bot
      email: bot@stazelabs.com
    directory: Formula
    homepage: "https://github.com/stazelabs/oza"
    description: "OZA format tools -- read, write, search, serve, and convert OZA archives"
    license: "Apache-2.0"
    install: |
      bin.install "ozacat"
      bin.install "ozainfo"
      bin.install "ozasearch"
      bin.install "ozaserve"
      bin.install "ozaverify"
      bin.install "ozamcp"
      bin.install "ozakeygen"
      bin.install "ozacmp"
      bin.install "zim2oza"
      bin.install "epub2oza"
    test: |
      system "#{bin}/ozacat", "--help"
      system "#{bin}/ozainfo", "--help"
```

### 4. Pass tap token in release workflow -- DONE

In `.github/workflows/release.yml`, added `HOMEBREW_TAP_TOKEN` env var to the GoReleaser step:

```yaml
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
```

### 5. Manual setup (outside this repo) -- TODO

These are manual steps the repo owner must do before the first release:

1. **Create `stazelabs/homebrew-oza`** public repo on GitHub (can use `gh repo create stazelabs/homebrew-oza --public`)
2. **Create a fine-grained GitHub PAT** with `Contents: write` scope on `stazelabs/homebrew-oza`
3. **Add the PAT** as repo secret `HOMEBREW_TAP_TOKEN` in `stazelabs/oza`

## Files to Modify

| File | Change |
|------|--------|
| `.goreleaser.yaml` | Add `ozacmp` + `epub2oza` builds, add `ldflags` to zim2oza/epub2oza, add `brews` section |
| `.github/workflows/release.yml` | Add `HOMEBREW_TAP_TOKEN` env var |

## Tag Format Consideration

The release workflow triggers on `cmd/v*` tags. GoReleaser should handle stripping the `cmd/` prefix to extract the version. This needs to be verified with `goreleaser release --snapshot --clean` locally before the first real release.

## Verification

1. Run `goreleaser release --snapshot --clean` locally to verify all 10 binaries build and the formula is generated correctly (check `dist/`)
2. After first real release (`git tag cmd/v0.x.0 && git push origin cmd/v0.x.0`):
   - Verify formula appears in `stazelabs/homebrew-oza/Formula/oza.rb`
   - `brew tap stazelabs/oza` succeeds
   - `brew install oza` installs all 10 binaries
   - `zim2oza --version` reports correct version (not "dev")
   - Each tool runs: `ozacat --help`, `ozainfo --help`, etc.

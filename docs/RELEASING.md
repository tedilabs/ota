# Releasing ota

Multi-arch build + distribution pipeline. Single source of truth is
[`.goreleaser.yaml`](../.goreleaser.yaml); CI lives in
[`.github/workflows/release.yml`](../.github/workflows/release.yml).

## What ships

| Channel | Artifact | Tagged as |
|---|---|---|
| GitHub Releases | `ota_<version>_<os>_<arch>.tar.gz` (+`.zip` for windows) + `checksums.txt` + SBOMs | `v<X.Y.Z>` |
| Homebrew tap | `Casks/ota.rb` formula in `tedilabs/homebrew-tap` | latest only |
| GHCR | `ghcr.io/tedilabs/ota:<X.Y.Z>` + `ghcr.io/tedilabs/ota:latest` (multi-arch manifest list) | both |
| `install.sh` | served from `main`; downloads the matching GitHub Release tarball + verifies checksums | latest by default, `--version vX.Y.Z` to pin |

Target architectures:

- `darwin/arm64`, `darwin/amd64` (plus a `darwin_all` universal binary
  for Homebrew + direct download convenience)
- `linux/arm64`, `linux/amd64`
- `windows/amd64`

Version metadata (Tag / Commit / BuildTime) is injected into
`internal/version` via ldflags — confirm with `ota --version`.

## Triggering a release

Two paths, both wired in `.github/workflows/release.yml`:

### Tag push (normal flow)

```sh
# bump CHANGELOG / docs, commit, then:
git tag -a v0.3.0 -m "ota v0.3.0"
git push origin v0.3.0
```

The `release` workflow fires on `tags: ['v*']`, runs GoReleaser, and
publishes everything in one job. CI also auto-generates release notes
from conventional-commit messages (`feat:` / `fix:` / `perf:` /
`refactor:` are bucketed into sections; `docs:` / `test:` / `ci:` /
`chore:` are filtered out).

### Manual (`workflow_dispatch`)

Useful when a previous tag's release flaked partway through (GHCR push
failed, Homebrew tap PAT expired, etc.) and you want to re-run without
moving the tag.

GitHub UI → Actions → **release** → Run workflow → optionally pin a
`tag:` input (e.g. `v0.3.0`); leave blank to use the current ref.

## Required secrets

| Secret | Scope | Purpose |
|---|---|---|
| `GITHUB_TOKEN` | auto-injected | GitHub Releases + GHCR push |
| `HOMEBREW_TAP_GITHUB_TOKEN` | PAT, `repo` on `tedilabs/homebrew-tap` | Push the cask formula update |

The Homebrew tap repo (`tedilabs/homebrew-tap`) must exist before the
first tag. If it doesn't, comment out the `homebrew_casks:` block in
`.goreleaser.yaml` for that release.

## Local dry-run

Anything CI runs you can reproduce locally — useful for catching config
mistakes before pushing a tag.

```sh
# Validate the config without building anything.
goreleaser check

# Build only the host arch (fastest sanity check). Output → ./dist/
goreleaser build --single-target --snapshot --clean

# Full multi-arch build, no publish. Skip docker if the daemon isn't up.
goreleaser release --snapshot --clean --skip=publish

# Just the docker images (requires docker + buildx).
goreleaser release --snapshot --clean --skip=archive,checksum,homebrew_casks
```

GoReleaser detects "snapshot" mode and stamps a synthetic version
(`v0.0.1-snapshot-<sha>`), so local builds never accidentally collide
with a real release tag.

## End-user install paths

Document these in the README once the first release is live:

```sh
# Homebrew (macOS / Linux)
brew tap tedilabs/tap
brew install ota

# install.sh (macOS / Linux)
curl -fsSL https://raw.githubusercontent.com/tedilabs/ota/main/install.sh | sh

# Pin to a specific version
curl -fsSL https://raw.githubusercontent.com/tedilabs/ota/main/install.sh | sh -s -- --version v0.3.0

# Docker (any Linux)
docker run --rm -it -e OKTA_API_TOKEN=$OKTA_API_TOKEN ghcr.io/tedilabs/ota:latest

# Direct download
gh release download v0.3.0 -p 'ota_*_macos_arm64.tar.gz'
tar -xzf ota_*_macos_arm64.tar.gz
./ota --version
```

## When something goes wrong

| Symptom | Likely cause |
|---|---|
| `goreleaser check` fails with "DEPRECATED" | bump the YAML to the latest schema; see [deprecations](https://goreleaser.com/deprecations/) |
| GHCR push 401 | The release workflow forgot to `docker login ghcr.io`; check `release.yml` |
| Homebrew step 403 / 404 | `HOMEBREW_TAP_GITHUB_TOKEN` missing / expired, or the tap repo doesn't exist |
| `ota` won't open on macOS via brew install | Gatekeeper quarantine — the cask's `xattr -dr` post-install hook should clear it; verify the hook ran |
| Empty release notes | the changelog filter dropped every commit; loosen the `exclude:` rules in `.goreleaser.yaml` |

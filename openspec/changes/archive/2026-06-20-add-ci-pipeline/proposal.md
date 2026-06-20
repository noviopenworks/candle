## Why

The previous change (`add-project-tooling`) gave the project a pinned toolchain
and a `task ci` gate, but **nothing runs it automatically**. Pull requests and
pushes get no build/lint/test/vuln signal, and there is no way to ship release
binaries — the locked decisions call for `go install` **plus release binaries**.
This change wires the existing `mise` + `Task` setup into GitHub Actions so every
change is gated and tagged releases produce cross-platform binaries.

## What Changes

- Add **`.github/workflows/ci.yml`** — on push to `master` and on every PR, run
  the full gate on a **Linux + macOS** matrix via `jdx/mise-action@v2` (reads
  `mise.toml`, so versions match local dev): `task ci` then `task coverage`,
  uploading the coverage report as an artifact.
- Add **`.github/workflows/release.yml`** — on a `v*` tag, set up the same
  toolchain and run `task release` (GoReleaser) with `GITHUB_TOKEN`.
- Add **`.goreleaser.yml`** — build `cmd/candlegraph` → `candlegraph` for
  linux/darwin × amd64/arm64, `-trimpath`, stripped, with the version injected
  via ldflags; produce `.tar.gz` archives (incl. LICENSE + README), checksums,
  and a filtered changelog.
- Make the build **version injectable**: `internal/version` now holds a `var`
  overridden by GoReleaser's ldflags, so release binaries report the real tag
  version through the MCP handshake (`server.go` already reads `version.String()`).
- Ignore release/coverage artifacts (`/dist/`, `coverage.html`) in `.gitignore`.

## Capabilities

### Modified Capabilities
<!-- None. CI/CD automation, release packaging, and a one-line version-var change. -->

## Impact

- **New files:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`,
  `.goreleaser.yml`.
- **Code:** `internal/version/version.go` — `func`→`var`-backed value for ldflags
  injection (no behavior change; default unchanged, test still passes).
- **`.gitignore`:** add `/dist/`, `coverage.html`.
- **Verification:** `goreleaser check` valid; `task release-snapshot` builds all
  4 targets, archives, and checksums; `task ci` green; full test suite passes.

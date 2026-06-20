# Design: add-ci-pipeline

Tweak: wire the pinned `mise` + `Task` toolchain into GitHub Actions for CI
(gate every PR/push) and releases (cross-platform binaries on a tag).

## Choices considered

- **Toolchain in CI: `mise-action` vs. `setup-go`** — `jdx/mise-action@v2` reads
  the committed `mise.toml`, so CI uses the *exact* Go/golangci-lint/goreleaser/
  govulncheck versions as local dev. Avoids version drift between a separate
  `setup-go` matrix and the pinned tools. Chosen.
- **CI invocation: `task ci` vs. inlined steps** — calling `task ci` keeps the
  gate definition in one place (`Taskfile.yml`); the workflow is a thin wrapper,
  so changing the gate doesn't require editing YAML in two repos of truth.
- **Release tool: GoReleaser vs. hand-rolled `go build` matrix** — GoReleaser
  handles archives, checksums, changelog, and the GitHub release in one config;
  it is already pinned in `mise.toml`. Chosen.
- **OS matrix** — Linux + macOS per the locked decision. Release builds run on
  `ubuntu-latest` only (cross-compiles all four targets with `CGO_ENABLED=0`).

## Decisions

- **Version injection.** `internal/version` changed from a constant-returning
  function to a `var version` that GoReleaser overrides via
  `-X …/internal/version.version={{.Version}}`. `internal/mcp/server.go` already
  reports `version.String()` in the MCP handshake, so released binaries advertise
  their real tag version. Default stays `0.1.0-dev`; the existing test (non-empty)
  still passes. No CLI `--version` flag is added — out of scope, and the value is
  surfaced where it matters (the MCP server identity).
- **`CGO_ENABLED=0`.** The SQLite driver is pure-Go (`modernc.org/sqlite`), so
  static cross-compilation needs no C toolchain — simplest reproducible builds.
- **Changelog filters.** Exclude `docs:`/`test:`/`chore:`/`chore(comet):` and
  merge commits so release notes show user-facing changes only (the comet
  workflow produces many `chore(comet):` commits).
- **Permissions.** CI is `contents: read`; release is `contents: write` (needs to
  create the GitHub release and upload assets). Least privilege per workflow.
- **Concurrency.** CI cancels superseded in-flight runs per ref.

## Files

- `.github/workflows/ci.yml` — push(master)/PR; matrix `[ubuntu-latest,
  macos-latest]`; mise-action → `task ci` → `task coverage` → upload artifact.
- `.github/workflows/release.yml` — tag `v*`; mise-action → `task release`.
- `.goreleaser.yml` — v2 config; builds, archives, checksum, changelog, release.
- `internal/version/version.go` — injectable `var`.
- `.gitignore` — `/dist/`, `coverage.html`.

## Verification

`goreleaser check` (config valid), `task release-snapshot` (4 targets + archives
+ checksums built locally), `task ci` green, `go test ./...` 116 pass. Workflow
YAML is exercised on the first PR/tag after merge.

## Out of scope

Installation docs/README install section is the next change
(`add-installation-docs`).

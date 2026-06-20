## Why

The project has **no pinned toolchain and no task runner**. Contributors must
guess which Go, linter, and release-tool versions to use, and there is no single
command that runs the same checks CI will run. Lint was also never wired up: a
first `golangci-lint` pass surfaced real findings (a blank import without
justification, an unused return value, wrapped-error comparisons that bypass
`errors.Is`). Before adding CI (next change) the project needs a reproducible,
pinned developer toolchain and a canonical set of tasks that CI can call.

## What Changes

- Add **`mise.toml`** pinning Go, `golangci-lint`, `goreleaser`, and
  `govulncheck` — one source of truth for local dev and CI.
- Add **`.golangci.yml`** (golangci-lint v2 config) and fix the findings it
  surfaces so the lint gate is clean:
  - `internal/store/store.go` — justify the blank `modernc.org/sqlite` import.
  - `internal/config/config.go` — `RepoConfig.validate` returned an unused
    `RepoConfig`; collapse to `error` and update call sites/tests.
  - `internal/mcp/server.go` — replace `err == ErrNotFound` comparisons with
    `errors.Is(err, ErrNotFound)` (correctly handles wrapped errors).
- Add **`Taskfile.yml`** with `build, vet, test, fmt, fmt-check, lint, vuln,
  coverage, install, tidy, release, release-snapshot, ci`. `task ci` runs the
  full gate CI will invoke.
- Bump pinned Go **1.26.3 → 1.26.4** in `go.mod` and `mise.toml` to clear two
  Go standard-library vulnerabilities (`GO-2026-5039`, `GO-2026-5037`) that
  `govulncheck` reports as reachable.

## Capabilities

### Modified Capabilities
<!-- None. Developer tooling, lint hygiene, and a toolchain patch bump only. -->

## Impact

- **New files:** `mise.toml`, `.golangci.yml`, `Taskfile.yml`.
- **Code:** small lint-correctness fixes in `internal/store`, `internal/config`,
  `internal/mcp` (no behavior change; `errors.Is` is a correctness improvement).
- **Toolchain:** Go pin 1.26.3 → 1.26.4.
- **Verification:** `go build/vet/test` unchanged (116 tests pass);
  `task ci` (fmt-check, vet, lint, test, vuln) is green.

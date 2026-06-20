# Design: add-project-tooling

Tweak: introduce a pinned developer toolchain (`mise`), a task runner (`Task`),
and a lint config (`golangci-lint`), then fix the findings the new lint gate
surfaces so the gate is clean.

## Choices considered

- **Toolchain pinning: `mise` vs. asdf vs. nothing** — `mise` pins Go *and*
  Go-installed tools (golangci-lint, goreleaser, govulncheck) from one file and
  has a first-class GitHub Action (`jdx/mise-action`), so the next change (CI)
  reuses the same `mise.toml` with no drift. Selected by the user.
- **Task runner: `Task` vs. Make** — user chose `Task` (Taskfile.yml, no
  Makefile, no Make shim). YAML tasks are readable and cross-platform for the
  Linux + macOS CI matrix.
- **Lint config strictness** — golangci-lint v2 default linters plus `revive`,
  with `package-comments`/`unused-parameter` disabled and pragmatic excludes
  (deferred `Close`/`Rollback`, `fmt.Fprint*`, manifest-derived gosec `G304`,
  test-file `errcheck`/`errorlint`/`gosec`). Keeps signal high without churn.

## Decisions

- **`fmt`/`fmt-check` exclude `testdata`.** `internal/link/testdata` contains an
  intentionally malformed `broken.go` fixture; `gofmt` cannot parse it and exits
  non-zero. The tasks run `gofmt` over `find . -name '*.go' -not -path
  '*/testdata/*'` so the format gate ignores deliberately-broken fixtures.
- **Go 1.26.3 → 1.26.4.** `govulncheck` flags `GO-2026-5039` (net/textproto) and
  `GO-2026-5037` (crypto/x509) as reachable through `config.Load` and
  `cobra.Command.Execute`. Both are fixed in 1.26.4. Bumping the pin in both
  `go.mod` and `mise.toml` (kept in lockstep, noted by comment) clears them.
- **`validate()` signature.** `RepoConfig.validate` returned `(RepoConfig,
  error)` but every caller discarded the value (`unparam`). Collapsed to
  `error`; updated the loop in `Load` and `config_test.go`.
- **`errors.Is` over `==`.** `internal/mcp/server.go` compared `err ==
  ErrNotFound` in six spots; `errors.Is` is correct under error wrapping and
  satisfies `errorlint`. Behavior is identical for the currently-unwrapped
  sentinel, so no test change is needed.

## Files

- `mise.toml` — `[tools]` go, golangci-lint, goreleaser, govulncheck;
  `[env]` adds `~/go/bin` to PATH for shells that opened before mise activated.
- `.golangci.yml` — golangci-lint v2 config (linters + excludes above).
- `Taskfile.yml` — task definitions; `ci` chains fmt-check → vet → lint → test → vuln.
- `internal/store/store.go`, `internal/config/config.go`,
  `internal/config/config_test.go`, `internal/mcp/server.go` — lint fixes.
- `go.mod` — `go 1.26.4`.

## Out of scope

CI workflows and release config (`.github/workflows`, `.goreleaser.yml`) are the
next change (`add-ci-pipeline`); installation docs are `add-installation-docs`.

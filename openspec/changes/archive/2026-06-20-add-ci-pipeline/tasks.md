# Tasks: add-ci-pipeline

- [x] Add `.github/workflows/ci.yml` (mise-action, Linux+macOS matrix, `task ci` + `task coverage` + coverage artifact).
- [x] Add `.goreleaser.yml` (v2; linux/darwin × amd64/arm64, ldflags version inject, archives, checksums, changelog) and make `internal/version` injectable; ignore `/dist/` + `coverage.html`.
- [x] Add `.github/workflows/release.yml` (tag `v*` → `task release`); verify `goreleaser check` and `task release-snapshot` build all 4 targets.

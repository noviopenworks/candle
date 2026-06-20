# Tasks: add-project-tooling

- [x] Add `mise.toml` (pin Go 1.26.4, golangci-lint, goreleaser, govulncheck) and `.golangci.yml`; fix the lint findings (store blank-import comment, `validate()` signature, `errors.Is` in mcp server).
- [x] Add `Taskfile.yml` (build/vet/test/fmt/fmt-check/lint/vuln/coverage/install/tidy/release/release-snapshot/ci); `fmt` tasks exclude `testdata`.
- [x] Bump Go pin 1.26.3 → 1.26.4 in `go.mod` + `mise.toml`; verify `task ci` is green (fmt-check, vet, lint, test, vuln all pass).

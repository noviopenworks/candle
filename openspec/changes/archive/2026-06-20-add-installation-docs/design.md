# Design: add-installation-docs

Tweak: document the now-supported install paths, surface the orphaned `flow.md`,
and point contributors at the `task ci` gate.

## Choices considered

- **Where the install instructions live** — a dedicated **Installation** section
  near the top of `README.md` (first thing a new user needs) plus a fuller
  "Install or build" step in `docs/getting-started.md`. The README stays concise
  and links to getting-started for the full walkthrough; no duplication of the
  index/serve flow.
- **Which methods to document** — exactly the two locked decisions (`go install`
  + release binaries), plus a from-source/`mise`+`task` path for contributors.
  Ordered easiest-first: `go install` → release binary → from source.
- **`flow.md` placement** — keep it at the repo root (it is a top-level narrative,
  peer to `README.md`), but link it from both the README docs table and
  `docs/README.md` (reading paths + index) so it is reachable from the two places
  readers browse. Content is left unchanged.

## Decisions

- **Match the real toolchain.** Getting-started prerequisites now read "Go 1.26+
  (the module targets `go 1.26.4`)" — kept in sync with the `go.mod`/`mise.toml`
  bump from `add-project-tooling` — and offer `mise` as the pinned alternative.
- **`docs/README.md` link depth.** `flow.md` is at the repo root and
  `docs/README.md` lives in `docs/`, so it is linked as `../flow.md`.
- **CONTRIBUTING gets a minimal note**, not a rewrite: the existing raw `go
  build/vet/test` block stays (works without mise); a short paragraph adds the
  `mise install` + `task ci` path that CI uses. Avoids duplicating `Taskfile.yml`.
- **No tool-count edits.** Pre-existing "13 vs 15 tools" wording across docs is a
  separate inconsistency, out of scope for this change.

## Files

- `README.md` — new "## Installation" section before "Quick start"; `flow.md`
  row added to the documentation table.
- `docs/getting-started.md` — prerequisites + "Install or build" step rewritten.
- `docs/README.md` — `flow.md` reading path and index row.
- `CONTRIBUTING.md` — `mise`/`task ci` paragraph in Development.

## Verification

Docs-only: `go build ./...` and `go test ./...` (116) remain green; all new links
resolve to existing files (`flow.md`, `docs/getting-started.md`); no stale
`1.26.3` references remain outside historical verify reports.

## Out of scope

Reconciling the 13-vs-15 MCP tool count across docs; any code or workflow change.

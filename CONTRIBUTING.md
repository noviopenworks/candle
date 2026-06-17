# Contributing to candlegraph

## Development

```bash
go build ./...      # compile everything
go vet ./...        # static checks
go test ./...       # full suite (incl. the MCP end-to-end test)
```

The `internal/mcp` e2e test builds the `candlegraph` binary, indexes a fixture
graph, and drives the stdio server — so a green `go test ./...` means the whole
index → serve path works.

### Layout

See [docs/architecture.md](docs/architecture.md) for the package map. Key rule:
**SDK types stay in `internal/mcp/server.go`**; tool methods are pure and
SDK-free so they can be unit-tested directly.

## Project conventions

- **Module path:** `github.com/noviopenworks/candlegraph`.
- **Binary / command:** `candlegraph` (`cmd/candlegraph`).
- Match the surrounding code's naming, comment density, and idioms.
- One logical change per commit; messages describe intent, not mechanics.

## The spec-driven workflow (OpenSpec + Comet)

This repo is developed with a dual-track workflow:

- **OpenSpec** owns *what* — proposals, capability specs (`openspec/specs/`),
  and the change lifecycle (`openspec/changes/`, archived under
  `openspec/changes/archive/`).
- **Comet / Superpowers** owns *how* — technical design docs
  (`docs/superpowers/specs/`), implementation plans (`docs/superpowers/plans/`),
  and verification reports (`docs/superpowers/reports/`).

A change flows through five phases:

```
open → design → build → verify → archive
```

To start a change, run `/comet` (or `/comet-open`) in an agent session and
follow the phase prompts. At archive, the change's delta spec is merged into the
main specs under `openspec/specs/` and the change is moved to the archive dir.

Browse `openspec/changes/archive/` for worked examples (e.g.
`2026-06-17-rename-to-candlegraph`), each with its proposal, spec, design, tasks,
and verification report.

## Where the design lives

[docs/design.md](docs/design.md) is the source-of-truth design spec (tool I/O,
DDL, edge catalog, end-to-end walkthrough). User-facing docs live under
[docs/](docs/README.md). Keep both in sync when behavior changes.

## Adding or changing a tool

1. Add the pure method to the relevant `internal/mcp/*_tools.go`, with a unit test.
2. Register it in `internal/mcp/server.go` (add to `ToolNames` + a `register*`).
3. Document it in [docs/tools.md](docs/tools.md) with example I/O.
4. Run `go test ./...` — the e2e test asserts the advertised tool surface.

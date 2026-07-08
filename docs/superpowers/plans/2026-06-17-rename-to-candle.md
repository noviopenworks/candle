---
change: rename-to-candle
design-doc: docs/superpowers/specs/2026-06-17-rename-to-candle-design.md
base-ref: 31d9cf50e80c11469c3b565ea04a3285ec04f6f3
archived-with: 2026-06-17-rename-to-candle
---

# Implementation Plan — rename-to-candle

> Retroactive capture: the rename was implemented and verified in the working tree
> before this change was opened. The full `base-ref`→working-tree diff is rename-only
> (`github.com/vend-ai/intel-mcp` → `github.com/noviopenworks/candle`,
> binary `intel-mcp` → `candle`). Execution = commit the existing verified diff
> and tick the tasks against it. No new code is written.

## Task 1 — Module path

- Update `go.mod` module directive → `github.com/noviopenworks/candle`.
- Update every internal `.go` import path to the new module path (module-path pass before bare-name pass).

**Verify:** `go build ./...` succeeds.

## Task 2 — Binary / command name

- `git mv cmd/intel-mcp cmd/candle`.
- Cobra root `Use: "candle"` in `cmd/candle/main.go`.
- MCP server `Name: "candle"` in `internal/mcp/server.go`.
- E2E-built binary name + comment in `internal/mcp/e2e_test.go`.

**Verify:** `go test ./internal/mcp/` passes (e2e compiles the renamed binary).

## Task 3 — Docs

- Update `intel-mcp` / `cmd/intel-mcp` references in the four `docs/superpowers/plans/` files.

## Task 4 — Verification

- `go build ./...`, `go vet ./...`, `go test ./...` (all packages) pass.
- `git grep` for `intel-mcp` / `vend-ai` / `candle/intel-mcp` over tracked source/config (excluding `graphify-out/` and this change's own docs) returns zero hits.

## Commit strategy

Single atomic commit: the rename diff plus the OpenSpec change artifacts and design/plan docs.

# Brainstorm Summary

- Change: rename-to-candle
- Date: 2026-06-17

## Confirmed Technical Approach

Mechanical identity correction, already implemented and verified in the working tree before this change was opened. Two-pass, order-sensitive textual replacement:

1. Module path: `github.com/candle/intel-mcp` → `github.com/noviopenworks/candle` (pass 1, first — the new path no longer contains `intel-mcp`).
2. Bare name: `intel-mcp` → `candle` (pass 2 — cobra `Use`, MCP server `Name`, e2e binary path/comment).
3. `git mv cmd/intel-mcp cmd/candle` to preserve history.

## Key Trade-offs and Risks

- Import-path break is real but inert: the only known consumer repo (`/home/mg/vend-ai/`) no longer exists.
- Generated `graphify-out/` knowledge graph still carries old-path nodes until regenerated — deferred, out of scope.

## Testing Strategy

- `go build ./...`, `go vet ./...`, `go test ./...` (11 packages) — all pass.
- `grep -r intel-mcp` over tracked files excluding `graphify-out/` — zero hits.

## Spec Patches

None. The `module-identity` delta spec already carries three requirements (canonical module path, single binary name, no stale prior name) each with WHEN/THEN scenarios.

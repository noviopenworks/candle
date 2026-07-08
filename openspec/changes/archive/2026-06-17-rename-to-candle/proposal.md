## Why

The Go module was published under a placeholder path and an `intel-mcp` binary sub-name that do not match the canonical repository, `github.com/noviopenworks/candle`. Aligning the module path and binary/command name with the real repo is required before the module can be imported or installed by its true path.

> Baseline note: the committed baseline module path was `github.com/vend-ai/intel-mcp`. The working tree additionally held a partial, uncommitted `vend-ai → candle/intel-mcp` rename; this change supersedes that intermediate state and lands the canonical `github.com/noviopenworks/candle` in one diff.

## What Changes

- **BREAKING** (import path): Go module path → `github.com/noviopenworks/candle` (from the committed `github.com/vend-ai/intel-mcp`). Every internal import is updated accordingly.
- Binary / command name `intel-mcp` → `candle` (cobra `Use`, MCP server `Name`, e2e-built binary).
- Command directory `cmd/intel-mcp/` → `cmd/candle/`.
- Historical implementation plans under `docs/superpowers/plans/` updated so their `cmd/intel-mcp` / `intel-mcp` references stay accurate.
- No behavior, API surface, storage schema, or dependency changes.

## Capabilities

### New Capabilities
- `module-identity`: Declares the canonical Go module path and the single binary/command name the project ships under, plus the rule that no stale prior name remains in tracked source.

### Modified Capabilities
<!-- None. This is a pure identity/path correction; no existing capability's requirements change. -->

## Impact

- **Code**: `go.mod`; all `.go` files (imports + literal name strings); `cmd/intel-mcp/ → cmd/candle/`; `internal/mcp/server.go` (`Name`), `cmd/candle/main.go` (cobra `Use`), `internal/mcp/e2e_test.go` (built binary name).
- **Docs**: 4 plan files under `docs/superpowers/plans/`.
- **Out of scope**: generated `graphify-out/` knowledge-graph artifacts (regenerated separately; still carry the old path until rebuilt); archived openspec changes (never referenced either name); external consumers (former consumer repo no longer exists).
- **Dependencies**: none added or removed.

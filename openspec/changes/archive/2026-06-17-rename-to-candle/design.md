## Context

The module shipped under a placeholder identity (`github.com/candle/intel-mcp`, binary `intel-mcp`). The canonical repository is `github.com/noviopenworks/candle`. This is a mechanical identity correction with no behavioral change. It was implemented and verified in the working tree before this change was opened; the design below records the approach taken.

## Approach

A two-pass, order-sensitive find-and-replace across tracked files (excluding generated artifacts and archived records):

1. **Module path first** — replace `github.com/candle/intel-mcp` → `github.com/noviopenworks/candle` everywhere. Doing this first means the resulting path no longer contains the substring `intel-mcp`, so the second pass cannot corrupt it.
2. **Bare name second** — replace remaining literal `intel-mcp` → `candle` (cobra `Use`, MCP server `Name`, e2e binary path/comment).
3. **Directory move** — `git mv cmd/intel-mcp cmd/candle` to preserve history.

```
   pass 1: module path          pass 2: bare name
   ┌──────────────────────┐     ┌──────────────────────┐
   │ candle/intel-mcp│     │ intel-mcp (literal)  │
   │        │             │     │        │             │
   │        ▼             │     │        ▼             │
   │ noviopenworks/       │     │   candle        │
   │   candle        │     │                      │
   └──────────────────────┘     └──────────────────────┘
   ordering guarantees pass 2 cannot touch the new module path
```

## Decisions and Rationale

- **Order matters**: module-path pass precedes bare-name pass; reversing it would split the module path mid-string. (See diagram.)
- **Scope exclusions**: generated `graphify-out/` graph artifacts are regenerated, not hand-edited; archived openspec changes are point-in-time records and never referenced either name, so both are excluded.
- **Directory rename via `git mv`**: preserves file history rather than delete+add.
- **No spec-behavior change**: the only spec captured is the non-functional `module-identity` capability; no existing capability's requirements move.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` (11 packages) — all pass.
- `grep -r intel-mcp` over tracked files excluding `graphify-out/` — zero hits.

## Risks / Trade-offs

- **Import-path break for downstream consumers**: the only known consumer repo (`/home/mg/vend-ai/`) no longer exists, so there is no live breakage. Any future consumer must import the new path.
- **Stale knowledge graph**: `graphify-out/graph.json` still carries old-path nodes until regenerated — deferred, out of scope.

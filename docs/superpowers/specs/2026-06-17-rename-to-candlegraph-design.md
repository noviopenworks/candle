---
comet_change: rename-to-candlegraph
role: technical-design
canonical_spec: openspec
---

# Rename to candlegraph — Technical Design

## Context

The module shipped under a placeholder identity. The committed baseline module path was `github.com/vend-ai/intel-mcp` with binary/command `intel-mcp`; the working tree also held a partial, uncommitted rename toward `github.com/candlegraph/intel-mcp`. The canonical repository is `github.com/noviopenworks/candlegraph`. This change supersedes the intermediate state and corrects the identity so the module can be imported and installed by its real path and ships under a single name, `candlegraph`. There is no behavioral, API, storage, or dependency change.

This work was implemented and verified in the working tree before the change was opened; this document records the approach taken (retroactive capture). Because the prior rename was never committed, the full `HEAD`→working-tree diff is rename-only (`github.com/vend-ai/intel-mcp` → `github.com/noviopenworks/candlegraph`).

## Approach

Order-sensitive, two-pass textual replacement across tracked files, plus a history-preserving directory move.

```
   pass 1: module path              pass 2: bare name
   ┌────────────────────────┐       ┌────────────────────────┐
   │ candlegraph/intel-mcp  │       │ intel-mcp (literal)    │
   │          │             │       │          │             │
   │          ▼             │       │          ▼             │
   │ noviopenworks/         │       │     candlegraph        │
   │   candlegraph          │       │                        │
   └────────────────────────┘       └────────────────────────┘
   ordering guarantees pass 2 cannot split the new module path
```

1. **Module path (pass 1)** — replace `github.com/candlegraph/intel-mcp` → `github.com/noviopenworks/candlegraph` across `go.mod` and every `.go` import. Run first: the resulting path contains no `intel-mcp` substring, so pass 2 cannot corrupt it.
2. **Bare name (pass 2)** — replace remaining literal `intel-mcp` → `candlegraph`: cobra root `Use`, MCP server `Name`, the e2e-built binary path and its comment.
3. **Directory move** — `git mv cmd/intel-mcp cmd/candlegraph`.
4. **Docs** — update `intel-mcp` / `cmd/intel-mcp` references in the four `docs/superpowers/plans/` files for accuracy.

## Components Touched

| Area | Files | Change |
|------|-------|--------|
| Module decl | `go.mod` | module directive → canonical path |
| Imports | all `.go` | import paths → canonical path |
| Entry point | `cmd/intel-mcp/ → cmd/candlegraph/`, `main.go` | dir move + cobra `Use` |
| Server identity | `internal/mcp/server.go` | `Name: "candlegraph"` |
| E2E test | `internal/mcp/e2e_test.go` | built binary name + comment |
| Plan docs | `docs/superpowers/plans/*.md` | name references |

## Decisions and Rationale

- **Pass ordering is load-bearing** — module-path pass must precede bare-name pass; otherwise the bare-name pass splits the module path mid-string.
- **`git mv` over delete+add** — preserves file history for the entrypoint.
- **Scope exclusions** — generated `graphify-out/` artifacts are regenerated (not hand-edited) and archived openspec changes are point-in-time records that never referenced either name; both are excluded.
- **Single non-functional spec** — captured as the `module-identity` capability; no existing capability's requirements change.

## Testing Strategy

- `go build ./...` — compiles under the new module path.
- `go vet ./...` — clean.
- `go test ./...` — all 11 packages pass, including the mcp e2e test that compiles the renamed binary.
- `git grep` for `intel-mcp` / `vend-ai` / `candlegraph/intel-mcp` over tracked source and config — excluding `graphify-out/` and this change's own docs (which quote the prior names) — zero hits (asserts requirement 3 of `module-identity`).

## Risks / Trade-offs

- **Downstream import break** — real but inert; the only known consumer (`/home/mg/vend-ai/`) no longer exists. Future consumers must use the new path.
- **Stale knowledge graph** — `graphify-out/graph.json` retains old-path nodes until regenerated; deferred and out of scope for this change.

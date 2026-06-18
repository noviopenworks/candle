# Brainstorm Summary

- Change: add-get-context-facade
- Date: 2026-06-18

## Confirmed Technical Approach

Additive pure method `Tools.GetContext(GetContextArgs) (ContextResult, error)` in new
`internal/mcp/context_tools.go`, registered via a thin `registerGetContext` in `server.go`
using existing `textResult`/`mustJSON`/`toolErr` helpers — mirrors the 13 existing tools.

- **Repo field (D3 resolved):** dedicated typed `RepoSummary{Repo, Commit, Branch, NodeCount}`
  struct. Stable JSON contract, decoupled from `registry.RepoInfo`. Resolves the source
  plan's `Repo any` vs `out.Repo.Repo` compile bug.
- **Modes:** `topic` empty → overview catalog; `topic` set → topic search composing existing
  store/Tools queries (`NodesByLabel`+`Callers`/`Callees` one-hop, `FindOperations`,
  `FindSchema`, `FindRPCs`+`ExplainRPC`, `FindPrivateLibrary`). `mode`
  (`overview|code|api|proto|library|all`, unknown→`all`) filters surfaces.
- **Refinement over source plan:** `mode:"overview"` = catalog only — suppresses topic
  matches even when a topic is supplied (the plan folded overview→all).
- `include_resources` emits commit-pinned `graph://`/`openapi://`/`proto://`/`lib://` URIs
  (fallback `latest`). Every response carries explicit `limitations`.

## Key Trade-offs and Risks

- Intentional entry-point overlap with precise tools — accepted; facade routes back via
  `suggested_next_calls`.
- Advertised tool count 13→14 forces `e2e_surface_test.go` updates (comments ~32/218 +
  assertions).
- Heterogeneous sub-fields (callers/callees, endpoints, private libs) stay loosely typed
  (`any`), tests type-assert; only the repo field is strictly typed.

## Testing Strategy

TDD, failing-test-first per group, mirroring the source plan: overview mode, topic-all-
surfaces (one-hop code context + schema + RPC + resource URIs), code-mode isolation,
overview-mode-suppresses-matches (new), unknown-repo→`ErrNotFound`, e2e surface (14 tools).
`go test ./...` + `go vet ./...`.

## Spec Patches

Add a scenario under "Requirement: Mode filter narrows searched surfaces" in
`specs/context-retrieval/spec.md`: overview mode returns the capability catalog only and
suppresses topic matches even when a topic is supplied.

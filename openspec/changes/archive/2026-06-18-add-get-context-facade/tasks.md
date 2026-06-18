# Tasks: add-get-context-facade

> TDD-oriented: each implementation group is preceded by a failing test. Implementation plan:
> `docs/superpowers/plans/2026-06-18-get-context-facade.md`.

## 1. Overview mode (test-first)

- [x] 1.1 Add `internal/mcp/context_tools_test.go` with `seedContextTools` and a failing `TestGetContextOverview` (repo summary, capability counts, suggested calls, resource schemes)
- [x] 1.2 Run `go test ./internal/mcp -run TestGetContextOverview -v` and confirm it fails (undefined `GetContextArgs`/`Tools.GetContext`)
- [x] 1.3 Create `internal/mcp/context_tools.go`: `GetContextArgs`, `ContextResult` (typed `RepoSummary` field per design D3), `ContextCapabilities`/`CapabilitySummary`, `ToolHint`, `ResourceScheme`, mode normalization, capability catalog, overview hints, resource schemes, limitations
- [x] 1.4 Run `go test ./internal/mcp -run TestGetContextOverview -v` and confirm it passes

## 2. Topic retrieval mode (test-first)

- [x] 2.1 Append failing tests: `TestGetContextTopicSearchesAllSurfaces`, `TestGetContextCodeModeOnlyReturnsCode`, `TestGetContextOverviewModeSuppressesMatches`, `TestGetContextUnknownRepo`
- [x] 2.2 Run the topic tests and confirm topic/code-mode/overview-suppress tests fail
- [x] 2.3 Implement `contextMatches` (code one-hop callers/callees, endpoints, schemas, RPCs, private libraries) with `mode` filtering, overview-mode match suppression (D6), and `include_resources` URI hints; wire it into `GetContext`
- [x] 2.4 Run `go test ./internal/mcp -run TestGetContext -v` and confirm all pass

## 3. MCP registration and surface

- [x] 3.1 Add `"get_context"` to `ToolNames` after `"resolve_repo"` and register via `registerGetContext` in `internal/mcp/server.go`
- [x] 3.2 Update `internal/mcp/e2e_surface_test.go` expected count/list from 13 to 14 (comments ~lines 32, 218 and assertions)
- [x] 3.3 Run `go test ./internal/mcp -v` and confirm pass

## 4. Documentation

- [x] 4.1 Update `docs/tools.md`: 14 tools, insert `get_context` reference (args table, overview + topic request examples, response shape) after `resolve_repo`
- [x] 4.2 Update `docs/examples.md`: add a "Start with get_context" first example and the precise-follow-up flow
- [x] 4.3 Update `README.md`: tool count 13 → 14 and a retrieval-first sentence near quick start

## 5. Final verification

- [x] 5.1 Run `go test ./...` and confirm pass
- [x] 5.2 Run `go vet ./...` and confirm pass
- [x] 5.3 Inspect `git diff` and confirm it only contains `get_context` implementation, tests, registration, and docs

<!-- Code review: performed inline (no subagent dispatch, per user's no-agent-spawn preference for this build). No Critical/Important findings. Minor finding fixed: deduped the nodes COUNT query in get_context overview. Accepted by-design: best-effort error handling (store query errors swallowed for graceful partial context) and Depth as a documented v1 no-op. -->

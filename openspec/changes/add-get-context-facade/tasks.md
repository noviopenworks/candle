# Tasks: add-get-context-facade

> TDD-oriented: each implementation group is preceded by a failing test. Source plan:
> `docs/superpowers/plans/2026-06-18-get-context-retrieval-facade.md`.

## 1. Overview mode (test-first)

- [ ] 1.1 Add `internal/mcp/context_tools_test.go` with `seedContextTools` and a failing `TestGetContextOverview` (repo summary, capability counts, suggested calls, resource schemes)
- [ ] 1.2 Run `go test ./internal/mcp -run TestGetContextOverview -v` and confirm it fails (undefined `GetContextArgs`/`Tools.GetContext`)
- [ ] 1.3 Create `internal/mcp/context_tools.go`: `GetContextArgs`, `ContextResult` (typed repo field per design D3), `ContextCapabilities`/`CapabilitySummary`, `ToolHint`, `ResourceScheme`, mode normalization, capability catalog, overview hints, resource schemes, limitations
- [ ] 1.4 Run `go test ./internal/mcp -run TestGetContextOverview -v` and confirm it passes

## 2. Topic retrieval mode (test-first)

- [ ] 2.1 Append failing tests: `TestGetContextTopicSearchesAllSurfaces`, `TestGetContextCodeModeOnlyReturnsCode`, `TestGetContextUnknownRepo`
- [ ] 2.2 Run the topic tests and confirm topic/code-mode tests fail
- [ ] 2.3 Implement `contextMatches` (code one-hop callers/callees, endpoints, schemas, RPCs, private libraries) with `mode` filtering and `include_resources` URI hints; wire it into `GetContext`
- [ ] 2.4 Run `go test ./internal/mcp -run TestGetContext -v` and confirm all pass

## 3. MCP registration and surface

- [ ] 3.1 Add `"get_context"` to `ToolNames` after `"resolve_repo"` and register via `registerGetContext` in `internal/mcp/server.go`
- [ ] 3.2 Update `internal/mcp/e2e_surface_test.go` expected count/list from 13 to 14 (comments ~lines 32, 218 and assertions)
- [ ] 3.3 Run `go test ./internal/mcp -v` and confirm pass

## 4. Documentation

- [ ] 4.1 Update `docs/tools.md`: 14 tools, insert `get_context` reference (args table, overview + topic request examples, response shape) after `resolve_repo`
- [ ] 4.2 Update `docs/examples.md`: add a "Start with get_context" first example and the precise-follow-up flow
- [ ] 4.3 Update `README.md`: tool count 13 → 14 and a retrieval-first sentence near quick start

## 5. Final verification

- [ ] 5.1 Run `go test ./...` and confirm pass
- [ ] 5.2 Run `go vet ./...` and confirm pass
- [ ] 5.3 Inspect `git diff` and confirm it only contains `get_context` implementation, tests, registration, and docs

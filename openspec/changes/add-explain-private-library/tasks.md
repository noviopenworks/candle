# Tasks: add-explain-private-library

> TDD-oriented: each implementation group is preceded by a failing test. Cross-repo
> consumer aggregation is the core new capability; code-graph linking (esp. consumer-side)
> is resolved during design brainstorming.

## 1. Cross-index consumer aggregation (store, test-first)

- [x] 1.1 Add a failing store test for cross-index aggregation by module path (seed 2 indexes consuming the same private module + a provider index)
- [x] 1.2 Run the store test and confirm it fails (undefined aggregation method)
- [x] 1.3 Implement the cross-index aggregation query in `internal/store/godep.go` joining `private_library_usages`/`dependencies` → `indexes` → `repos` by `module_path`, returning per-repo identity, version, used packages, used symbols
- [x] 1.4 Run the store test and confirm it passes

## 2. ExplainPrivateLibrary provider + consumer aggregation (test-first)

- [x] 2.1 Add a failing test for `Tools.ExplainPrivateLibrary`: provider exports + cross-repo consumers for a known library
- [x] 2.2 Run and confirm it fails (undefined `ExplainPrivateLibrary`/result types)
- [x] 2.3 Implement `Tools.ExplainPrivateLibrary` + result types: fuzzy resolution (best + candidates), provider section via `PrivateLibraryByModule`, consumer aggregation via the new store query, `limitations`
- [x] 2.4 Run and confirm it passes

## 3. Fuzzy resolution and boundary behavior (test-first)

- [x] 3.1 Add failing tests: ambiguous query → best + candidates; provider-less library → consumers only, no error; unknown query → `ErrNotFound`
- [x] 3.2 Run and confirm the new tests fail where expected
- [x] 3.3 Implement disambiguation + provider-less handling
- [x] 3.4 Run and confirm all pass

## 4. Code-graph linking (test-first)

- [x] 4.1 Add failing tests: export → provider node link resolves; unresolved consumer usage is marked (per the brainstorming-decided matching rule)
- [x] 4.2 Run and confirm they fail
- [x] 4.3 Implement export→provider-node linking (`NodesByLabel` in provider index) and best-effort consumer usage→node linking with explicit unresolved markers
- [x] 4.4 Run and confirm all pass

## 5. MCP registration and surface

- [x] 5.1 Add `"explain_private_library"` to `ToolNames` and register via `registerExplainPrivateLibrary` in `internal/mcp/server.go`
- [x] 5.2 Update `internal/mcp/e2e_surface_test.go` advertised count/comments 14 → 15
- [x] 5.3 Run `go test ./internal/mcp -v` and confirm pass

## 6. Documentation

- [ ] 6.1 Update `docs/tools.md`: 15 tools, add `explain_private_library` reference (args, request/response shape) in the private-library section
- [ ] 6.2 Update `docs/examples.md`: add a cross-repo "who consumes this library across the org?" example
- [ ] 6.3 Update `README.md`: tool count 14 → 15

## 7. Final verification

- [ ] 7.1 Run `go test ./...` and confirm pass
- [ ] 7.2 Run `go vet ./...` and confirm pass
- [ ] 7.3 Inspect `git diff` and confirm scope matches the plan

# Verification Report: hydrate-github-source-content

**Date:** 2026-07-09
**Verify mode:** full
**Change:** hydrate-github-source-content
**Base ref:** 7bae63b570201af99393f47cbb65e4d353ec452c

## Summary

| Dimension    | Status |
|--------------|--------|
| Completeness | 15/15 tasks done, 6/6 requirements implemented |
| Correctness  | 11/11 scenarios covered |
| Coherence    | Design decisions followed, no spec/design contradictions |

## Verification Evidence

### Build, Test, Lint

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build ./...` | PASS |
| Tests | `go test ./... -count=1` | PASS (157 tests, 12 packages, 0 failures) |
| Vet | `go vet ./...` | PASS (no issues) |
| Lint | `golangci-lint run ./...` | PASS (no issues) |

### Changed Files (30 files, +3570/-45)

**Implementation files:** `internal/store/query.go`, `internal/mcp/source_content.go`, `internal/mcp/tools.go`, `internal/mcp/context_tools.go`, `internal/mcp/server.go`

**Test files:** `internal/store/query_test.go`, `internal/mcp/source_content_test.go`, `internal/mcp/tools_test.go`, `internal/mcp/context_tools_test.go`, `internal/mcp/e2e_surface_test.go`

**Documentation:** `docs/tools.md`

**OpenSpec/Superpowers artifacts:** proposal, design, spec, tasks, plan, design doc

### Requirement Coverage

1. **Source hydration is opt-in** — `SourceContentOptions` with mode field; default tests prove metadata-only behavior; auto mode has trigger conditions.
2. **GitHub source content resolved from indexed provenance** — `normalizeGitHubSourceURL` handles raw and blob URLs; `rawURLForNode` prefers SourceURL, falls back to repo+commit+file; unsupported hosts return structured status.
3. **Snippet and full-file modes** — `snippetWindow` extracts bounded line windows; `fetchText` caps at maxBytes+1; truncation flag set when exceeded.
4. **Fetch failures do not fail whole response** — network errors, non-text content, unsupported URLs all return `SourceContent` status envelopes, not Go errors; tests verify each path.
5. **Bounded and structured envelopes** — `SourceContent` struct carries status, mode, source URL, line range, truncation, content; wrapper result types used by all tools.
6. **Direct source reads via dedicated tool** — `read_source_content` registered as 17th MCP tool; `ReadSourceContent` method handles node and file reads.

### Scenario Coverage

| Scenario | Test Evidence |
|----------|---------------|
| Default metadata-only | `TestQueryRepoWithSourcePreservesDefaultShape`, `TestExplainSymbolWithSourcePreservesDefaultShape`, `TestGetFileContextWithSourcePreservesDefaultShape`, `TestGetContextSourceContentPreservesDefaultCodeSymbolShape` |
| Auto hydration | Trigger logic in `queryRepoShouldHydrateAuto` and context matches |
| Source_url fetchable | `TestHydrateSourceContentSnippet`, `TestReadSourceContentByNode` |
| Unsupported URL | `TestHydrateSourceContentUnsupportedURL` |
| Snippet with source location | `TestHydrateSourceContentSnippet` (L3 → lines 2-4) |
| Full-file truncation | `TestHydrateSourceContentFullTruncated` (maxBytes=3, truncated=true) |
| Fetch error → status | `TestHydrateSourceContentFetchErrorIsStatus` |
| Non-text content → status | `TestHydrateSourceContentRejectsNonText` |
| Missing provenance → status | `TestHydrateSourceContentMissingProvenance` |
| Direct read by node | `TestReadSourceContentByNode` |
| Direct read by file | `TestReadSourceContentByFile` |

### Design Doc Adherence

| Design Decision | Implementation |
|----------------|----------------|
| Expose stored provenance | `NodeRow` extended with 4 fields; `nodeCols` and `scanNodes` updated |
| Explicit source hydration option with auto trigger | `SourceContentOptions` with mode off/auto/snippet/full; trigger conditions per tool |
| GitHub text content from externally authorized URLs | No auth management; URL normalization; only github.com and raw.githubusercontent.com |
| Structured source content envelopes | `SourceContent` struct; wrapper types: `SourceNodeResult`, `SourceSymbolExplanation`, `SourceFileContextResult` |
| Conservative fetch limits | 64KiB maxBytes, 20 line radius, 5 candidates, 5s timeout; 1MiB hard ceiling on caller-supplied maxBytes |

No contradictions between delta spec and design doc.

## Issues

### WARNING

1. **Tool-count drift in other docs** — `README.md`, `docs/architecture.md`, `docs/getting-started.md`, `Roadmap.md`, `AGENTS.md`, `CLAUDE.md` still reference "16 tools". `docs/tools.md` and `e2e_surface_test.go` correctly say 17. Recommend a follow-up sweep before archive.

### SUGGESTION

1. **Auto-trigger test coverage** — `queryRepoShouldHydrateAuto` and auto-branches in `ExplainSymbolWithSource` / `contextMatches` have no dedicated positive/negative tests. Functionally covered by trigger logic + manual review.
2. **Snippet→full fallback untested** — When mode is snippet but `parseSourceLocation` fails, code silently upgrades to full. No dedicated test covers this branch.
3. **`parseSourceLine` vs `parseSourceLocation` naming** — New helper named `parseSourceLocation` to avoid collision with existing `parseSourceLine` in `library_explain.go`. Consolidation recommended in a future cleanup task.

## Final Assessment

No CRITICAL issues. 1 WARNING (tool-count drift) with recommendation for follow-up. 3 SUGGESTION items that are non-blocking.

**Verification result: PASS.** Ready for archive after branch handling.

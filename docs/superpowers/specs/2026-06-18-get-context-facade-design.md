---
comet_change: add-get-context-facade
role: technical-design
canonical_spec: openspec
---

# get_context Retrieval Facade — Technical Design

## Summary

Add `get_context` as candlegraph's primary, Context7-style MCP retrieval tool. It is a
repo-scoped facade that composes the existing precise tools' underlying queries into two
shapes: an **overview catalog** (what candlegraph knows about a repo) and **topic
retrieval** (focused context across code, HTTP, protobuf, and private libraries). It is
purely additive — no existing tool behavior changes.

This design refines the validated source plan at
`docs/superpowers/plans/2026-06-18-get-context-retrieval-facade.md` with two confirmed
decisions: a typed `RepoSummary` result field (D3) and `mode:"overview"` semantics.

## Architecture

```
get_context (MCP tool)
   │  registerGetContext(server.go) — thin SDK glue: textResult/mustJSON/toolErr
   ▼
Tools.GetContext(GetContextArgs) (ContextResult, error)   [context_tools.go, SDK-free]
   │
   ├─ reg.Resolve(repo) → RepoInfo | ErrNotFound
   ├─ overview catalog  → contextCapabilities(indexID)
   │     ├─ nodeCount (COUNT(*) nodes)
   │     ├─ ListAPISpecs / ListProtoFiles
   │     └─ FindPrivateLibraries / FindPrivateDeps
   └─ topic search (topic != "" AND mode != "overview") → contextMatches(...)
         ├─ code    : NodesByLabel + Callers + Callees   (one-hop)
         ├─ api      : FindOperations
         ├─ schema   : Tools.FindSchema  (api or proto)
         ├─ proto    : FindRPCs + Tools.ExplainRPC
         └─ library  : Tools.FindPrivateLibrary
```

All underlying queries already exist and are exercised by the precise tools; `get_context`
adds composition and routing, not new storage access patterns.

## Components

### `internal/mcp/context_tools.go` (new)

Owns the method and result types.

- `GetContextArgs{ Repo, Topic, Mode, Depth, IncludeResources }` — `Repo` required;
  `Topic` empty ⇒ overview, non-empty ⇒ topic search.
- `RepoSummary{ Repo, Commit, Branch, NodeCount }` — **typed** repo field (D3). Decoupled
  from `registry.RepoInfo`; gives a stable JSON contract and lets tests read
  `out.Repo.Repo` / `out.Repo.Commit` without type assertions.
- `ContextResult{ Repo RepoSummary, Topic, Mode, Capabilities, Matches, SuggestedNextCalls,
  ResourceSchemes, Resources, Limitations }`.
- `ContextCapabilities` / `CapabilitySummary{ Available, Count, Tools }` — per-surface
  catalog with the precise follow-up tool names.
- `ContextMatches{ CodeSymbols, Endpoints, Schemas, RPCs, PrivateLibraries }`,
  `CodeContext{ Node, Callers, Callees, Resource }`, `ToolHint`, `ResourceScheme`.
  Heterogeneous sub-fields wrapping existing results stay `any`; only `Repo` is strictly
  typed.
- Helpers: `normalizeContextMode`, `contextCapabilities`, `nodeCount`, `overviewHints`,
  `contextResourceSchemes`, `contextLimitations`, `contextMatches`, `graphNodeResource`,
  `commitOrLatest`.

### `internal/mcp/server.go` (modified)

- Add `"get_context"` to `ToolNames` immediately after `"resolve_repo"`.
- `registerGetContext(srv, tools)` after `registerResolveRepo`, marshaling via the shared
  helpers.

### Tests & docs

- `internal/mcp/context_tools_test.go` (new) — see Testing.
- `internal/mcp/e2e_surface_test.go` — advertised count 13 → 14 (comments ≈ lines 32, 218,
  and the surface assertions).
- `docs/tools.md`, `docs/examples.md`, `README.md` — retrieval-first documentation and the
  14-tool count.

## Key Decisions

| ID | Decision | Rationale |
|----|----------|-----------|
| D1 | Pure method + thin registration | Consistent with all 13 existing tools; keeps logic SDK-free and unit-testable |
| D2 | Single tool, two modes, `mode` filter | Serves discovery and focused lookup without tool proliferation |
| D3 | Typed `RepoSummary` field | Resolves source-plan `Repo any` compile bug; stable JSON contract decoupled from registry internals |
| D4 | One-hop code context only | Reuses `Callers`/`Callees`; deeper traversal recorded as a limitation (`depth` accepted, v1 honors one hop) |
| D5 | Reuse existing resource schemes | `include_resources` emits commit-pinned `graph://`/`openapi://`/`proto://`/`lib://` URIs (fallback `latest`) |
| D6 | `mode:"overview"` = catalog only | Refines the plan (which folded overview→all): overview suppresses topic matches even when a topic is supplied; `mode` is also normalized so unknown/empty ⇒ `all` |

### Mode semantics (D2 + D6)

```
                      topic == ""        topic != ""
mode = overview   →   catalog            catalog (matches suppressed)   ← D6
mode = all/""      →   catalog            catalog + all-surface matches
mode = code        →   catalog            catalog + code matches only
mode = api          →   catalog            catalog + endpoint+schema matches
mode = proto        →   catalog            catalog + rpc+schema matches
mode = library      →   catalog            catalog + private-library matches
unknown            →   treated as all
```

Schema search runs when `mode` includes `api` or `proto` (a topic may name an OpenAPI
schema or a proto message). Code-mode therefore excludes schemas, matching the test.

## Data Flow

1. Resolve repo → `ErrNotFound` short-circuits with no partial result.
2. Always build the overview: `RepoSummary` + capability catalog + suggested next calls +
   resource schemes + limitations.
3. If `topic != ""` and `mode != "overview"`: run `contextMatches` for the included
   surfaces; prepend match-specific hints to suggested calls; collect resource URIs when
   `include_resources` is true.
4. Marshal to JSON via `mustJSON`; errors via `toolErr`.

## Error Handling & Boundaries

- Unknown repo → `ErrNotFound` (consistent with existing tools).
- Topic matching nothing → empty `Matches`, overview still returned (non-error).
- `depth > 1` → accepted but only one hop honored; noted in `limitations`.
- `limitations` always non-empty: OpenAPI endpoint→handler linking deferred; cross-repo RPC
  consumer and cross-repo private-library consumer aggregation deferred.

## Testing Strategy

TDD, failing-test-first per group (`seedContextTools` builds an in-memory store with one
code symbol + edge, one OpenAPI op + schema, one proto file/service/RPC/message, one
private dep + usage):

1. `TestGetContextOverview` — repo summary (`RepoSummary`), capability counts/availability,
   non-empty suggested calls and resource schemes, empty topic.
2. `TestGetContextTopicSearchesAllSurfaces` — one-hop code callees, schema match, RPC match,
   resource URIs (`include_resources:true`).
3. `TestGetContextCodeModeOnlyReturnsCode` — code matches present; endpoints/schemas/RPCs empty.
4. `TestGetContextOverviewModeSuppressesMatches` (**new, D6**) — `mode:"overview"` + topic ⇒
   catalog only, no matches.
5. `TestGetContextUnknownRepo` — `ErrNotFound`.
6. `e2e_surface_test.go` — `tools/list` advertises 14 tools including `get_context`.

Gates: `go test ./...`, `go vet ./...`, and a `git diff` scope check.

## Non-Goals

OpenAPI handler linking inside `get_context` v1, cross-repo RPC consumers, cross-repo
library consumer aggregation, embeddings/semantic search, multi-hop traversal. Each
deferred item is surfaced at runtime in `limitations`.

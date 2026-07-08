# Comet Design Handoff

- Change: add-get-context-facade
- Phase: design
- Mode: compact
- Context hash: bf3265978bebb94d552ee914292fe0c113d9c4266c25888eddd7294ec854cd5d

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/add-get-context-facade/proposal.md

- Source: openspec/changes/add-get-context-facade/proposal.md
- Lines: 1-48
- SHA256: 45d1777da647b56a1135c64be7d61263bf3c873ac24a26142ca84243401ea9d6

```md
## Why

Agents querying candle today must already know which precise tool to reach for
(`explain_symbol`, `find_endpoint`, `find_rpc`, `find_private_library`, …) and how a
repo's knowledge is organized before they can ask anything useful. There is no single
"start here" entry point that tells an agent what candle knows about a repo and
routes it to the right follow-up. This mirrors the Context7 retrieval pattern: one call
to discover capabilities, then focused topic lookups.

## What Changes

- Add a new MCP tool `get_context` as the primary, Context7-style retrieval entry point.
  - **Overview mode** (`repo` only): returns a repo snapshot summary plus a catalog of
    available capabilities (code graph, OpenAPI, protobuf, private libraries) with counts,
    the precise tools that serve each surface, suggested next calls, and resource schemes.
  - **Topic mode** (`repo` + `topic`): searches all surfaces for the topic and returns
    code symbols with one-hop callers/callees (codegraph-style), matched HTTP operations,
    schemas/proto messages, RPCs, and private libraries, plus exact resource-URI hints.
  - `mode` filter (`overview|code|api|proto|library|all`) narrows which surfaces are searched.
  - Every response carries explicit `limitations` describing what is deferred.
- Register `get_context` as the 14th advertised MCP tool (after `resolve_repo`).
- Update user-facing docs (`docs/tools.md`, `docs/examples.md`, `README.md`) to make
  `get_context` the recommended first call, with precise tools as follow-ups.

Non-breaking: existing tools and their behavior are unchanged; `get_context` is additive
and composes existing store queries.

## Capabilities

### New Capabilities
- `context-retrieval`: a repo-scoped retrieval facade (`get_context`) that composes existing
  code-graph, OpenAPI, protobuf, and private-library queries into overview and topic-oriented
  context for agent consumption.

### Modified Capabilities
<!-- None. mcp-core's existing tool requirements are unchanged; the advertised tool count
     moves from 13 to 14 but that is captured by the new context-retrieval capability and a
     test-surface update, not a behavioral change to existing tools. -->

## Impact

- **New code:** `internal/mcp/context_tools.go` (the `Tools.GetContext` method + result types),
  `internal/mcp/context_tools_test.go`.
- **Modified code:** `internal/mcp/server.go` (`ToolNames` + registration),
  `internal/mcp/e2e_surface_test.go` (advertised tool count 13 → 14, comments at lines ~32/218).
- **Docs:** `docs/tools.md`, `docs/examples.md`, `README.md` (tool count + retrieval-first flow).
- **No change** to `internal/store`, parsers, the registry, or any existing tool's output.
- **Dependencies:** none added; reuses the MCP Go SDK registration pattern already in `server.go`.
```

## openspec/changes/add-get-context-facade/design.md

- Source: openspec/changes/add-get-context-facade/design.md
- Lines: 1-83
- SHA256: be613b071c93f0df783671cbd0e6409ae5e87ccbb49e99b5e947905b728d2127

[TRUNCATED]

```md
## Context

candle exposes 13 MCP tools today, each a thin pure method on `*Tools` over the
SQLite store, registered in `internal/mcp/server.go`. Resolution goes through
`t.reg.Resolve(repo) -> (registry.RepoInfo, ok, err)`; `RepoInfo` carries `IndexID`,
`Repo` (`org/name`), `Branch`, and `Commit`. The precise tools already implement every
underlying query `get_context` needs to compose (`NodesByLabel`, `Callers`/`Callees`,
`ListAPISpecs`/`FindOperations`, `ListProtoFiles`/`FindRPCs`, `FindPrivateLibraries`/
`FindPrivateDeps`, plus `Tools.FindSchema`, `Tools.ExplainRPC`, `Tools.FindPrivateLibrary`).

This change is grounded in the existing plan at
`docs/superpowers/plans/2026-06-18-get-context-retrieval-facade.md`, validated against the
current codebase.

## Goals / Non-Goals

**Goals**
- One additive `Tools.GetContext` method exposing overview and topic retrieval.
- Reuse existing store/Tools queries — no new store methods, no parser changes.
- Codegraph-style one-hop caller/callee context for matched code symbols.
- Explicit, machine-readable `limitations` so agents know what is deferred.

**Non-Goals (deferred; surfaced as runtime `limitations` strings)**
- OpenAPI endpoint → handler implementation linking inside `get_context` v1.
- Cross-repo RPC `consumed_by` aggregation.
- Cross-repo private-library consumer aggregation.
- Embeddings / semantic search and multi-hop traversal (`depth > 1`; v1 supports one hop).

## Decisions

### D1: Pure method + thin registration, mirroring existing tools
`Tools.GetContext(GetContextArgs) (ContextResult, error)` holds all logic; `server.go` adds
a `registerGetContext` that marshals the result via the existing `textResult`/`mustJSON`/
`toolErr` helpers. Keeps `get_context` consistent with the other 13 tools and SDK-free in
the method itself.

### D2: Single tool, two modes, mode filter
`topic` empty → overview (capability catalog). `topic` set → topic retrieval. A `mode`
argument (`overview|code|api|proto|library|all`, default/unknown → `all`) narrows the
searched surfaces. This avoids proliferating tools while serving both Context7-style
discovery and focused lookup.

### D3: Typed repo summary, not `any`
The result's repo field SHALL be a typed value (embed/return `registry.RepoInfo` or a
dedicated typed repo-summary struct) so callers and tests can access `.Repo` and `.Commit`
directly. **This resolves the inconsistency in the source plan**, whose draft declared
`Repo any` yet whose test accesses `out.Repo.Repo` / `out.Repo.Commit` — that would not
compile. The build phase MUST use a typed field. The exact struct shape is a build-phase
detail; the constraint is: typed, with accessible `Repo` and `Commit`.

### D4: One-hop code context via existing edge queries
Code matches reuse `NodesByLabel` + `Callers`/`Callees` (one hop), matching the shape
`explain_symbol` already returns. `depth` is accepted but v1 honors only one hop; deeper
traversal is a non-goal recorded in `limitations`.

### D5: Resource URIs reuse existing schemes
`include_resources` emits URIs in the established `graph://`, `openapi://`, `proto://`,
`lib://` schemes (commit-pinned, falling back to `latest` when commit is empty), so hints
are directly usable against the resource layer.

## Risks / Trade-offs

- **Surface-count drift:** advertised tool count moves 13 → 14; `e2e_surface_test.go`
  comments and assertions (≈ lines 32, 218) must be updated or the suite fails. Mitigated
  by an explicit task.
- **Overlap with precise tools:** `get_context` intentionally duplicates entry points the
  precise tools expose. Accepted — it is a router, and responses point back to the precise
  tools via `suggested_next_calls`.
- **Result-type breadth:** several result sub-fields stay loosely typed (`any`) where they
  wrap existing heterogeneous results; D3 constrains only the repo field, which the tests
  exercise directly.

## Migration Plan

Additive only. No data migration, no behavior change to existing tools. Ship the method +
registration + test-surface bump together so `tools/list` and the e2e surface stay
consistent within a single change.

## Open Questions

```

Full source: openspec/changes/add-get-context-facade/design.md

## openspec/changes/add-get-context-facade/tasks.md

- Source: openspec/changes/add-get-context-facade/tasks.md
- Lines: 1-36
- SHA256: 5f0e7a624a6276213e6d0073b6b9f5415fffd0eb71524b91461ffd98f84e30bd

```md
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
```

## openspec/changes/add-get-context-facade/specs/context-retrieval/spec.md

- Source: openspec/changes/add-get-context-facade/specs/context-retrieval/spec.md
- Lines: 1-75
- SHA256: 1ca073c2557556f3b060fa6aba7343cd55df7c012e99c522a2f337e34786052a

```md
# context-retrieval Specification

## ADDED Requirements

### Requirement: get_context retrieval facade tool
The system SHALL provide a `get_context` MCP tool that is the primary, repo-scoped
retrieval entry point composing existing code-graph, OpenAPI, protobuf, and private-library
queries. It SHALL require a `repo` argument and accept optional `topic`, `mode`, `depth`,
and `include_resources` arguments. It SHALL be additive: existing tools and their outputs
remain unchanged.

#### Scenario: Unknown repo returns not-found
- **WHEN** `get_context` is called with a `repo` that does not resolve to an indexed repo
- **THEN** it returns a not-found error (`ErrNotFound`) and no partial result

#### Scenario: Tool is advertised
- **WHEN** an MCP client sends `tools/list`
- **THEN** `get_context` appears in the advertised tool set, registered after `resolve_repo`

### Requirement: Overview mode catalogs repo capabilities
When called with `repo` and no `topic`, `get_context` SHALL return a repo snapshot summary
plus a capability catalog covering code graph, OpenAPI, protobuf, and private libraries.
Each capability entry SHALL report whether it is available, a count, and the precise
follow-up tools that serve that surface. The response SHALL include suggested next calls,
resource schemes, and explicit limitations.

#### Scenario: Overview returns repo summary and capabilities
- **WHEN** `get_context` is called with `{repo}` for an indexed repo
- **THEN** it returns the repo identity and commit, an empty `topic`, and capability
  summaries with correct availability and counts for code graph, OpenAPI, protobuf, and
  private libraries

#### Scenario: Overview suggests follow-up calls and resource schemes
- **WHEN** `get_context` is called in overview mode
- **THEN** the response includes a non-empty list of suggested next tool calls and a
  non-empty list of resource schemes

### Requirement: Topic mode retrieves focused context across surfaces
When called with `repo` and a non-empty `topic`, `get_context` SHALL search code symbols,
HTTP operations, schemas, proto messages, RPCs, and private libraries for the topic. Code
symbol matches SHALL include one-hop callers and callees (codegraph-style one-hop context).
When `include_resources` is true, matched items SHALL also yield exact resource-URI hints.

#### Scenario: Topic search spans all surfaces with one-hop code context
- **WHEN** `get_context` is called with `{repo, topic, include_resources:true}` where the
  topic matches a code symbol, a schema, and an RPC
- **THEN** it returns the matching code symbol with its one-hop callers and callees, the
  matched schema, the matched RPC, and a non-empty list of resource URIs

### Requirement: Mode filter narrows searched surfaces
`get_context` SHALL accept a `mode` argument of `overview`, `code`, `api`, `proto`,
`library`, or `all`. Unrecognized or empty modes SHALL be treated as `all`. A specific
non-`all` mode in topic search SHALL restrict matches to that surface only.

#### Scenario: code mode returns only code matches
- **WHEN** `get_context` is called with `{repo, topic, mode:"code"}`
- **THEN** the result contains code symbol matches and contains no endpoint, schema, RPC,
  or private-library matches

#### Scenario: overview mode returns catalog only and suppresses topic matches
- **WHEN** `get_context` is called with `{repo, topic, mode:"overview"}` where the topic
  would otherwise match one or more surfaces
- **THEN** the result contains the capability catalog and no topic matches (no code, endpoint,
  schema, RPC, or private-library matches)

### Requirement: Responses declare deferred limitations
Every `get_context` response SHALL include an explicit `limitations` list naming
deferred behavior, including that OpenAPI endpoint-to-handler implementation linking is
not yet available, and that cross-repo RPC consumer and cross-repo private-library
consumer aggregation are deferred.

#### Scenario: Limitations are present
- **WHEN** `get_context` returns any successful result
- **THEN** the `limitations` list is non-empty and names the deferred OpenAPI linking and
  cross-repo aggregation behaviors
```


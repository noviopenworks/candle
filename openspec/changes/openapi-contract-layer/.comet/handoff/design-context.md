# Comet Design Handoff

- Change: openapi-contract-layer
- Phase: design
- Mode: compact
- Context hash: 9795a57dd9db7b8c412f6a41787eecd3f5dd885a001e75524dc32846002dbfde

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/openapi-contract-layer/proposal.md

- Source: openspec/changes/openapi-contract-layer/proposal.md
- Lines: 1-28
- SHA256: db5414272ca6a1afc4101527e132fd4bb3472d40b3d1baa9f93a4b6ae1e1f79f

```md
## Why

Service relationships defined by HTTP APIs are invisible in a pure code-symbol graph. An OpenAPI spec says an operation exists, but only by linking that operation back to the Go handler symbol (and onward to its service/repository calls) can an agent answer "which handler implements `reserveProduct`?" or "what's the request/response schema and call flow for `POST /products/{id}/reservations`?". This change adds the OpenAPI contract layer on top of the core foundation.

This is split change **2 of 4** of the MVP. It **depends on `mcp-core-foundation`** (server, storage, repo registry, code-symbol graph) and is independent of the protobuf and Go-library changes.

## What Changes

- **OpenAPI parser**: scan `openapi.{yaml,yml,json}`, `swagger.{yaml,json}`, `api/**/*.{yaml,json}`; extract title, version, servers, paths, methods, `operationId`, tags, summary, parameters, request/response schemas, error responses, security, referenced components.
- **Storage**: `api_specs`, `http_operations`, `api_schemas` tables (keyed off `index_id`).
- **Contract→code linking**: `http_operation → implemented_by` handler symbol, `→ uses_schema` request/response, `→ calls` service method — by joining against the Graphify code graph.
- **Tools**: `list_apis` (introduced here for HTTP; extended for proto in change 3), `find_endpoint`, `explain_endpoint` (returns handler + `service_flow`), `find_schema` (OpenAPI side).
- **Resources**: `openapi://org/repo/commit/<sha>/{spec|operation|schema}/…`.

## Capabilities

### New Capabilities
- `openapi-index`: parse OpenAPI specs into storage and link operations/schemas to Graphify code symbols.
- `openapi-tools`: `list_apis`, `find_endpoint`, `explain_endpoint`, `find_schema` (HTTP), plus `openapi://` resources.

### Modified Capabilities
<!-- None yet; `list_apis` is introduced here and later extended by protobuf-contract-layer. -->

## Impact

- Depends on the `index_id`/`repo` conventions and code-node graph from `mcp-core-foundation`.
- **Operation→handler linkage is the core technical risk** (heuristic name/route matching vs. annotation parsing) — resolved in this change's design phase.
- `list_apis` output shape must be forward-compatible with protobuf entries added in change 3.
```

## openspec/changes/openapi-contract-layer/design.md

- Source: openspec/changes/openapi-contract-layer/design.md
- Lines: 1-39
- SHA256: a6857c32b500fb967268bb410d7c27a97208aed18a80548868a5247e0d1d4215

```md
# Design — openapi-contract-layer (high-level)

> Open-phase design: architecture decisions and approach only. Detailed Design Doc + delta specs come in the design phase.

## Architecture

```
 OpenAPI specs in repo            openapi-contract-layer
 ┌──────────────────────┐  parse  ┌───────────────────────────────────────┐
 │ openapi.yaml / .json │ ──────▶ │ api_specs / http_operations / api_schemas│
 │ swagger.* / api/**   │         │            (index_id-scoped)             │
 └──────────────────────┘         │            │ link                        │
                                  │            ▼                             │
                                  │   Graphify code nodes (from foundation)  │
                                  │     implemented_by / uses_schema / calls │
                                  │   tools: list_apis, find_endpoint,       │
                                  │          explain_endpoint, find_schema   │
                                  │   resources: openapi://…                 │
                                  └───────────────────────────────────────┘
```

## Key Decisions

1. **Parse with a maintained OpenAPI library** (e.g. `kin-openapi`) rather than hand-rolling YAML/JSON + `$ref` resolution. Supports OpenAPI 3.x; Swagger 2.0 handling decided in design phase.
2. **`$ref` resolution + component flattening** at parse time so schema names are stable join keys.
3. **Linking strategy (the crux)**: the codebase uses the **chi** router, so the strongest signal is chi route registration — `r.Method("/path", handler.Fn)` / `r.Post(...)` / `r.Route(...)` call sites that bind a method+path directly to a handler symbol. Primary plan: extract chi route→handler bindings from the code graph and match them to OpenAPI `(method, path)`; fall back to `operationId` ↔ handler-func-name and path/tag heuristics. Each link carries a confidence. Final algorithm is the main design-phase output.
4. **`list_apis` shape** carries a `kind` discriminator (`openapi` now, `protobuf` later) so change 3 extends without breaking.

## Approach Selection

- `find_endpoint`: match natural language / path / method / `operationId` against indexed operations (lexical + simple ranking).
- `explain_endpoint`: assemble OpenAPI facts + linked handler, then walk `calls` edges in the code graph to build `service_flow` and `related_files`.
- `find_schema`: search `api_schemas` by name/purpose; returns OpenAPI matches (proto matches added in change 3).

## Open Questions (for design phase)

- Operation→handler linkage algorithm + confidence model (primary risk).
- Swagger 2.0 support in MVP or defer.
- Mapping OpenAPI schema → generated Go struct (needed for full `uses_schema`).
```

## openspec/changes/openapi-contract-layer/tasks.md

- Source: openspec/changes/openapi-contract-layer/tasks.md
- Lines: 1-35
- SHA256: fa51dba603ab2ea44fe8987e7e21bec1051cbd681b15ea18e3528d45504ffebe

```md
# Tasks — openapi-contract-layer

> Scope: pure contract serving (parse + serve). Operation→handler linking and service_flow are deferred to a future change (see design doc). Refined against the Design Doc + delta specs.

## 1. Manifest + storage
- [ ] 1.1 Extend `RepoConfig` with `openapi []string` (`mapstructure:"openapi"`); resolve relative to manifest dir
- [ ] 1.2 Add `api_specs`, `http_operations`, `api_schemas` tables + indexes to `schemaSQL`; migration

## 2. OpenAPI parsing (`internal/openapi`)
- [ ] 2.1 Parse OpenAPI 3.x with `kin-openapi`; resolve `$ref`, flatten components
- [ ] 2.2 Detect + skip Swagger 2.0 with a warning
- [ ] 2.3 Normalize spec meta, operations (method/path/operationId/summary/schemas/security/tags), and schemas
- [ ] 2.4 Tolerate missing/malformed spec files (skip + warn)

## 3. Spec indexing
- [ ] 3.1 Store parsed specs/operations/schemas under the repo's `index_id`
- [ ] 3.2 Idempotent re-index (delete+reinsert per index_id, cascade by api_spec_id)
- [ ] 3.3 Wire spec indexing into the `index` flow after graph ingest

## 4. Tools (pure functions, registered with the base tools)
- [ ] 4.1 `list_apis` (kind-discriminated, forward-compatible with protobuf)
- [ ] 4.2 `find_endpoint` (lexical match: NL / path / method / operationId)
- [ ] 4.3 `explain_endpoint` (contract data only — no handler/service_flow)
- [ ] 4.4 `find_schema` (OpenAPI schema matches)

## 5. Resources
- [ ] 5.1 `openapi://…/spec/<path>`
- [ ] 5.2 `openapi://…/operation/<operationId>`
- [ ] 5.3 `openapi://…/schema/<schemaName>`

## 6. Verification
- [ ] 6.1 Sample spec parsed, operations/schemas indexed, `list_apis` returns it
- [ ] 6.2 `explain_endpoint` returns correct contract data on a fixture
- [ ] 6.3 Swagger 2.0 / missing / malformed specs skipped (warn, no crash); unknown repo/endpoint/schema → empty/not-found
- [ ] 6.4 End-to-end: index fixture repo, serve over stdio, `list_apis` + `explain_endpoint`
```

## openspec/changes/openapi-contract-layer/specs/openapi-index/spec.md

- Source: openspec/changes/openapi-contract-layer/specs/openapi-index/spec.md
- Lines: 1-34
- SHA256: e44a3b7a6f5ae607b8d7fdd698aab95b356355f49b3d013d24eb8df317dceadb

```md
## ADDED Requirements

### Requirement: OpenAPI spec discovery via manifest
The system SHALL read the OpenAPI spec file paths for a repo from an explicit `openapi:` list in that repo's manifest entry. The system SHALL NOT auto-discover specs by globbing the filesystem.

#### Scenario: Specs listed in manifest are indexed
- **WHEN** a manifest entry declares `openapi: [api/openapi.yaml]` and the file exists
- **THEN** that spec is parsed and stored under the repo's `index_id`

#### Scenario: Repo without openapi list indexes no specs
- **WHEN** a manifest entry has no `openapi:` field
- **THEN** the repo indexes successfully with zero API specs and no error

### Requirement: Parse OpenAPI 3.x into storage
The system SHALL parse OpenAPI 3.0/3.1 documents, resolving `$ref` and flattening components, and persist normalized spec metadata, HTTP operations, and schemas in tables tied to the repo's `index_id`.

#### Scenario: Operations and schemas are normalized
- **WHEN** an OpenAPI 3.x spec with paths and component schemas is indexed
- **THEN** each `(path, method)` is stored as an http_operation with operation_id, summary, request/response schema names, security, and tags, and each component schema is stored as an api_schema

#### Scenario: Swagger 2.0 is skipped with a warning
- **WHEN** a referenced spec file declares `swagger: "2.0"`
- **THEN** it is skipped with a warning and does not abort indexing of other specs

#### Scenario: Malformed or missing spec is tolerated
- **WHEN** a referenced spec file is missing or fails to parse
- **THEN** it is skipped with a warning and the rest of the run continues

### Requirement: Idempotent spec indexing
The system SHALL make spec indexing idempotent per `index_id`: re-indexing replaces that repo's API spec/operation/schema rows without duplication.

#### Scenario: Re-indexing the same repo does not duplicate
- **WHEN** a repo with one spec is indexed twice
- **THEN** the api_specs / http_operations / api_schemas counts are identical after the second run
```

## openspec/changes/openapi-contract-layer/specs/openapi-tools/spec.md

- Source: openspec/changes/openapi-contract-layer/specs/openapi-tools/spec.md
- Lines: 1-44
- SHA256: 28faa263a3628caaf8fee000dda8419e4525611fa9e72880806f69aedaa9b75b

```md
## ADDED Requirements

### Requirement: list_apis lists OpenAPI contracts
The system SHALL provide `list_apis` returning the OpenAPI contracts indexed for a repo, each carrying a `kind` discriminator so other contract kinds (e.g. protobuf) can be added without breaking the output shape.

#### Scenario: Indexed specs are listed
- **WHEN** `list_apis` is called for a repo with one indexed OpenAPI spec
- **THEN** it returns an entry `{kind:"openapi", name, version, path}` for that spec

### Requirement: find_endpoint locates operations
The system SHALL provide `find_endpoint` that matches indexed operations by natural language, path, HTTP method, or operationId.

#### Scenario: Match by operationId
- **WHEN** `find_endpoint` is called with a query equal to a known operationId
- **THEN** the matching operation (method, path, operation_id, spec_path) is returned

#### Scenario: Match by path or method
- **WHEN** `find_endpoint` is called with a path fragment or method
- **THEN** operations whose path or method match are returned

### Requirement: explain_endpoint returns contract data
The system SHALL provide `explain_endpoint` that returns the OpenAPI contract data for a `(method, path)`: summary, operationId, request schema, response schema, security, tags, and spec path. It SHALL NOT return handler implementation or a service call flow in this change (linking is deferred).

#### Scenario: Contract data returned
- **WHEN** `explain_endpoint` is called with a known method and path
- **THEN** it returns the summary, operation_id, request_schema, response_schema, security, tags, and spec_path

#### Scenario: Unknown endpoint returns not-found
- **WHEN** `explain_endpoint` is called with a method/path that is not indexed
- **THEN** it returns a structured not-found result, not an error/crash

### Requirement: find_schema locates OpenAPI schemas
The system SHALL provide `find_schema` that returns OpenAPI schemas matching a query by name.

#### Scenario: Schema found by name
- **WHEN** `find_schema` is called with a query matching a component schema name
- **THEN** it returns `{kind:"openapi_schema", name, spec_path}` for that schema

### Requirement: OpenAPI resources
The system SHALL expose `openapi://` resources for a spec, an operation, and a schema, commit-pinned from manifest metadata and degrading to branch or latest when no commit is recorded.

#### Scenario: Operation resource returns the operation
- **WHEN** a client reads `openapi://org/name/commit/<sha>/operation/<operationId>` for an indexed operation
- **THEN** it returns that operation's contract data
```


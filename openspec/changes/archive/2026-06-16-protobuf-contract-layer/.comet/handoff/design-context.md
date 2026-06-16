# Comet Design Handoff

- Change: protobuf-contract-layer
- Phase: design
- Mode: compact
- Context hash: ef40027461aa61c57d26a5e87e0e38e72f263fc32e6e1716ed8f5ebc5a3b8b68

Generated-by: comet-handoff.sh

OpenSpec remains the canonical capability spec. This handoff is a deterministic, source-traceable context pack, not an agent-authored summary.

## openspec/changes/protobuf-contract-layer/proposal.md

- Source: openspec/changes/protobuf-contract-layer/proposal.md
- Lines: 1-28
- SHA256: 5ba1293ba587c0f12c01c260df3d3b9eaf97324a5ab1d021c64b7fa14d5eaa8d

```md
## Why

gRPC-heavy systems define their service boundaries in `.proto` files, not in code symbols alone. To answer "which proto service owns this RPC?", "where is `ReserveProductRequest` used?", or "which services consume `InventoryService.ReserveProduct`?", the system must parse protobuf contracts, link each RPC to its gRPC server implementation, and resolve cross-repo consumers. This change adds the protobuf contract layer.

This is split change **3 of 4** of the MVP. It **depends on `mcp-core-foundation`** and is independent of the OpenAPI and Go-library changes. It **extends** `list_apis`/`find_schema` introduced by `openapi-contract-layer` (coordinate the shared shape; do not break it).

## What Changes

- **Protobuf parser**: scan `proto/**/*.proto`, `api/**/*.proto`, `internal/**/*.proto`; extract package, imports, services, RPCs, request/response messages, message fields, enums, options, generated Go package.
- **Storage**: `proto_files`, `proto_services`, `proto_rpcs`, `proto_messages` tables (index_id-scoped).
- **Contract→code linking**: `rpc → implemented_by` gRPC server method, `→ uses_message` request/response, `→ calls` domain service; **cross-repo `consumed_by`** from client call sites in other repos' merged graphs.
- **Tools**: `find_rpc`, `explain_rpc` (includes `consumed_by`), `find_schema` extended to return proto messages; `list_apis` extended to include protobuf entries.
- **Resources**: `proto://org/repo/commit/<sha>/{file|service|rpc|message}/…`.

## Capabilities

### New Capabilities
- `protobuf-index`: parse `.proto` files into storage and link RPCs/messages to Graphify code symbols, including cross-repo consumer detection.
- `protobuf-tools`: `find_rpc`, `explain_rpc`, plus `proto://` resources.

### Modified Capabilities
- `openapi-tools`: `list_apis` and `find_schema` gain protobuf results (additive `kind`-discriminated entries; no breaking change to HTTP output).

## Impact

- Depends on `index_id`/`repo`/code-graph conventions from `mcp-core-foundation` and the `list_apis`/`find_schema` shape from `openapi-contract-layer`.
- **Cross-repo `consumed_by`** is the hard part: it needs the merged multi-repo graph plus matching client call sites to a proto package/service — primary risk, resolved in design phase.
- RPC→server-impl linkage shares the linker design with the OpenAPI handler linker; reuse where possible.
```

## openspec/changes/protobuf-contract-layer/design.md

- Source: openspec/changes/protobuf-contract-layer/design.md
- Lines: 1-42
- SHA256: a33bb7e9cdc7c4daf23cc01262c8314e525707673b8980cb0d512f275556c5ee

```md
# Design — protobuf-contract-layer (high-level)

> Open-phase design: decisions and approach only. Detailed Design Doc + delta specs come in the design phase.

## Architecture

```
 .proto files in repos            protobuf-contract-layer
 ┌──────────────────────┐  parse  ┌────────────────────────────────────────┐
 │ proto/**/*.proto      │ ──────▶ │ proto_files / proto_services /          │
 │ api/** internal/**    │         │ proto_rpcs / proto_messages (index_id)  │
 └──────────────────────┘         │            │ link                         │
                                  │            ▼                              │
                                  │  code nodes (impl) + merged cross-repo    │
                                  │  graph (consumers)                        │
                                  │  implemented_by / uses_message / calls /  │
                                  │  consumed_by                              │
                                  │  tools: find_rpc, explain_rpc;            │
                                  │  extends list_apis, find_schema           │
                                  │  resources: proto://…                     │
                                  └────────────────────────────────────────┘
```

## Key Decisions

1. **Parse protos with a real grammar**, not regex — e.g. `protoparse` (jhump/protoreflect) or `protocompile`. Handles imports, nested messages, enums, options. Library choice finalized in design phase.
2. **Resolve imports across files** so message references and the generated Go package option are stable join keys.
3. **RPC→impl linkage** reuses the foundation/OpenAPI linker: match `Service.Rpc` against gRPC server method symbols (generated `RegisterXServer` patterns + method names) in the code graph.
4. **Cross-repo `consumed_by`**: search the *merged* multi-repo graph for client call sites referencing the proto package/service (generated client stubs). This is the primary risk and the main design-phase output.
5. **Shared `list_apis`/`find_schema` shape** with `openapi-contract-layer`: add proto entries via the `kind` discriminator; never break HTTP output.

## Approach Selection

- `find_rpc`: match NL / service / method against indexed RPCs.
- `explain_rpc`: proto facts + linked server impl + `calls` walk + `consumed_by` consumer repos.
- `find_schema` (extension): include `proto_message` matches alongside OpenAPI schemas.

## Open Questions (for design phase)

- Cross-repo consumer detection precision (generated stub recognition) and confidence model.
- Protobuf parsing library selection.
- Whether to index streaming RPC semantics in MVP.
```

## openspec/changes/protobuf-contract-layer/tasks.md

- Source: openspec/changes/protobuf-contract-layer/tasks.md
- Lines: 1-39
- SHA256: 02ceead1d9d5ab2e03b7eb6d7ef99abb9695cc4f8fc4fe227b294d28aaabc247

```md
# Tasks — protobuf-contract-layer

> Refined against design doc `docs/superpowers/specs/2026-06-16-protobuf-contract-layer-design.md`
> and delta specs. Scope: parse + same-repo linking; cross-repo `consumed_by` deferred.

## 1. Storage
- [ ] 1.1 Add `proto_files`, `proto_services`, `proto_rpcs`, `proto_messages`, `proto_enums`, `proto_rpc_impls` tables (index_id-scoped) to `schema.go`
- [ ] 1.2 `internal/store/proto.go`: bundle types + `ReplaceProtoFiles` (idempotent per index_id); impl-link write/read; find/lookup queries

## 2. Protobuf parsing
- [ ] 2.1 Add `proto: { roots, files }` block to `RepoConfig` (`internal/config`)
- [ ] 2.2 `internal/proto`: bufbuild/protocompile compiler with SourceResolver over roots + well-known types; expand directory entries
- [ ] 2.3 Extract services, RPCs (request/response message names + `stream_kind`), messages (fields), enums (values), file package + go_package + imports
- [ ] 2.4 Normalize into store bundles; tolerate missing/malformed/unresolvable files with warnings

## 3. Contract → code linking
- [ ] 3.1 `internal/link` (new shared package): RPC→server-impl matcher — name + service association + streaming-aware signature check; confidence tiers + match_reason
- [ ] 3.2 Run linker in `ingest.Run` after `graph.Load`; persist `proto_rpc_impls`
- [ ] 3.3 `uses_message` via resolvable request/response message references (no separate table)
- [ ] 3.4 ~~Cross-repo `consumed_by`~~ **DEFERRED to a future change** (out of scope; `explain_rpc` returns a deferred marker)

## 4. Tools
- [ ] 4.1 `find_rpc` (lexical match + optional `stream_kind` filter)
- [ ] 4.2 `explain_rpc` (proto facts + resolved messages + `implemented_by` + best-effort one-hop `calls` + deferred `consumed_by` marker)
- [ ] 4.3 Extend `list_apis` with `{kind:"protobuf"}` entries (HTTP output unchanged)
- [ ] 4.4 Extend `find_schema` with `{kind:"proto_message"}` entries

## 5. Resources
- [ ] 5.1 `proto://…/file/<path>`
- [ ] 5.2 `proto://…/service/<package>/<service>`
- [ ] 5.3 `proto://…/rpc/<package>/<service>/<rpc>`
- [ ] 5.4 `proto://…/message/<package>/<message>`

## 6. Verification
- [ ] 6.1 Parser unit tests: cross-file imports, nested messages, enums, options, go_package, all four stream kinds
- [ ] 6.2 Storage idempotency: re-index → identical row counts; impl links cleared
- [ ] 6.3 Linker tests: HIGH-confidence match, streaming signature disambiguation, no false-positive on unrelated same-named method, ambiguous → LOW confidence
- [ ] 6.4 Tool/resource tests: `find_rpc` filter, `explain_rpc` impl + one-hop calls + deferred marker, not-found behavior
- [ ] 6.5 Regression: `list_apis`/`find_schema` HTTP output unchanged
```

## openspec/changes/protobuf-contract-layer/specs/openapi-tools/spec.md

- Source: openspec/changes/protobuf-contract-layer/specs/openapi-tools/spec.md
- Lines: 1-23
- SHA256: 82666f6fdf9960602ebdf66102e02117d4890c79dcb1d609c3fa4478fcc663d8

```md
## MODIFIED Requirements

### Requirement: list_apis lists API contracts
The system SHALL provide `list_apis` returning the API contracts indexed for a repo, each carrying a `kind` discriminator. It SHALL return OpenAPI specs as `{kind:"openapi", …}` entries and protobuf files as `{kind:"protobuf", …}` entries, so additional contract kinds can be added without breaking the output shape.

#### Scenario: Indexed OpenAPI specs are listed
- **WHEN** `list_apis` is called for a repo with one indexed OpenAPI spec
- **THEN** it returns an entry `{kind:"openapi", name, version, path}` for that spec

#### Scenario: Indexed protobuf files are listed
- **WHEN** `list_apis` is called for a repo with one indexed protobuf file
- **THEN** it returns an entry `{kind:"protobuf", name, version, path}` for that file alongside any OpenAPI entries

### Requirement: find_schema locates schemas and messages
The system SHALL provide `find_schema` that returns OpenAPI schemas and protobuf messages matching a query by name, each carrying a `kind` discriminator.

#### Scenario: OpenAPI schema found by name
- **WHEN** `find_schema` is called with a query matching a component schema name
- **THEN** it returns `{kind:"openapi_schema", name, spec_path}` for that schema

#### Scenario: Proto message found by name
- **WHEN** `find_schema` is called with a query matching a proto message name
- **THEN** it returns `{kind:"proto_message", name, spec_path}` for that message alongside any OpenAPI schema matches
```

## openspec/changes/protobuf-contract-layer/specs/protobuf-index/spec.md

- Source: openspec/changes/protobuf-contract-layer/specs/protobuf-index/spec.md
- Lines: 1-64
- SHA256: fa71320ea99238359bbf8e5c88ebb5470497226498e7eba3909de533c8fd44c8

```md
## ADDED Requirements

### Requirement: Protobuf discovery via manifest
The system SHALL read the protobuf import roots and entry files for a repo from an explicit `proto:` block in that repo's manifest entry, containing `roots` (import-resolution roots) and `files` (entry `.proto` files or directories to index). The system SHALL NOT auto-discover `.proto` files by globbing the filesystem.

#### Scenario: Protos listed in manifest are indexed
- **WHEN** a manifest entry declares `proto: { roots: [proto], files: [proto/inventory.proto] }` and the file exists
- **THEN** that proto file is parsed and stored under the repo's `index_id`

#### Scenario: Directory entry expands to contained protos
- **WHEN** a manifest `proto.files` entry names a directory
- **THEN** the `.proto` files beneath that directory are indexed

#### Scenario: Repo without proto block indexes no protos
- **WHEN** a manifest entry has no `proto:` block
- **THEN** the repo indexes successfully with zero proto files and no error

### Requirement: Parse protobuf into storage
The system SHALL compile `.proto` files with a real protobuf grammar (bufbuild/protocompile) using a source resolver rooted at the manifest `roots` plus bundled well-known types, resolving imports across files, and persist normalized proto files, services, RPCs, messages, and enums in tables tied to the repo's `index_id`.

#### Scenario: Services, RPCs, messages, and enums are normalized
- **WHEN** a `.proto` file with a service, RPCs, messages, and enums is indexed
- **THEN** the proto file (path, package, go_package option, imports) is stored, each service is stored with its full name, each RPC is stored with its full name, fully-qualified request and response message names, and `stream_kind`, each message is stored with its fields, and each enum is stored with its values

#### Scenario: Streaming kind is classified
- **WHEN** an RPC is declared `unary`, server-streaming, client-streaming, or bidirectional
- **THEN** its `stream_kind` is stored as `unary`, `server_stream`, `client_stream`, or `bidi` respectively

#### Scenario: Malformed or unresolvable proto is tolerated
- **WHEN** a referenced proto file is missing, fails to parse, or has an unresolvable import
- **THEN** it is skipped with a warning and the rest of the run continues

### Requirement: Same-repo RPC implementation linking
The system SHALL link each proto RPC to its gRPC server implementation within the same repo's code graph, recording zero or more candidate implementations with a confidence value and a match reason. Linking SHALL run after the code graph for the repo is loaded. Cross-repo consumer linking (`consumed_by`) is out of scope for this change.

#### Scenario: RPC links to its server method
- **WHEN** a repo's code graph contains a gRPC server method matching an indexed RPC by name and service association
- **THEN** a `proto_rpc_impls` record is stored referencing that code node with a confidence value and match reason

#### Scenario: Streaming signature disambiguates a match
- **WHEN** a candidate server method's source signature shape matches the RPC's `stream_kind`
- **THEN** the implementation link is recorded at higher confidence than a name-only match

#### Scenario: Ambiguous match is recorded, not guessed
- **WHEN** multiple candidate methods match an RPC by name with no disambiguating signal
- **THEN** the candidates are recorded at low confidence rather than collapsed into a single false-positive or dropped

#### Scenario: No implementation found is a valid result
- **WHEN** no code node matches an indexed RPC
- **THEN** the RPC is stored with zero implementation links and no error

### Requirement: uses_message reference
The system SHALL represent the `uses_message` relationship as each RPC's resolvable fully-qualified request and response message names, which resolve to stored `proto_messages` rows within the same `index_id`.

#### Scenario: Request and response resolve to messages
- **WHEN** an RPC references request and response messages that are indexed in the same repo
- **THEN** those message names resolve to the stored message rows (with fields) for that index

### Requirement: Idempotent proto indexing
The system SHALL make proto indexing idempotent per `index_id`: re-indexing replaces that repo's proto file/service/rpc/message/enum and implementation-link rows without duplication.

#### Scenario: Re-indexing the same repo does not duplicate
- **WHEN** a repo with one proto file is indexed twice
- **THEN** the proto_files / proto_services / proto_rpcs / proto_messages / proto_enums / proto_rpc_impls counts are identical after the second run
```

## openspec/changes/protobuf-contract-layer/specs/protobuf-tools/spec.md

- Source: openspec/changes/protobuf-contract-layer/specs/protobuf-tools/spec.md
- Lines: 1-38
- SHA256: 76a71289d3ce0346ea7c9b69e87df6bcded07b682478a0ddb50ab4ce20556c3d

```md
## ADDED Requirements

### Requirement: find_rpc locates RPCs
The system SHALL provide `find_rpc` that matches indexed proto RPCs by natural language, service, RPC name, or fully-qualified name, and SHALL accept an optional `stream_kind` filter.

#### Scenario: Match by RPC or service name
- **WHEN** `find_rpc` is called with a query matching an indexed RPC or service name
- **THEN** matching RPCs are returned with `{full_name, service, rpc, request_message, response_message, stream_kind, proto_path}`

#### Scenario: Filter by stream_kind
- **WHEN** `find_rpc` is called with a `stream_kind` filter
- **THEN** only RPCs whose `stream_kind` matches the filter are returned

### Requirement: explain_rpc returns contract data and same-repo implementation
The system SHALL provide `explain_rpc` that returns, for a `(service, rpc)`: the RPC's proto facts (`full_name`, `stream_kind`, request/response message names and their fields), the same-repo implementations (`implemented_by` with node reference, confidence, and match reason), and a best-effort one-hop `calls` expansion from the implementation node. The `consumed_by` field SHALL return an explicit deferred marker, since cross-repo consumer linking is out of scope for this change.

#### Scenario: Contract data and implementation returned
- **WHEN** `explain_rpc` is called with a known service and RPC that has a linked implementation
- **THEN** it returns the RPC proto facts, resolved request/response messages, `implemented_by` entries with confidence, a one-hop `calls` expansion from the implementation node, and a deferred `consumed_by` marker

#### Scenario: consumed_by is a deferred marker, not an error
- **WHEN** `explain_rpc` is called for any RPC
- **THEN** `consumed_by` is an explicit "deferred / not available in this change" marker rather than an error or omitted field

#### Scenario: Unknown RPC returns not-found
- **WHEN** `explain_rpc` is called with a service/RPC that is not indexed
- **THEN** it returns a structured not-found result, not an error/crash

### Requirement: Protobuf resources
The system SHALL expose `proto://` resources for a proto file, a service, an RPC, and a message, commit-pinned from manifest metadata and degrading to branch or latest when no commit is recorded.

#### Scenario: RPC resource returns the RPC
- **WHEN** a client reads `proto://org/name/commit/<sha>/rpc/<package>/<service>/<rpc>` for an indexed RPC
- **THEN** it returns that RPC's contract data and same-repo implementation references

#### Scenario: Message resource returns the message
- **WHEN** a client reads `proto://org/name/commit/<sha>/message/<package>/<message>` for an indexed message
- **THEN** it returns that message's fields
```


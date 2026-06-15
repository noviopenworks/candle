# Brainstorm Summary

- Change: openapi-contract-layer
- Date: 2026-06-15

## Confirmed Technical Approach

- **Scope (reduced from proposal): pure contract serving.** Parse OpenAPI specs and serve the contract data. NO operation→handler linking, NO `service_flow`. Linking (chi-route AST → handler) is deferred to a future change. Reason: route path strings are not in the Graphify graph, so precise linking needs Go-source AST parsing — descoped to de-risk this layer.
- **Spec discovery**: explicit `openapi: [paths]` list per manifest entry (no globbing, no source-root scan). Extends the foundation's `RepoConfig`.
- **Parser**: `github.com/getkin/kin-openapi`, **OpenAPI 3.0/3.1 only**. Swagger 2.0 detected and skipped with a warning (deferred).
- **Storage** (new tables, tied to foundation `index_id`): `api_specs(index_id, kind, name, version, path)`, `http_operations(api_spec_id, method, path, operation_id, summary, request_schema, response_schema, security, tags)`, `api_schemas(api_spec_id, name, kind, raw_ref)`.
- **Indexing**: extend the `index` flow — for each manifest repo, after graph ingest, parse its `openapi:` specs into the same `index_id`. Idempotent (delete+reinsert per index_id).
- **Tools** (pure functions, mirroring foundation pattern, registered alongside the 5 base tools): `list_apis` (kind-discriminated, forward-compat for proto), `find_endpoint` (lexical match on NL/path/method/operationId), `explain_endpoint` (contract data only — summary, operationId, request/response schema, security, tags, spec_path; NO handler/service_flow), `find_schema` (OpenAPI schema matches).
- **Resources**: `openapi://repo/commit/<sha>/{spec/<path>|operation/<operationId>|schema/<name>}`.
- **Reuse**: `registry.Resolve` for repo→index_id; same MCP SDK adapter pattern; pure-Go SQLite; same ErrNotFound empty-not-error convention.

## Key Trade-offs and Risks

- Descoping linking removes the highest-value cross-cut (which handler implements this) but also the highest risk; revisit in a dedicated linking change once a chi-AST approach is designed.
- Explicit spec paths in manifest = more user maintenance but zero discovery ambiguity.
- kin-openapi 3.x-only leaves Swagger 2.0 repos unindexed (warned, not errored).
- `request_schema`/`response_schema` stored as schema names/refs (strings), not resolved Go structs (that mapping was part of linking — deferred).

## Testing Strategy

- Unit: parser (fixture openapi.yaml → normalized operations/schemas), store CRUD for api_* tables, idempotent re-index.
- Golden: each tool's JSON output against a fixture spec.
- Degradation: missing spec file (warn+skip), Swagger 2.0 (skip+warn), unknown repo/endpoint/schema → empty/ErrNotFound, malformed spec → skip with warning.
- E2E: index a fixture repo, serve, call list_apis + explain_endpoint over stdio.

## Spec Patches

- Delta specs `openapi-index` and `openapi-tools` will be CREATED this phase reflecting pure-serving scope (open phase produced only proposal/design/tasks). Acceptance scenarios must NOT include implemented_by/service_flow.
- tasks.md will be revised: remove linking tasks (operation→handler linker, uses_schema Go-struct mapping, calls walk); keep parse/store/serve/resources/verify.
- proposal.md left as-is (original intent); design doc records the linking descope + deferral.

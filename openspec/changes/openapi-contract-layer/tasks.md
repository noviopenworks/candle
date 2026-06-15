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

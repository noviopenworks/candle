# Tasks — openapi-contract-layer

> Open-phase outline. Refined against the Design Doc + delta specs.

## 1. Storage
- [ ] 1.1 Add `api_specs`, `http_operations`, `api_schemas` tables (index_id-scoped); migration

## 2. OpenAPI parsing
- [ ] 2.1 Discover spec files (openapi/swagger/api globs)
- [ ] 2.2 Parse with OpenAPI library; resolve `$ref` and flatten components
- [ ] 2.3 Normalize operations (method, path, operationId, schemas, security, tags) into storage
- [ ] 2.4 Normalize schemas into `api_schemas`

## 3. Contract → code linking
- [ ] 3.1 Implement operation→handler linker (design-phase algorithm) with confidence
- [ ] 3.2 `uses_schema` edges (request/response → schema; schema → Go struct where resolvable)
- [ ] 3.3 `calls` walk from handler into service/repository symbols

## 4. Tools
- [ ] 4.1 `list_apis` (HTTP entries; forward-compatible `kind` field)
- [ ] 4.2 `find_endpoint`
- [ ] 4.3 `explain_endpoint` (handler + service_flow + related_files)
- [ ] 4.4 `find_schema` (OpenAPI side)

## 5. Resources
- [ ] 5.1 `openapi://…/spec/<path>`
- [ ] 5.2 `openapi://…/operation/<operationId>`
- [ ] 5.3 `openapi://…/schema/<schemaName>`

## 6. Verification
- [ ] 6.1 Sample repo: spec parsed, operations indexed, `list_apis` returns them
- [ ] 6.2 `explain_endpoint` returns correct handler + service_flow on a known fixture
- [ ] 6.3 Spec with unresolved/missing handler degrades gracefully (no link, no crash)

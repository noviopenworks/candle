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

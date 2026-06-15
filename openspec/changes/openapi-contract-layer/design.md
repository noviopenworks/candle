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

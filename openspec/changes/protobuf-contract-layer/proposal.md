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

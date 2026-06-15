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

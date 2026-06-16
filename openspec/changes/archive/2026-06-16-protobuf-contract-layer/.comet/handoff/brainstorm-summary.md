# Brainstorm Summary

- Change: protobuf-contract-layer
- Date: 2026-06-16
- Status: design CONFIRMED by user (2026-06-16); ready for Design Doc + delta specs

## Confirmed Decisions

1. **Linking scope = parse + same-repo linking.** Index proto files/services/RPCs/
   messages/enums; build the RPC→gRPC-server-impl linker and `uses_message` edges
   within a single repo's graph. **Defer cross-repo `consumed_by`** to a later
   dedicated change. (This is the linker infra the OpenAPI layer deferred; OpenAPI
   can reuse it later.)
2. **Discovery = manifest-declared.** Per-repo manifest gains protobuf config:
   import roots (for compiler import resolution + well-known types) and entry
   files/dirs to index. No filesystem globbing (consistent with openapi-index's
   "SHALL NOT glob" rule).
3. **Parser library = bufbuild/protocompile.** Compile to fully-resolved
   protoreflect.FileDescriptors via a SourceResolver over manifest import roots +
   bundled well-known types. Gives resolved message refs, enums, options, go_package.
4. **Streaming = full classification modeling (static).** `stream_kind` enum on
   proto_rpcs (unary/server_stream/client_stream/bidi); `find_rpc` accepts a
   stream_kind filter; `explain_rpc` surfaces the kind; the linker is
   streaming-aware and matches the streaming server-method signature shape
   (e.g. bidi `Rpc(Service_RpcServer) error` vs unary `Rpc(ctx, *Req) (*Resp, error)`).

## Key Trade-offs and Risks

- Same-repo linking establishes new infra not present in the shipped OpenAPI layer
  (which kept contracts in dedicated tables, separate from graph nodes/edges).
  Linker confidence model is the main design risk now (cross-repo risk deferred).
- protocompile pulls the buf module's transitive deps; accepted for resolution fidelity.

## Testing Strategy (draft)

- Unit: parser over fixture .proto sets (imports across files, nested messages,
  enums, options, go_package, all four stream kinds).
- Linker: fixture repo with generated gRPC server stubs + impl methods; assert
  RPC→impl edges and streaming-aware signature matching; assert no false impl on
  unrelated methods.
- Regression: list_apis/find_schema still return HTTP results unchanged.
- Idempotency: re-index same repo → identical proto row counts.

## Spec Patches (delta spec to write back)

- New delta capability `protobuf-index`: manifest-declared discovery (not glob),
  protocompile parsing, stream_kind, same-repo RPC→impl + uses_message linking.
- New delta capability `protobuf-tools`: find_rpc (+stream_kind filter),
  explain_rpc (impl + same-repo only; **consumed_by deferred**), proto:// resources.
- Modify `openapi-tools`: list_apis gains kind:"protobuf" entries; find_schema gains
  kind:"proto_message" entries (additive, no HTTP break).
- Note in spec: cross-repo `consumed_by` explicitly out of scope for this change.

## Resolved in design presentation

- Storage: dedicated `index_id`-scoped tables `proto_files/services/rpcs/messages/enums`
  + `proto_rpc_impls` link table; fields/enums stored as JSON blobs; uses_message is the
  resolvable request/response fully-qualified message reference (no separate edge table).
- Linker: new **shared `internal/link/`** package; confidence tiers HIGH/MEDIUM/LOW;
  signals = method name + service association + streaming-aware signature check; ambiguous
  matches recorded at lower confidence, never guessed; runs after graph.Load in ingest.
- explain_rpc: **includes** best-effort one-hop `calls` expansion from impl node; consumed_by
  returns explicit "deferred" marker.
- find_rpc has optional stream_kind filter; list_apis adds kind:"protobuf"; find_schema adds
  kind:"proto_message".
- Parser/code layout: `internal/proto/`, `internal/store/proto.go`, `internal/link/`,
  `internal/mcp/proto_tools.go`, config `proto: {roots, files}`.

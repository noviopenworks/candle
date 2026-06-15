# Tasks — protobuf-contract-layer

> Open-phase outline. Refined against the Design Doc + delta specs.

## 1. Storage
- [ ] 1.1 Add `proto_files`, `proto_services`, `proto_rpcs`, `proto_messages` tables (index_id-scoped); migration

## 2. Protobuf parsing
- [ ] 2.1 Discover `.proto` files (proto/api/internal globs)
- [ ] 2.2 Parse with protobuf grammar library; resolve imports across files
- [ ] 2.3 Extract services, RPCs, messages, fields, enums, options, generated Go package
- [ ] 2.4 Normalize into storage

## 3. Contract → code linking
- [ ] 3.1 RPC→server-impl linker (reuse shared linker; gRPC `RegisterXServer`/method patterns)
- [ ] 3.2 `uses_message` (request/response → message) edges
- [ ] 3.3 `calls` walk from server impl into domain service
- [ ] 3.4 Cross-repo `consumed_by` via merged graph client call-site detection

## 4. Tools
- [ ] 4.1 `find_rpc`
- [ ] 4.2 `explain_rpc` (impl + calls + consumed_by)
- [ ] 4.3 Extend `list_apis` with protobuf entries
- [ ] 4.4 Extend `find_schema` with proto messages

## 5. Resources
- [ ] 5.1 `proto://…/file/<path>`
- [ ] 5.2 `proto://…/service/<package>/<service>`
- [ ] 5.3 `proto://…/rpc/<package>/<service>/<rpc>`
- [ ] 5.4 `proto://…/message/<package>/<message>`

## 6. Verification
- [ ] 6.1 Sample repo: protos parsed, RPCs/messages indexed, `find_rpc` works
- [ ] 6.2 `explain_rpc` returns server impl + consumed_by on a multi-repo fixture
- [ ] 6.3 `list_apis`/`find_schema` still return HTTP results unchanged (no regression)

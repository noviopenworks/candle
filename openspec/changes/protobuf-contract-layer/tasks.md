# Tasks — protobuf-contract-layer

> Refined against design doc `docs/superpowers/specs/2026-06-16-protobuf-contract-layer-design.md`
> and delta specs. Scope: parse + same-repo linking; cross-repo `consumed_by` deferred.

## 1. Storage
- [x] 1.1 Add `proto_files`, `proto_services`, `proto_rpcs`, `proto_messages`, `proto_enums`, `proto_rpc_impls` tables (index_id-scoped) to `schema.go`
- [x] 1.2 `internal/store/proto.go`: bundle types + `ReplaceProtoFiles` (idempotent per index_id); impl-link write/read; find/lookup queries

## 2. Protobuf parsing
- [x] 2.1 Add `proto: { roots, files }` block to `RepoConfig` (`internal/config`)
- [x] 2.2 `internal/proto`: bufbuild/protocompile compiler with SourceResolver over roots + well-known types; expand directory entries
- [x] 2.3 Extract services, RPCs (request/response message names + `stream_kind`), messages (fields), enums (values), file package + go_package + imports
- [x] 2.4 Normalize into store bundles; tolerate missing/malformed/unresolvable files with warnings

## 3. Contract → code linking
- [x] 3.1 `internal/link` (new shared package): RPC→server-impl matcher — name + service association + streaming-aware signature check; confidence tiers + match_reason
- [x] 3.2 Run linker in `ingest.Run` after `graph.Load`; persist `proto_rpc_impls`
- [x] 3.3 `uses_message` via resolvable request/response message references (no separate table)
- [ ] 3.4 ~~Cross-repo `consumed_by`~~ **DEFERRED to a future change** (out of scope; `explain_rpc` returns a deferred marker)

## 4. Tools
- [x] 4.1 `find_rpc` (lexical match + optional `stream_kind` filter)
- [x] 4.2 `explain_rpc` (proto facts + resolved messages + `implemented_by` + best-effort one-hop `calls` + deferred `consumed_by` marker)
- [x] 4.3 Extend `list_apis` with `{kind:"protobuf"}` entries (HTTP output unchanged)
- [x] 4.4 Extend `find_schema` with `{kind:"proto_message"}` entries

## 5. Resources
- [x] 5.1 `proto://…/file/<path>`
- [x] 5.2 `proto://…/service/<package>/<service>`
- [x] 5.3 `proto://…/rpc/<package>/<service>/<rpc>`
- [x] 5.4 `proto://…/message/<package>/<message>`

## 6. Verification
- [x] 6.1 Parser unit tests: cross-file imports, nested messages, enums, options, go_package, all four stream kinds
- [x] 6.2 Storage idempotency: re-index → identical row counts; impl links cleared
- [x] 6.3 Linker tests: HIGH-confidence match, streaming signature disambiguation, no false-positive on unrelated same-named method, ambiguous → LOW confidence
- [x] 6.4 Tool/resource tests: `find_rpc` filter, `explain_rpc` impl + one-hop calls + deferred marker, not-found behavior
- [x] 6.5 Regression: `list_apis`/`find_schema` HTTP output unchanged

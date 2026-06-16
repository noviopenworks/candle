# Verification Report: protobuf-contract-layer

- Date: 2026-06-16
- Mode: full
- Branch: feature/20260616/protobuf-contract-layer
- Base ref: 9973fcb38d6a2d6ef5f36abab3244a1c80f85a84

## Fresh verification evidence

| Check | Command | Result |
|-------|---------|--------|
| Build | `go build ./...` | exit 0 |
| Tests | `go test ./... -count=1` | 57 passed, 11 packages, 0 fail |
| Vet | `go vet ./...` | no issues |
| Format | `gofmt -l internal/ cmd/` | no drift |
| Secrets | grep for hardcoded keys/secrets/tokens | none found |

## Summary

| Dimension    | Status |
|--------------|--------|
| Completeness | 21/21 in-scope tasks `[x]`; 3 delta capabilities implemented; cross-repo `consumed_by` explicitly out of scope |
| Correctness  | All 23 delta-spec scenarios covered by code + tests |
| Coherence    | Implementation follows design doc; new code matches existing OpenAPI/store/MCP patterns |

## Completeness

- Tasks: all in-scope tasks in `openspec/changes/protobuf-contract-layer/tasks.md` checked. Item 3.4 (cross-repo `consumed_by`) reframed as a documented out-of-scope note, not an incomplete task.
- Spec coverage: `protobuf-index`, `protobuf-tools`, and the `openapi-tools` MODIFIED requirements are all implemented.

## Correctness — scenario coverage

### protobuf-index
- Protos listed in manifest are indexed → `internal/ingest` (`TestRunIndexesProtos`).
- Directory entry expands to contained protos → `internal/proto` `expandEntries` (`TestParseDirectoryEntry`).
- Repo without proto block indexes no protos → `ParseFiles` short-circuits on empty input; ingest tolerates (existing OpenAPI-only ingest tests).
- Services/RPCs/messages/enums normalized → `TestParseInventory`.
- Streaming kind classified (all four) → `TestParseInventory`.
- Malformed/unresolvable proto tolerated → `TestParseMissingFileWarns`.
- RPC links to server method → `internal/link` (`TestMatchRPCsConfidence`, `TestRunIndexesProtos`).
- Streaming signature disambiguates → `TestMatchRPCsConfidence` (bidi Sync → HIGH).
- Ambiguous match recorded, not guessed → `TestMatchRPCsAmbiguousLowConfidence` (both candidates at LOW).
- No implementation found is valid → `TestMatchRPCsConfidence` (Ghost → none).
- Request/response resolve to messages → `TestExplainRPC` (messageFields).
- Re-indexing does not duplicate → `TestProtoStorageAndIdempotent`, `TestLinkRPCImplsRoundTrip`.

### protobuf-tools
- Match by RPC/service name → `TestFindRPCAndFilter`.
- Filter by stream_kind → `TestFindRPCAndFilter`.
- Contract data + implementation returned → `TestExplainRPC`.
- consumed_by is a deferred marker, not an error → `TestExplainRPC` (non-empty `ConsumedBy`).
- Unknown RPC returns not-found → `TestExplainRPC` (ErrNotFound).
- RPC resource returns the RPC → `TestProtoResources`.
- Message resource returns the message → `TestProtoResources`.

### openapi-tools (MODIFIED)
- Indexed OpenAPI specs listed → existing OpenAPI tests unchanged.
- Indexed protobuf files listed → `TestListAPIsIncludesProto`, `TestProtoDoesNotRegressHTTP`.
- OpenAPI schema found by name → existing tests + regression.
- Proto message found by name → `TestFindSchemaIncludesProtoMessage`, regression.

## Coherence — design adherence

Implementation follows `docs/superpowers/specs/2026-06-16-protobuf-contract-layer-design.md`:
manifest discovery (no globbing), bufbuild/protocompile parser, `stream_kind` enum, same-repo
RPC→impl linker with confidence tiers in the shared `internal/link` package, `uses_message`
as resolvable message references (no separate table), dedicated `index_id`-scoped tables,
additive `list_apis`/`find_schema`, `proto://` resources, and the deferred `consumed_by` marker.

No spec drift: delta specs were unchanged during the build phase and remain consistent with the design doc (both defer `consumed_by`).

## Issues

### CRITICAL
None.

### WARNING
None.

### SUGGESTION
1. `proposal.md` (open-phase artifact) still describes cross-repo `consumed_by` as in-scope for
   `explain_rpc`. The design phase deliberately deferred it (recorded in the design doc and the
   `protobuf-tools` delta spec, user-approved). Consider a one-line scope note in proposal.md, or
   leave as-is since the delta spec is canonical. Non-blocking.
2. Linker HIGH confidence tier (signature match) reads the graph node's `source_file` directly,
   so it only fires when that path is readable from the process working directory; otherwise it
   degrades to the name+service (MEDIUM) tier. Documented as best-effort in `internal/link/link.go`;
   revisit if/when real Graphify graphs with repo-relative paths are ingested. Non-blocking.

## Final Assessment

All checks passed. No critical or warning issues. Two non-blocking suggestions noted. **Ready for archive.**

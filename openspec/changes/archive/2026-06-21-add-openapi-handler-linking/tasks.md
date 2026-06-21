# Tasks — add-openapi-handler-linking

Implementation order. Each task = one focused commit; keep the verification
baseline green (`go build ./...`, `go vet ./...`, `go test ./...`, `mise exec -- task ci`).

## 1. Store linkage
- [x] 1.1 Add an `http_operation_impls` table (keyed by `index_id` + operation identity) with a migration, mirroring the proto impl-link table.
- [x] 1.2 Add `HTTPOpImplLink` type and `LinkHTTPOpImpls` writer + reader on `internal/store`, with unit tests (parallel to `LinkRPCImpls`).

## 2. Linker
- [x] 2.1 Add `MatchOpenAPI` to `internal/link`, reusing `readSourceUnderRoot` and the confidence tiers; define the operation→handler candidate query (operationId/name) and the AST handler-signature confirmation.
- [x] 2.2 Unit-test `MatchOpenAPI`: AST-confirmed HIGH, name-only fallback tier, no-source fallback, and no-candidate (empty) cases.

## 3. Ingest wiring
- [x] 3.1 Call `MatchOpenAPI` in `internal/ingest` alongside `MatchRPCs`, gated on `root`, and persist via `LinkHTTPOpImpls`.

## 4. MCP surface
- [x] 4.1 Extend `ExplainEndpoint` (`internal/mcp/openapi_tools.go`) to read links and return `implemented_by[]` (empty list when none).
- [x] 4.2 Update the stale limitation strings in `internal/mcp/context_tools.go` (and `get_context` if it surfaces the same note).

## 5. End-to-end + docs
- [x] 5.1 Extend the e2e (`internal/mcp/e2e_test.go` / `e2e_surface_test.go`) to assert HTTP `implemented_by`, mirroring the proto assertion.
- [x] 5.2 Reconcile any doc/count drift introduced by the new field (design.md "deferred" note, getting-started, concepts).
- [x] 5.3 Flip roadmap item 0.2 status (🔎 → ✅) in `Roadmap.md` and update the matching deferred note in `docs/`.

## Code review outcome
Reviewed at HEAD (commit before this note). Verdict: ready to merge, no Critical.
- Important I2 (bare `"Handle"` inflated MEDIUM repo-wide) — **fixed** (commit b516819).
- Important I1 (multi-candidate false-positive untested) — **fixed**: added disambiguation test (handler HIGH vs same-named non-handler LOW).
- Minor M1/M2 (tier-string divergence from explain_rpc; tierLabel thresholds vs link constants) — **addressed** with clarifying comments.
- Minor M3 (`httpSignatureScan` line-based fallback) and M4 (`titleFirst` ASCII-only) — **accepted as-is**: both faithfully mirror the established gRPC linker pattern (`signatureMatches`, exported-symbol naming) and are documented in code.

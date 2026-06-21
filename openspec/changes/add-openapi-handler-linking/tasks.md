# Tasks — add-openapi-handler-linking

Implementation order. Each task = one focused commit; keep the verification
baseline green (`go build ./...`, `go vet ./...`, `go test ./...`, `mise exec -- task ci`).

## 1. Store linkage
- [ ] 1.1 Add an `http_operation_impls` table (keyed by `index_id` + operation identity) with a migration, mirroring the proto impl-link table.
- [ ] 1.2 Add `HTTPOpImplLink` type and `LinkHTTPOpImpls` writer + reader on `internal/store`, with unit tests (parallel to `LinkRPCImpls`).

## 2. Linker
- [ ] 2.1 Add `MatchOpenAPI` to `internal/link`, reusing `readSourceUnderRoot` and the confidence tiers; define the operation→handler candidate query (operationId/name) and the AST handler-signature confirmation.
- [ ] 2.2 Unit-test `MatchOpenAPI`: AST-confirmed HIGH, name-only fallback tier, no-source fallback, and no-candidate (empty) cases.

## 3. Ingest wiring
- [ ] 3.1 Call `MatchOpenAPI` in `internal/ingest` alongside `MatchRPCs`, gated on `root`, and persist via `LinkHTTPOpImpls`.

## 4. MCP surface
- [ ] 4.1 Extend `ExplainEndpoint` (`internal/mcp/openapi_tools.go`) to read links and return `implemented_by[]` (empty list when none).
- [ ] 4.2 Update the stale limitation strings in `internal/mcp/context_tools.go` (and `get_context` if it surfaces the same note).

## 5. End-to-end + docs
- [ ] 5.1 Extend the e2e (`internal/mcp/e2e_test.go` / `e2e_surface_test.go`) to assert HTTP `implemented_by`, mirroring the proto assertion.
- [ ] 5.2 Reconcile any doc/count drift introduced by the new field (design.md "deferred" note, getting-started, concepts).
- [ ] 5.3 Flip roadmap item 0.2 status (🔎 → ✅) in `Roadmap.md` and update the matching deferred note in `docs/`.

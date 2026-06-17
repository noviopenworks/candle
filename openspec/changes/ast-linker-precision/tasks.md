# Tasks — ast-linker-precision

> Go-only. AST at index time via `go/parser` (syntax-only). Additive manifest
> `root`; fallback to today's heuristic when source is unavailable. TDD: each
> task starts with a failing test.

## 1. Manifest source root

- [x] 1.1 Add optional `root` field to `RepoConfig` in `internal/config` (+ test)
- [x] 1.2 Validate `root` (absolute or repo-relative); empty is allowed

## 2. AST matcher in internal/link

- [x] 2.1 Add a source-root parameter to the link entrypoints (`MatchRPCs`, `MatchExports`)
- [x] 2.2 Implement `astSignatureMatch`: parse the candidate file with `go/parser`, find the `FuncDecl` (Name == rpc, receiver present), classify unary vs streaming from params/returns
- [x] 2.3 Recalibrate `score`: AST-confirmed → HIGH; name+service → MEDIUM; name → LOW; reason string reflects AST vs heuristic
- [x] 2.4 Fallback path: when root absent / file unreadable / unparseable, use existing string-scan/name-service tiers (no regression)
- [x] 2.5 AST-confirm `MatchExports`: prefer the node whose declaration is in the export's package

## 3. Ingest wiring

- [ ] 3.1 Resolve each repo's source root and pass it into the linker in `internal/ingest`
- [ ] 3.2 Keep indexing successful when `root` is absent (warn, don't fail)

## 4. Tests

- [ ] 4.1 Unit fixtures: unary, server-stream, client-stream, multi-line signature, wrong receiver
- [ ] 4.2 Unit: same-name-different-package export disambiguation
- [ ] 4.3 Unit: unparseable/missing source → fallback tier (regression guard)
- [ ] 4.4 Integration: ingest passes `root`; mcp e2e still green

## 5. Verification

- [ ] 5.1 `go build ./...` passes
- [ ] 5.2 `go vet ./...` passes
- [ ] 5.3 `go test ./...` passes (all packages)
- [ ] 5.4 A repo without `root` produces identical link tiers to pre-change behavior

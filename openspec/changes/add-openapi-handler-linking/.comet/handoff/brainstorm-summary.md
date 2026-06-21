# Brainstorm Summary

- Change: add-openapi-handler-linking
- Date: 2026-06-21

## Confirmed Technical Approach

Mirror the proven gRPC `MatchRPCs` path for HTTP operations. `MatchOpenAPI` in
`internal/link` reuses the package's generic AST scaffolding (`readSourceUnderRoot`,
confidence constants, `score()`-style ladder).

- **Q1 — primary signal: (A) name-based + AST shape.** operation → derived handler
  name → `NodesByLabel` candidates → AST confirms HTTP-handler signature for HIGH.
  Router-agnostic; minimal new machinery.
- **Q2 — name derivation: operationId-only {exact, PascalCase}.** No operationId →
  no link (empty `implemented_by`). No method/path guessing.
- **Q3 — confidence model: full 3-tier RPC parity + string-scan fallback.**
  - LOW `name`: candidate found by derived name.
  - MEDIUM `name+route`: + route-registration **presence** in repo (coarse,
    existence-based analog of `hasServiceRegistration`; precise per-route arg-binding
    DEFERRED).
  - HIGH `name+route+ast` / `+signature`: AST confirms handler signature under
    `root`, OR (root absent/unreadable) legacy string-scan of the node's source_file
    confirms the `(http.ResponseWriter, *http.Request)` shape.
  - No candidate → no link.

The AST shape-gate doubles as a disambiguator: a same-named domain-service method
`ReserveProduct(ctx, req)` fails the handler signature and stays at the name tier
rather than being falsely promoted.

## Key Trade-offs and Risks

- Name-based linking depends on an operationId↔handler-name convention; acceptable
  because we author the demo codebase (change 2) and no-operationId yields an honest
  empty link.
- MEDIUM route-presence signal is coarse (repo-level routing existence, not
  path→handler binding). Precise per-router binding is a deferred follow-on.
- HTTP handler signatures are non-unique across operations; disambiguation rests on
  the name. AST confirms "is a handler", not "is THE handler for this path".

## Testing Strategy

- `internal/link` unit tests: AST-confirmed HIGH; name-only LOW (domain-service
  shape mismatch); no-root string-scan HIGH; no-source LOW; no-operationId → no
  link; no-candidate → no link.
- `internal/store` unit tests: `LinkHTTPOpImpls` write + read (parallel to
  `LinkRPCImpls`).
- e2e (`internal/mcp/e2e_test.go` / `e2e_surface_test.go`): add an HTTP handler to
  the fixture graph + source under root; assert `explain_endpoint` returns
  `implemented_by` HIGH for `reserveProduct`.

## Spec Patches

APPLIED (user-approved): the `ast-linking` delta requirement "AST-confirmed HTTP
handler matching" was expanded with explicit per-tier scenarios — AST-HIGH,
string-scan-fallback HIGH, route-presence MEDIUM, same-named-non-handler LOW, and
no-operationId/no-candidate → no link. Additive scenario supplement only; no scope
or structure rewrite. The `openapi-tools` delta is unchanged from the open phase.

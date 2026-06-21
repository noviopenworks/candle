## ADDED Requirements

### Requirement: AST-confirmed HTTP handler matching

The linker SHALL match an OpenAPI HTTP operation to its handler symbol in the code
graph and record an `implemented_by` link, mirroring the existing AST-confirmed RPC
implementation matching and reusing the same source-resolution (`root`) and
graceful-fallback behavior.

Candidate discovery SHALL derive handler-name candidates from the operation's
`operationId` only (exact and PascalCase forms); an operation without an
`operationId` SHALL produce no link. The link SHALL be scored on a three-tier
ladder analogous to the RPC linker:

- **HIGH** — a name candidate is confirmed to be an HTTP handler by parsing its
  declaration with `go/ast` (parameters `http.ResponseWriter` and `*http.Request`)
  under `root`; or, when `root` is unavailable, a legacy string-scan of the node's
  `source_file` confirms the same handler shape.
- **MEDIUM** — a name candidate exists and the repo has HTTP route-registration
  **presence** (a coarse, existence-based signal analogous to the RPC
  service-registration check), but the handler signature is not AST-confirmed.
- **LOW** — a name candidate exists by name alone, with neither signature
  confirmation nor route-registration presence.

A node that matches by name but whose AST declaration is not an HTTP handler (for
example a same-named domain-service method) SHALL NOT be promoted to HIGH on the
strength of the name.

#### Scenario: operation confirmed by AST handler signature

- **WHEN** an operation `reserveProduct` (`POST /products/{productId}/reservations`)
  has a candidate handler node whose `go/ast` declaration is an HTTP handler
  (`func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request)`)
  reachable under the repo `root`
- **THEN** the linker records an `implemented_by` link at HIGH confidence with a
  reason indicating AST confirmation

#### Scenario: HIGH via string-scan fallback when root is absent

- **WHEN** the repo sets no `root` but the candidate handler's `source_file` is
  directly readable and its signature matches the HTTP-handler shape
- **THEN** the linker records the `implemented_by` link at HIGH confidence via the
  string-scan fallback

#### Scenario: MEDIUM via route-registration presence

- **WHEN** a name candidate exists and the repo contains HTTP route-registration
  infrastructure, but the candidate's handler signature is not AST-confirmed
- **THEN** the linker records the `implemented_by` link at MEDIUM confidence

#### Scenario: LOW for a same-named non-handler symbol

- **WHEN** the only name candidate is a same-named domain-service method
  `func (s *Service) ReserveProduct(ctx context.Context, req *Request) (*Reservation, error)`
  (not an HTTP handler) and there is no route-registration presence
- **THEN** the linker records the candidate at LOW confidence and does not promote
  it to HIGH

#### Scenario: no operationId or no candidate yields no link

- **WHEN** an operation has no `operationId`, or no node in the code graph matches
  the derived handler name
- **THEN** the linker records no `implemented_by` link for that operation and does
  not error

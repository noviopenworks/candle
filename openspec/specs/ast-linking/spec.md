# ast-linking Specification

## Purpose
TBD - created by archiving change ast-linker-precision. Update Purpose after archive.
## Requirements
### Requirement: AST-confirmed RPC implementation matching

The linker SHALL, when a repo's Go source is available, confirm a gRPC RPC's
implementation by parsing the candidate method's declaration with `go/ast` —
inspecting the receiver, parameter types, and return types — rather than scanning
raw source lines. An AST-confirmed match SHALL be recorded at HIGH confidence.

#### Scenario: unary RPC confirmed by AST

- **WHEN** an RPC `ReserveProduct` (unary) has a candidate method
  `func (s *Server) ReserveProduct(ctx context.Context, req *pb.ReserveProductRequest) (*pb.Reservation, error)`
- **THEN** the linker records an `implemented_by` link at HIGH confidence with a
  reason indicating AST signature confirmation

#### Scenario: streaming RPC classified by AST

- **WHEN** an RPC `Sync` (server stream) has a candidate method
  `func (s *Server) Sync(req *pb.SyncRequest, stream pb.InventoryService_SyncServer) error`
- **THEN** the linker classifies it as streaming, matches it against the RPC's
  `stream_kind`, and records the link at HIGH confidence

#### Scenario: multi-line signature matched

- **WHEN** a candidate method's signature spans multiple source lines
- **THEN** the AST matcher still confirms it (where a line-based string scan would fail)

### Requirement: Source resolution via manifest root

The manifest SHALL accept an optional per-repo `root` field giving the absolute
path to the repo's source tree. The linker SHALL resolve a node's `source_file`
against this root for AST parsing.

#### Scenario: root enables AST parsing

- **WHEN** a repo entry sets `root: /abs/path/repo` and a node's `source_file` is `internal/grpc/server.go`
- **THEN** the linker parses `/abs/path/repo/internal/grpc/server.go` with `go/ast`

#### Scenario: root is optional

- **WHEN** a repo entry omits `root`
- **THEN** indexing still succeeds and linking proceeds via the fallback path

### Requirement: Graceful fallback without source

The linker SHALL fall back to the existing name + service-registration heuristic
when source is unavailable (no `root`, unreadable file, or unparseable Go),
recording the link at its corresponding non-HIGH tier, without error and without
dropping the candidate.

#### Scenario: no regression when source is missing

- **WHEN** a repo has no reachable source for a matched RPC
- **THEN** the linker still produces the `implemented_by` candidate at the
  name/service (MEDIUM) or name-only (LOW) tier, exactly as before this change

### Requirement: AST-confirmed export linking

When source is available, private-library **export** linking SHALL prefer the
code node whose AST declaration matches the exported symbol in the expected
package, rather than the first same-named node.

#### Scenario: disambiguate same-named exports

- **WHEN** two code nodes share the symbol name `ValidateToken` in different packages
- **THEN** the linker selects the node whose AST declaration is in the export's package

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


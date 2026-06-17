# ast-linking

## ADDED Requirements

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

When source is unavailable (no `root`, unreadable file, or unparseable Go), the
linker SHALL fall back to the existing name + service-registration heuristic and
record the link at its corresponding non-HIGH tier, without error and without
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

## ADDED Requirements

### Requirement: Protobuf discovery via manifest
The system SHALL read the protobuf import roots and entry files for a repo from an explicit `proto:` block in that repo's manifest entry, containing `roots` (import-resolution roots) and `files` (entry `.proto` files or directories to index). The system SHALL NOT auto-discover `.proto` files by globbing the filesystem.

#### Scenario: Protos listed in manifest are indexed
- **WHEN** a manifest entry declares `proto: { roots: [proto], files: [proto/inventory.proto] }` and the file exists
- **THEN** that proto file is parsed and stored under the repo's `index_id`

#### Scenario: Directory entry expands to contained protos
- **WHEN** a manifest `proto.files` entry names a directory
- **THEN** the `.proto` files beneath that directory are indexed

#### Scenario: Repo without proto block indexes no protos
- **WHEN** a manifest entry has no `proto:` block
- **THEN** the repo indexes successfully with zero proto files and no error

### Requirement: Parse protobuf into storage
The system SHALL compile `.proto` files with a real protobuf grammar (bufbuild/protocompile) using a source resolver rooted at the manifest `roots` plus bundled well-known types, resolving imports across files, and persist normalized proto files, services, RPCs, messages, and enums in tables tied to the repo's `index_id`.

#### Scenario: Services, RPCs, messages, and enums are normalized
- **WHEN** a `.proto` file with a service, RPCs, messages, and enums is indexed
- **THEN** the proto file (path, package, go_package option, imports) is stored, each service is stored with its full name, each RPC is stored with its full name, fully-qualified request and response message names, and `stream_kind`, each message is stored with its fields, and each enum is stored with its values

#### Scenario: Streaming kind is classified
- **WHEN** an RPC is declared `unary`, server-streaming, client-streaming, or bidirectional
- **THEN** its `stream_kind` is stored as `unary`, `server_stream`, `client_stream`, or `bidi` respectively

#### Scenario: Malformed or unresolvable proto is tolerated
- **WHEN** a referenced proto file is missing, fails to parse, or has an unresolvable import
- **THEN** it is skipped with a warning and the rest of the run continues

### Requirement: Same-repo RPC implementation linking
The system SHALL link each proto RPC to its gRPC server implementation within the same repo's code graph, recording zero or more candidate implementations with a confidence value and a match reason. Linking SHALL run after the code graph for the repo is loaded. Cross-repo consumer linking (`consumed_by`) is out of scope for this change.

#### Scenario: RPC links to its server method
- **WHEN** a repo's code graph contains a gRPC server method matching an indexed RPC by name and service association
- **THEN** a `proto_rpc_impls` record is stored referencing that code node with a confidence value and match reason

#### Scenario: Streaming signature disambiguates a match
- **WHEN** a candidate server method's source signature shape matches the RPC's `stream_kind`
- **THEN** the implementation link is recorded at higher confidence than a name-only match

#### Scenario: Ambiguous match is recorded, not guessed
- **WHEN** multiple candidate methods match an RPC by name with no disambiguating signal
- **THEN** the candidates are recorded at low confidence rather than collapsed into a single false-positive or dropped

#### Scenario: No implementation found is a valid result
- **WHEN** no code node matches an indexed RPC
- **THEN** the RPC is stored with zero implementation links and no error

### Requirement: uses_message reference
The system SHALL represent the `uses_message` relationship as each RPC's resolvable fully-qualified request and response message names, which resolve to stored `proto_messages` rows within the same `index_id`.

#### Scenario: Request and response resolve to messages
- **WHEN** an RPC references request and response messages that are indexed in the same repo
- **THEN** those message names resolve to the stored message rows (with fields) for that index

### Requirement: Idempotent proto indexing
The system SHALL make proto indexing idempotent per `index_id`: re-indexing replaces that repo's proto file/service/rpc/message/enum and implementation-link rows without duplication.

#### Scenario: Re-indexing the same repo does not duplicate
- **WHEN** a repo with one proto file is indexed twice
- **THEN** the proto_files / proto_services / proto_rpcs / proto_messages / proto_enums / proto_rpc_impls counts are identical after the second run

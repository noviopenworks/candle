# protobuf-tools Specification

## Purpose
TBD - created by archiving change protobuf-contract-layer. Update Purpose after archive.
## Requirements
### Requirement: find_rpc locates RPCs
The system SHALL provide `find_rpc` that matches indexed proto RPCs by natural language, service, RPC name, or fully-qualified name, and SHALL accept an optional `stream_kind` filter.

#### Scenario: Match by RPC or service name
- **WHEN** `find_rpc` is called with a query matching an indexed RPC or service name
- **THEN** matching RPCs are returned with `{full_name, service, rpc, request_message, response_message, stream_kind, proto_path}`

#### Scenario: Filter by stream_kind
- **WHEN** `find_rpc` is called with a `stream_kind` filter
- **THEN** only RPCs whose `stream_kind` matches the filter are returned

### Requirement: explain_rpc returns contract data and same-repo implementation
The system SHALL provide `explain_rpc` that returns, for a `(service, rpc)`: the RPC's proto facts (`full_name`, `stream_kind`, request/response message names and their fields), the same-repo implementations (`implemented_by` with node reference, confidence, and match reason), and a best-effort one-hop `calls` expansion from the implementation node. The `consumed_by` field SHALL return an explicit deferred marker, since cross-repo consumer linking is out of scope for this change.

#### Scenario: Contract data and implementation returned
- **WHEN** `explain_rpc` is called with a known service and RPC that has a linked implementation
- **THEN** it returns the RPC proto facts, resolved request/response messages, `implemented_by` entries with confidence, a one-hop `calls` expansion from the implementation node, and a deferred `consumed_by` marker

#### Scenario: consumed_by is a deferred marker, not an error
- **WHEN** `explain_rpc` is called for any RPC
- **THEN** `consumed_by` is an explicit "deferred / not available in this change" marker rather than an error or omitted field

#### Scenario: Unknown RPC returns not-found
- **WHEN** `explain_rpc` is called with a service/RPC that is not indexed
- **THEN** it returns a structured not-found result, not an error/crash

### Requirement: Protobuf resources
The system SHALL expose `proto://` resources for a proto file, a service, an RPC, and a message, commit-pinned from manifest metadata and degrading to branch or latest when no commit is recorded.

#### Scenario: RPC resource returns the RPC
- **WHEN** a client reads `proto://org/name/commit/<sha>/rpc/<package>/<service>/<rpc>` for an indexed RPC
- **THEN** it returns that RPC's contract data and same-repo implementation references

#### Scenario: Message resource returns the message
- **WHEN** a client reads `proto://org/name/commit/<sha>/message/<package>/<message>` for an indexed message
- **THEN** it returns that message's fields


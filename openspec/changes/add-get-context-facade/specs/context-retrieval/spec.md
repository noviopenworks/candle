# context-retrieval Specification

## ADDED Requirements

### Requirement: get_context retrieval facade tool
The system SHALL provide a `get_context` MCP tool that is the primary, repo-scoped
retrieval entry point composing existing code-graph, OpenAPI, protobuf, and private-library
queries. It SHALL require a `repo` argument and accept optional `topic`, `mode`, `depth`,
and `include_resources` arguments. It SHALL be additive: existing tools and their outputs
remain unchanged.

#### Scenario: Unknown repo returns not-found
- **WHEN** `get_context` is called with a `repo` that does not resolve to an indexed repo
- **THEN** it returns a not-found error (`ErrNotFound`) and no partial result

#### Scenario: Tool is advertised
- **WHEN** an MCP client sends `tools/list`
- **THEN** `get_context` appears in the advertised tool set, registered after `resolve_repo`

### Requirement: Overview mode catalogs repo capabilities
When called with `repo` and no `topic`, `get_context` SHALL return a repo snapshot summary
plus a capability catalog covering code graph, OpenAPI, protobuf, and private libraries.
Each capability entry SHALL report whether it is available, a count, and the precise
follow-up tools that serve that surface. The response SHALL include suggested next calls,
resource schemes, and explicit limitations.

#### Scenario: Overview returns repo summary and capabilities
- **WHEN** `get_context` is called with `{repo}` for an indexed repo
- **THEN** it returns the repo identity and commit, an empty `topic`, and capability
  summaries with correct availability and counts for code graph, OpenAPI, protobuf, and
  private libraries

#### Scenario: Overview suggests follow-up calls and resource schemes
- **WHEN** `get_context` is called in overview mode
- **THEN** the response includes a non-empty list of suggested next tool calls and a
  non-empty list of resource schemes

### Requirement: Topic mode retrieves focused context across surfaces
When called with `repo` and a non-empty `topic`, `get_context` SHALL search code symbols,
HTTP operations, schemas, proto messages, RPCs, and private libraries for the topic. Code
symbol matches SHALL include one-hop callers and callees (codegraph-style one-hop context).
When `include_resources` is true, matched items SHALL also yield exact resource-URI hints.

#### Scenario: Topic search spans all surfaces with one-hop code context
- **WHEN** `get_context` is called with `{repo, topic, include_resources:true}` where the
  topic matches a code symbol, a schema, and an RPC
- **THEN** it returns the matching code symbol with its one-hop callers and callees, the
  matched schema, the matched RPC, and a non-empty list of resource URIs

### Requirement: Mode filter narrows searched surfaces
`get_context` SHALL accept a `mode` argument of `overview`, `code`, `api`, `proto`,
`library`, or `all`. Unrecognized or empty modes SHALL be treated as `all`. A specific
non-`all` mode in topic search SHALL restrict matches to that surface only.

#### Scenario: code mode returns only code matches
- **WHEN** `get_context` is called with `{repo, topic, mode:"code"}`
- **THEN** the result contains code symbol matches and contains no endpoint, schema, RPC,
  or private-library matches

#### Scenario: overview mode returns catalog only and suppresses topic matches
- **WHEN** `get_context` is called with `{repo, topic, mode:"overview"}` where the topic
  would otherwise match one or more surfaces
- **THEN** the result contains the capability catalog and no topic matches (no code, endpoint,
  schema, RPC, or private-library matches)

### Requirement: Responses declare deferred limitations
Every `get_context` response SHALL include an explicit `limitations` list naming
deferred behavior, including that OpenAPI endpoint-to-handler implementation linking is
not yet available, and that cross-repo RPC consumer and cross-repo private-library
consumer aggregation are deferred.

#### Scenario: Limitations are present
- **WHEN** `get_context` returns any successful result
- **THEN** the `limitations` list is non-empty and names the deferred OpenAPI linking and
  cross-repo aggregation behaviors

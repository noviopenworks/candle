## ADDED Requirements

### Requirement: MCP stdio server
The system SHALL run an MCP server over stdio that advertises its tools and resources and responds to standard MCP protocol requests.

#### Scenario: Server advertises tools
- **WHEN** an MCP client connects and sends `initialize` then `tools/list`
- **THEN** the server returns the five base tools (`list_repos`, `resolve_repo`, `query_repo`, `explain_symbol`, `get_file_context`)

### Requirement: Repo listing and resolution tools
The system SHALL provide `list_repos` returning indexed repos with snapshot metadata, and `resolve_repo` returning the best matching repo for a fuzzy query.

#### Scenario: list_repos returns indexed repos
- **WHEN** `list_repos` is called with at least one indexed repo
- **THEN** it returns each repo with `repo`, `branch`, `commit`, `indexed_at`, and `node_count`

#### Scenario: resolve_repo fuzzy matches
- **WHEN** `resolve_repo` is called with a partial or approximate name
- **THEN** it returns the best matching `org/repo` identity (with candidates when ambiguous)

### Requirement: Structural code navigation tools
The system SHALL provide `query_repo`, `explain_symbol`, and `get_file_context` as deterministic structural queries over the SQLite store, with no natural-language/semantic layer and no runtime dependency on the Graphify CLI.

#### Scenario: query_repo finds a symbol structurally
- **WHEN** `query_repo` is called with `{repo, name}`
- **THEN** it returns matching nodes (and edges when requested) from that repo's index, deterministically

#### Scenario: explain_symbol returns symbol context
- **WHEN** `explain_symbol` is called with a resolvable symbol or node id
- **THEN** it returns the node with its `source_file`, `source_location`, callers, callees, and related edges

#### Scenario: get_file_context returns file symbols
- **WHEN** `get_file_context` is called with a file path in an indexed repo
- **THEN** it returns the symbols defined in that file and their edges

#### Scenario: Unresolved input returns empty, not error
- **WHEN** any tool is called with an unknown repo, symbol, or file
- **THEN** it returns an empty result or a structured "not found", never a crash

### Requirement: Repo and graph resources
The system SHALL expose `repo://org/name` and `graph://org/name/commit/<sha>/...` resources, commit-pinned from manifest metadata and degrading to branch or `latest` when no commit is available.

#### Scenario: repo resource returns snapshot summary
- **WHEN** a client reads `repo://org/name`
- **THEN** it returns that repo's snapshot summary (branch, commit, counts)

#### Scenario: graph resource is commit-pinned when available
- **WHEN** a client reads `graph://org/name/commit/<sha>/node/<node_id>` and the snapshot has that commit
- **THEN** it returns the node JSON for that pinned snapshot; if no commit is recorded, the resource degrades to branch or latest

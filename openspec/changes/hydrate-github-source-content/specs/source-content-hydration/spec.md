## ADDED Requirements

### Requirement: Source hydration is opt-in for Graphify-backed MCP tools
The system SHALL keep `get_context`, `query_repo`, `explain_symbol`, and `get_file_context` metadata-only by default, and SHALL expose an explicit source hydration option that allows callers to request GitHub source content for Graphify-backed results.

#### Scenario: Default tool calls remain metadata-only
- **WHEN** a caller invokes a Graphify-backed MCP tool without the source hydration option
- **THEN** the response SHALL NOT fetch or include source file content

#### Scenario: Caller enables automatic source hydration
- **WHEN** a caller invokes a Graphify-backed MCP tool with source hydration mode set to automatic
- **THEN** the system SHALL consider source content fetching only for ambiguous results, results missing source locations, or explicit source resource/file-context requests

### Requirement: GitHub source content is resolved from indexed provenance
The system SHALL resolve source content from indexed Graphify provenance, preferring a node `source_url` that points to GitHub raw content or a GitHub blob URL convertible to raw content.

#### Scenario: GitHub source_url is fetchable
- **WHEN** a hydrated result has a GitHub raw URL or convertible GitHub blob URL in `source_url`
- **THEN** the system SHALL fetch source content from the corresponding raw GitHub URL

#### Scenario: Source URL is unsupported or unavailable
- **WHEN** a hydrated result has no supported GitHub source URL
- **THEN** the system SHALL return the graph metadata with a source-content status explaining that content was not fetched

### Requirement: Hydrated content supports snippet and full-file modes
The system SHALL support a snippet mode that returns a bounded line window and a full-file mode that returns bounded text content for a source file.

#### Scenario: Snippet mode has a source location
- **WHEN** snippet hydration is requested for a node with a source location
- **THEN** the system SHALL return a bounded line window around that source location and include the returned line range

#### Scenario: Full-file mode exceeds the content cap
- **WHEN** full-file hydration is requested and the fetched text exceeds the configured content cap
- **THEN** the system SHALL return capped content and mark the source-content result as truncated

### Requirement: Source fetch failures do not fail the whole MCP response
The system SHALL return per-source fetch status for unavailable, unreachable, oversized, non-text, or unsupported content without failing the whole MCP tool call when graph metadata is still available.

#### Scenario: GitHub fetch returns an error
- **WHEN** source hydration is requested and the GitHub fetch fails
- **THEN** the MCP tool response SHALL still include the relevant graph result and SHALL include a source-content status with the failure reason

#### Scenario: Source content is non-text
- **WHEN** source hydration is requested and the fetched content is detected as non-text
- **THEN** the MCP tool response SHALL omit the content body and SHALL include a source-content status indicating unsupported content type

### Requirement: Hydration responses are bounded and structured
The system SHALL represent hydrated source content in a structured envelope that includes fetch status, mode, source URL, source file, optional line range, truncation status, and content only when content is successfully fetched.

#### Scenario: Agent receives hydrated ambiguous matches
- **WHEN** `query_repo` returns multiple matching nodes and automatic source hydration is enabled
- **THEN** each hydrated candidate SHALL include a source-content envelope that helps distinguish the candidates without changing the node identity fields

### Requirement: Direct source reads are available through a dedicated MCP tool
The system SHALL expose a dedicated MCP tool for explicit source-content reads, while existing Graphify-backed tools continue to support source enrichment through their structured source hydration option.

#### Scenario: Caller reads source content directly
- **WHEN** a caller invokes the dedicated source-read MCP tool with a repo and node or file reference
- **THEN** the system SHALL return the same structured source-content envelope used by hydrated tool results

#### Scenario: Existing tools enrich results and direct tool reads exact content
- **WHEN** a caller needs automatic disambiguation in a lookup tool and later needs a direct source read
- **THEN** the lookup tool SHALL be able to return wrapper results with source-content status and the dedicated tool SHALL be able to return explicit source content for the selected node or file

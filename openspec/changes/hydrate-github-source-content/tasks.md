## 1. Provenance and Hydration Core

- [x] 1.1 Expose stored node provenance (`source_url`, `captured_at`, `author`, `contributor`) through query-facing store rows without changing default metadata-only behavior.
- [ ] 1.2 Add a source hydration request/response model with modes for `off`, automatic snippets, explicit snippets, and capped full-file content.
- [ ] 1.3 Implement GitHub source URL normalization for raw URLs and convertible blob URLs, returning a structured unsupported status for other URLs.
- [ ] 1.4 Implement bounded text fetching with timeout, byte cap, text detection, line-window extraction, truncation reporting, and per-source error status.

## 2. MCP Tool Integration

- [ ] 2.1 Add source hydration options to `query_repo`, preserving existing output when the option is absent.
- [ ] 2.2 Add source hydration options to `explain_symbol`, preserving existing output when the option is absent.
- [ ] 2.3 Add source hydration options to `get_file_context`, including explicit file/resource-triggered hydration.
- [ ] 2.4 Add source hydration options to `get_context`, including automatic hydration for ambiguous matches or missing source locations when enabled.
- [ ] 2.5 Add a dedicated direct source-read MCP tool that accepts repo plus node or file reference and returns the structured source-content envelope.
- [ ] 2.6 Register the new source-read tool in the MCP server and update the e2e advertised tool surface expectations.

## 3. Tests and Documentation

- [ ] 3.1 Add unit tests for URL normalization, unsupported URL handling, fetch limits, text detection, snippet extraction, and truncation status.
- [ ] 3.2 Add MCP tool tests proving default metadata-only behavior and opt-in hydration behavior for `query_repo`, `explain_symbol`, `get_file_context`, and `get_context`, plus direct source reads through the new tool.
- [ ] 3.3 Add failure-path tests for unreachable GitHub content, non-text content, missing `source_url`, and oversized responses.
- [ ] 3.4 Update docs for the new MCP source hydration options, response envelope, limits, and Graphify `source_url` expectations.
- [ ] 3.5 Run `go test ./...` and address any regressions before entering verification.

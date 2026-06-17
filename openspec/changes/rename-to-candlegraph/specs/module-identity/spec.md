# module-identity

## ADDED Requirements

### Requirement: Canonical Go module path

The project SHALL declare its Go module path as `github.com/noviopenworks/candlegraph`, matching the canonical repository, and all internal imports SHALL resolve under that path.

#### Scenario: go.mod declares the canonical path

- **WHEN** `go.mod` is inspected
- **THEN** the module directive reads `module github.com/noviopenworks/candlegraph`

#### Scenario: internal imports resolve under the canonical path

- **WHEN** the module is built with `go build ./...`
- **THEN** every internal import path begins with `github.com/noviopenworks/candlegraph/` and the build succeeds

### Requirement: Single canonical binary name

The project SHALL ship under a single binary/command name, `candlegraph`, used consistently for the command entrypoint, the cobra root command, and the MCP server identity.

#### Scenario: command and server identify as candlegraph

- **WHEN** the command directory, cobra root `Use`, and MCP server `Name` are inspected
- **THEN** each is `candlegraph` and the entrypoint lives at `cmd/candlegraph/`

### Requirement: No stale prior name in tracked source

Tracked source, configuration, and documentation (excluding generated `graphify-out/` artifacts and archived openspec changes) SHALL contain no references to any prior name: the `intel-mcp` binary name, or the prior module paths `github.com/vend-ai/intel-mcp` and `github.com/candlegraph/intel-mcp`.

#### Scenario: grep finds no prior-name references

- **WHEN** `git grep` for `intel-mcp`, `vend-ai`, and `candlegraph/intel-mcp` is run over tracked files, excluding `graphify-out/`
- **THEN** zero matches are returned

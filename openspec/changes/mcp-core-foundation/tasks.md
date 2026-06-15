# Tasks — mcp-core-foundation

> Open-phase task outline. Refined against the Design Doc + delta specs in the design/build phases.

## 1. Project scaffolding
- [x] 1.1 Initialize Go module, repo layout (`cmd/`, `internal/`), lint/test tooling
- [ ] 1.2 Choose + wire MCP Go server library; minimal stdio server that lists zero tools

## 2. Storage
- [ ] 2.1 Define SQLite schema: `repos`/`index` (snapshot = `index_id`), `nodes`, `edges`
- [ ] 2.2 Migration/bootstrap on startup; idempotent open

## 3. Graphify ingestion
- [ ] 3.1 Parse `graph.json` (nodes/edges/hyperedges) per the Graphify schema
- [ ] 3.2 Loader: upsert nodes by Graphify `id`, edges by `(source,target,relation)`; idempotent re-ingest
- [ ] 3.3 Support cross-repo merged graph input
- [ ] 3.4 Tolerate missing/empty/partial graphs without erroring

## 4. Repo registry / resolution
- [ ] 4.1 Decide + implement repo-snapshot discovery (resolves the design-phase unknown)
- [ ] 4.2 Map `org/repo` (+ branch/commit) → indexed snapshot

## 5. Base MCP tools
- [ ] 5.1 `list_repos`
- [ ] 5.2 `resolve_repo`
- [ ] 5.3 `query_repo`
- [ ] 5.4 `explain_symbol`
- [ ] 5.5 `get_file_context`

## 6. Resources
- [ ] 6.1 `repo://…` resource handler
- [ ] 6.2 `graph://…` resource handler (commit-pinned, graceful degrade)

## 7. Verification
- [ ] 7.1 Ingest a sample repo graph; tools return expected results
- [ ] 7.2 Empty/no-graph repo indexes cleanly and tools return empty (not errors)
- [ ] 7.3 End-to-end: server starts over stdio, advertises tools, responds to a query

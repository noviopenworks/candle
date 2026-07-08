# Contributing to candle

## Development

```bash
go build ./...      # compile everything
go vet ./...        # static checks
go test ./...       # full suite (incl. the MCP end-to-end test)
```

The `internal/mcp` e2e test builds the `candle` binary, indexes a fixture
graph, and drives the stdio server — so a green `go test ./...` means the whole
index → serve path works.

The toolchain (Go, golangci-lint, goreleaser, govulncheck) is pinned in
`mise.toml` and builds run through [Task](https://taskfile.dev). With
[mise](https://mise.jdx.dev) installed, `mise install` provisions everything and
`task -l` lists the tasks. Before pushing, run the same gate CI runs:

```bash
task ci   # fmt-check, vet, lint, test, vuln
```

### Layout

See [docs/architecture.md](docs/architecture.md) for the package map. Key rule:
**SDK types stay in `internal/mcp/server.go`**; tool methods are pure and
SDK-free so they can be unit-tested directly.

## Project conventions

- **Module path:** `github.com/noviopenworks/candle`.
- **Binary / command:** `candle` (`cmd/candle`).
- Match the surrounding code's naming, comment density, and idioms.
- One logical change per commit; messages describe intent, not mechanics.

## Contribution flow

1. Open an issue (or comment on an existing one) describing the change.
2. Branch from `main`, keep one logical change per commit, and write commit
   messages that describe intent.
3. Before pushing, run the same gate CI runs: `task ci` (fmt-check, vet, lint,
   test, vuln).
4. Open a pull request against `main`. Reference the issue if there is one.

## Documentation conventions

candle's docs are the first thing a new user reads; they must stay accurate and
internally consistent. When you change behavior, update the docs in the same
commit.

- **Keep the existing format.** Match the heading depth, list style, table
  shape, and code-fence convention of the file you are editing. Do not
  reformat prose you did not change.
- **Numbers must match reality.** Tool counts, resource-scheme counts, and
  test/package counts appear in several files (`README.md`, `Roadmap.md`,
  `docs/getting-started.md`, `docs/tools.md`). When a count changes, update it
  everywhere. The source of truth for the tool count is the registration order
  in `internal/mcp/server.go`; for tests, the `go test ./...` summary.
- **No agentic drift ("vibecoding").** When an AI agent edits docs, it must
  edit the minimum necessary, preserve structure, and verify every count and
  cross-reference it states. Wholesale rewrites, tone changes, or invented
  details are not acceptable. If you are unsure whether a statement is still
  true, check the code before writing it.
- **Cross-references** use relative paths (`docs/tools.md`, not absolute or
  URL form) and must resolve.

## Where the design lives

[docs/design.md](docs/design.md) is the source-of-truth design spec (tool I/O,
DDL, edge catalog, end-to-end walkthrough). User-facing docs live under
[docs/](docs/README.md). Keep both in sync when behavior changes.

## Adding or changing a tool

1. Add the pure method to the relevant `internal/mcp/*_tools.go`, with a unit test.
2. Register it in `internal/mcp/server.go` (add to `ToolNames` + a `register*`).
3. Document it in [docs/tools.md](docs/tools.md) with example I/O.
4. Run `go test ./...` — the e2e test asserts the advertised tool surface.

# Roadmap

This is candle's forward-looking roadmap: what works today, where it's
going, and what is deliberately out of scope. It is the single source of truth
for project direction and supersedes the scattered "not yet implemented" notes in
[`docs/concepts.md`](docs/concepts.md) and [`docs/design.md`](docs/design.md).

Status legend: ✅ done · 🔄 in progress · 🔎 not started

---

## Current state (what works today)

candle ships a working **MCP stdio server** that joins three layers into
one queryable SQLite-backed graph:

- **Code graph** — consumes Graphify `graph.json` (does not parse source).
- **API contracts** — OpenAPI and protobuf, parsed and stored.
- **Private Go libraries** — indexed from provider and consumer sides.

**15 tools**, **5 resource URI schemes**, commit-pinned lookups, idempotent
per-repo indexing, scoped multi-instance serving, and a real end-to-end test
(127 tests, 12 packages). MIT-licensed; mise + task + golangci-lint + GitHub
Actions CI + GoReleaser all in place.

See [`docs/getting-started.md`](docs/getting-started.md) to use it, and
[`docs/architecture.md`](docs/architecture.md) for the package map.

## North star

From [`docs/design.md`](docs/design.md) — candle exists so an agent can
answer, across many repos:

> What does this service expose? What does it consume? Which private libraries
> does it depend on? Which repos use this API? Which repos use this library?
> **Where is this contract implemented? What breaks if I change it?**

The roadmap is prioritized by how much each item moves those two bolded
questions from "partially" to "reliably."

---

## Phase 0 — Adoption unblock

Before anything else: a new user must be able to **try it** and the headline
promise must **hold**. Today neither is fully true.

| # | Item | Status | Why it matters |
|---|---|---|---|
| 0.2 | **OpenAPI → handler linking.** Add `MatchOpenAPI` to `internal/link` (analogous to `MatchRPCs`) so `explain_endpoint` returns `implemented_by`. | ✅ | Answers the README's flagship question — *"which handler implements this endpoint?"* — via operationId-derived name candidates + AST-confirmed handler shape (HIGH), with route-registration presence as a coarse MEDIUM signal. Path→handler binding stays coarse (precise per-router binding is a later follow-on). REST is the majority case. |
| 0.3 | **Graphify quickstart.** A verified, versioned walkthrough for producing a compliant `graph.json` (exact commands, expected schema, validation), or a built-in fallback extractor. | 🔎 | candle does nothing without graph inputs; today producing them is undocumented. (`docs/getting-started.md:9-12`.) |

**Exit criterion:** a stranger can clone, follow the Graphify quickstart (0.3)
to produce a `graph.json` for the example fixture or their own repo, run
`candle index` + `candle serve`, and watch an agent answer "which handler
implements endpoint X?" in under five minutes.

## Phase 1 — Completeness & trust

Make the value prop reliable and the project credible.

| # | Item | Status | Why it matters |
|---|---|---|---|
| 1.1 | **Documentation sweep.** Reconcile tool-count drift (`getting-started.md` says 13, reality is 15), the stale tool list in `design.md` "Updated MVP Scope", and resource counts across docs. | ✅ | Cheap credibility win; the first number a new user reads is currently wrong. |
| 1.2 | **Observability.** `--verbose`/`--debug` structured logging of index + serve activity, and not-found reasons on every tool. | ✅ | Today a silent empty result is indistinguishable from "broken." Worst first-impression failure mode. |
| 1.3 | **Multi-hop traversal.** A `call_path` / `traverse` tool (or a `depth` arg on `explain_symbol`) so handler → service → repository → client is one call, not manual chaining. | 🔎 | Deepens "what breaks if I change X." Currently one-hop only (`internal/mcp/context_tools.go:264`). |
| 1.4 | **Cross-repo RPC `consumed_by`.** Aggregate gRPC consumers across indexed repos, mirroring what `explain_private_library` does for libraries. | ✅ | Completes the gRPC side of "what breaks." Implemented as a heuristic: repos with a node labelled like the RPC (gRPC client calls are not indexed), excluding providers. `ConsumedBy` is now a `[]string` of consumer repos. |
| 1.5 | **Stability policy.** A `docs/stability.md` tagging each tool/resource as stable or experimental, plus a semver/deprecation policy. | 🔎 | Pre-1.0, early adopters have no guarantee the surface won't break. |

## Phase 2 — Reach

Broaden who can use it, beyond a single Go shop on Claude Desktop.

| # | Item | Status | Why it matters |
|---|---|---|---|
| 2.1 | **Non-Go private-library ecosystems.** npm, PyPI, Maven/Gradle, Cargo — at least one beyond Go. | 🔎 | Layer 3 is inert for any non-Go org today (`docs/concepts.md:118`). |
| 2.2 | **Transport + client compatibility.** SSE/HTTP transport option; verified compatibility with Cursor, Continue, Cline, and generic MCP runners (not just Claude). | 🔎 | stdio-only limits deployment shapes; "works with my client" is currently assumed, not proven. |
| 2.3 | **Automatic breaking-change detection & API diffing.** Compare an operation/schema/RPC across commits or versions. | 🔎 | Long-promised in `design.md` "Final Direction"; today change-impact is manual. |
| 2.4 | **Generated-client / SDK analysis.** Recognize generated code and link it to its generator source. | 🔎 | Most real services ship generated clients; today those are invisible. |

## Phase 3 — Scale & operations

For shared and long-running deployments.

| # | Item | Status | Why it matters |
|---|---|---|---|
| 3.1 | **Scale benchmarks & limits.** Document behavior on large monorepos (10k+ nodes) and many-repo indexes; publish numbers. | 🔎 | SQLite single-writer + query-time cross-repo joins have unknown ceilings. |
| 3.2 | **Incremental & scheduled indexing.** `watch` mode, CI hook, or freshness signal in tool responses; avoid full re-index. | 🔎 | Indexes go silently stale today; there is no "snapshot is N days old" warning. |
| 3.3 | **Multi-tenancy.** Tenancy boundaries stronger than per-process `--config` scoping, for shared org deployments over one store. | 🔎 | Enables a team-shared server, not just per-developer local use. |
| 3.4 | **PR-review automation.** An indexer/reporter that surfaces contract impact on a pull request. | 🔎 | The natural CI home for the "what breaks" answer. |

---

## Explicitly out of scope (non-goals)

These are deliberately **not** on the roadmap right now, to keep focus. They may
return later.

- **Multi-language code graphs.** candle consumes Graphify output; it will
  not grow its own multi-language source parser. (A single-language fallback
  extractor for Phase 0.3 is the exception, and only to unblock evaluation.)
- **A web UI.** candle is an MCP server; the agent is the UI.
- **Replacing your service catalog / service mesh.** This is a knowledge layer
  for agents, not a control plane.
- **Real-time.** Snapshots are commit-pinned and indexed on demand.

## Picking up a roadmap item

1. Open an issue (or claim an existing one) named after the item, e.g.
   `graphify-quickstart` for 0.3.
2. Treat the item's "why it matters" as the change motivation; the file:line
   references as the impact map.
3. Keep the verification baseline green: `go build ./...`, `go vet ./...`,
   `go test ./...` (127/12), `mise exec -- task ci`.
4. On merge, flip the item's status here (🔎 → ✅, or 🔎 → 🔄 when started).

## Maintaining this roadmap

- One row per outcome, not per task. Tasks live in change folders.
- When a deferred item in `concepts.md` / `design.md` lands, update both that doc
  and the matching row here — this file is canonical.
- Phase exits are judgment calls by the maintainer; the exit *criteria* are not.

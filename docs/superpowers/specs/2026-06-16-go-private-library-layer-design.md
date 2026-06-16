---
comet_change: go-private-library-layer
role: technical-design
canonical_spec: openspec
archived-with: 2026-06-16-go-private-library-layer
status: final
---

# Go Private-Library Layer — Technical Design

## Context

MVP split 4 of 4. Adds Go private libraries as first-class indexed objects from both
sides: the **provider** repo that defines a private module (its exported API) and the
**consumer** repo that imports it (which version it pins and which exported symbols it
actually references). Depends on `mcp-core-foundation` (`index_id`/`repo` conventions,
code graph). Independent of the OpenAPI and protobuf layers.

Grounded in the just-completed protobuf-contract-layer, which established: manifest-declared
discovery (no filesystem globbing), dedicated `index_id`-scoped store tables separate from
the graph tables, a parse-files-don't-build philosophy, dedicated MCP tools + a URI resource
scheme, and the shared `internal/link` package for contract→code linking.

## Scope

In scope:
- Parse `go.mod`, `go.sum`, `go.work` (via `golang.org/x/mod/modfile`).
- Provider side: exported symbols of a repo's own private module(s), package doc synopses,
  README, each export best-effort linked to a Graphify code node.
- Consumer side: imports of private modules with pinned version and used exported symbols
  (file/line), via a go/ast import + selector heuristic (no build/type-check).
- Per-repo private classification by module-path prefix.
- Tools `find_private_library`, `find_library_consumers` (single-repo); `lib://` resources.

Out of scope (deferred):
- **Cross-repo consumer aggregation** ("who across all indexed repos depends on module X").
  `find_library_consumers` answers for the queried repo only and returns an explicit deferred
  marker for the cross-repo dimension.
- Unused-export detection, type-checked symbol resolution, non-Go ecosystems, version-diff
  / breaking-change analysis.

## Architecture

```
manifest go:{modules,private_prefixes}     go-private-library-layer
 ┌───────────────────────────┐  parse  ┌──────────────────────────────────────────────┐
 │ go.mod / go.sum / go.work  │ ──────▶ │ internal/godep (x/mod/modfile + go/ast)       │
 │ + module *.go source       │         │   → dependencies (ecosystem, is_private)      │
 └───────────────────────────┘         │   → private_libraries (provider modules)      │
                                        │   → private_library_exports (+ node_id)       │
 graph.json (Graphify) ──load─────────▶ │   → private_library_usages (consumer, file/ln)│
                                        │            │ link (after graph.Load)           │
                                        │            ▼                                   │
                                        │  exports → code nodes (internal/link)         │
                                        │  tools: find_private_library,                 │
                                        │         find_library_consumers                │
                                        │  resources: lib://…                           │
                                        └──────────────────────────────────────────────┘
```

Ingest order per repo: parse graph.json → `graph.Load` (populates nodes) → parse Go
modules/exports/usages → link exports to code nodes (needs nodes) → persist. Mirrors the
proto pass, which already runs after `graph.Load`.

## Components

### internal/config (manifest)

`RepoConfig` gains a per-repo Go block:

```go
Go struct {
    Modules         []string `mapstructure:"modules"`          // exact go.mod / go.work paths
    PrivatePrefixes []string `mapstructure:"private_prefixes"` // internal module-path prefixes
} `mapstructure:"go"`
```

A repo with no `go:` block indexes zero Go data and no error, like a repo with no
`openapi:`/`proto:` config.

### internal/godep (parser)

New package. Responsibilities:

- **Modules:** parse each listed `go.mod` with `modfile.Parse` (module path, `require`
  entries with the indirect flag, `replace`); parse `go.work` with `modfile.ParseWork`
  (`use` directives → workspace module dirs, each with its own `go.mod`). Cross-check
  `require` versions against `go.sum`; a missing/mismatched sum entry is a warning.
- **Private classification:** a module path is private iff it starts with one of the
  repo's `private_prefixes`. Public dependencies are recorded shallow
  (`module_path`, `version`, `is_private=false`, `direct`) and not deeply indexed.
- **Provider exports:** for each module the repo *defines* (its `module` path) that is
  private, walk the module directory's `*.go` (skip `_test.go`, `vendor/`, generated files
  are kept), go/ast-collect exported top-level declarations:
  - `func` with a capitalized name → `func`; capitalized `NewXxx` returning a type → `constructor`.
  - `type` spec capitalized → `type` or `interface` (interface if the underlying type is an interface).
  - capitalized `const`/`var` → `const`/`var`.
  Capture each declaration's package import path, its doc comment, the `// Package …`
  synopsis per package, and the module README text (e.g. `README.md` at the module root).
- **Consumer usages:** go/ast-parse each `*.go` file's import specs; for an import path
  whose longest matching prefix is a private `require`d module path, resolve module+version,
  record the import (package path), and scan the file for selector expressions
  `alias.Symbol` (alias = the import's local name, default = package name) to record used
  symbols with file and line. Dot-imports and shadowing are best-effort (documented limitation).
- Tolerance: a missing/malformed `go.mod`/`go.work` or an unparseable `.go` file is skipped
  with a warning; the run continues. Idempotent per `index_id`.

### internal/store (storage)

New `internal/store/godep.go` + `schema.go` tables, all `index_id`-scoped, dedicated-table
pattern consistent with the proto family:

```sql
CREATE TABLE dependencies (
  id          INTEGER PRIMARY KEY,
  index_id    INTEGER NOT NULL REFERENCES indexes(id),
  module_path TEXT NOT NULL,
  version     TEXT,
  ecosystem   TEXT NOT NULL,   -- "go"
  is_private  INTEGER NOT NULL,-- 0/1
  direct      INTEGER NOT NULL -- 0/1 (require non-indirect)
);
CREATE TABLE private_libraries (
  id           INTEGER PRIMARY KEY,
  index_id     INTEGER NOT NULL REFERENCES indexes(id),
  module_path  TEXT NOT NULL,
  readme       TEXT,
  doc_synopsis TEXT
);
CREATE TABLE private_library_exports (
  id                 INTEGER PRIMARY KEY,
  private_library_id INTEGER NOT NULL REFERENCES private_libraries(id),
  package_path       TEXT NOT NULL,
  symbol             TEXT NOT NULL,
  kind               TEXT NOT NULL,  -- func|constructor|type|interface|const|var
  doc                TEXT,
  node_id            TEXT            -- linked Graphify code node, nullable
);
CREATE TABLE private_library_usages (
  id           INTEGER PRIMARY KEY,
  index_id     INTEGER NOT NULL REFERENCES indexes(id),
  module_path  TEXT NOT NULL,
  version      TEXT,
  package_path TEXT NOT NULL,
  symbol       TEXT,
  file         TEXT,
  line         INTEGER
);
```

Indexing is idempotent per `index_id`: `ReplaceGoDeps(indexID, bundle)` deletes this index's
rows (children before parents) and re-inserts, identical counts on re-run.

### internal/link (export→node linker)

Extend the shared linker package with a small export matcher: each exported symbol is
best-effort matched to a code node in the provider's index by `label == symbol`, preferring
a node whose `source_file` is within the export's package directory when the package path is
known. Stores the matched `node_id` on the export (nullable when no match). Reuses
`store.NodesByLabel` like the RPC linker; no confidence tiers needed here (it is a direct
name link), so it records the node_id or leaves it null.

### internal/mcp (tools + resources)

- `find_private_library(repo, query)` → provider libraries matching `query`
  (case-insensitive substring) over module path, package paths, `doc_synopsis`, and `readme`,
  each returning `{module_path, packages[], export_count, doc_synopsis}`. Also matches
  `is_private` dependency module paths (path-only) with no indexed provider.
- `find_library_consumers(repo, module_path)` → `{module_path, version, used_packages[],
  used_symbols[{symbol, file, line}], consumed_across_repos}` for the queried repo, where
  `consumed_across_repos` is the explicit marker
  `"deferred: cross-repo consumer aggregation not available in this change"`.
  Unknown module → structured not-found.
- Resources: `lib://<module-path>[/version/<v>][/package/<p>][/symbol/<s>]` resolves the
  provider `private_libraries` row by `module_path` via a single indexed lookup; a module not
  defined by an indexed repo returns not-found. No cross-repo consumer aggregation.

## Data Flow (example)

`find_library_consumers("acme/web", "git.acme.local/platform/auth")`:
1. Resolve repo → `index_id`.
2. Look up the dependency (version) in `dependencies` for that module.
3. Read `private_library_usages` rows for the module → used packages + symbols (file/line).
4. `consumed_across_repos` → deferred marker.
5. Return assembled JSON; unknown module → not-found.

## Error Handling

- Missing/malformed `go.mod`/`go.work`/`.go` → warning on the ingest `Report`; run continues.
- Unknown module/library in a tool or resource → structured not-found, never a crash.
- No provider for a consumed module → consumer data still served; `find_private_library`
  returns the path-only entry.
- Public dependencies are recorded but not deeply indexed (no exports/usages walked).

## Testing Strategy

- **Parser units:** `go.mod` require/replace + indirect flag; `go.work` `use` directives;
  `go.sum` cross-check (match + mismatch warning); export extraction (func/constructor/type/
  interface/const/var) + package doc synopsis + README; consumer import + selector resolution
  with file/line; dot-import/aliased-import handling at best-effort level.
- **Classification:** private prefix match → `is_private=1`, deep-indexed; public dep →
  `is_private=0`, shallow.
- **Linker:** exported symbol matched to a seeded code node by label (+ package-file scope);
  unmatched export → null node_id.
- **Storage:** idempotency (re-index → identical counts).
- **Tools/resources:** `find_private_library` (path/readme/doc match), `find_library_consumers`
  (version + used symbols + deferred marker, not-found), `lib://` provider resolution.

## Decisions and Rationale

- **go/ast import+selector heuristic, not go/packages.** No module download/build required;
  consistent with the parse-files-don't-build approach of the other parsers. Accepts
  best-effort accuracy for dot-imports/shadowing.
- **Single-repo `find_library_consumers`, defer cross-repo aggregation.** Keeps the layer
  consistent with the other repo-scoped tools; cross-repo aggregation is a separate concern.
- **`go.work` included.** Monorepo/workspace layouts are common enough to warrant `use`
  directive + per-module resolution.
- **Per-repo private prefixes.** Avoids global-config coupling; each repo declares its own
  internal namespace.
- **Manifest lists exact go.mod/go.work paths.** Deterministic discovery (no globbing);
  source tree = the file's directory, walked for `*.go`.
- **Purpose match over path + package doc synopsis + README.** Cheap descriptive signal for
  `find_private_library` purpose queries, mostly from data already parsed.
- **Exports link to code nodes.** Enables navigation from a library's exported API into the
  code graph; reuses the shared `internal/link` package.
- **lib:// single-index provider lookup.** Module-path-keyed per CLAUDE.md; serves provider
  data without building cross-repo consumer machinery.

## Spec Patches (written back to OpenSpec delta specs)

The change had no delta specs; this design authors them:

- `go-dependency-index` (ADDED): manifest discovery, modfile/go.work parsing + go.sum
  cross-check, private classification, provider exports + code-node linking, consumer usages,
  idempotency, tolerance.
- `private-library-tools` (ADDED): `find_private_library`, `find_library_consumers`
  (single-repo + deferred cross-repo marker), `lib://` resources.

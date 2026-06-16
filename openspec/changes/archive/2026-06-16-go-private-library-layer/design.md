# Design — go-private-library-layer (high-level)

> Open-phase design: decisions and approach only. Detailed Design Doc + delta specs come in the design phase.

## Architecture

```
 go.mod/sum/work + imports        go-private-library-layer
 ┌──────────────────────┐  parse  ┌────────────────────────────────────────┐
 │ go.mod / go.sum       │ ──────▶ │ dependencies (ecosystem, is_private)    │
 │ go.work / import sites│         │ private_library_exports (provider)      │
 └──────────────────────┘         │ private_library_usages  (consumer)      │
                                  │            │ join                         │
                                  │            ▼                              │
                                  │  code graph (foundation) + merged graph  │
                                  │  tools: find_private_library,            │
                                  │         find_library_consumers           │
                                  │  resources: lib://…                      │
                                  └────────────────────────────────────────┘
```

## Key Decisions

1. **Parse `go.mod`/`go.work` with `golang.org/x/mod/modfile`** (canonical, handles require/replace/exclude). `go.sum` for version verification only.
2. **Exported-symbol extraction via `go/packages` + `go/ast`** (capitalized identifiers in package API). Provider side = the repo defining the module.
3. **Private classification is config-driven**: a list of internal module-path prefixes (viper config shared with the foundation). Everything else is public and not deeply indexed for MVP.
4. **Consumer usage** = imports of a private module + which exported symbols are referenced, with file/line. Resolution depth (full type-check vs. import + identifier heuristic) is a design-phase decision.
5. **Provider↔consumer join** is by module path + version; cross-repo consumer listing uses the merged multi-repo graph.

## Approach Selection

- `find_private_library`: match query against module path / package paths / inferred purpose (from labels/comments).
- `find_library_consumers`: given a module, list repos that `depends_on` it, each with pinned version and `used_symbols`.
- Unused-export detection (exports with no consumer usage) is a natural follow-on but **deferred** unless cheap to include.

## Open Questions (for design phase)

- Used-symbol resolution fidelity (type-checked vs. heuristic) and performance on large dep sets.
- Whether `go.work` multi-module workspaces are in MVP scope.
- Purpose inference for `find_private_library` (labels/doc comments vs. README).

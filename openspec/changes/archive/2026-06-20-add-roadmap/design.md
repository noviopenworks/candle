# Design: add-roadmap

Tweak: add a canonical `Roadmap.md` consolidating the project's direction and
deferred items.

## Input

The content is the output of a deep "what's missing for others to adopt this"
analysis covering: the Graphify activation wall, the missing OpenAPI→handler
linker, the absence of a runnable demo, doc drift, observability gaps, the
one-hop traversal limit, deferred cross-repo RPC `consumed_by`, the Go-only
private-library layer, transport/client compatibility, scale data, incremental
indexing, and multi-tenancy.

## Structure

`Roadmap.md` is organized as:

1. **Current state** — honest baseline of what works (links to getting-started,
   architecture).
2. **North star** — quoted from `design.md` "Final Direction," with the two
   load-bearing questions (*where is this implemented? what breaks if I change
   it?*) called out as the prioritization compass.
3. **Status legend** + four phases, each a table of `# | item | status | why`:
   - Phase 0 — Adoption unblock (demo, OpenAPI linking, Graphify quickstart).
   - Phase 1 — Completeness & trust (doc sweep, observability, multi-hop, RPC
     `consumed_by`, stability policy).
   - Phase 2 — Reach (non-Go ecosystems, transport/client compat, breaking-change
     detection, generated-client analysis).
   - Phase 3 — Scale & operations (benchmarks, incremental indexing,
     multi-tenancy, PR-review automation).
4. **Out of scope** — explicit non-goals (multi-language source parsing, web UI,
   replacing service catalogs, real-time).
5. **Picking up an item** — points at the Comet workflow and the verification
   baseline.
6. **Maintaining this roadmap** — one row per outcome; keep in sync with
   `concepts.md`/`design.md` deferred notes when items land.

## Why these phases / this priority

Phase 0 is "no one adopts until these land": without a runnable demo and the
headline OpenAPI linking actually working, the project cannot be evaluated or
trusted at first contact. Phase 1 hardens the value prop and credibility. Phases
2–3 broaden the audience and the deployment shape. This matches the project's
own stated north star in `design.md` and avoids inventing new direction.

## Verification

- `go build ./...` / `go vet ./...` / `go test ./...` unchanged (documentation
  only; baseline 116/12).
- All in-document links resolve.
- The roadmap does not contradict `design.md` "Final Direction."

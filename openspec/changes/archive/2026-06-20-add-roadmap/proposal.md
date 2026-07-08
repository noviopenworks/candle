## Why

The project's "what's deferred" notes are scattered across `docs/concepts.md`
("What's deferred"), `docs/design.md` ("Updated MVP Scope", "Final Direction"),
and inline code comments (`internal/mcp/context_tools.go`,
`internal/mcp/proto_tools.go`). A new user or contributor has no single forward-
looking view of where candle is going or what is deliberately out of scope.
A deep analysis of "what's missing to make this useful to others" produced a
prioritized set of gaps; capturing them as a committed `Roadmap.md` makes the
direction canonical, reviewable, and maintainable.

## What Changes

- Add a top-level **`Roadmap.md`** that:
  - States the current state and the north star (sourced from `design.md`).
  - Lists outcomes in four phases (Adoption unblock → Completeness & trust →
    Reach → Scale & operations) with a status legend and a "why it matters" per
    item, including file:line references for actionability.
  - Collects the non-goals in one place.
  - Documents how to pick up a roadmap item via the existing Comet workflow.
  - Declares itself the canonical source, superseding the scattered deferred
    notes.

## Capabilities

### Modified Capabilities
<!-- None. Documentation only. -->

## Impact

- **Files:** `Roadmap.md` (new). No code, build, test, tool, or schema change.
- This file becomes the canonical direction reference; the deferred notes in
  `concepts.md` / `design.md` remain accurate until those items land, at which
  point both they and the matching roadmap row update together.

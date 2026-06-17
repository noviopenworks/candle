## Why

candlegraph's value is linking contracts back to code. Today that linking
(`internal/link`) decides `implemented_by` for gRPC RPCs with a **line-by-line
string scan** of the source (`signatureMatches`): it matches an RPC to a code
node by label, checks for a `Register<Svc>Server` label, and confirms the impl
by scanning raw source lines for `rpcName(` and substrings like `context.Context`
/ `Server)`. This is fragile — it breaks on multi-line signatures, can be fooled
by comments or strings, cannot reason about receiver types or generics, and only
reaches its HIGH-confidence tier when the source path happens to be readable from
the process working directory. The linking edges are the product's differentiator,
so their trustworthiness matters more than almost anything else.

## What Changes

- Add an **AST-backed matcher** (`go/ast`) used at **index time** to confirm RPC
  implementations by parsing the real method declaration: receiver, parameter
  shapes, return types, and unary-vs-streaming classification.
- Resolve source through a new **optional per-repo `root:` field** in the
  manifest, so AST reliably finds `source_file` paths instead of depending on the
  process CWD.
- Recalibrate confidence tiers to AST-derived signals (an AST-confirmed signature
  is HIGH; name+service without AST is MEDIUM; name-only is LOW).
- **Fall back** to today's string-scan/name-service heuristic when source is
  unavailable, so repos without a `root:` see no regression.
- Apply the same AST confirmation to private-library **export** linking
  (`MatchExports`) so a same-named symbol in the correct package/declaration wins.

## Capabilities

### New Capabilities
- `ast-linking`: AST-backed confirmation of contract→code links (RPC
  implementations and library exports) at index time, with a manifest source
  root and graceful fallback.

### Modified Capabilities
<!-- None. explain_rpc's and the Go export tools' observable contracts are
     unchanged; only the precision/confidence of the underlying links improves,
     and the new manifest field is additive/optional. -->

## Impact

- **Code:** `internal/link` (new AST matcher + recalibrated scoring + fallback);
  `internal/config` (optional `root` field); `internal/ingest` (pass source root
  into linking). No store schema change — links already carry confidence/reason.
- **Config:** new optional `repos[].root` in `manifest.yaml` (backward compatible).
- **Languages:** Go only (matches the existing gRPC-Go impls and Go private-library layer).
- **Out of scope:** new MCP tools (`find_implementations`/`find_references`), live
  LSP/gopls, replacing the Graphify code-graph dependency, OpenAPI handler
  linking, cross-repo `consumed_by`, non-Go ecosystems.

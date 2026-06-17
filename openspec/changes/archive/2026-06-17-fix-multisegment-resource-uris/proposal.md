## Why

The MCP resource templates advertise commit-pinned URIs whose trailing segment
contains slashes — `proto://…/{kind}/{ref}` (e.g. `rpc/<pkg>/<service>/<rpc>`,
`file/<path>`), `openapi://…/{kind}/{ref}` (spec paths), and `lib://{module}`
(module paths). But these reads return **"Resource not found"**.

**Root cause:** the SDK matches a read URI against each template via
`uritemplate.New(tmpl).Regexp().MatchString(uri)` (go-sdk `serverResourceTemplate.Matches`).
A plain `{ref}` / `{module}` variable uses RFC 6570 *simple* expansion, whose
regexp does **not** match `/`. So any URI with a multi-segment ref fails to route
to the handler — even though the server's own `parse*URI` functions already
handle slashes correctly. Verified empirically: `{ref}` → no match,
`{+ref}` (reserved expansion) → match.

## What Changes

- In `internal/mcp/server.go`, change the resource template variables to RFC 6570
  **reserved expansion** so they match across `/`:
  - `openapi://…/{kind}/{ref}` → `{kind}/{+ref}`
  - `proto://…/{kind}/{ref}` → `{kind}/{+ref}`
  - `lib://{module}` → `lib://{+module}`
  - `graph://…/node/{nodeID}` → `node/{+nodeID}` (consistency/safety)
- Flip the characterization guards in `internal/mcp/e2e_surface_test.go` from
  `mustErr` to `mustContain` for the proto rpc/file and lib resources, now that
  they route.
- No change to the `parse*URI` functions — they already handle slashes.

## Capabilities

### Modified Capabilities
<!-- None. This restores the resource-read behavior the existing specs already
     describe (the URIs were always intended to be readable); no requirement or
     acceptance scenario changes. -->

## Impact

- **Code:** `internal/mcp/server.go` (resource template strings only).
- **Tests:** `internal/mcp/e2e_surface_test.go` (guards flip to assertions).
- No new dependency, API, or schema change. `go-sdk` and `yosida95/uritemplate`
  already support reserved expansion.

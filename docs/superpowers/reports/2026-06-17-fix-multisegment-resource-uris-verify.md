# Verification Report — fix-multisegment-resource-uris

- Date: 2026-06-17
- Workflow: hotfix
- Mode: light (overridden from auto=full — the file count included OpenSpec change
  artifacts; the actual code change is 2 files with no delta spec)

## Result: PASS

## Lightweight checks

| # | Check | Result |
|---|-------|--------|
| 1 | tasks.md all `[x]` | PASS (0 unchecked) |
| 2 | changed files match tasks | PASS — `internal/mcp/server.go` (template vars) + `internal/mcp/e2e_surface_test.go` (guards → assertions) |
| 3 | build passes | PASS — `go build ./...` exit 0 |
| 4 | tests pass | PASS — `go test ./...` all packages, incl. `TestEndToEndToolSurface` now asserting multi-segment resource reads route |
| 5 | no security issues | PASS — resource-template string change, no secrets/unsafe ops |

## Root cause eliminated

The slash-bearing resource template variables now use RFC 6570 reserved
expansion (`{+ref}`/`{+module}`/`{+nodeID}`); no simple-expansion var remains on a
multi-segment path. `repo://{org}/{name}` keeps simple vars (single-segment).
Confirmed by the e2e: `proto://…/rpc/<pkg>/<svc>/<rpc>`, `proto://…/file/<path>`,
and `lib://<module>` all route and return their JSON.

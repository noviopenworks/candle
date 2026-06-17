# Tasks — fix-multisegment-resource-uris

- [x] 1. Change resource template variables to reserved expansion in `internal/mcp/server.go`: `{ref}`→`{+ref}` (openapi, proto), `{module}`→`{+module}` (lib), `{nodeID}`→`{+nodeID}` (graph)
- [x] 2. Flip the e2e characterization guards in `internal/mcp/e2e_surface_test.go` from `mustErr` to `mustContain` for the proto rpc, proto file, and lib resources
- [x] 3. Verify: `go build ./...`, `go vet ./...`, `go test ./...` all pass (incl. `TestEndToEndToolSurface` now asserting the multi-segment reads route)

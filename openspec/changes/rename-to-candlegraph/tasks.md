# Tasks — rename-to-candlegraph

> Implementation was completed in the working tree before this change was opened (retroactive capture). Tasks are checked off in the build phase against the existing diff.

## 1. Module path

- [x] 1.1 Update `go.mod` module directive to `github.com/noviopenworks/candlegraph`
- [x] 1.2 Update all internal `.go` import paths to the new module path (pass 1, before bare-name pass)

## 2. Binary / command name

- [x] 2.1 `git mv cmd/intel-mcp cmd/candlegraph`
- [x] 2.2 Update cobra root `Use` to `candlegraph` in `cmd/candlegraph/main.go`
- [x] 2.3 Update MCP server `Name` to `candlegraph` in `internal/mcp/server.go`
- [x] 2.4 Update e2e-built binary name and comment in `internal/mcp/e2e_test.go`

## 3. Docs

- [x] 3.1 Update `intel-mcp` / `cmd/intel-mcp` references in the 4 `docs/superpowers/plans/` files

## 4. Verification

- [x] 4.1 `go build ./...` passes
- [x] 4.2 `go vet ./...` passes
- [x] 4.3 `go test ./...` passes (all packages)
- [x] 4.4 `git grep` for prior names over tracked source/config (excluding `graphify-out/` and this change's own docs) returns zero hits

# Verification: add-config-scoped-serving

## Task 6: worked example + manual verification

Date: 2026-06-18

### Build

- `go build -o /tmp/candlegraph ./cmd/candlegraph` — PASS
- `go build ./cmd/candlegraph` — PASS

### Manual MCP verification

Prerequisites found:

- `/tmp/vs/intel.db` — present
- `/tmp/vs/scope-inv-wh.yaml` — created for verification from the commits already indexed in `/tmp/vs/intel.db`

Store contents before scoping:

- `VendSYSTEM/bff-service` at `928859b63e7829133964042c01c7c74256c44462`
- `VendSYSTEM/platform-go` at `f9ba13025871e35dad770e95f3eccd750d287141`
- `VendSYSTEM/service-inventory` at `6b5aaa507dd54b5f32e904950261cfb0234ae411`
- `VendSYSTEM/service-user` at `e259b0baef9c2f9873ea1f1f58761d75c27b03bd`
- `VendSYSTEM/warehouse-service` at `85eee1188105bd2f0805d94dfeab487113d4b2a6`

Commands:

```bash
go run . /tmp/vs/scope-inv-wh.yaml
go run . /home/mg/candlegraph/examples/serve-scope.yaml
```

Both commands used a temporary MCP SDK client in `/tmp/opencode/candlegraph-scope-verify` to launch:

```bash
/tmp/candlegraph serve --db /tmp/vs/intel.db --config <scope-file>
```

The client called `list_repos` and then checked all repo-scoped tools with each
omitted repo. `list_repos` returned exactly:

- `VendSYSTEM/service-inventory` at `6b5aaa507dd54b5f32e904950261cfb0234ae411`
- `VendSYSTEM/warehouse-service` at `85eee1188105bd2f0805d94dfeab487113d4b2a6`

The client asserted these repos were omitted from `list_repos` and from each
repo-scoped tool result:

- `VendSYSTEM/service-user`
- `VendSYSTEM/bff-service`
- `VendSYSTEM/platform-go`

Initial result: FAIL.

`list_repos` scoped correctly, and repo-scoped tools did not expose the omitted
repos, but the global `explain_private_library` path still returned the
out-of-scope `VendSYSTEM/platform-go` provider for query `platform`:

```text
explain_private_library returned forbidden repo VendSYSTEM/platform-go
provider.module_path: github.com/VendSYSTEM/platform-go
provider exports node IndexID: 5
```

Root cause: `explain_private_library` filtered cross-repo consumers by scope, but
provider candidate selection still searched private-library module paths store-wide
before checking whether the provider's `index_id` was in scope.

Fix: added a regression test for an out-of-scope provider and filtered
`explain_private_library` candidates to modules with an in-scope provider, or to
providerless modules with in-scope consumers.

Regression verification:

- `go test ./internal/mcp -run TestExplainPrivateLibraryIgnoresOutOfScopeProvider -v` — PASS
- `go test ./internal/mcp -run TestExplainPrivateLibrary -v` — PASS

Manual rerun:

```bash
go build -o /tmp/candlegraph ./cmd/candlegraph
go run . /home/mg/candlegraph/examples/serve-scope.yaml
```

Result: PASS. `list_repos` returned only `VendSYSTEM/service-inventory` and
`VendSYSTEM/warehouse-service`; the verifier completed without any scoped tool
returning `VendSYSTEM/service-user`, `VendSYSTEM/bff-service`, or
`VendSYSTEM/platform-go`.

## Task 7: final verification

Date: 2026-06-18

Commands:

```bash
go test ./...
go vet ./...
git diff 52222b301e473956102b78d2cad37923e3c7dc61 --stat
```

Results:

- `go test ./...` — PASS. All 12 packages passed.
- `go vet ./...` — PASS. No diagnostics.
- `git diff 52222b301e473956102b78d2cad37923e3c7dc61 --stat` — PASS. Diff scope is limited to the planned registry, MCP tools/server/library explain, `cmd/candlegraph`, examples, docs, OpenSpec/comet metadata, plan, tasks, and verification artifacts.

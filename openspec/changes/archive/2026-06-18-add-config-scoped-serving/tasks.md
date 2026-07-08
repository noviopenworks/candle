# Tasks: add-config-scoped-serving

> TDD-oriented. The worked example throughout is an instance scoped to
> `VendSYSTEM/service-inventory` + `VendSYSTEM/warehouse-service` (omitting the other
> indexed repos), per the user's target use case.

## 1. Scope model in the registry (test-first)

- [x] 1.1 Add a failing registry test: given a store with multiple repos and multiple snapshots of one repo, a scoped registry over a chosen `(repo, commit)` set lists/resolves only those; resolution is deterministic to the pinned snapshot
- [x] 1.2 Run the test and confirm it fails
- [x] 1.3 Make `registry` scope-aware: build a scope from allowed `index_id`s; `List`/`Resolve`/`Match` filter to the scope; `nil` scope = serve-all (unchanged). Resolve deterministically to the single in-scope snapshot per repo
- [x] 1.4 Run the test and confirm it passes

## 2. Resolve config → allowed snapshots (test-first)

- [x] 2.1 Add a failing test: given the manifest entries (`repo` + optional `commit`) and the store, compute the allowed `index_id` set; pinned commit selects that snapshot; missing `(repo, commit)` yields a warning, not an error; commit-omitted resolves per the design-decided default
- [x] 2.2 Run and confirm it fails
- [x] 2.3 Implement the config→scope resolver (matches configured `(org/name, commit)` against `indexes`/`repos`), returning allowed `index_id`s + warnings
- [x] 2.4 Run and confirm it passes

## 3. Wire `serve` to the scope config

- [x] 3.1 Add a failing test (or e2e surface assertion) that `serve` with a config scoping to a subset exposes only those repos via `tools/list`-reachable tools (`list_repos` returns only configured repos)
- [x] 3.2 Wire `--config` into `serve` in `cmd/candle/main.go`; on startup build the scope and construct a scoped registry/Tools; no config → serve-all
- [x] 3.3 Implement working-location discovery + precedence (explicit `--config` wins; else discover from cwd; else serve-all) per design
- [x] 3.4 Run and confirm pass

## 4. Constrain cross-repo aggregation to the scope

- [x] 4.1 Add a failing test: `explain_private_library` / `find_library_consumers` cross-repo aggregation under a scope only aggregates configured repos
- [x] 4.2 Run and confirm it fails
- [x] 4.3 Constrain `store.PrivateConsumersAcrossRepos` (or the Tools-layer caller) to the allowed `index_id`s per the design decision
- [x] 4.4 Run and confirm it passes

## 5. Worked example + manual verification: inventory + warehouse

- [x] 5.1 Provide an example serve config scoping to exactly `VendSYSTEM/service-inventory` and `VendSYSTEM/warehouse-service` (e.g. `examples/serve-scope.yaml`)
- [x] 5.2 Manually verify against a multi-repo store: with that config, `list_repos` returns only service-inventory + warehouse-service; service-user / bff-service / platform-go are omitted from every tool
- [x] 5.3 Record the manual verification result in the verification report

## 6. Documentation

- [x] 6.1 Update `docs/configuration.md`: serve-time scope config, `commit` pinning semantics, discovery/precedence, missing-snapshot warning
- [x] 6.2 Update `docs/getting-started.md` / `README.md`: running multiple isolated, config-scoped MCP instances; the inventory+warehouse example

## 7. Final verification

- [x] 7.1 Run `go test ./...` and confirm pass
- [x] 7.2 Run `go vet ./...` and confirm pass
- [x] 7.3 Inspect `git diff` and confirm scope matches the plan

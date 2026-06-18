---
change: add-config-scoped-serving
design-doc: docs/superpowers/specs/2026-06-18-config-scoped-serving-design.md
base-ref: 52222b301e473956102b78d2cad37923e3c7dc61
---

# Config-scoped MCP serving Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scope `candlegraph serve` to a YAML config so an MCP instance exposes only the configured `(repo, commit)` snapshots, deterministically; no config ⇒ serve everything (backward compatible).

**Architecture:** A scope is an allow-set of `index_id`s built from the manifest at serve startup and injected into a scope-aware `registry`. Tools resolve through the registry unchanged; cross-repo aggregation filters to the allow-set in the Tools layer. All new entry points are additive `*Scoped` variants so existing callers/tests are untouched.

**Tech Stack:** Go 1.26, cobra CLI, `internal/config`, `internal/registry`, `internal/mcp`, SQLite.

## Global Constraints

- Module path `github.com/noviopenworks/candlegraph`.
- **Additive / non-breaking:** keep `registry.New`, `mcp.NewTools`, `mcp.NewServer`, `mcp.Serve` signatures; add `*Scoped` variants. No existing test edits required to compile.
- Scope is `map[int64]bool` (allowed `index_id`s); `nil`/absent ⇒ unscoped (serve all).
- `commit` set ⇒ exact-commit match; `commit` omitted ⇒ latest snapshot by `ingested_at`.
- Missing configured `(repo, commit)` ⇒ warning (stderr), non-fatal.
- serve reuses `config.Load` (the manifest); manifest entries already carry `graph:` (required by the loader) — serve ignores graph paths, documented.
- Discovery: explicit `--config` (error if path missing) wins; else default `manifest.yaml` in cwd if present; else no scope.
- Gates: `go test ./...`, `go vet ./...` pass.

---

### Task 1: Scope-aware registry

**Files:**
- Modify: `internal/registry/registry.go`
- Test: `internal/registry/registry_scope_test.go`

**Interfaces:**
- Consumes: `store.Store` + `indexes`/`repos` schema; existing `RepoInfo`.
- Produces: `func NewScoped(s *store.Store, allowed map[int64]bool) *Registry`; scope-filtered `List`/`Resolve`/`Match`; `func (r *Registry) InScope(indexID int64) bool`.

- [x] **Step 1: Write the failing test**

Create `internal/registry/registry_scope_test.go`:

```go
package registry

import (
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

func seedTwoSnapshots(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web1.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertIndex("org", "web", "c2", "release", "/g/web2.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertIndex("org", "other", "x1", "main", "/g/other.json"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestScopedRegistryFiltersAndResolvesDeterministically(t *testing.T) {
	s := seedTwoSnapshots(t)
	// Find the index_id of org/web @ c2 to scope to it.
	var c2 int64
	if err := s.DB.QueryRow(`SELECT i.id FROM indexes i JOIN repos r ON r.id=i.repo_id WHERE r.name='web' AND i.commit_sha='c2'`).Scan(&c2); err != nil {
		t.Fatal(err)
	}
	reg := NewScoped(s, map[int64]bool{c2: true})

	list, err := reg.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Commit != "c2" {
		t.Fatalf("scoped List should return only c2, got %+v", list)
	}
	ri, ok, err := reg.Resolve("org/web")
	if err != nil || !ok {
		t.Fatalf("resolve org/web: ok=%v err=%v", ok, err)
	}
	if ri.Commit != "c2" {
		t.Fatalf("scoped Resolve must be deterministic to c2, got %q", ri.Commit)
	}
	if reg.InScope(c2) != true {
		t.Fatal("c2 should be in scope")
	}
	if reg.InScope(c2 + 100) != false {
		t.Fatal("unknown id should be out of scope")
	}
}

func TestUnscopedRegistryUnchanged(t *testing.T) {
	s := seedTwoSnapshots(t)
	reg := New(s) // nil scope
	list, err := reg.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("unscoped List should return all 3 snapshots, got %d", len(list))
	}
	if !reg.InScope(999) {
		t.Fatal("unscoped InScope must be true for any id")
	}
}
```

- [x] **Step 2: Run test, verify it fails**

Run: `go test ./internal/registry -run 'TestScopedRegistry|TestUnscoped' -v`
Expected: FAIL — undefined `NewScoped` / `InScope`.

- [x] **Step 3: Implement scope-aware registry**

In `internal/registry/registry.go`:

Change the struct and constructors:

```go
// Registry resolves repo identities to indexed snapshots, optionally scoped to
// an allow-set of index ids (nil = unscoped, serve all).
type Registry struct {
	s       *store.Store
	allowed map[int64]bool // nil = unscoped
}

// New builds an unscoped Registry over the store.
func New(s *store.Store) *Registry { return &Registry{s: s} }

// NewScoped builds a Registry limited to the given index ids.
func NewScoped(s *store.Store, allowed map[int64]bool) *Registry {
	return &Registry{s: s, allowed: allowed}
}

// InScope reports whether an index id is served. Unscoped registries serve all.
func (r *Registry) InScope(indexID int64) bool {
	if r.allowed == nil {
		return true
	}
	return r.allowed[indexID]
}
```

In `List`, after scanning each `ri`, skip out-of-scope rows:

```go
	for rows.Next() {
		var ri RepoInfo
		var org, name string
		if err := rows.Scan(&ri.IndexID, &org, &name, &ri.Branch, &ri.Commit, &ri.IngestedAt, &ri.NodeCount); err != nil {
			return nil, err
		}
		if !r.InScope(ri.IndexID) {
			continue
		}
		ri.Repo = org + "/" + name
		out = append(out, ri)
	}
```

`Resolve` and `Match` already iterate `List()`, so they inherit the scope filter automatically; no further change needed. (Scoped `List` returns at most one snapshot per repo because the scope pins one, so `Resolve`'s first-match is deterministic.)

- [x] **Step 4: Run test, verify it passes**

Run: `go test ./internal/registry -run 'TestScopedRegistry|TestUnscoped' -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/registry/registry.go internal/registry/registry_scope_test.go
git commit -m "feat(registry): scope-aware registry (allow-set of index ids)"
```

---

### Task 2: Build scope from config

**Files:**
- Modify: `internal/registry/registry.go`
- Test: `internal/registry/registry_scope_test.go`

**Interfaces:**
- Consumes: `config.Config`/`config.RepoConfig` (`Org()`, `Name()`, `Commit`), store `indexes`/`repos`.
- Produces: `func BuildScope(s *store.Store, cfg *config.Config) (map[int64]bool, []string, error)`.

- [x] **Step 1: Write the failing test**

Append to `internal/registry/registry_scope_test.go`:

```go
import (
	// add to existing imports:
	"github.com/noviopenworks/candlegraph/internal/config"
)

func TestBuildScopePinAndLatestAndMissing(t *testing.T) {
	s := seedTwoSnapshots(t)
	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "org/web", Commit: "c1", Graph: "/g/web1.json"},   // pinned
		{Repo: "org/other", Graph: "/g/other.json"},              // commit omitted -> latest
		{Repo: "org/ghost", Commit: "zz", Graph: "/g/ghost.json"}, // missing -> warning
	}}
	allowed, warns, err := BuildScope(s, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 2 {
		t.Fatalf("expected 2 allowed snapshots, got %d (%v)", len(allowed), allowed)
	}
	if len(warns) != 1 {
		t.Fatalf("expected 1 warning for the missing entry, got %v", warns)
	}
	// org/web must be pinned to c1, not c2.
	var c1, c2 int64
	s.DB.QueryRow(`SELECT i.id FROM indexes i JOIN repos r ON r.id=i.repo_id WHERE r.name='web' AND i.commit_sha='c1'`).Scan(&c1)
	s.DB.QueryRow(`SELECT i.id FROM indexes i JOIN repos r ON r.id=i.repo_id WHERE r.name='web' AND i.commit_sha='c2'`).Scan(&c2)
	if !allowed[c1] || allowed[c2] {
		t.Fatalf("org/web must be pinned to c1 only: allowed=%v", allowed)
	}
}
```

- [x] **Step 2: Run test, verify it fails**

Run: `go test ./internal/registry -run TestBuildScope -v`
Expected: FAIL — undefined `BuildScope`.

- [x] **Step 3: Implement BuildScope**

Add to `internal/registry/registry.go` (add `"github.com/noviopenworks/candlegraph/internal/config"` to imports):

```go
// snapshot is one indexes row used for scope resolution.
type snapshot struct {
	indexID    int64
	repo       string
	commit     string
	ingestedAt string
}

// BuildScope resolves config entries to an allow-set of index ids. A config
// entry with a commit pins that snapshot; without a commit it selects the
// repo's latest snapshot (by ingested_at). Entries with no matching snapshot
// produce a warning and are skipped. Returns nil only on a hard query error.
func BuildScope(s *store.Store, cfg *config.Config) (map[int64]bool, []string, error) {
	rows, err := s.DB.Query(`
		SELECT i.id, r.org, r.name, COALESCE(i.commit_sha,''), i.ingested_at
		FROM indexes i JOIN repos r ON r.id=i.repo_id`)
	if err != nil {
		return nil, nil, err
	}
	byRepo := map[string][]snapshot{}
	for rows.Next() {
		var sn snapshot
		var org, name string
		if err := rows.Scan(&sn.indexID, &org, &name, &sn.commit, &sn.ingestedAt); err != nil {
			rows.Close()
			return nil, nil, err
		}
		sn.repo = org + "/" + name
		byRepo[sn.repo] = append(byRepo[sn.repo], sn)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	allowed := map[int64]bool{}
	var warns []string
	for _, rc := range cfg.Repos {
		snaps := byRepo[rc.Repo]
		if len(snaps) == 0 {
			warns = append(warns, fmt.Sprintf("%s: no indexed snapshot in store", rc.Repo))
			continue
		}
		if rc.Commit != "" {
			found := false
			for _, sn := range snaps {
				if sn.commit == rc.Commit {
					allowed[sn.indexID] = true
					found = true
					break
				}
			}
			if !found {
				warns = append(warns, fmt.Sprintf("%s: commit %s not indexed", rc.Repo, rc.Commit))
			}
			continue
		}
		// commit omitted: pick the latest by ingested_at (RFC3339 sorts lexically).
		latest := snaps[0]
		for _, sn := range snaps[1:] {
			if sn.ingestedAt > latest.ingestedAt {
				latest = sn
			}
		}
		allowed[latest.indexID] = true
	}
	return allowed, warns, nil
}
```

Add `"fmt"` to imports if not present.

- [x] **Step 4: Run test, verify it passes**

Run: `go test ./internal/registry -run TestBuildScope -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/registry/registry.go internal/registry/registry_scope_test.go
git commit -m "feat(registry): BuildScope resolves config to allowed index ids"
```

---

### Task 3: Scoped Tools/Server/Serve entry points

**Files:**
- Modify: `internal/mcp/tools.go`, `internal/mcp/server.go`

**Interfaces:**
- Produces: `NewToolsScoped(s, allowed)`, `NewServerScoped(s, allowed)`, `ServeScoped(ctx, s, allowed)`. Existing `NewTools`/`NewServer`/`Serve` delegate with `nil` scope (unchanged behavior).

- [x] **Step 1: Add scoped Tools constructor**

In `internal/mcp/tools.go`:

```go
// NewTools builds an unscoped tool set.
func NewTools(s *store.Store) *Tools { return NewToolsScoped(s, nil) }

// NewToolsScoped builds a tool set limited to the given index ids (nil = all).
func NewToolsScoped(s *store.Store, allowed map[int64]bool) *Tools {
	return &Tools{s: s, reg: registry.NewScoped(s, allowed)}
}
```

(`registry.NewScoped(s, nil)` is equivalent to `registry.New(s)`, so existing callers are unaffected.)

- [x] **Step 2: Add scoped Server + Serve**

In `internal/mcp/server.go`, change `NewServer` to delegate and add scoped variants:

```go
func NewServer(s *store.Store) *mcpsdk.Server { return NewServerScoped(s, nil) }

func NewServerScoped(s *store.Store, allowed map[int64]bool) *mcpsdk.Server {
	tools := NewToolsScoped(s, allowed)
	// ... existing body that builds srv and registers tools, using `tools` ...
}

func Serve(ctx context.Context, s *store.Store) error { return ServeScoped(ctx, s, nil) }

func ServeScoped(ctx context.Context, s *store.Store, allowed map[int64]bool) error {
	return NewServerScoped(s, allowed).Run(ctx, &mcpsdk.StdioTransport{})
}
```

Move the existing `NewServer` body into `NewServerScoped` (replace its `tools := NewTools(s)` line with the `tools` built from `allowed`).

- [x] **Step 3: Run mcp tests**

Run: `go test ./internal/mcp`
Expected: PASS (existing tests use `NewTools`/`NewServer`/`Serve`, all still valid).

- [x] **Step 4: Commit**

```bash
git add internal/mcp/tools.go internal/mcp/server.go
git commit -m "feat(mcp): scoped Tools/Server/Serve entry points (additive)"
```

---

### Task 4: Constrain cross-repo aggregation to the scope

**Files:**
- Modify: `internal/mcp/library_explain.go`, `internal/mcp/godep_tools.go`
- Test: `internal/mcp/library_explain_test.go`

**Interfaces:**
- Consumes: `t.reg.InScope(indexID)`; `store.RepoConsumer.IndexID`.

- [x] **Step 1: Write the failing test**

Append to `internal/mcp/library_explain_test.go`:

```go
func TestExplainPrivateLibraryRespectsScope(t *testing.T) {
	// Reuse seedExplain (provider + one consumer org/web). Build a scope that
	// EXCLUDES the consumer index, and assert the consumer is filtered out.
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// provider only in scope; consumer indexed but out of scope.
	pid, _ := s.UpsertIndex("org", "auth-lib", "p1", "main", "/g/a.json")
	_ = s.ReplaceGoDeps(pid, store.GoDepBundle{Libraries: []store.PrivateLibraryBundle{{
		Library: store.PrivateLibrary{ModulePath: "github.com/org/auth", DocSynopsis: "x"},
		Exports: []store.PrivateExport{{PackagePath: "github.com/org/auth", Symbol: "F"}},
	}}})
	cid, _ := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	_ = s.ReplaceGoDeps(cid, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/auth", Version: "v1", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/auth", Version: "v1", PackagePath: "github.com/org/auth", Symbol: "F", File: "x.go", Line: 1}},
	})
	tools := NewToolsScoped(s, map[int64]bool{pid: true}) // consumer cid NOT in scope
	out, err := tools.ExplainPrivateLibrary("github.com/org/auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Consumers) != 0 {
		t.Fatalf("out-of-scope consumer must be filtered, got %+v", out.Consumers)
	}
}
```

- [x] **Step 2: Run test, verify it fails**

Run: `go test ./internal/mcp -run TestExplainPrivateLibraryRespectsScope -v`
Expected: FAIL — consumer still present (no scope filter yet).

- [x] **Step 3: Filter consumers by scope**

In `internal/mcp/library_explain.go`, where consumers are appended in `ExplainPrivateLibrary`, skip out-of-scope index ids:

```go
	for _, c := range cons {
		if !t.reg.InScope(c.IndexID) {
			continue
		}
		ci := ConsumerInfo{Repo: c.Repo, Commit: c.Commit, Version: c.Version, UsedPackages: c.UsedPackages}
		// ... unchanged usage loop ...
		out.Consumers = append(out.Consumers, ci)
	}
```

(`find_library_consumers` is already single-repo via `Resolve`, which is scope-filtered in Task 1, so an out-of-scope repo resolves to not-found — no change needed there. If a test shows otherwise, add the same `InScope` guard.)

- [x] **Step 4: Run tests, verify pass**

Run: `go test ./internal/mcp -run TestExplainPrivateLibrary -v`
Expected: PASS (all explain tests, incl. the new scope test).

- [x] **Step 5: Commit**

```bash
git add internal/mcp/library_explain.go internal/mcp/library_explain_test.go
git commit -m "feat(mcp): scope cross-repo consumer aggregation to allowed index ids"
```

---

### Task 5: Wire `serve` to discover and apply the scope config

**Files:**
- Modify: `cmd/candlegraph/main.go`

**Interfaces:**
- Consumes: cobra `cmd.Flags().Changed("config")`, `config.Load`, `registry.BuildScope`, `mcp.ServeScoped`, `mcp.Serve`.

- [x] **Step 1: Implement serve scope resolution**

Replace the `serveCmd` `RunE` body in `cmd/candlegraph/main.go`:

```go
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			// Resolve the scope config: explicit --config wins (must exist);
			// else the default manifest.yaml in cwd if present; else no scope.
			scopePath := ""
			if cmd.Flags().Changed("config") {
				scopePath = manifest
			} else if _, statErr := os.Stat(manifest); statErr == nil {
				scopePath = manifest
			}

			var allowed map[int64]bool
			if scopePath != "" {
				cfg, lerr := config.Load(scopePath)
				if lerr != nil {
					return lerr
				}
				a, warns, berr := registry.BuildScope(s, cfg)
				if berr != nil {
					return berr
				}
				for _, w := range warns {
					fmt.Fprintln(os.Stderr, "scope warning:", w)
				}
				allowed = a
				fmt.Fprintf(os.Stderr, "serving %d configured snapshot(s) from %s\n", len(allowed), scopePath)
			}

			if allowed == nil {
				return mcp.Serve(context.Background(), s)
			}
			return mcp.ServeScoped(context.Background(), s, allowed)
		},
```

Add imports as needed: `"os"` (already present), `"github.com/noviopenworks/candlegraph/internal/config"`, `"github.com/noviopenworks/candlegraph/internal/registry"`.

- [x] **Step 2: Build the binary**

Run: `go build ./cmd/candlegraph`
Expected: compiles cleanly.

- [x] **Step 3: Commit**

```bash
git add cmd/candlegraph/main.go
git commit -m "feat(cli): serve scopes to a discovered/explicit config (back-compat serve-all)"
```

---

### Task 6: Worked example + manual verification (inventory + warehouse)

**Files:**
- Create: `examples/serve-scope.yaml`
- Modify: `docs/configuration.md`, `docs/getting-started.md`, `README.md`

- [x] **Step 1: Add an example serve-scope manifest**

Create `examples/serve-scope.yaml` (a manifest subset; `graph:` is required by the loader and may point at the same graphs used to index):

```yaml
# Serve-scope example: expose only inventory + warehouse from a larger store.
# `candlegraph serve --db intel.db --config examples/serve-scope.yaml`
repos:
  - repo: VendSYSTEM/service-inventory
    graph: /abs/path/service-inventory/graphify-out/graph.json
    commit: 6b5aaa507dd54b5f32e904950261cfb0234ae411
  - repo: VendSYSTEM/warehouse-service
    graph: /abs/path/warehouse-service/graphify-out/graph.json
    commit: 85eee1188105bd2f0805d94dfeab487113d4b2a6
```

- [x] **Step 2: Document serve scoping**

In `docs/configuration.md`, add a "Serve scope" section: serve reads `--config` (default `manifest.yaml`) and exposes only the listed `(repo, commit)` snapshots; `commit` omitted ⇒ latest; missing snapshot ⇒ warning; no config ⇒ serve all. Note `graph:` is still required by the loader (point it at the index graphs).
In `docs/getting-started.md` + `README.md`, add a short "Running multiple scoped instances" note with the inventory+warehouse example.

- [x] **Step 3: Manual verification against a multi-repo store**

Run (using the existing 5-repo store at /tmp/vs/intel.db, with a scope file pinning the two repos' commits):

```bash
go build -o /tmp/candlegraph ./cmd/candlegraph
# scope file lists only service-inventory + warehouse-service (with their commits)
/tmp/candlegraph serve --db /tmp/vs/intel.db --config /tmp/vs/scope-inv-wh.yaml
# Then via an MCP client: list_repos returns ONLY service-inventory + warehouse-service.
```

Expected: `list_repos` shows exactly the two configured repos; service-user / bff-service / platform-go are omitted. Record the result in the verification report.

- [x] **Step 4: Commit**

```bash
git add examples/serve-scope.yaml docs/configuration.md docs/getting-started.md README.md
git commit -m "docs: document config-scoped serving + serve-scope example"
```

---

### Task 7: Final verification

**Files:** all files touched above.

- [x] **Step 1: Full test suite** — Run: `go test ./...` → PASS.
- [x] **Step 2: Static checks** — Run: `go vet ./...` → PASS.
- [x] **Step 3: Diff scope** — Run: `git diff 52222b301e473956102b78d2cad37923e3c7dc61 --stat` → only registry, mcp (tools/server/library_explain), cmd/main.go, examples, docs (plus OpenSpec/comet + plan/design artifacts).

---

## Self-Review

- **Spec coverage:** Task 1 (deterministic scoped resolution + list filter), Task 2 (config→scope: pin/latest/missing-warning), Task 3 (serve-time injection, additive), Task 4 (cross-repo respects scope), Task 5 (serve discovery/precedence + back-compat serve-all), Task 6 (docs + inventory/warehouse worked example), Task 7 (gates).
- **Placeholder scan:** none — concrete code/commands throughout.
- **Type consistency:** scope is `map[int64]bool` end-to-end; `registry.New`→unscoped, `NewScoped`→scoped; `mcp.NewTools/NewServer/Serve` retained, `*Scoped` added; `RepoConsumer.IndexID` drives the Task 4 filter; `config.RepoConfig` fields `Repo`/`Commit`/`Graph` + `Org()`/`Name()` exist.
- **Scope check:** one capability (mcp-core) extension; additive entry points; no storage/parser/index changes.
- **Known ergonomic wrinkle (documented):** serve configs require `graph:` (loader validation); acceptable since the scope config is the index manifest. A serve-tolerant loader is a possible future relaxation, out of scope here.

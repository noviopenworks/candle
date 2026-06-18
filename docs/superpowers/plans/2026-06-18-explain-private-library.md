---
change: add-explain-private-library
design-doc: docs/superpowers/specs/2026-06-18-explain-private-library-design.md
base-ref: 7028fefe6d7fa8da8e0c1a754818b23c7726f202
---

# explain_private_library Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `explain_private_library`, a both-sides MCP tool that explains an internal Go library — provider exports (code-graph linked) plus cross-repo consumer aggregation (versions, used symbols, best-effort code-graph linked).

**Architecture:** Two new cross-index store helpers (`SearchPrivateModulePaths`, `PrivateConsumersAcrossRepos`) following the `NodesByLabelAllIndexes` pattern; a pure SDK-free `Tools.ExplainPrivateLibrary` in a new `internal/mcp/library_explain.go`; thin registration in `server.go` (15th tool).

**Tech Stack:** Go 1.26, `internal/store` (SQLite), `internal/mcp`, MCP Go SDK.

## Global Constraints

- Module path: `github.com/noviopenworks/candlegraph`.
- Additive only: do NOT change `find_private_library` / `find_library_consumers` behavior or existing outputs.
- `:memory:` caveat: in cross-index store queries, fully scan and **close the cursor before** issuing nested queries (a second pooled connection to `:memory:` is a separate empty DB). Reuse existing index-scoped helpers after closing.
- Export linking uses `PrivateExport.NodeID` (resolve via `NodeByID`); fall back to `NodesByLabel` only if `NodeID` is empty; mark unresolved otherwise.
- Consumer linking: `NodesByFile(consumerIndex, usage.File)`, parse `source_location` `L<n>`, link the node with the greatest `n ≤ usage.Line`; unresolved-marked otherwise.
- Advertised tool count 14 → 15.
- Gates: `go test ./...` and `go vet ./...` pass.

---

### Task 1: Cross-index consumer aggregation (store)

**Files:**
- Modify: `internal/store/godep.go`
- Test: `internal/store/godep_test.go`

**Interfaces:**
- Consumes (existing): `UpsertIndex`, `ReplaceGoDeps`, `PrivateUsagesByModule(indexID, modulePath)`, `DependencyByModule(indexID, modulePath)`, `s.DB`.
- Produces: type `RepoConsumer{ IndexID int64; Repo, Commit, Version string; UsedPackages []string; UsedSymbols []PrivateUsage }`; `func (s *Store) PrivateConsumersAcrossRepos(modulePath string) ([]RepoConsumer, error)`.

- [x] **Step 1: Write the failing store test**

Append to `internal/store/godep_test.go` (create the file with `package store` + imports if it does not exist):

```go
func seedConsumer(t *testing.T, s *Store, org, name, commit, modulePath, version, sym, file string, line int) {
	t.Helper()
	id, err := s.UpsertIndex(org, name, commit, "main", "/g/"+name+".json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(id, GoDepBundle{
		Dependencies: []Dependency{{ModulePath: modulePath, Version: version, Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []PrivateUsage{{ModulePath: modulePath, Version: version, PackagePath: modulePath, Symbol: sym, File: file, Line: line}},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateConsumersAcrossRepos(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	seedConsumer(t, s, "org", "web", "c1", "github.com/org/auth", "v1.2.0", "ValidateToken", "internal/http/a.go", 12)
	seedConsumer(t, s, "org", "worker", "c2", "github.com/org/auth", "v1.1.0", "NewClient", "internal/job/b.go", 30)

	cons, err := s.PrivateConsumersAcrossRepos("github.com/org/auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(cons) != 2 {
		t.Fatalf("expected 2 consumers across repos, got %d: %+v", len(cons), cons)
	}
	byRepo := map[string]RepoConsumer{}
	for _, c := range cons {
		byRepo[c.Repo] = c
	}
	if byRepo["org/web"].Version != "v1.2.0" || byRepo["org/worker"].Version != "v1.1.0" {
		t.Fatalf("version aggregation mismatch: %+v", byRepo)
	}
	if len(byRepo["org/web"].UsedSymbols) != 1 || byRepo["org/web"].UsedSymbols[0].Symbol != "ValidateToken" {
		t.Fatalf("used symbols mismatch: %+v", byRepo["org/web"])
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store -run TestPrivateConsumersAcrossRepos -v`
Expected: FAIL — undefined `PrivateConsumersAcrossRepos` / `RepoConsumer`.

- [x] **Step 3: Implement the cross-index aggregation**

Add to `internal/store/godep.go`:

```go
// RepoConsumer is one repo's consumption of a private module (cross-repo aggregation).
type RepoConsumer struct {
	IndexID      int64          `json:"-"`
	Repo         string         `json:"repo"`
	Commit       string         `json:"commit"`
	Version      string         `json:"version"`
	UsedPackages []string       `json:"used_packages"`
	UsedSymbols  []PrivateUsage `json:"used_symbols"`
}

// PrivateConsumersAcrossRepos aggregates, across all indexes, every repo that
// uses or depends on modulePath. It collects the consuming index ids first
// (closing the cursor) and then reuses index-scoped helpers, per the :memory:
// pooled-connection caveat.
func (s *Store) PrivateConsumersAcrossRepos(modulePath string) ([]RepoConsumer, error) {
	rows, err := s.DB.Query(`
		SELECT DISTINCT i.id, r.org, r.name, COALESCE(i.commit_sha,'')
		FROM indexes i JOIN repos r ON r.id=i.repo_id
		WHERE i.id IN (SELECT index_id FROM private_library_usages WHERE module_path=?)
		   OR i.id IN (SELECT index_id FROM dependencies WHERE module_path=? AND is_private=1)
		ORDER BY r.org, r.name`, modulePath, modulePath)
	if err != nil {
		return nil, err
	}
	type ident struct {
		id     int64
		repo   string
		commit string
	}
	var idents []ident
	for rows.Next() {
		var it ident
		var org, name string
		if err := rows.Scan(&it.id, &org, &name, &it.commit); err != nil {
			rows.Close()
			return nil, err
		}
		it.repo = org + "/" + name
		idents = append(idents, it)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var out []RepoConsumer
	for _, it := range idents {
		usages, err := s.PrivateUsagesByModule(it.id, modulePath)
		if err != nil {
			return nil, err
		}
		rc := RepoConsumer{IndexID: it.id, Repo: it.repo, Commit: it.commit, UsedSymbols: usages}
		if dep, found, err := s.DependencyByModule(it.id, modulePath); err != nil {
			return nil, err
		} else if found {
			rc.Version = dep.Version
		}
		if rc.Version == "" {
			for _, u := range usages {
				if u.Version != "" {
					rc.Version = u.Version
					break
				}
			}
		}
		seen := map[string]bool{}
		for _, u := range usages {
			if u.PackagePath != "" && !seen[u.PackagePath] {
				seen[u.PackagePath] = true
				rc.UsedPackages = append(rc.UsedPackages, u.PackagePath)
			}
		}
		out = append(out, rc)
	}
	return out, nil
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store -run TestPrivateConsumersAcrossRepos -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/store/godep.go internal/store/godep_test.go
git commit -m "feat(store): cross-repo private library consumer aggregation"
```

---

### Task 2: Cross-index module-path resolution (store)

**Files:**
- Modify: `internal/store/godep.go`
- Test: `internal/store/godep_test.go`

**Interfaces:**
- Produces: `func (s *Store) SearchPrivateModulePaths(query string) ([]string, error)` — distinct private module paths across all indexes matching query.

- [x] **Step 1: Write the failing test**

Append:

```go
func TestSearchPrivateModulePaths(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	seedConsumer(t, s, "org", "web", "c1", "github.com/org/auth", "v1.2.0", "ValidateToken", "a.go", 1)
	got, err := s.SearchPrivateModulePaths("auth")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "github.com/org/auth" {
		t.Fatalf("expected [github.com/org/auth], got %v", got)
	}
	none, err := s.SearchPrivateModulePaths("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no matches, got %v", none)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store -run TestSearchPrivateModulePaths -v`
Expected: FAIL — undefined `SearchPrivateModulePaths`.

- [x] **Step 3: Implement**

Add to `internal/store/godep.go`:

```go
// SearchPrivateModulePaths returns distinct private module paths across all
// indexes whose module path, doc synopsis, readme, or package path matches
// query, plus path-only private dependencies matching by module path.
func (s *Store) SearchPrivateModulePaths(query string) ([]string, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`
		SELECT module_path FROM private_libraries
		WHERE LOWER(module_path) LIKE ? OR LOWER(COALESCE(doc_synopsis,'')) LIKE ? OR LOWER(COALESCE(readme,'')) LIKE ?
		   OR id IN (SELECT private_library_id FROM private_library_exports WHERE LOWER(package_path) LIKE ?)
		UNION
		SELECT module_path FROM dependencies WHERE is_private=1 AND LOWER(module_path) LIKE ?`,
		q, q, q, q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	var out []string
	for rows.Next() {
		var mp string
		if err := rows.Scan(&mp); err != nil {
			return nil, err
		}
		if !seen[mp] {
			seen[mp] = true
			out = append(out, mp)
		}
	}
	return out, rows.Err()
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store -run TestSearchPrivateModulePaths -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/store/godep.go internal/store/godep_test.go
git commit -m "feat(store): cross-index private module path search"
```

---

### Task 3: ExplainPrivateLibrary — provider + consumers

**Files:**
- Create: `internal/mcp/library_explain.go`
- Test: `internal/mcp/library_explain_test.go`

**Interfaces:**
- Consumes (existing): `t.s.SearchPrivateModulePaths`, `t.s.PrivateLibraryByModule`, `t.s.PrivateConsumersAcrossRepos`, `t.s.NodeByID`, `t.s.NodesByLabel`, `t.s.NodesByFile`, `ErrNotFound`; `store.PrivateLibraryRow{ PrivateLibrary{ModulePath,DocSynopsis}, IndexID, Exports []PrivateExport }`, `store.PrivateExport{PackagePath,Symbol,Kind,Doc,NodeID}`, `store.RepoConsumer`, `store.PrivateUsage`, `store.NodeRow`.
- Produces: `func (t *Tools) ExplainPrivateLibrary(query string) (LibraryExplanation, error)` + result types.

- [x] **Step 1: Write the failing test**

Create `internal/mcp/library_explain_test.go`:

```go
package mcp

import (
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

func seedExplain(t *testing.T) *Tools {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Provider repo defines github.com/org/auth with one export node.
	pid, err := s.UpsertIndex("org", "auth-lib", "p1", "main", "/g/auth.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		pid, "n_validate", "ValidateToken", "code", "auth/token.go", "L5"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(pid, store.GoDepBundle{
		Libraries: []store.PrivateLibraryBundle{{
			Library: store.PrivateLibrary{ModulePath: "github.com/org/auth", DocSynopsis: "auth helpers"},
			Exports: []store.PrivateExport{{PackagePath: "github.com/org/auth", Symbol: "ValidateToken", Kind: "func", Doc: "validates", NodeID: "n_validate"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	// Consumer repo with a node enclosing the usage at line 12 (def at L8).
	cid, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file,source_location) VALUES(?,?,?,?,?,?)`,
		cid, "n_login", "Login", "code", "internal/http/auth.go", "L8"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(cid, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/auth", Version: "v1.2.0", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/auth", Version: "v1.2.0", PackagePath: "github.com/org/auth", Symbol: "ValidateToken", File: "internal/http/auth.go", Line: 12}},
	}); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestExplainPrivateLibraryProviderAndConsumers(t *testing.T) {
	tools := seedExplain(t)
	out, err := tools.ExplainPrivateLibrary("auth")
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider.ModulePath != "github.com/org/auth" {
		t.Fatalf("provider module mismatch: %+v", out.Provider)
	}
	if len(out.Provider.Exports) != 1 || !out.Provider.Exports[0].Resolved || out.Provider.Exports[0].Node == nil {
		t.Fatalf("expected resolved export node, got %+v", out.Provider.Exports)
	}
	if len(out.Consumers) != 1 || out.Consumers[0].Repo != "org/web" || out.Consumers[0].Version != "v1.2.0" {
		t.Fatalf("consumer mismatch: %+v", out.Consumers)
	}
	if len(out.Consumers[0].Usages) != 1 || !out.Consumers[0].Usages[0].Resolved || out.Consumers[0].Usages[0].Node == nil {
		t.Fatalf("expected resolved consumer usage link: %+v", out.Consumers[0].Usages)
	}
	if out.Consumers[0].Usages[0].Node.NodeID != "n_login" {
		t.Fatalf("expected enclosing node n_login, got %+v", out.Consumers[0].Usages[0].Node)
	}
	if len(out.Limitations) == 0 {
		t.Fatalf("expected limitations")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp -run TestExplainPrivateLibraryProviderAndConsumers -v`
Expected: FAIL — undefined `ExplainPrivateLibrary` / result types.

- [x] **Step 3: Implement the tool**

Create `internal/mcp/library_explain.go`:

```go
package mcp

import (
	"strconv"
	"strings"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// LibraryExplanation is the explain_private_library result: provider definition
// plus cross-repo consumers, with code-graph links where resolvable.
type LibraryExplanation struct {
	Query       string         `json:"query"`
	Provider    ProviderInfo   `json:"provider"`
	Consumers   []ConsumerInfo `json:"consumers"`
	Candidates  []string       `json:"candidates,omitempty"`
	Limitations []string       `json:"limitations"`
}

// ProviderInfo is the provider side of a private library.
type ProviderInfo struct {
	ModulePath  string       `json:"module_path"`
	Repo        string       `json:"repo,omitempty"`
	Commit      string       `json:"commit,omitempty"`
	DocSynopsis string       `json:"doc_synopsis,omitempty"`
	Packages    []string     `json:"packages,omitempty"`
	Exports     []ExportInfo `json:"exports,omitempty"`
}

// ExportInfo is one provider export with an optional code-graph link.
type ExportInfo struct {
	PackagePath string         `json:"package_path"`
	Symbol      string         `json:"symbol"`
	Kind        string         `json:"kind,omitempty"`
	Doc         string         `json:"doc,omitempty"`
	Node        *store.NodeRow `json:"node,omitempty"`
	Resolved    bool           `json:"resolved"`
}

// ConsumerInfo is one consuming repo.
type ConsumerInfo struct {
	Repo         string      `json:"repo"`
	Commit       string      `json:"commit,omitempty"`
	Version      string      `json:"version,omitempty"`
	UsedPackages []string    `json:"used_packages,omitempty"`
	Usages       []UsageLink `json:"usages,omitempty"`
}

// UsageLink is one usage with an optional best-effort consumer node link.
type UsageLink struct {
	Usage    store.PrivateUsage `json:"usage"`
	Node     *store.NodeRow     `json:"node,omitempty"`
	Resolved bool               `json:"resolved"`
}

func explainLimitations() []string {
	return []string{
		"Version-diff and breaking-change analysis are out of scope for explain_private_library.",
		"Multi-hop call-path expansion and transitive dependents are deferred.",
		"Only Go private libraries are supported.",
	}
}

// ExplainPrivateLibrary implements explain_private_library: resolve a fuzzy
// query to a private library, then explain provider exports and cross-repo
// consumers with code-graph links.
func (t *Tools) ExplainPrivateLibrary(query string) (LibraryExplanation, error) {
	paths, err := t.s.SearchPrivateModulePaths(query)
	if err != nil {
		return LibraryExplanation{}, err
	}
	if len(paths) == 0 {
		return LibraryExplanation{}, ErrNotFound
	}
	best := paths[0]
	for _, p := range paths {
		if p == strings.TrimSpace(query) {
			best = p
			break
		}
	}
	var candidates []string
	for _, p := range paths {
		if p != best {
			candidates = append(candidates, p)
		}
	}

	out := LibraryExplanation{
		Query:       query,
		Provider:    ProviderInfo{ModulePath: best},
		Candidates:  candidates,
		Limitations: explainLimitations(),
	}

	if lib, found, err := t.s.PrivateLibraryByModule(best); err != nil {
		return LibraryExplanation{}, err
	} else if found {
		out.Provider.DocSynopsis = lib.DocSynopsis
		out.Provider.Commit = ""
		pkgSeen := map[string]bool{}
		for _, e := range lib.Exports {
			ei := ExportInfo{PackagePath: e.PackagePath, Symbol: e.Symbol, Kind: e.Kind, Doc: e.Doc}
			if node, ok := t.resolveExportNode(lib.IndexID, e); ok {
				ei.Node = node
				ei.Resolved = true
			}
			out.Provider.Exports = append(out.Provider.Exports, ei)
			if e.PackagePath != "" && !pkgSeen[e.PackagePath] {
				pkgSeen[e.PackagePath] = true
				out.Provider.Packages = append(out.Provider.Packages, e.PackagePath)
			}
		}
	}

	cons, err := t.s.PrivateConsumersAcrossRepos(best)
	if err != nil {
		return LibraryExplanation{}, err
	}
	for _, c := range cons {
		ci := ConsumerInfo{Repo: c.Repo, Commit: c.Commit, Version: c.Version, UsedPackages: c.UsedPackages}
		for _, u := range c.UsedSymbols {
			ul := UsageLink{Usage: u}
			if node, ok := t.resolveUsageNode(c.IndexID, u); ok {
				ul.Node = node
				ul.Resolved = true
			}
			ci.Usages = append(ci.Usages, ul)
		}
		out.Consumers = append(out.Consumers, ci)
	}
	return out, nil
}

// resolveExportNode links an export to its provider node, preferring the stored
// NodeID, falling back to a label match.
func (t *Tools) resolveExportNode(indexID int64, e store.PrivateExport) (*store.NodeRow, bool) {
	if e.NodeID != "" {
		if n, found, err := t.s.NodeByID(indexID, e.NodeID); err == nil && found {
			return &n, true
		}
	}
	if e.Symbol != "" {
		if nodes, err := t.s.NodesByLabel(indexID, e.Symbol); err == nil && len(nodes) > 0 {
			n := nodes[0]
			return &n, true
		}
	}
	return nil, false
}

// resolveUsageNode best-effort links a usage to the enclosing consumer node:
// the node in the usage's file with the greatest definition line <= usage line.
func (t *Tools) resolveUsageNode(indexID int64, u store.PrivateUsage) (*store.NodeRow, bool) {
	if u.File == "" {
		return nil, false
	}
	nodes, err := t.s.NodesByFile(indexID, u.File)
	if err != nil || len(nodes) == 0 {
		return nil, false
	}
	bestLine := -1
	var best *store.NodeRow
	for i := range nodes {
		line := parseSourceLine(nodes[i].SourceLocation)
		if line >= 0 && line <= u.Line && line > bestLine {
			bestLine = line
			best = &nodes[i]
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

// parseSourceLine parses a "L<n>" source location into an int, or -1.
func parseSourceLine(loc string) int {
	s := strings.TrimPrefix(strings.TrimSpace(loc), "L")
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp -run TestExplainPrivateLibraryProviderAndConsumers -v`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add internal/mcp/library_explain.go internal/mcp/library_explain_test.go
git commit -m "feat(mcp): add ExplainPrivateLibrary provider+consumers with graph links"
```

---

### Task 4: Boundary behavior — candidates, provider-less, not-found, unresolved link

**Files:**
- Modify: `internal/mcp/library_explain_test.go`

**Interfaces:**
- Consumes (from Task 3): `Tools.ExplainPrivateLibrary`, all result types.

- [x] **Step 1: Write failing/boundary tests**

Append to `internal/mcp/library_explain_test.go`:

```go
func TestExplainPrivateLibraryUnknownQuery(t *testing.T) {
	tools := seedExplain(t)
	_, err := tools.ExplainPrivateLibrary("does-not-exist")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestExplainPrivateLibraryProviderLess(t *testing.T) {
	// A module consumed but with no indexed provider: provider section empty,
	// consumers still returned, no error.
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	cid, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(cid, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "github.com/org/nostub", Version: "v0.1.0", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Usages:       []store.PrivateUsage{{ModulePath: "github.com/org/nostub", Version: "v0.1.0", PackagePath: "github.com/org/nostub", Symbol: "Do", File: "x.go", Line: 3}},
	}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)
	out, err := tools.ExplainPrivateLibrary("nostub")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Provider.Exports) != 0 {
		t.Fatalf("expected no provider exports, got %+v", out.Provider.Exports)
	}
	if len(out.Consumers) != 1 || out.Consumers[0].Repo != "org/web" {
		t.Fatalf("expected consumer org/web, got %+v", out.Consumers)
	}
	// No node in x.go -> usage link unresolved, not errored.
	if out.Consumers[0].Usages[0].Resolved {
		t.Fatalf("expected unresolved consumer link, got %+v", out.Consumers[0].Usages[0])
	}
}

func TestExplainPrivateLibraryAmbiguousReturnsCandidates(t *testing.T) {
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	id, err := s.UpsertIndex("org", "web", "c1", "main", "/g/web.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(id, store.GoDepBundle{
		Dependencies: []store.Dependency{
			{ModulePath: "github.com/org/authcore", Version: "v1.0.0", Ecosystem: "go", IsPrivate: true, Direct: true},
			{ModulePath: "github.com/org/authutil", Version: "v1.0.0", Ecosystem: "go", IsPrivate: true, Direct: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)
	out, err := tools.ExplainPrivateLibrary("auth")
	if err != nil {
		t.Fatal(err)
	}
	if out.Provider.ModulePath == "" {
		t.Fatalf("expected a best match")
	}
	if len(out.Candidates) != 1 {
		t.Fatalf("expected 1 candidate alongside best match, got %+v", out.Candidates)
	}
}
```

- [x] **Step 2: Run tests**

Run: `go test ./internal/mcp -run TestExplainPrivateLibrary -v`
Expected: PASS (Task 3's implementation already covers these boundaries; these tests lock the behavior in).

- [x] **Step 3: Commit**

```bash
git add internal/mcp/library_explain_test.go
git commit -m "test(mcp): explain_private_library boundary cases"
```

---

### Task 5: Register the MCP tool

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/e2e_surface_test.go`

**Interfaces:**
- Consumes: `mcpsdk.AddTool`, `Tools.ExplainPrivateLibrary`, helpers `textResult`/`mustJSON`/`toolErr`, `context`.

- [x] **Step 1: Add to ToolNames**

In `internal/mcp/server.go` `var ToolNames`, add `"explain_private_library",` immediately after `"find_library_consumers",`.

- [x] **Step 2: Register in NewServer**

In `NewServer`, add after `registerFindLibraryConsumers(srv, tools)`:

```go
	registerExplainPrivateLibrary(srv, tools)
```

- [x] **Step 3: Add registration function**

Add after `registerFindLibraryConsumers`:

```go
type explainPrivateLibraryArgs struct {
	Query string `json:"query" jsonschema:"library name, module path, or purpose"`
}

func registerExplainPrivateLibrary(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "explain_private_library",
		Description: "Explain an internal Go library from both sides: provider exports (code-graph linked) and cross-repo consumers with versions and used symbols.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args explainPrivateLibraryArgs) (*mcpsdk.CallToolResult, any, error) {
		out, err := tools.ExplainPrivateLibrary(args.Query)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(out)), nil, nil
	})
}
```

- [x] **Step 4: Update e2e surface comments**

In `internal/mcp/e2e_surface_test.go`, update the two comments referencing "14 tools" / "advertises all 14" to "15". The assertion loops over `ToolNames`, so adding the entry there extends the checked surface.

- [x] **Step 5: Run MCP tests**

Run: `go test ./internal/mcp -v`
Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add internal/mcp/server.go internal/mcp/e2e_surface_test.go
git commit -m "feat(mcp): register explain_private_library as the 15th tool"
```

---

### Task 6: Documentation

**Files:**
- Modify: `docs/tools.md`
- Modify: `docs/examples.md`
- Modify: `README.md`

- [x] **Step 1: Update `docs/tools.md`**

Change the tool count to **15 tools**, add `explain_private_library` to the list, and add a reference section in the private-library tools area:

```markdown
### `explain_private_library`

Explain an internal Go library from both sides: the provider definition (exports with code-graph node links, packages, doc synopsis) and **cross-repo consumers** — every indexed repo that uses the library, with its pinned version and used symbols (each best-effort linked to the enclosing consumer code-graph node).

| Arg | Type | Description |
|-----|------|-------------|
| `query` | string | library name, module path, doc synopsis, or purpose (fuzzy) |

**Request:**

```json
{"query": "auth"}
```

**Response:** `provider` (module path, exports with `node`/`resolved`, packages), `consumers` (per repo: version, used packages, usages with `node`/`resolved`), `candidates` when ambiguous, and `limitations`. Unlike `find_library_consumers` (single repo, deferred cross-repo marker), this aggregates consumers across all indexed repos.
```

- [x] **Step 2: Update `docs/examples.md`**

Add an example titled `Who consumes this library across the org?`:

```json
{"query": "auth"}
```

Explain that the response lists every consuming repo with its pinned version and used symbols, so an agent can spot version skew and usage hotspots in one call, then follow `explain_symbol` on a linked consumer node.

- [x] **Step 3: Update `README.md`**

Change the advertised count from `14 tools` to `15 tools` (the ASCII diagram line and the Tools reference table row).

- [x] **Step 4: Verify build is unaffected**

Run: `go test ./...`
Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add docs/tools.md docs/examples.md README.md
git commit -m "docs: document explain_private_library"
```

---

### Task 7: Final verification

**Files:** all files touched above.

- [x] **Step 1: Full test suite**

Run: `go test ./...`
Expected: PASS.

- [x] **Step 2: Static checks**

Run: `go vet ./...`
Expected: PASS.

- [x] **Step 3: Inspect diff scope**

Run: `git diff 7028fefe6d7fa8da8e0c1a754818b23c7726f202 --stat`
Expected: only `internal/store/godep.go`, `internal/store/godep_test.go`, `internal/mcp/library_explain.go`, `internal/mcp/library_explain_test.go`, `internal/mcp/server.go`, `internal/mcp/e2e_surface_test.go`, the three docs (plus OpenSpec/comet + plan/design artifacts).

---

## Self-Review

- **Spec coverage:** Task 1 (cross-repo aggregation requirement), Task 2+3 (fuzzy resolution + both-sides explain + unknown→not-found), Task 3 (code-graph linking: export via NodeID, consumer via enclosing line), Task 4 (candidates, provider-less, unresolved link), Task 5 (advertised tool), Task 6 (docs), Task 7 (gates).
- **Placeholder scan:** none — all steps carry concrete code/commands.
- **Type consistency:** `RepoConsumer` (store) → `ConsumerInfo`/`UsageLink` (mcp); `PrivateExport.NodeID` resolved via `NodeByID`; `parseSourceLine` handles `L<n>`; `resolveUsageNode` returns greatest line ≤ usage line. `ReplaceGoDeps` uses `Libraries []PrivateLibraryBundle` and `Dependencies`/`Usages` fields per `GoDepBundle`.
- **Scope check:** one capability extension; additive; store changes are two focused cross-index helpers reusing existing index-scoped methods.

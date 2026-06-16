---
change: go-private-library-layer
design-doc: docs/superpowers/specs/2026-06-16-go-private-library-layer-design.md
base-ref: b023b33ddfc354de7b0bdf0fa6c374ce2527dcb8
---

# Go Private-Library Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Index Go private libraries from both sides — provider exports (linked to code nodes) and consumer usages — and serve them via `find_private_library`, `find_library_consumers`, and `lib://` resources.

**Architecture:** Mirror the proto layer. A new `internal/godep` package parses `go.mod`/`go.sum`/`go.work` (golang.org/x/mod/modfile) and walks module source with `go/ast` to extract exports + usages (no build). Dedicated `index_id`-scoped SQLite tables. Exports link to Graphify nodes via the shared `internal/link` package. Ingest runs the Go pass after `graph.Load`.

**Tech Stack:** Go 1.26, modernc.org/sqlite, golang.org/x/mod, go/ast+go/parser (stdlib), modelcontextprotocol/go-sdk, spf13/viper.

---

## Conventions (read before starting)

- Module path: `github.com/vend-ai/intel-mcp`.
- Store tests open `store.Open(":memory:")`; follow `internal/store/proto_test.go`.
- Pure tool logic in `internal/mcp/*_tools.go`; SDK registration in `internal/mcp/server.go`.
- Run full suite: `go test ./...`. Build: `go build ./...`. Vet: `go vet ./...`. Format check: `gofmt -l internal/ cmd/`.
- Commit after every task; check off the matching `openspec/changes/go-private-library-layer/tasks.md` item in the same commit.
- `kind` values for exports: exactly `func | constructor | type | interface | const | var`.

## File Structure

- Modify `internal/store/schema.go` — add 4 tables + indexes.
- Create `internal/store/godep.go` — types, `GoDepBundle`, `ReplaceGoDeps`, queries.
- Create `internal/store/godep_test.go`.
- Modify `internal/config/config.go` — add `go:` block to `RepoConfig`.
- Modify `internal/config/config_test.go`.
- Create `internal/godep/modfile.go` — go.mod/go.work/go.sum parsing + classification.
- Create `internal/godep/exports.go` — provider export extraction (go/ast).
- Create `internal/godep/usages.go` — consumer usage extraction (go/ast).
- Create `internal/godep/godep.go` — `Parse` orchestrator + shared types.
- Create `internal/godep/godep_test.go` + `internal/godep/testdata/…`.
- Modify `internal/link/link.go` — add `MatchExports`.
- Modify `internal/link/link_test.go`.
- Modify `internal/ingest/ingest.go` — Go pass after `graph.Load`.
- Modify `internal/ingest/ingest_test.go`.
- Create `internal/mcp/godep_tools.go` — `FindPrivateLibrary`, `FindLibraryConsumers`.
- Create `internal/mcp/godep_tools_test.go`.
- Modify `internal/mcp/resources.go` — `lib://` resource methods.
- Modify `internal/mcp/server.go` — register tools/resources; extend `ToolNames`; `parseLibURI`.
- Modify `internal/mcp/resources_test.go`.

---

## Task 1: Storage schema and tables

**Files:**
- Modify: `internal/store/schema.go`
- Create: `internal/store/godep.go`
- Test: `internal/store/godep_test.go`

- [x] **Step 1: Add tables to schema**

In `internal/store/schema.go`, append inside the `schemaSQL` string before the closing backtick:

```sql
CREATE TABLE IF NOT EXISTS dependencies (
  id          INTEGER PRIMARY KEY,
  index_id    INTEGER NOT NULL REFERENCES indexes(id),
  module_path TEXT NOT NULL,
  version     TEXT,
  ecosystem   TEXT NOT NULL,
  is_private  INTEGER NOT NULL,
  direct      INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS private_libraries (
  id           INTEGER PRIMARY KEY,
  index_id     INTEGER NOT NULL REFERENCES indexes(id),
  module_path  TEXT NOT NULL,
  readme       TEXT,
  doc_synopsis TEXT
);
CREATE TABLE IF NOT EXISTS private_library_exports (
  id                 INTEGER PRIMARY KEY,
  private_library_id INTEGER NOT NULL REFERENCES private_libraries(id),
  package_path       TEXT NOT NULL,
  symbol             TEXT NOT NULL,
  kind               TEXT NOT NULL,
  doc                TEXT,
  node_id            TEXT
);
CREATE TABLE IF NOT EXISTS private_library_usages (
  id           INTEGER PRIMARY KEY,
  index_id     INTEGER NOT NULL REFERENCES indexes(id),
  module_path  TEXT NOT NULL,
  version      TEXT,
  package_path TEXT NOT NULL,
  symbol       TEXT,
  file         TEXT,
  line         INTEGER
);
CREATE INDEX IF NOT EXISTS idx_dependencies_index ON dependencies(index_id);
CREATE INDEX IF NOT EXISTS idx_private_libs_index ON private_libraries(index_id);
CREATE INDEX IF NOT EXISTS idx_private_libs_module ON private_libraries(module_path);
CREATE INDEX IF NOT EXISTS idx_private_exports_lib ON private_library_exports(private_library_id);
CREATE INDEX IF NOT EXISTS idx_private_usages_index ON private_library_usages(index_id);
CREATE INDEX IF NOT EXISTS idx_private_usages_module ON private_library_usages(index_id, module_path);
```

- [x] **Step 2: Write the failing storage test**

Create `internal/store/godep_test.go`:

```go
package store

import "testing"

func seedGoDeps(t *testing.T) (*Store, int64) {
	t.Helper()
	s, _ := Open(":memory:")
	id, _ := s.UpsertIndex("acme", "web", "abc", "main", "/g")
	bundle := GoDepBundle{
		Dependencies: []Dependency{
			{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", Ecosystem: "go", IsPrivate: true, Direct: true},
			{ModulePath: "github.com/spf13/viper", Version: "v1.21.0", Ecosystem: "go", IsPrivate: false, Direct: true},
		},
		Libraries: []PrivateLibraryBundle{{
			Library: PrivateLibrary{ModulePath: "git.acme.local/platform/auth", Readme: "Auth helpers", DocSynopsis: "Package auth provides tokens"},
			Exports: []PrivateExport{{PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", Kind: "constructor", Doc: "NewClient builds a client", NodeID: "n1"}},
		}},
		Usages: []PrivateUsage{
			{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", File: "main.go", Line: 12},
		},
	}
	if err := s.ReplaceGoDeps(id, bundle); err != nil {
		t.Fatalf("replace: %v", err)
	}
	return s, id
}

func TestGoDepStorageAndIdempotent(t *testing.T) {
	s, id := seedGoDeps(t)
	defer s.Close()

	libs, err := s.FindPrivateLibraries(id, "auth")
	if err != nil || len(libs) != 1 || libs[0].ModulePath != "git.acme.local/platform/auth" || libs[0].ExportCount != 1 {
		t.Fatalf("find libs: %+v err=%v", libs, err)
	}
	dep, ok, err := s.DependencyByModule(id, "git.acme.local/platform/auth")
	if err != nil || !ok || dep.Version != "v1.2.0" {
		t.Fatalf("dep: %+v ok=%v err=%v", dep, ok, err)
	}
	usages, err := s.PrivateUsagesByModule(id, "git.acme.local/platform/auth")
	if err != nil || len(usages) != 1 || usages[0].Symbol != "NewClient" || usages[0].Line != 12 {
		t.Fatalf("usages: %+v err=%v", usages, err)
	}
	lib, ok, err := s.PrivateLibraryByModule("git.acme.local/platform/auth")
	if err != nil || !ok || len(lib.Exports) != 1 || lib.Exports[0].NodeID != "n1" {
		t.Fatalf("lib by module: %+v ok=%v err=%v", lib, ok, err)
	}

	// Idempotent: empty replace clears.
	if err := s.ReplaceGoDeps(id, GoDepBundle{}); err != nil {
		t.Fatalf("re-replace: %v", err)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM dependencies WHERE index_id=?`, id).Scan(&n)
	if n != 0 {
		t.Fatalf("expected 0 deps after empty replace, got %d", n)
	}
}
```

- [x] **Step 3: Run the test, verify it FAILS**

Run: `go test ./internal/store/ -run TestGoDep -v`
Expected: FAIL — `GoDepBundle` etc. undefined.

- [x] **Step 4: Implement `internal/store/godep.go`**

```go
package store

import "strings"

// Dependency is a stored module dependency.
type Dependency struct {
	ModulePath string
	Version    string
	Ecosystem  string
	IsPrivate  bool
	Direct     bool
}

// PrivateExport is one exported symbol of a private library.
type PrivateExport struct {
	PackagePath string
	Symbol      string
	Kind        string
	Doc         string
	NodeID      string
}

// PrivateLibrary is a provider module's metadata.
type PrivateLibrary struct {
	ID          int64
	ModulePath  string
	Readme      string
	DocSynopsis string
}

// PrivateLibraryBundle groups a library with its exports for insertion.
type PrivateLibraryBundle struct {
	Library PrivateLibrary
	Exports []PrivateExport
}

// PrivateLibraryRow is a library plus its exports (read side).
type PrivateLibraryRow struct {
	PrivateLibrary
	IndexID int64
	Exports []PrivateExport
}

// PrivateLibraryResult is a find_private_library match.
type PrivateLibraryResult struct {
	ModulePath  string
	Packages    []string
	ExportCount int
	DocSynopsis string
}

// PrivateUsage is a consumer's use of a private module symbol.
type PrivateUsage struct {
	ModulePath  string
	Version     string
	PackagePath string
	Symbol      string
	File        string
	Line        int
}

// GoDepBundle is the full Go dependency data for one index.
type GoDepBundle struct {
	Dependencies []Dependency
	Libraries    []PrivateLibraryBundle
	Usages       []PrivateUsage
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ReplaceGoDeps replaces all Go dependency data for indexID. Idempotent.
func (s *Store) ReplaceGoDeps(indexID int64, b GoDepBundle) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`DELETE FROM private_library_exports WHERE private_library_id IN (SELECT id FROM private_libraries WHERE index_id=?)`,
		`DELETE FROM private_libraries WHERE index_id=?`,
		`DELETE FROM private_library_usages WHERE index_id=?`,
		`DELETE FROM dependencies WHERE index_id=?`,
	}
	for _, q := range stmts {
		if _, err := tx.Exec(q, indexID); err != nil {
			return err
		}
	}
	for _, d := range b.Dependencies {
		if _, err := tx.Exec(`INSERT INTO dependencies(index_id, module_path, version, ecosystem, is_private, direct) VALUES(?,?,?,?,?,?)`,
			indexID, d.ModulePath, d.Version, d.Ecosystem, boolToInt(d.IsPrivate), boolToInt(d.Direct)); err != nil {
			return err
		}
	}
	for _, lb := range b.Libraries {
		res, err := tx.Exec(`INSERT INTO private_libraries(index_id, module_path, readme, doc_synopsis) VALUES(?,?,?,?)`,
			indexID, lb.Library.ModulePath, lb.Library.Readme, lb.Library.DocSynopsis)
		if err != nil {
			return err
		}
		libID, _ := res.LastInsertId()
		for _, e := range lb.Exports {
			if _, err := tx.Exec(`INSERT INTO private_library_exports(private_library_id, package_path, symbol, kind, doc, node_id) VALUES(?,?,?,?,?,?)`,
				libID, e.PackagePath, e.Symbol, e.Kind, e.Doc, e.NodeID); err != nil {
				return err
			}
		}
	}
	for _, u := range b.Usages {
		if _, err := tx.Exec(`INSERT INTO private_library_usages(index_id, module_path, version, package_path, symbol, file, line) VALUES(?,?,?,?,?,?,?)`,
			indexID, u.ModulePath, u.Version, u.PackagePath, u.Symbol, u.File, u.Line); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) exportsByLib(libID int64) ([]PrivateExport, error) {
	rows, err := s.DB.Query(`SELECT package_path, symbol, kind, COALESCE(doc,''), COALESCE(node_id,'')
		FROM private_library_exports WHERE private_library_id=?`, libID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrivateExport
	for rows.Next() {
		var e PrivateExport
		if err := rows.Scan(&e.PackagePath, &e.Symbol, &e.Kind, &e.Doc, &e.NodeID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// FindPrivateLibraries matches provider libraries in indexID by module path,
// doc synopsis, readme, or any export package path (case-insensitive).
func (s *Store) FindPrivateLibraries(indexID int64, query string) ([]PrivateLibraryResult, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT id, module_path, COALESCE(doc_synopsis,'') FROM private_libraries
		WHERE index_id=? AND (LOWER(module_path) LIKE ? OR LOWER(COALESCE(doc_synopsis,'')) LIKE ? OR LOWER(COALESCE(readme,'')) LIKE ?
		  OR id IN (SELECT private_library_id FROM private_library_exports WHERE LOWER(package_path) LIKE ?))`,
		indexID, q, q, q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrivateLibraryResult
	for rows.Next() {
		var id int64
		var r PrivateLibraryResult
		if err := rows.Scan(&id, &r.ModulePath, &r.DocSynopsis); err != nil {
			return nil, err
		}
		exps, err := s.exportsByLib(id)
		if err != nil {
			return nil, err
		}
		r.ExportCount = len(exps)
		seen := map[string]bool{}
		for _, e := range exps {
			if !seen[e.PackagePath] {
				seen[e.PackagePath] = true
				r.Packages = append(r.Packages, e.PackagePath)
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FindPrivateDeps returns private dependencies in indexID whose module path
// matches query (path-only matches for find_private_library).
func (s *Store) FindPrivateDeps(indexID int64, query string) ([]Dependency, error) {
	q := "%" + strings.ToLower(query) + "%"
	rows, err := s.DB.Query(`SELECT module_path, COALESCE(version,''), ecosystem, is_private, direct
		FROM dependencies WHERE index_id=? AND is_private=1 AND LOWER(module_path) LIKE ?`, indexID, q)
	if err != nil {
		return nil, err
	}
	return scanDeps(rows)
}

func scanDeps(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]Dependency, error) {
	defer rows.Close()
	var out []Dependency
	for rows.Next() {
		var d Dependency
		var priv, direct int
		if err := rows.Scan(&d.ModulePath, &d.Version, &d.Ecosystem, &priv, &direct); err != nil {
			return nil, err
		}
		d.IsPrivate, d.Direct = priv == 1, direct == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

// DependencyByModule returns the dependency for a module path in indexID.
func (s *Store) DependencyByModule(indexID int64, modulePath string) (Dependency, bool, error) {
	rows, err := s.DB.Query(`SELECT module_path, COALESCE(version,''), ecosystem, is_private, direct
		FROM dependencies WHERE index_id=? AND module_path=?`, indexID, modulePath)
	if err != nil {
		return Dependency{}, false, err
	}
	deps, err := scanDeps(rows)
	if err != nil || len(deps) == 0 {
		return Dependency{}, false, err
	}
	return deps[0], true, nil
}

// PrivateUsagesByModule returns consumer usages of a module in indexID.
func (s *Store) PrivateUsagesByModule(indexID int64, modulePath string) ([]PrivateUsage, error) {
	rows, err := s.DB.Query(`SELECT module_path, COALESCE(version,''), package_path, COALESCE(symbol,''), COALESCE(file,''), COALESCE(line,0)
		FROM private_library_usages WHERE index_id=? AND module_path=?`, indexID, modulePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrivateUsage
	for rows.Next() {
		var u PrivateUsage
		if err := rows.Scan(&u.ModulePath, &u.Version, &u.PackagePath, &u.Symbol, &u.File, &u.Line); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// PrivateLibraryByModule returns the provider library (with exports) for a module
// path, searched store-wide (the defining repo is unique). For lib:// resources.
func (s *Store) PrivateLibraryByModule(modulePath string) (PrivateLibraryRow, bool, error) {
	rows, err := s.DB.Query(`SELECT id, index_id, module_path, COALESCE(readme,''), COALESCE(doc_synopsis,'')
		FROM private_libraries WHERE module_path=? LIMIT 1`, modulePath)
	if err != nil {
		return PrivateLibraryRow{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return PrivateLibraryRow{}, false, rows.Err()
	}
	var r PrivateLibraryRow
	if err := rows.Scan(&r.ID, &r.IndexID, &r.ModulePath, &r.Readme, &r.DocSynopsis); err != nil {
		return PrivateLibraryRow{}, false, err
	}
	rows.Close()
	exps, err := s.exportsByLib(r.ID)
	if err != nil {
		return PrivateLibraryRow{}, false, err
	}
	r.Exports = exps
	return r, true, nil
}
```

- [x] **Step 5: Run the test, verify it PASSES**

Run: `go test ./internal/store/ -v` → PASS (all store tests). Then `go build ./...`, `go vet ./internal/store/`.

- [x] **Step 6: Commit**

```bash
git add internal/store/schema.go internal/store/godep.go internal/store/godep_test.go openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(store): go dependency / private-library tables and queries

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 1.1, 1.2 `[x]`.

---

## Task 2: Manifest go config

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [x] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestGoConfigParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	yaml := "repos:\n" +
		"  - repo: acme/web\n" +
		"    graph: /tmp/g.json\n" +
		"    go:\n" +
		"      modules: [go.mod]\n" +
		"      private_prefixes: [git.acme.local/]\n"
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	r := cfg.Repos[0]
	if len(r.Go.Modules) != 1 || r.Go.Modules[0] != "go.mod" {
		t.Fatalf("modules: %+v", r.Go.Modules)
	}
	if len(r.Go.PrivatePrefixes) != 1 || r.Go.PrivatePrefixes[0] != "git.acme.local/" {
		t.Fatalf("prefixes: %+v", r.Go.PrivatePrefixes)
	}
}
```

Ensure `os`, `path/filepath`, `testing` are imported.

- [x] **Step 2: Run, verify FAIL**

Run: `go test ./internal/config/ -run TestGoConfig -v` → FAIL (`r.Go` undefined).

- [x] **Step 3: Add the go block to `RepoConfig`**

In `internal/config/config.go`, add to the `RepoConfig` struct after the `Proto` block:

```go
	Go struct {
		Modules         []string `mapstructure:"modules"`
		PrivatePrefixes []string `mapstructure:"private_prefixes"`
	} `mapstructure:"go"`
```

- [x] **Step 4: Run, verify PASS**

Run: `go test ./internal/config/ -v` → PASS. Then `go build ./...`.

- [x] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(config): per-repo go modules/private_prefixes block

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 2.1 `[x]`.

---

## Task 3: Module parsing (go.mod / go.work / go.sum)

**Files:**
- Create: `internal/godep/godep.go`
- Create: `internal/godep/modfile.go`
- Create: `internal/godep/godep_test.go`
- Create: `internal/godep/testdata/provider/go.mod`, `internal/godep/testdata/consumer/go.mod`, `internal/godep/testdata/consumer/go.sum`
- Modify: `go.mod` / `go.sum` (add golang.org/x/mod)

- [x] **Step 1: Add the x/mod dependency**

Run: `go get golang.org/x/mod@latest`

- [x] **Step 2: Create fixtures**

`internal/godep/testdata/provider/go.mod`:
```
module git.acme.local/platform/auth

go 1.26
```

`internal/godep/testdata/consumer/go.mod`:
```
module git.acme.local/apps/web

go 1.26

require (
	git.acme.local/platform/auth v1.2.0
	github.com/spf13/viper v1.21.0 // indirect
)
```

`internal/godep/testdata/consumer/go.sum`:
```
git.acme.local/platform/auth v1.2.0 h1:abc=
git.acme.local/platform/auth v1.2.0/go.mod h1:def=
```

- [x] **Step 3: Write the failing test**

Create `internal/godep/godep_test.go`:

```go
package godep

import "testing"

func TestParseConsumerModule(t *testing.T) {
	res, warns, err := Parse([]string{"testdata/consumer/go.mod"}, []string{"git.acme.local/"})
	if err != nil {
		t.Fatalf("parse: %v (warns=%v)", err, warns)
	}
	if res.ModulePath != "git.acme.local/apps/web" {
		t.Fatalf("module path: %q", res.ModulePath)
	}
	byPath := map[string]Dependency{}
	for _, d := range res.Dependencies {
		byPath[d.ModulePath] = d
	}
	auth, ok := byPath["git.acme.local/platform/auth"]
	if !ok || auth.Version != "v1.2.0" || !auth.IsPrivate || !auth.Direct {
		t.Fatalf("auth dep: %+v ok=%v", auth, ok)
	}
	viper, ok := byPath["github.com/spf13/viper"]
	if !ok || viper.IsPrivate || viper.Direct {
		t.Fatalf("viper dep should be public+indirect: %+v ok=%v", viper, ok)
	}
}
```

- [x] **Step 4: Run, verify FAIL**

Run: `go test ./internal/godep/ -run TestParseConsumerModule -v` → FAIL (undefined).

- [x] **Step 5: Implement `internal/godep/godep.go` (types + orchestrator)**

```go
package godep

import (
	"os"
	"path/filepath"
	"strings"
)

// Dependency is a normalized module dependency.
type Dependency struct {
	ModulePath string
	Version    string
	IsPrivate  bool
	Direct     bool
}

// Export is a normalized exported symbol.
type Export struct {
	PackagePath string
	Symbol      string
	Kind        string // func|constructor|type|interface|const|var
	Doc         string
}

// Library is a provider module's exported API.
type Library struct {
	ModulePath  string
	Readme      string
	DocSynopsis string
	Packages    []string
	Exports     []Export
}

// Usage is a consumer's reference to a private module symbol.
type Usage struct {
	ModulePath  string
	Version     string
	PackagePath string
	Symbol      string
	File        string
	Line        int
}

// Result is the parsed Go data for one repo.
type Result struct {
	ModulePath   string
	Dependencies []Dependency
	Library      *Library // set when the repo's own module is private
	Usages       []Usage
}

func isPrivate(modulePath string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(modulePath, p) {
			return true
		}
	}
	return false
}

// Parse reads the given go.mod/go.work files and returns the combined Result.
// privatePrefixes classifies internal modules. Per-file errors are returned as
// warnings; only unexpected failures return a hard error.
func Parse(modules, privatePrefixes []string) (*Result, []string, error) {
	res := &Result{}
	var warns []string
	for _, m := range modules {
		base := filepath.Base(m)
		if base == "go.work" {
			ws, w := parseWork(m, privatePrefixes)
			warns = append(warns, w...)
			mergeResults(res, ws)
			continue
		}
		mr, w := parseModuleDir(filepath.Dir(m), m, privatePrefixes)
		warns = append(warns, w...)
		mergeResults(res, mr)
	}
	return res, warns, nil
}

func mergeResults(dst, src *Result) {
	if src == nil {
		return
	}
	if dst.ModulePath == "" {
		dst.ModulePath = src.ModulePath
	}
	dst.Dependencies = append(dst.Dependencies, src.Dependencies...)
	dst.Usages = append(dst.Usages, src.Usages...)
	if src.Library != nil && dst.Library == nil {
		dst.Library = src.Library
	}
}

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }
```

- [x] **Step 6: Implement `internal/godep/modfile.go`**

```go
package godep

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// parseModuleDir parses one module rooted at dir (go.mod at modPath) and returns
// its Result (deps + own module path). Exports/usages are filled by later passes
// (exports.go / usages.go) via parseModuleDir's calls below.
func parseModuleDir(dir, modPath string, privatePrefixes []string) (*Result, []string) {
	var warns []string
	data, err := readFile(modPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", modPath, err)}
	}
	mf, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", modPath, err)}
	}
	res := &Result{ModulePath: mf.Module.Mod.Path}
	sums := readGoSum(filepath.Join(dir, "go.sum"))
	for _, req := range mf.Require {
		d := Dependency{
			ModulePath: req.Mod.Path,
			Version:    req.Mod.Version,
			IsPrivate:  isPrivate(req.Mod.Path, privatePrefixes),
			Direct:     !req.Indirect,
		}
		if len(sums) > 0 {
			if _, ok := sums[req.Mod.Path+" "+req.Mod.Version]; !ok {
				warns = append(warns, fmt.Sprintf("%s: %s@%s not found in go.sum", modPath, req.Mod.Path, req.Mod.Version))
			}
		}
		res.Dependencies = append(res.Dependencies, d)
	}
	// Provider exports: own module private → extract from dir.
	if isPrivate(res.ModulePath, privatePrefixes) {
		lib, w := extractExports(dir, res.ModulePath)
		warns = append(warns, w...)
		res.Library = lib
	}
	// Consumer usages: imports of private deps in dir source.
	usages, w := extractUsages(dir, res.Dependencies)
	warns = append(warns, w...)
	res.Usages = usages
	return res, warns
}

// parseWork parses a go.work file and merges each used module's Result.
func parseWork(workPath string, privatePrefixes []string) (*Result, []string) {
	data, err := readFile(workPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", workPath, err)}
	}
	wf, err := modfile.ParseWork(workPath, data, nil)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", workPath, err)}
	}
	res := &Result{}
	var warns []string
	workDir := filepath.Dir(workPath)
	for _, u := range wf.Use {
		modDir := filepath.Join(workDir, filepath.FromSlash(u.Path))
		mr, w := parseModuleDir(modDir, filepath.Join(modDir, "go.mod"), privatePrefixes)
		warns = append(warns, w...)
		mergeResults(res, mr)
	}
	return res, warns
}

// readGoSum returns a set of "module version" present in a go.sum (empty if absent).
func readGoSum(path string) map[string]struct{} {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	out := map[string]struct{}{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 {
			ver := strings.TrimSuffix(fields[1], "/go.mod")
			out[fields[0]+" "+ver] = struct{}{}
		}
	}
	return out
}
```

Note: `extractExports` and `extractUsages` are added in Tasks 4 and 5. For this task, add temporary stubs at the bottom of `modfile.go` so it compiles, then replace them in the later tasks:

```go
func extractExports(dir, modulePath string) (*Library, []string) { return &Library{ModulePath: modulePath}, nil }
func extractUsages(dir string, deps []Dependency) ([]Usage, []string) { return nil, nil }
```

- [x] **Step 7: Run, verify PASS; tidy**

Run: `go mod tidy && go test ./internal/godep/ -v` → PASS. Then `go build ./...`, `go vet ./internal/godep/`.

- [x] **Step 8: Commit**

```bash
git add internal/godep/ go.mod go.sum openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(godep): parse go.mod/go.work/go.sum with private classification

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 2.2, 2.3, 2.4 `[x]`.

---

## Task 4: Provider export extraction

**Files:**
- Create: `internal/godep/exports.go`
- Modify: `internal/godep/modfile.go` (remove the `extractExports` stub)
- Test: `internal/godep/exports_test.go`
- Create: `internal/godep/testdata/provider/auth.go`, `internal/godep/testdata/provider/README.md`

- [x] **Step 1: Create provider source fixtures**

`internal/godep/testdata/provider/auth.go`:
```go
// Package auth provides token helpers.
package auth

// Client is an auth client.
type Client struct{}

// NewClient builds a Client.
func NewClient() *Client { return &Client{} }

// Verify checks a token.
func (c *Client) Verify(token string) bool { return token != "" }

// MaxRetries is the retry cap.
const MaxRetries = 3

func internalHelper() {}
```

`internal/godep/testdata/provider/README.md`:
```
# auth

Authentication helpers for platform services.
```

- [x] **Step 2: Write the failing test**

Create `internal/godep/exports_test.go`:

```go
package godep

import "testing"

func TestExtractExports(t *testing.T) {
	lib, warns := extractExports("testdata/provider", "git.acme.local/platform/auth")
	if lib == nil {
		t.Fatalf("nil lib (warns=%v)", warns)
	}
	if !contains(lib.Readme, "Authentication helpers") {
		t.Fatalf("readme: %q", lib.Readme)
	}
	if !contains(lib.DocSynopsis, "token helpers") {
		t.Fatalf("doc synopsis: %q", lib.DocSynopsis)
	}
	got := map[string]string{} // symbol -> kind
	for _, e := range lib.Exports {
		got[e.Symbol] = e.Kind
	}
	if got["NewClient"] != "constructor" {
		t.Fatalf("NewClient kind: %q", got["NewClient"])
	}
	if got["Client"] != "type" {
		t.Fatalf("Client kind: %q", got["Client"])
	}
	if got["MaxRetries"] != "const" {
		t.Fatalf("MaxRetries kind: %q", got["MaxRetries"])
	}
	if _, ok := got["internalHelper"]; ok {
		t.Fatalf("unexported symbol leaked")
	}
	if _, ok := got["Verify"]; ok {
		t.Fatalf("method should not be a top-level export")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [x] **Step 3: Run, verify FAIL** (stub returns empty Library)

Run: `go test ./internal/godep/ -run TestExtractExports -v` → FAIL.

- [x] **Step 4: Remove the stub and implement `internal/godep/exports.go`**

Delete the `extractExports` stub line from `modfile.go`. Create `internal/godep/exports.go`:

```go
package godep

import (
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// extractExports walks the module rooted at dir and collects exported top-level
// declarations per package, plus the module README and the package doc synopsis.
func extractExports(dir, modulePath string) (*Library, []string) {
	lib := &Library{ModulePath: modulePath}
	var warns []string
	if data, err := os.ReadFile(filepath.Join(dir, "README.md")); err == nil {
		lib.Readme = string(data)
	}
	pkgSeen := map[string]bool{}
	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "vendor" || (strings.HasPrefix(d.Name(), ".") && path != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if perr != nil {
			warns = append(warns, path+": "+perr.Error())
			return nil
		}
		pkgPath := importPath(modulePath, dir, filepath.Dir(path))
		if !pkgSeen[pkgPath] && f.Doc != nil {
			lib.DocSynopsis = strings.TrimSpace(doc.Synopsis(f.Doc.Text()))
			pkgSeen[pkgPath] = true
		}
		if pkgPath != "" {
			has := false
			for _, p := range lib.Packages {
				if p == pkgPath {
					has = true
					break
				}
			}
			if !has {
				lib.Packages = append(lib.Packages, pkgPath)
			}
		}
		lib.Exports = append(lib.Exports, fileExports(f, pkgPath)...)
		return nil
	})
	if walkErr != nil {
		warns = append(warns, dir+": "+walkErr.Error())
	}
	return lib, warns
}

func importPath(modulePath, moduleDir, fileDir string) string {
	rel, err := filepath.Rel(moduleDir, fileDir)
	if err != nil || rel == "." {
		return modulePath
	}
	return modulePath + "/" + filepath.ToSlash(rel)
}

func fileExports(f *ast.File, pkgPath string) []Export {
	var out []Export
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil || !ast.IsExported(d.Name.Name) {
				continue // skip methods and unexported
			}
			kind := "func"
			if strings.HasPrefix(d.Name.Name, "New") && hasResults(d) {
				kind = "constructor"
			}
			out = append(out, Export{PackagePath: pkgPath, Symbol: d.Name.Name, Kind: kind, Doc: docText(d.Doc)})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if !ast.IsExported(s.Name.Name) {
						continue
					}
					kind := "type"
					if _, ok := s.Type.(*ast.InterfaceType); ok {
						kind = "interface"
					}
					out = append(out, Export{PackagePath: pkgPath, Symbol: s.Name.Name, Kind: kind, Doc: docText(d.Doc)})
				case *ast.ValueSpec:
					vk := "var"
					if d.Tok.String() == "const" {
						vk = "const"
					}
					for _, n := range s.Names {
						if ast.IsExported(n.Name) {
							out = append(out, Export{PackagePath: pkgPath, Symbol: n.Name, Kind: vk, Doc: docText(d.Doc)})
						}
					}
				}
			}
		}
	}
	return out
}

func hasResults(d *ast.FuncDecl) bool { return d.Type.Results != nil && len(d.Type.Results.List) > 0 }

func docText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	return strings.TrimSpace(g.Text())
}
```

- [x] **Step 5: Run, verify PASS**

Run: `go test ./internal/godep/ -v` → PASS (all godep tests). `go build ./...`, `go vet ./internal/godep/`.

- [x] **Step 6: Commit**

```bash
git add internal/godep/ openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(godep): extract provider exports via go/ast

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 3.1, 3.2 `[x]`.

---

## Task 5: Consumer usage extraction

**Files:**
- Create: `internal/godep/usages.go`
- Modify: `internal/godep/modfile.go` (remove the `extractUsages` stub)
- Test: `internal/godep/usages_test.go`
- Create: `internal/godep/testdata/consumer/main.go`

- [ ] **Step 1: Create consumer source fixture**

`internal/godep/testdata/consumer/main.go`:
```go
package main

import (
	"fmt"

	"git.acme.local/platform/auth"
	"github.com/spf13/viper"
)

func main() {
	c := auth.NewClient()
	fmt.Println(c.Verify("x"))
	_ = viper.New()
}
```

- [ ] **Step 2: Write the failing test**

Create `internal/godep/usages_test.go`:

```go
package godep

import "testing"

func TestExtractUsages(t *testing.T) {
	deps := []Dependency{
		{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", IsPrivate: true, Direct: true},
		{ModulePath: "github.com/spf13/viper", Version: "v1.21.0", IsPrivate: false, Direct: true},
	}
	usages, warns := extractUsages("testdata/consumer", deps)
	if len(warns) != 0 {
		t.Fatalf("warns: %v", warns)
	}
	var newClient *Usage
	for i := range usages {
		if usages[i].Symbol == "NewClient" {
			newClient = &usages[i]
		}
	}
	if newClient == nil {
		t.Fatalf("NewClient usage not found: %+v", usages)
	}
	if newClient.ModulePath != "git.acme.local/platform/auth" || newClient.Version != "v1.2.0" {
		t.Fatalf("usage module/version: %+v", newClient)
	}
	if newClient.PackagePath != "git.acme.local/platform/auth" || newClient.Line == 0 {
		t.Fatalf("usage package/line: %+v", newClient)
	}
	for _, u := range usages {
		if u.ModulePath == "github.com/spf13/viper" {
			t.Fatalf("public dep should not produce usages: %+v", u)
		}
	}
}
```

- [ ] **Step 3: Run, verify FAIL** (stub returns nil)

Run: `go test ./internal/godep/ -run TestExtractUsages -v` → FAIL.

- [ ] **Step 4: Remove the stub and implement `internal/godep/usages.go`**

Delete the `extractUsages` stub line from `modfile.go`. Create `internal/godep/usages.go`:

```go
package godep

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// extractUsages walks dir's .go files and records references to exported symbols
// of private modules (imports + selector expressions), with file and line.
func extractUsages(dir string, deps []Dependency) ([]Usage, []string) {
	var privateMods []Dependency
	for _, d := range deps {
		if d.IsPrivate {
			privateMods = append(privateMods, d)
		}
	}
	if len(privateMods) == 0 {
		return nil, nil
	}
	var usages []Usage
	var warns []string
	_ = filepath.WalkDir(dir, func(path string, de os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if de.IsDir() {
			if de.Name() == "vendor" || (strings.HasPrefix(de.Name(), ".") && path != dir) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(de.Name(), ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			warns = append(warns, path+": "+perr.Error())
			return nil
		}
		usages = append(usages, fileUsages(fset, f, path, privateMods)...)
		return nil
	})
	return usages, warns
}

func fileUsages(fset *token.FileSet, f *ast.File, path string, privateMods []Dependency) []Usage {
	// alias -> (module, version, packagePath) for private imports in this file.
	type imp struct{ module, version, pkg string }
	aliases := map[string]imp{}
	for _, spec := range f.Imports {
		pkgPath := strings.Trim(spec.Path.Value, `"`)
		dep, ok := longestPrefixModule(pkgPath, privateMods)
		if !ok {
			continue
		}
		alias := pkgPath[strings.LastIndex(pkgPath, "/")+1:]
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		aliases[alias] = imp{module: dep.ModulePath, version: dep.Version, pkg: pkgPath}
	}
	if len(aliases) == 0 {
		return nil
	}
	var out []Usage
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		im, ok := aliases[id.Name]
		if !ok {
			return true
		}
		out = append(out, Usage{
			ModulePath:  im.module,
			Version:     im.version,
			PackagePath: im.pkg,
			Symbol:      sel.Sel.Name,
			File:        path,
			Line:        fset.Position(sel.Sel.Pos()).Line,
		})
		return true
	})
	return out
}

// longestPrefixModule returns the private dep whose module path is the longest
// prefix of importPath (so a package under a module resolves to that module).
func longestPrefixModule(importPath string, mods []Dependency) (Dependency, bool) {
	var best Dependency
	found := false
	for _, m := range mods {
		if importPath == m.ModulePath || strings.HasPrefix(importPath, m.ModulePath+"/") {
			if !found || len(m.ModulePath) > len(best.ModulePath) {
				best, found = m, true
			}
		}
	}
	return best, found
}
```

- [ ] **Step 5: Run, verify PASS**

Run: `go test ./internal/godep/ -v` → PASS. `go build ./...`, `go vet ./internal/godep/`.

- [ ] **Step 6: Commit**

```bash
git add internal/godep/ openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(godep): extract consumer usages via go/ast selectors

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 4.1, 4.2 `[x]`.

---

## Task 6: Export → code-node linker

**Files:**
- Modify: `internal/link/link.go`
- Test: `internal/link/link_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/link/link_test.go`:

```go
func TestMatchExports(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "auth", "abc", "main", "/g")
	mustNode(t, s, id, "n1", "NewClient", "auth/auth.go")
	mustNode(t, s, id, "n2", "Verify", "auth/auth.go")

	exports := []Export{
		{PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", SourceHint: "auth/"},
		{PackagePath: "git.acme.local/platform/auth", Symbol: "Ghost"},
	}
	linked := MatchExports(s, id, exports)
	byID := map[string]string{} // symbol -> node_id
	for _, e := range linked {
		byID[e.Symbol] = e.NodeID
	}
	if byID["NewClient"] != "n1" {
		t.Fatalf("NewClient should link to n1: %+v", linked)
	}
	if byID["Ghost"] != "" {
		t.Fatalf("Ghost should have no node: %+v", linked)
	}
}
```

(`mustNode` already exists in this test file from the RPC linker task.)

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/link/ -run TestMatchExports -v` → FAIL (`Export`, `MatchExports` undefined).

- [ ] **Step 3: Implement in `internal/link/link.go`**

Append to `internal/link/link.go`:

```go
// Export is the subset of a private-library export the linker needs. SourceHint
// is an optional path fragment (e.g. package dir) used to prefer a co-located node.
type Export struct {
	PackagePath string
	Symbol      string
	SourceHint  string
	NodeID      string // filled by MatchExports
}

// MatchExports links each export to a code node by exact symbol name within the
// index, preferring a node whose source_file contains SourceHint. Unmatched
// exports keep an empty NodeID. Returns the exports with NodeID populated.
func MatchExports(s *store.Store, indexID int64, exports []Export) []Export {
	out := make([]Export, len(exports))
	copy(out, exports)
	for i := range out {
		nodes, err := s.NodesByLabel(indexID, out[i].Symbol)
		if err != nil || len(nodes) == 0 {
			continue
		}
		pick := nodes[0].NodeID
		if out[i].SourceHint != "" {
			for _, n := range nodes {
				if strings.Contains(n.SourceFile, out[i].SourceHint) {
					pick = n.NodeID
					break
				}
			}
		}
		out[i].NodeID = pick
	}
	return out
}
```

(`strings` and `store` are already imported in link.go.)

- [ ] **Step 4: Run, verify PASS**

Run: `go test ./internal/link/ -v` → PASS. `go build ./...`, `go vet ./internal/link/`.

- [ ] **Step 5: Commit**

```bash
git add internal/link/ openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(link): export-to-code-node matcher

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 5.1 `[x]` (linker portion; ingest wiring in Task 7).

---

## Task 7: Ingest wiring

**Files:**
- Modify: `internal/ingest/ingest.go`
- Test: `internal/ingest/ingest_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/ingest/ingest_test.go`:

```go
func TestRunIndexesGoDeps(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "g.json")
	if err := os.WriteFile(graphPath, []byte(`{"nodes":[],"edges":[],"hyperedges":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs("../godep/testdata/consumer")
	cfg := &config.Config{Repos: []config.RepoConfig{{Repo: "acme/web", Graph: graphPath}}}
	cfg.Repos[0].Go.Modules = []string{filepath.Join(root, "go.mod")}
	cfg.Repos[0].Go.PrivatePrefixes = []string{"git.acme.local/"}

	s, _ := store.Open(":memory:")
	defer s.Close()
	if _, err := Run(s, cfg); err != nil {
		t.Fatalf("run: %v", err)
	}
	id, _ := s.UpsertIndex("acme", "web", "", "", graphPath)
	usages, err := s.PrivateUsagesByModule(id, "git.acme.local/platform/auth")
	if err != nil || len(usages) == 0 {
		t.Fatalf("usages: %+v err=%v", usages, err)
	}
}
```

Note: the consumer fixture imports `git.acme.local/platform/auth`; `extractUsages` records the `NewClient`/`Verify` selector usages regardless of whether the imported source is present (it parses the consumer file only). Resolve the index id via the idempotent `UpsertIndex` (Run created it with empty commit/branch).

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/ingest/ -run TestRunIndexesGoDeps -v` → FAIL.

- [ ] **Step 3: Wire the Go pass into `ingest.Run`**

In `internal/ingest/ingest.go`, add `"github.com/vend-ai/intel-mcp/internal/godep"` to imports. After the protobuf block (after `s.LinkRPCImpls(...)`), add:

```go
		// Go private libraries.
		gres, gwarns, gerr := godep.Parse(r.Go.Modules, r.Go.PrivatePrefixes)
		if gerr != nil {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: go: %v", r.Repo, gerr))
		}
		for _, w := range gwarns {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: go %s", r.Repo, w))
		}
		if err := s.ReplaceGoDeps(indexID, toGoDepBundle(s, indexID, gres)); err != nil {
			return rep, err
		}
```

Add the helper at the bottom of the file:

```go
func toGoDepBundle(s *store.Store, indexID int64, res *godep.Result) store.GoDepBundle {
	var b store.GoDepBundle
	if res == nil {
		return b
	}
	for _, d := range res.Dependencies {
		b.Dependencies = append(b.Dependencies, store.Dependency{
			ModulePath: d.ModulePath, Version: d.Version, Ecosystem: "go",
			IsPrivate: d.IsPrivate, Direct: d.Direct})
	}
	if res.Library != nil {
		var le []link.Export
		for _, e := range res.Library.Exports {
			le = append(le, link.Export{PackagePath: e.PackagePath, Symbol: e.Symbol, SourceHint: lastPathSeg(e.PackagePath)})
		}
		linked := link.MatchExports(s, indexID, le)
		nodeBySym := map[string]string{}
		for _, l := range linked {
			if l.NodeID != "" {
				nodeBySym[l.Symbol] = l.NodeID
			}
		}
		var exps []store.PrivateExport
		for _, e := range res.Library.Exports {
			exps = append(exps, store.PrivateExport{
				PackagePath: e.PackagePath, Symbol: e.Symbol, Kind: e.Kind, Doc: e.Doc, NodeID: nodeBySym[e.Symbol]})
		}
		b.Libraries = append(b.Libraries, store.PrivateLibraryBundle{
			Library: store.PrivateLibrary{ModulePath: res.Library.ModulePath, Readme: res.Library.Readme, DocSynopsis: res.Library.DocSynopsis},
			Exports: exps,
		})
	}
	for _, u := range res.Usages {
		b.Usages = append(b.Usages, store.PrivateUsage{
			ModulePath: u.ModulePath, Version: u.Version, PackagePath: u.PackagePath,
			Symbol: u.Symbol, File: u.File, Line: u.Line})
	}
	return b
}

func lastPathSeg(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
```

Add `"strings"` to the ingest imports if not present.

- [ ] **Step 4: Run, verify PASS**

Run: `go test ./internal/ingest/ -v` → PASS. `go test ./...`, `go build ./...`, `go vet ./internal/ingest/`.

- [ ] **Step 5: Commit**

```bash
git add internal/ingest/ openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(ingest): parse and link go private libraries after graph load

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Confirm tasks.md 5.1 stays `[x]`.

---

## Task 8: MCP tools (find_private_library, find_library_consumers)

**Files:**
- Create: `internal/mcp/godep_tools.go`
- Test: `internal/mcp/godep_tools_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mcp/godep_tools_test.go`:

```go
package mcp

import (
	"testing"

	"github.com/vend-ai/intel-mcp/internal/store"
)

func seedGoDepTools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("acme", "web", "abc", "main", "/g")
	bundle := store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Libraries: []store.PrivateLibraryBundle{{
			Library: store.PrivateLibrary{ModulePath: "git.acme.local/platform/auth", DocSynopsis: "Package auth provides tokens"},
			Exports: []store.PrivateExport{{PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", Kind: "constructor"}},
		}},
		Usages: []store.PrivateUsage{{ModulePath: "git.acme.local/platform/auth", Version: "v1.2.0", PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", File: "main.go", Line: 12}},
	}
	if err := s.ReplaceGoDeps(id, bundle); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestFindPrivateLibrary(t *testing.T) {
	tools := seedGoDepTools(t)
	got, err := tools.FindPrivateLibrary("acme/web", "auth")
	if err != nil || len(got) != 1 || got[0].ModulePath != "git.acme.local/platform/auth" || got[0].ExportCount != 1 {
		t.Fatalf("find: %+v err=%v", got, err)
	}
}

func TestFindLibraryConsumers(t *testing.T) {
	tools := seedGoDepTools(t)
	out, err := tools.FindLibraryConsumers("acme/web", "git.acme.local/platform/auth")
	if err != nil {
		t.Fatalf("consumers: %v", err)
	}
	if out.Version != "v1.2.0" || len(out.UsedSymbols) != 1 || out.UsedSymbols[0].Symbol != "NewClient" {
		t.Fatalf("shape: %+v", out)
	}
	if out.ConsumedAcrossRepos == "" {
		t.Fatalf("expected deferred marker")
	}
	if _, err := tools.FindLibraryConsumers("acme/web", "git.acme.local/none"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/mcp/ -run 'PrivateLibrary|LibraryConsumers' -v` → FAIL.

- [ ] **Step 3: Implement `internal/mcp/godep_tools.go`**

```go
package mcp

import "github.com/vend-ai/intel-mcp/internal/store"

const consumersDeferred = "deferred: cross-repo consumer aggregation not available in this change"

// LibraryConsumers is the find_library_consumers result for one repo.
type LibraryConsumers struct {
	ModulePath          string             `json:"module_path"`
	Version             string             `json:"version"`
	UsedPackages        []string           `json:"used_packages"`
	UsedSymbols         []store.PrivateUsage `json:"used_symbols"`
	ConsumedAcrossRepos string             `json:"consumed_across_repos"`
}

// FindPrivateLibrary implements find_private_library: provider libraries plus
// path-only private dependencies matching the query.
func (t *Tools) FindPrivateLibrary(repo, query string) ([]store.PrivateLibraryResult, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	libs, err := t.s.FindPrivateLibraries(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, l := range libs {
		seen[l.ModulePath] = true
	}
	deps, err := t.s.FindPrivateDeps(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	for _, d := range deps {
		if !seen[d.ModulePath] {
			libs = append(libs, store.PrivateLibraryResult{ModulePath: d.ModulePath})
			seen[d.ModulePath] = true
		}
	}
	return libs, nil
}

// FindLibraryConsumers implements find_library_consumers (single-repo).
func (t *Tools) FindLibraryConsumers(repo, modulePath string) (LibraryConsumers, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return LibraryConsumers{}, err
	}
	if !ok {
		return LibraryConsumers{}, ErrNotFound
	}
	dep, found, err := t.s.DependencyByModule(ri.IndexID, modulePath)
	if err != nil {
		return LibraryConsumers{}, err
	}
	if !found {
		return LibraryConsumers{}, ErrNotFound
	}
	usages, err := t.s.PrivateUsagesByModule(ri.IndexID, modulePath)
	if err != nil {
		return LibraryConsumers{}, err
	}
	out := LibraryConsumers{
		ModulePath: modulePath, Version: dep.Version,
		UsedSymbols: usages, ConsumedAcrossRepos: consumersDeferred,
	}
	seen := map[string]bool{}
	for _, u := range usages {
		if !seen[u.PackagePath] {
			seen[u.PackagePath] = true
			out.UsedPackages = append(out.UsedPackages, u.PackagePath)
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run, verify PASS**

Run: `go test ./internal/mcp/ -v` → PASS. `go build ./...`, `go vet ./internal/mcp/`.

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/godep_tools.go internal/mcp/godep_tools_test.go openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(mcp): find_private_library and find_library_consumers

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 6.1, 6.2 `[x]`.

---

## Task 9: lib:// resources + server registration

**Files:**
- Modify: `internal/mcp/resources.go`
- Modify: `internal/mcp/server.go`
- Test: `internal/mcp/resources_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/mcp/resources_test.go` (reuse `seedGoDepTools`):

```go
func TestLibResources(t *testing.T) {
	tools := seedGoDepTools(t)
	body, err := tools.LibraryResource("git.acme.local/platform/auth")
	if err != nil || !strings.Contains(body, "NewClient") {
		t.Fatalf("lib resource: %q err=%v", body, err)
	}
	symBody, err := tools.LibrarySymbolResource("git.acme.local/platform/auth", "NewClient")
	if err != nil || !strings.Contains(symBody, "constructor") {
		t.Fatalf("symbol resource: %q err=%v", symBody, err)
	}
	if _, err := tools.LibrarySymbolResource("git.acme.local/platform/auth", "Nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestParseLibURI(t *testing.T) {
	mod, kind, ref, err := parseLibURI("lib://git.acme.local/platform/auth/symbol/NewClient")
	if err != nil || mod != "git.acme.local/platform/auth" || kind != "symbol" || ref != "NewClient" {
		t.Fatalf("parse: mod=%q kind=%q ref=%q err=%v", mod, kind, ref, err)
	}
	mod2, kind2, _, err := parseLibURI("lib://git.acme.local/platform/auth")
	if err != nil || mod2 != "git.acme.local/platform/auth" || kind2 != "" {
		t.Fatalf("parse bare: mod=%q kind=%q err=%v", mod2, kind2, err)
	}
}
```

Add `"strings"` to test imports if needed.

- [ ] **Step 2: Run, verify FAIL**

Run: `go test ./internal/mcp/ -run 'LibResources|ParseLibURI' -v` → FAIL.

- [ ] **Step 3: Implement resource methods in `internal/mcp/resources.go`**

```go
// LibraryResource returns JSON for lib://<module-path> (provider library + exports).
func (t *Tools) LibraryResource(modulePath string) (string, error) {
	lib, ok, err := t.s.PrivateLibraryByModule(modulePath)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	return mustJSON(lib), nil
}

// LibraryPackageResource returns JSON for lib://<module-path>/package/<pkg>.
func (t *Tools) LibraryPackageResource(modulePath, pkg string) (string, error) {
	lib, ok, err := t.s.PrivateLibraryByModule(modulePath)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	var exps []store.PrivateExport
	for _, e := range lib.Exports {
		if e.PackagePath == pkg {
			exps = append(exps, e)
		}
	}
	if len(exps) == 0 {
		return "", ErrNotFound
	}
	return mustJSON(map[string]any{"module_path": modulePath, "package": pkg, "exports": exps}), nil
}

// LibrarySymbolResource returns JSON for lib://<module-path>/symbol/<symbol>.
func (t *Tools) LibrarySymbolResource(modulePath, symbol string) (string, error) {
	lib, ok, err := t.s.PrivateLibraryByModule(modulePath)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	for _, e := range lib.Exports {
		if e.Symbol == symbol {
			return mustJSON(e), nil
		}
	}
	return "", ErrNotFound
}
```

- [ ] **Step 4: Register tools, resource, parser in `internal/mcp/server.go`**

1. Add to `ToolNames` after `"explain_rpc"`:
```go
	"find_private_library",
	"find_library_consumers",
```

2. In `NewServer`, after `registerExplainRPC(srv, tools)`:
```go
	registerFindPrivateLibrary(srv, tools)
	registerFindLibraryConsumers(srv, tools)
```

3. Add registrations:
```go
type findPrivateLibraryArgs struct {
	Repo  string `json:"repo" jsonschema:"repo identity (org/name)"`
	Query string `json:"query" jsonschema:"lexical match: name / module path / purpose"`
}

func registerFindPrivateLibrary(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "find_private_library",
		Description: "Find internal Go libraries by name, module path, package path, or purpose.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args findPrivateLibraryArgs) (*mcpsdk.CallToolResult, any, error) {
		libs, err := tools.FindPrivateLibrary(args.Repo, args.Query)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(libs)), nil, nil
	})
}

type findLibraryConsumersArgs struct {
	Repo   string `json:"repo" jsonschema:"repo identity (org/name)"`
	Module string `json:"module" jsonschema:"module path of the private library"`
}

func registerFindLibraryConsumers(srv *mcpsdk.Server, tools *Tools) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "find_library_consumers",
		Description: "Show how a repo consumes a private Go module: version and used symbols.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, args findLibraryConsumersArgs) (*mcpsdk.CallToolResult, any, error) {
		out, err := tools.FindLibraryConsumers(args.Repo, args.Module)
		if err != nil {
			return toolErr(err)
		}
		return textResult(mustJSON(out)), nil, nil
	})
}
```

4. Add `parseLibURI` next to `parseProtoURI`:
```go
// parseLibURI parses lib://<module-path>[/version/<v>][/package/<p>][/symbol/<s>].
// Returns the module path, an optional kind (version|package|symbol), and its ref.
func parseLibURI(uri string) (modulePath, kind, ref string, err error) {
	rest := strings.TrimPrefix(uri, "lib://")
	for _, k := range []string{"/version/", "/package/", "/symbol/"} {
		if i := strings.Index(rest, k); i >= 0 {
			return rest[:i], strings.Trim(k, "/"), rest[i+len(k):], nil
		}
	}
	if rest == "" {
		return "", "", "", fmt.Errorf("malformed lib uri %q", uri)
	}
	return rest, "", "", nil
}
```

5. In `registerResources`, add after the proto template:
```go
	srv.AddResourceTemplate(&mcpsdk.ResourceTemplate{
		Name:        "lib",
		Description: "A private Go library (module/package/symbol) as JSON.",
		MIMEType:    "application/json",
		URITemplate: "lib://{module}",
	}, func(_ context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
		mod, kind, ref, err := parseLibURI(req.Params.URI)
		if err != nil {
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		var body string
		switch kind {
		case "", "version":
			body, err = tools.LibraryResource(mod)
		case "package":
			body, err = tools.LibraryPackageResource(mod, ref)
		case "symbol":
			body, err = tools.LibrarySymbolResource(mod, ref)
		default:
			return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
		}
		if err != nil {
			if err == ErrNotFound {
				return nil, mcpsdk.ResourceNotFoundError(req.Params.URI)
			}
			return nil, err
		}
		return resourceText(req.Params.URI, body), nil
	})
```

- [ ] **Step 5: Run, verify PASS**

Run: `go test ./internal/mcp/ -v` → PASS (update `e2e_test.go` only if it asserts exact `ToolNames` length — it iterates, so new tools must be registered, which they are). `go test ./...`, `go build ./...`, `go vet ./...`.

- [ ] **Step 6: Commit**

```bash
git add internal/mcp/resources.go internal/mcp/server.go internal/mcp/resources_test.go openspec/changes/go-private-library-layer/tasks.md
git commit -m "feat(mcp): lib:// resources and tool/resource registration

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 7.1 `[x]`.

---

## Task 10: Verification and regression

**Files:**
- Modify: `internal/mcp/e2e_test.go`

- [ ] **Step 1: Add a coexistence regression test**

Add to `internal/mcp/e2e_test.go` a test seeding OpenAPI + proto + Go data in one index and asserting each tool surface is intact:

```go
func TestGoDepDoesNotRegressOthers(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "web", "abc", "main", "/g")
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec: store.APISpec{Kind: "openapi", Name: "Web API", Version: "1.0", Path: "api/openapi.yaml"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceGoDeps(id, store.GoDepBundle{
		Dependencies: []store.Dependency{{ModulePath: "git.acme.local/platform/auth", Version: "v1", Ecosystem: "go", IsPrivate: true, Direct: true}},
		Libraries: []store.PrivateLibraryBundle{{
			Library: store.PrivateLibrary{ModulePath: "git.acme.local/platform/auth"},
			Exports: []store.PrivateExport{{PackagePath: "git.acme.local/platform/auth", Symbol: "NewClient", Kind: "constructor"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)

	apis, _ := tools.ListAPIs("acme/web")
	if len(apis) != 1 || apis[0].Kind != "openapi" {
		t.Fatalf("list_apis regressed: %+v", apis)
	}
	libs, _ := tools.FindPrivateLibrary("acme/web", "auth")
	if len(libs) != 1 {
		t.Fatalf("find_private_library: %+v", libs)
	}
}
```

- [ ] **Step 2: Full verification**

Run and confirm all clean:
- `go test ./... -count=1` → every package ok
- `go build ./...` → success
- `go vet ./...` → no issues
- `gofmt -l internal/ cmd/` → no files listed

- [ ] **Step 3: Commit**

```bash
git add internal/mcp/e2e_test.go openspec/changes/go-private-library-layer/tasks.md
git commit -m "test: go private-library coexistence regression

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```
Mark tasks.md 8.1–8.6 `[x]`.

---

## Self-Review Notes

- **Spec coverage:** go-dependency-index (discovery → Task 2; modfile/go.work/go.sum → Task 3; private classification → Task 3; provider exports → Task 4; export→node link → Task 6/7; consumer usages → Task 5; idempotency → Task 1/7). private-library-tools (find_private_library → Task 8; find_library_consumers + deferred marker → Task 8; lib:// → Task 9). Regression → Task 10.
- **Deferred:** cross-repo consumer aggregation is the `consumedAcrossRepos` constant marker, not implemented — matches the delta spec.
- **Type consistency:** `godep.Result/Library/Export/Usage/Dependency` (parser) map to `store.*` types in Task 7's `toGoDepBundle`. `link.Export` (Task 6) is produced and consumed in Task 7. `store.PrivateLibraryResult`/`LibraryConsumers` are the tool return types (Tasks 1, 8). `parseLibURI` (Task 9) returns module/kind/ref.
- **Known follow-up during execution:** `extractExports`/`extractUsages` start as stubs in Task 3 (so modfile.go compiles) and are replaced in Tasks 4/5 — when executing out of order, ensure the stub is removed before implementing the real function (duplicate definitions otherwise). The go/ast usage heuristic does not resolve dot-imports or shadowing (documented best-effort in the design).

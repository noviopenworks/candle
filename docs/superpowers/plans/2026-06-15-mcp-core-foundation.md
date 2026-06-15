---
change: mcp-core-foundation
design-doc: docs/superpowers/specs/2026-06-15-mcp-core-foundation-design.md
base-ref: 337453d2b73d76f03aecb8eeadf1aded3fbf2752
---

# MCP Core Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go MCP (stdio) server that ingests Graphify `graph.json` into SQLite and exposes five base code-navigation tools and `repo://`/`graph://` resources.

**Architecture:** A cobra CLI (`serve`, `index`) over an `internal/` core: `config` (viper manifest), `store` (pure-Go SQLite schema + queries), `graph` (graph.json parse + idempotent loader), `registry` (repo→index_id resolution), and `mcp` (tools as pure functions + a thin official-SDK adapter). Tools are pure functions over the store so they are testable without the MCP SDK; the SDK is isolated to one adapter file.

**Tech Stack:** Go, `modernc.org/sqlite` (pure Go, no cgo), `github.com/modelcontextprotocol/go-sdk` (MCP), `github.com/spf13/cobra` (CLI), `github.com/spf13/viper` (config).

**Module path:** `github.com/vend-ai/intel-mcp` (local/private; not published — adjust if a real remote is chosen).

**Conventions:** Each task is TDD (failing test → run-fail → implement → run-pass → commit). Run all tests with `go test ./...`. The store uses an in-memory or temp-file DB in tests.

---

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/intel-mcp/main.go`
- Create: `internal/version/version_test.go`
- Create: `internal/version/version.go`

- [x] **Step 1: Initialize the module and a smoke test**

Create `internal/version/version_test.go`:

```go
package version

import "testing"

func TestString(t *testing.T) {
	if String() == "" {
		t.Fatal("version string must not be empty")
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go mod init github.com/vend-ai/intel-mcp && go test ./internal/version/`
Expected: FAIL — `undefined: String`

- [x] **Step 3: Implement minimal code**

Create `internal/version/version.go`:

```go
package version

// String returns the build version of the server.
func String() string { return "0.1.0-dev" }
```

Create `cmd/intel-mcp/main.go` (placeholder, fleshed out in Task 9/12):

```go
package main

import (
	"fmt"

	"github.com/vend-ai/intel-mcp/internal/version"
)

func main() {
	fmt.Println("intel-mcp", version.String())
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go build ./... && go test ./internal/version/`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add go.mod cmd internal/version
git commit -m "chore: scaffold go module and version package"
```

---

### Task 2: SQLite store — open and migrate schema

**Files:**
- Create: `internal/store/store_test.go`
- Create: `internal/store/store.go`
- Create: `internal/store/schema.go`

- [ ] **Step 1: Write the failing test**

Create `internal/store/store_test.go`:

```go
package store

import "testing"

func TestOpenCreatesSchema(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	for _, tbl := range []string{"repos", "indexes", "nodes", "edges", "hyperedges", "hyperedge_members"} {
		var name string
		err := s.DB.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil {
			t.Fatalf("expected table %q to exist: %v", tbl, err)
		}
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	// Re-running migrate must not error.
	if err := s.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/`
Expected: FAIL — `undefined: Open`

- [ ] **Step 3: Implement the store**

Create `internal/store/store.go`:

```go
package store

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite connection.
type Store struct {
	DB *sql.DB
}

// Open opens (or creates) the SQLite database at dsn and applies the schema.
// Use ":memory:" for an in-memory database.
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying connection.
func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate() error {
	_, err := s.DB.Exec(schemaSQL)
	return err
}
```

Create `internal/store/schema.go`:

```go
package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS repos (
  id    INTEGER PRIMARY KEY,
  org   TEXT NOT NULL,
  name  TEXT NOT NULL,
  UNIQUE(org, name)
);
CREATE TABLE IF NOT EXISTS indexes (
  id          INTEGER PRIMARY KEY,
  repo_id     INTEGER NOT NULL REFERENCES repos(id),
  commit_sha  TEXT,
  branch      TEXT,
  graph_path  TEXT NOT NULL,
  ingested_at TEXT NOT NULL,
  UNIQUE(repo_id, commit_sha)
);
CREATE TABLE IF NOT EXISTS nodes (
  index_id        INTEGER NOT NULL REFERENCES indexes(id),
  node_id         TEXT NOT NULL,
  label           TEXT,
  file_type       TEXT,
  source_file     TEXT,
  source_location TEXT,
  source_url      TEXT,
  captured_at     TEXT,
  author          TEXT,
  contributor     TEXT,
  PRIMARY KEY (index_id, node_id)
);
CREATE TABLE IF NOT EXISTS edges (
  index_id         INTEGER NOT NULL REFERENCES indexes(id),
  source           TEXT NOT NULL,
  target           TEXT NOT NULL,
  relation         TEXT NOT NULL,
  confidence       TEXT,
  confidence_score REAL,
  weight           REAL,
  source_file      TEXT
);
CREATE TABLE IF NOT EXISTS hyperedges (
  index_id         INTEGER NOT NULL REFERENCES indexes(id),
  hyperedge_id     TEXT NOT NULL,
  label            TEXT,
  relation         TEXT,
  confidence       TEXT,
  confidence_score REAL,
  source_file      TEXT,
  PRIMARY KEY (index_id, hyperedge_id)
);
CREATE TABLE IF NOT EXISTS hyperedge_members (
  index_id     INTEGER NOT NULL,
  hyperedge_id TEXT NOT NULL,
  node_id      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_nodes_label    ON nodes(index_id, label);
CREATE INDEX IF NOT EXISTS idx_nodes_ftype    ON nodes(index_id, file_type);
CREATE INDEX IF NOT EXISTS idx_nodes_file     ON nodes(index_id, source_file);
CREATE INDEX IF NOT EXISTS idx_edges_source   ON edges(index_id, source);
CREATE INDEX IF NOT EXISTS idx_edges_target   ON edges(index_id, target);
CREATE INDEX IF NOT EXISTS idx_edges_relation ON edges(index_id, relation);
`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): sqlite open + schema migration"
```

---

### Task 3: Store — upsert repo and index snapshot

**Files:**
- Modify: `internal/store/store_test.go`
- Create: `internal/store/repos.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/store/store_test.go`:

```go
func TestUpsertRepoAndIndex(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()

	id1, err := s.UpsertIndex("org", "svc", "abc123", "main", "/p/graph.json")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Same (repo, commit) returns the same index_id (idempotent).
	id2, err := s.UpsertIndex("org", "svc", "abc123", "main", "/p/graph.json")
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("expected idempotent index id, got %d and %d", id1, id2)
	}
	var repoCount int
	s.DB.QueryRow(`SELECT COUNT(*) FROM repos`).Scan(&repoCount)
	if repoCount != 1 {
		t.Fatalf("expected 1 repo, got %d", repoCount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestUpsertRepoAndIndex`
Expected: FAIL — `undefined: (*Store).UpsertIndex`

- [ ] **Step 3: Implement**

Create `internal/store/repos.go`:

```go
package store

import "time"

// UpsertIndex ensures a repo row and an index (snapshot) row exist for
// (org/name, commit), returning the index_id. Idempotent on (repo, commit).
func (s *Store) UpsertIndex(org, name, commit, branch, graphPath string) (int64, error) {
	if _, err := s.DB.Exec(
		`INSERT INTO repos(org, name) VALUES(?, ?)
		 ON CONFLICT(org, name) DO NOTHING`, org, name); err != nil {
		return 0, err
	}
	var repoID int64
	if err := s.DB.QueryRow(
		`SELECT id FROM repos WHERE org=? AND name=?`, org, name).Scan(&repoID); err != nil {
		return 0, err
	}
	if _, err := s.DB.Exec(
		`INSERT INTO indexes(repo_id, commit_sha, branch, graph_path, ingested_at)
		 VALUES(?, ?, ?, ?, ?)
		 ON CONFLICT(repo_id, commit_sha)
		 DO UPDATE SET branch=excluded.branch, graph_path=excluded.graph_path, ingested_at=excluded.ingested_at`,
		repoID, commit, branch, graphPath, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return 0, err
	}
	var indexID int64
	if err := s.DB.QueryRow(
		`SELECT id FROM indexes WHERE repo_id=? AND commit_sha=?`, repoID, commit).Scan(&indexID); err != nil {
		return 0, err
	}
	return indexID, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestUpsertRepoAndIndex`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): idempotent repo+index upsert"
```

---

### Task 4: Graph — graph.json types and parser

**Files:**
- Create: `internal/graph/graph_test.go`
- Create: `internal/graph/graph.go`
- Create: `internal/graph/testdata/sample.json`

- [ ] **Step 1: Write the failing test and fixture**

Create `internal/graph/testdata/sample.json`:

```json
{
  "nodes": [
    {"id": "http_reservation_reserveproduct", "label": "ReserveProduct", "file_type": "code", "source_file": "internal/http/reservation_handler.go", "source_location": "L10"},
    {"id": "reservation_service_reserveproduct", "label": "ReserveProduct", "file_type": "code", "source_file": "internal/reservation/service.go"}
  ],
  "edges": [
    {"source": "http_reservation_reserveproduct", "target": "reservation_service_reserveproduct", "relation": "calls", "confidence": "EXTRACTED", "confidence_score": 1.0, "weight": 1.0, "source_file": "internal/http/reservation_handler.go"}
  ],
  "hyperedges": []
}
```

Create `internal/graph/graph_test.go`:

```go
package graph

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	f, err := os.Open("testdata/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	g, err := Parse(f)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(g.Edges))
	}
	if g.Nodes[0].ID != "http_reservation_reserveproduct" {
		t.Fatalf("unexpected node id %q", g.Nodes[0].ID)
	}
}

func TestParseEmpty(t *testing.T) {
	g, err := ParseBytes([]byte(`{"nodes":[],"edges":[]}`))
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatal("expected empty graph")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/graph/`
Expected: FAIL — `undefined: Parse`

- [ ] **Step 3: Implement**

Create `internal/graph/graph.go`:

```go
package graph

import (
	"encoding/json"
	"io"
)

// Node mirrors a Graphify graph.json node.
type Node struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	FileType       string `json:"file_type"`
	SourceFile     string `json:"source_file"`
	SourceLocation string `json:"source_location"`
	SourceURL      string `json:"source_url"`
	CapturedAt     string `json:"captured_at"`
	Author         string `json:"author"`
	Contributor    string `json:"contributor"`
}

// Edge mirrors a Graphify graph.json edge.
type Edge struct {
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	Relation        string  `json:"relation"`
	Confidence      string  `json:"confidence"`
	ConfidenceScore float64 `json:"confidence_score"`
	Weight          float64 `json:"weight"`
	SourceFile      string  `json:"source_file"`
}

// Hyperedge mirrors a Graphify graph.json hyperedge.
type Hyperedge struct {
	ID              string   `json:"id"`
	Label           string   `json:"label"`
	Nodes           []string `json:"nodes"`
	Relation        string   `json:"relation"`
	Confidence      string   `json:"confidence"`
	ConfidenceScore float64  `json:"confidence_score"`
	SourceFile      string   `json:"source_file"`
}

// Graph is a parsed Graphify graph.json document.
type Graph struct {
	Nodes      []Node      `json:"nodes"`
	Edges      []Edge      `json:"edges"`
	Hyperedges []Hyperedge `json:"hyperedges"`
}

// Parse decodes a Graphify graph.json document from r.
func Parse(r io.Reader) (*Graph, error) {
	var g Graph
	if err := json.NewDecoder(r).Decode(&g); err != nil {
		return nil, err
	}
	return &g, nil
}

// ParseBytes decodes a graph.json document from b.
func ParseBytes(b []byte) (*Graph, error) {
	var g Graph
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, err
	}
	return &g, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/graph/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/graph
git commit -m "feat(graph): graph.json types and parser"
```

---

### Task 5: Graph — idempotent loader into the store

**Files:**
- Create: `internal/graph/loader_test.go`
- Create: `internal/graph/loader.go`

- [ ] **Step 1: Write the failing test**

Create `internal/graph/loader_test.go`:

```go
package graph

import (
	"testing"

	"github.com/vend-ai/intel-mcp/internal/store"
)

func TestLoadIsIdempotentAndSkipsMalformed(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	indexID, _ := s.UpsertIndex("org", "svc", "abc", "main", "testdata/sample.json")

	g := &Graph{
		Nodes: []Node{
			{ID: "a", Label: "A", FileType: "code", SourceFile: "a.go"},
			{ID: "", Label: "bad"}, // malformed: no id -> skipped
		},
		Edges: []Edge{{Source: "a", Target: "b", Relation: "calls"}},
	}

	r1, err := Load(s, indexID, g)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if r1.Nodes != 1 {
		t.Fatalf("expected 1 node inserted (malformed skipped), got %d", r1.Nodes)
	}

	// Re-load: counts identical, no duplicates.
	if _, err := Load(s, indexID, g); err != nil {
		t.Fatalf("reload: %v", err)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM nodes WHERE index_id=?`, indexID).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 node after reload, got %d", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/graph/ -run TestLoadIsIdempotent`
Expected: FAIL — `undefined: Load`

- [ ] **Step 3: Implement**

Create `internal/graph/loader.go`:

```go
package graph

import "github.com/vend-ai/intel-mcp/internal/store"

// LoadResult reports how many rows were ingested.
type LoadResult struct {
	Nodes, Edges, Hyperedges, Skipped int
}

// Load ingests g into the store under indexID. It is idempotent: existing rows
// for indexID are deleted first, then re-inserted in one transaction. Malformed
// entries (e.g. nodes without an id, edges without endpoints) are skipped.
func Load(s *store.Store, indexID int64, g *Graph) (LoadResult, error) {
	var res LoadResult
	tx, err := s.DB.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	for _, tbl := range []string{"nodes", "edges", "hyperedges", "hyperedge_members"} {
		if _, err := tx.Exec("DELETE FROM "+tbl+" WHERE index_id=?", indexID); err != nil {
			return res, err
		}
	}

	for _, n := range g.Nodes {
		if n.ID == "" {
			res.Skipped++
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO nodes(index_id, node_id, label, file_type, source_file, source_location, source_url, captured_at, author, contributor)
			 VALUES(?,?,?,?,?,?,?,?,?,?)`,
			indexID, n.ID, n.Label, n.FileType, n.SourceFile, n.SourceLocation, n.SourceURL, n.CapturedAt, n.Author, n.Contributor); err != nil {
			return res, err
		}
		res.Nodes++
	}
	for _, e := range g.Edges {
		if e.Source == "" || e.Target == "" || e.Relation == "" {
			res.Skipped++
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO edges(index_id, source, target, relation, confidence, confidence_score, weight, source_file)
			 VALUES(?,?,?,?,?,?,?,?)`,
			indexID, e.Source, e.Target, e.Relation, e.Confidence, e.ConfidenceScore, e.Weight, e.SourceFile); err != nil {
			return res, err
		}
		res.Edges++
	}
	for _, h := range g.Hyperedges {
		if h.ID == "" {
			res.Skipped++
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO hyperedges(index_id, hyperedge_id, label, relation, confidence, confidence_score, source_file)
			 VALUES(?,?,?,?,?,?,?)`,
			indexID, h.ID, h.Label, h.Relation, h.Confidence, h.ConfidenceScore, h.SourceFile); err != nil {
			return res, err
		}
		for _, m := range h.Nodes {
			if _, err := tx.Exec(
				`INSERT INTO hyperedge_members(index_id, hyperedge_id, node_id) VALUES(?,?,?)`,
				indexID, h.ID, m); err != nil {
				return res, err
			}
		}
		res.Hyperedges++
	}
	return res, tx.Commit()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/graph/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/graph
git commit -m "feat(graph): idempotent loader with malformed-entry skipping"
```

---

### Task 6: Config — viper manifest loader

**Files:**
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.go`
- Create: `internal/config/testdata/manifest.yaml`

- [ ] **Step 1: Write the failing test and fixture**

Create `internal/config/testdata/manifest.yaml`:

```yaml
repos:
  - repo: org/inventory-service
    graph: /abs/inventory/graphify-out/graph.json
    commit: abc123
    branch: main
  - repo: org/warehouse-service
    graph: /abs/warehouse/graphify-out/graph.json
```

Create `internal/config/config_test.go`:

```go
package config

import "testing"

func TestLoadManifest(t *testing.T) {
	cfg, err := Load("testdata/manifest.yaml")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}
	r := cfg.Repos[0]
	if r.Org() != "org" || r.Name() != "inventory-service" {
		t.Fatalf("bad split: org=%q name=%q", r.Org(), r.Name())
	}
	if r.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", r.Commit)
	}
}

func TestInvalidRepoIdentity(t *testing.T) {
	_, err := (RepoConfig{Repo: "noslash"}).validate()
	if err == nil {
		t.Fatal("expected error for repo without org/name slash")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Load`

- [ ] **Step 3: Implement**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// RepoConfig is one manifest entry.
type RepoConfig struct {
	Repo   string `mapstructure:"repo"`
	Graph  string `mapstructure:"graph"`
	Commit string `mapstructure:"commit"`
	Branch string `mapstructure:"branch"`
}

// Config is the parsed manifest.
type Config struct {
	Repos []RepoConfig `mapstructure:"repos"`
}

// Org returns the org segment of "org/name".
func (r RepoConfig) Org() string {
	parts := strings.SplitN(r.Repo, "/", 2)
	return parts[0]
}

// Name returns the name segment of "org/name".
func (r RepoConfig) Name() string {
	parts := strings.SplitN(r.Repo, "/", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (r RepoConfig) validate() (RepoConfig, error) {
	if !strings.Contains(r.Repo, "/") {
		return r, fmt.Errorf("repo %q must be in org/name form", r.Repo)
	}
	if r.Graph == "" {
		return r, fmt.Errorf("repo %q missing graph path", r.Repo)
	}
	return r, nil
}

// Load reads and validates a viper manifest at path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	for i, r := range cfg.Repos {
		if _, err := r.validate(); err != nil {
			return nil, err
		}
		_ = i
	}
	return &cfg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config
git commit -m "feat(config): viper manifest loader with validation"
```

---

### Task 7: Store — node/edge queries and cross-index helper

**Files:**
- Create: `internal/store/query_test.go`
- Create: `internal/store/query.go`

- [ ] **Step 1: Write the failing test**

Create `internal/store/query_test.go`:

```go
package store

import "testing"

func seed(t *testing.T) (*Store, int64, int64) {
	t.Helper()
	s, _ := Open(":memory:")
	idA, _ := s.UpsertIndex("org", "svc-a", "a1", "main", "/a")
	idB, _ := s.UpsertIndex("org", "svc-b", "b1", "main", "/b")
	mustExec(t, s, `INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		idA, "n1", "ReserveProduct", "code", "h.go")
	mustExec(t, s, `INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		idA, "n2", "ReserveSvc", "code", "s.go")
	mustExec(t, s, `INSERT INTO edges(index_id,source,target,relation) VALUES(?,?,?,?)`,
		idA, "n1", "n2", "calls")
	mustExec(t, s, `INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		idB, "m1", "ReserveProduct", "code", "client.go")
	return s, idA, idB
}

func mustExec(t *testing.T, s *Store, q string, args ...any) {
	t.Helper()
	if _, err := s.DB.Exec(q, args...); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

func TestNodesByLabel(t *testing.T) {
	s, idA, _ := seed(t)
	defer s.Close()
	ns, err := s.NodesByLabel(idA, "ReserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 || ns[0].NodeID != "n1" {
		t.Fatalf("unexpected nodes: %+v", ns)
	}
}

func TestNeighbors(t *testing.T) {
	s, idA, _ := seed(t)
	defer s.Close()
	callees, err := s.Callees(idA, "n1")
	if err != nil {
		t.Fatal(err)
	}
	if len(callees) != 1 || callees[0].Target != "n2" {
		t.Fatalf("unexpected callees: %+v", callees)
	}
}

func TestNodesByLabelAcrossIndexes(t *testing.T) {
	s, _, _ := seed(t)
	defer s.Close()
	hits, err := s.NodesByLabelAllIndexes("ReserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 cross-index hits, got %d", len(hits))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'Nodes|Neighbors'`
Expected: FAIL — `undefined: (*Store).NodesByLabel`

- [ ] **Step 3: Implement**

Create `internal/store/query.go`:

```go
package store

// NodeRow is a stored graph node.
type NodeRow struct {
	IndexID        int64
	NodeID         string
	Label          string
	FileType       string
	SourceFile     string
	SourceLocation string
}

// EdgeRow is a stored graph edge.
type EdgeRow struct {
	Source   string
	Target   string
	Relation string
}

func scanNodes(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
}) ([]NodeRow, error) {
	defer rows.Close()
	var out []NodeRow
	for rows.Next() {
		var n NodeRow
		if err := rows.Scan(&n.IndexID, &n.NodeID, &n.Label, &n.FileType, &n.SourceFile, &n.SourceLocation); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

const nodeCols = `index_id, node_id, label, file_type, source_file, source_location`

// NodesByLabel returns nodes in indexID whose label matches exactly.
func (s *Store) NodesByLabel(indexID int64, label string) ([]NodeRow, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE index_id=? AND label=?`, indexID, label)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}

// NodeByID returns a single node by id, or (zero,false) if absent.
func (s *Store) NodeByID(indexID int64, nodeID string) (NodeRow, bool, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE index_id=? AND node_id=?`, indexID, nodeID)
	if err != nil {
		return NodeRow{}, false, err
	}
	ns, err := scanNodes(rows)
	if err != nil || len(ns) == 0 {
		return NodeRow{}, false, err
	}
	return ns[0], true, nil
}

// Callees returns edges where nodeID is the source.
func (s *Store) Callees(indexID int64, nodeID string) ([]EdgeRow, error) {
	return s.edges(`source`, indexID, nodeID)
}

// Callers returns edges where nodeID is the target.
func (s *Store) Callers(indexID int64, nodeID string) ([]EdgeRow, error) {
	return s.edges(`target`, indexID, nodeID)
}

func (s *Store) edges(col string, indexID int64, nodeID string) ([]EdgeRow, error) {
	rows, err := s.DB.Query(`SELECT source, target, relation FROM edges WHERE index_id=? AND `+col+`=?`, indexID, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgeRow
	for rows.Next() {
		var e EdgeRow
		if err := rows.Scan(&e.Source, &e.Target, &e.Relation); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// NodesByLabelAllIndexes is the cross-index helper downstream layers use for
// cross-repo relationships. It matches a label across every index.
func (s *Store) NodesByLabelAllIndexes(label string) ([]NodeRow, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE label=?`, label)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}

// NodesByFile returns nodes whose source_file matches in indexID.
func (s *Store) NodesByFile(indexID int64, file string) ([]NodeRow, error) {
	rows, err := s.DB.Query(`SELECT `+nodeCols+` FROM nodes WHERE index_id=? AND source_file=?`, indexID, file)
	if err != nil {
		return nil, err
	}
	return scanNodes(rows)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): node/edge queries + cross-index helper"
```

---

### Task 8: Registry — resolve org/repo → index_id with fuzzy match

**Files:**
- Create: `internal/registry/registry_test.go`
- Create: `internal/registry/registry.go`

- [ ] **Step 1: Write the failing test**

Create `internal/registry/registry_test.go`:

```go
package registry

import (
	"testing"

	"github.com/vend-ai/intel-mcp/internal/store"
)

func TestResolveExactAndFuzzy(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	s.UpsertIndex("org", "inventory-service", "abc", "main", "/g")

	r := New(s)

	got, ok, err := r.Resolve("org/inventory-service")
	if err != nil || !ok {
		t.Fatalf("exact resolve failed: ok=%v err=%v", ok, err)
	}
	if got.Repo != "org/inventory-service" {
		t.Fatalf("unexpected repo %q", got.Repo)
	}

	// Fuzzy: partial name.
	m, err := r.Match("inventory")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 || m[0].Repo != "org/inventory-service" {
		t.Fatalf("fuzzy match failed: %+v", m)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/registry/`
Expected: FAIL — `undefined: New`

- [ ] **Step 3: Implement**

Create `internal/registry/registry.go`:

```go
package registry

import (
	"strings"

	"github.com/vend-ai/intel-mcp/internal/store"
)

// RepoInfo describes a resolved repo snapshot.
type RepoInfo struct {
	IndexID    int64
	Repo       string // org/name
	Branch     string
	Commit     string
	IngestedAt string
	NodeCount  int
}

// Registry resolves repo identities to indexed snapshots.
type Registry struct{ s *store.Store }

// New builds a Registry over the store.
func New(s *store.Store) *Registry { return &Registry{s: s} }

// List returns all indexed repo snapshots.
func (r *Registry) List() ([]RepoInfo, error) {
	rows, err := r.s.DB.Query(`
		SELECT i.id, r.org, r.name, i.branch, i.commit_sha, i.ingested_at,
		       (SELECT COUNT(*) FROM nodes n WHERE n.index_id=i.id)
		FROM indexes i JOIN repos r ON r.id=i.repo_id
		ORDER BY r.org, r.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RepoInfo
	for rows.Next() {
		var ri RepoInfo
		var org, name string
		if err := rows.Scan(&ri.IndexID, &org, &name, &ri.Branch, &ri.Commit, &ri.IngestedAt, &ri.NodeCount); err != nil {
			return nil, err
		}
		ri.Repo = org + "/" + name
		out = append(out, ri)
	}
	return out, rows.Err()
}

// Resolve returns the snapshot for an exact org/name identity.
func (r *Registry) Resolve(repo string) (RepoInfo, bool, error) {
	all, err := r.List()
	if err != nil {
		return RepoInfo{}, false, err
	}
	for _, ri := range all {
		if ri.Repo == repo {
			return ri, true, nil
		}
	}
	return RepoInfo{}, false, nil
}

// Match returns snapshots whose repo identity contains the query substring,
// case-insensitively (simple fuzzy match).
func (r *Registry) Match(query string) ([]RepoInfo, error) {
	all, err := r.List()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []RepoInfo
	for _, ri := range all {
		if strings.Contains(strings.ToLower(ri.Repo), q) {
			out = append(out, ri)
		}
	}
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/registry/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/registry
git commit -m "feat(registry): repo resolution + fuzzy match"
```

---

### Task 9: Tools — pure functions over the store

**Files:**
- Create: `internal/mcp/tools_test.go`
- Create: `internal/mcp/tools.go`

- [ ] **Step 1: Write the failing test**

Create `internal/mcp/tools_test.go`:

```go
package mcp

import (
	"testing"

	"github.com/vend-ai/intel-mcp/internal/store"
)

func seedTools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("org", "svc", "abc", "main", "/g")
	s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		id, "n1", "ReserveProduct", "code", "h.go")
	s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		id, "n2", "ReserveSvc", "code", "s.go")
	s.DB.Exec(`INSERT INTO edges(index_id,source,target,relation) VALUES(?,?,?,?)`, id, "n1", "n2", "calls")
	return NewTools(s)
}

func TestListRepos(t *testing.T) {
	tl := seedTools(t)
	repos, err := tl.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Repo != "org/svc" || repos[0].NodeCount != 2 {
		t.Fatalf("unexpected: %+v", repos)
	}
}

func TestExplainSymbol(t *testing.T) {
	tl := seedTools(t)
	out, err := tl.ExplainSymbol("org/svc", "ReserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if out.Node.NodeID != "n1" {
		t.Fatalf("unexpected node: %+v", out.Node)
	}
	if len(out.Callees) != 1 || out.Callees[0].Target != "n2" {
		t.Fatalf("expected one callee n2, got %+v", out.Callees)
	}
}

func TestExplainSymbolUnknownReturnsEmpty(t *testing.T) {
	tl := seedTools(t)
	_, err := tl.ExplainSymbol("org/svc", "DoesNotExist")
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetFileContext(t *testing.T) {
	tl := seedTools(t)
	syms, err := tl.GetFileContext("org/svc", "h.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(syms) != 1 || syms[0].NodeID != "n1" {
		t.Fatalf("unexpected file context: %+v", syms)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/`
Expected: FAIL — `undefined: NewTools`

- [ ] **Step 3: Implement**

Create `internal/mcp/tools.go`:

```go
package mcp

import (
	"errors"

	"github.com/vend-ai/intel-mcp/internal/registry"
	"github.com/vend-ai/intel-mcp/internal/store"
)

// ErrNotFound is returned when a repo, symbol, or file cannot be resolved.
var ErrNotFound = errors.New("not found")

// Tools holds the pure tool implementations over the store.
type Tools struct {
	s   *store.Store
	reg *registry.Registry
}

// NewTools builds the tool set.
func NewTools(s *store.Store) *Tools {
	return &Tools{s: s, reg: registry.New(s)}
}

// ListRepos implements the list_repos tool.
func (t *Tools) ListRepos() ([]registry.RepoInfo, error) {
	return t.reg.List()
}

// ResolveRepo implements resolve_repo: exact first, else fuzzy candidates.
func (t *Tools) ResolveRepo(query string) (best *registry.RepoInfo, candidates []registry.RepoInfo, err error) {
	if ri, ok, e := t.reg.Resolve(query); e != nil {
		return nil, nil, e
	} else if ok {
		return &ri, nil, nil
	}
	m, e := t.reg.Match(query)
	if e != nil {
		return nil, nil, e
	}
	if len(m) == 0 {
		return nil, nil, nil
	}
	return &m[0], m, nil
}

// SymbolExplanation is the explain_symbol result.
type SymbolExplanation struct {
	Node    store.NodeRow
	Callers []store.EdgeRow
	Callees []store.EdgeRow
}

// ExplainSymbol implements explain_symbol. symbol may be a node id or a label.
func (t *Tools) ExplainSymbol(repo, symbol string) (SymbolExplanation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return SymbolExplanation{}, err
	}
	if !ok {
		return SymbolExplanation{}, ErrNotFound
	}
	node, found, err := t.s.NodeByID(ri.IndexID, symbol)
	if err != nil {
		return SymbolExplanation{}, err
	}
	if !found {
		byLabel, err := t.s.NodesByLabel(ri.IndexID, symbol)
		if err != nil {
			return SymbolExplanation{}, err
		}
		if len(byLabel) == 0 {
			return SymbolExplanation{}, ErrNotFound
		}
		node = byLabel[0]
	}
	callers, err := t.s.Callers(ri.IndexID, node.NodeID)
	if err != nil {
		return SymbolExplanation{}, err
	}
	callees, err := t.s.Callees(ri.IndexID, node.NodeID)
	if err != nil {
		return SymbolExplanation{}, err
	}
	return SymbolExplanation{Node: node, Callers: callers, Callees: callees}, nil
}

// GetFileContext implements get_file_context.
func (t *Tools) GetFileContext(repo, file string) ([]store.NodeRow, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.NodesByFile(ri.IndexID, file)
}

// QueryRepo implements query_repo: structural node lookup by label.
func (t *Tools) QueryRepo(repo, name string) ([]store.NodeRow, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.NodesByLabel(ri.IndexID, name)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp
git commit -m "feat(mcp): pure tool implementations over the store"
```

---

### Task 10: Ingest pipeline — config → loader (the `index` use case)

**Files:**
- Create: `internal/ingest/ingest_test.go`
- Create: `internal/ingest/ingest.go`

- [ ] **Step 1: Write the failing test**

Create `internal/ingest/ingest_test.go`:

```go
package ingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/store"
)

func TestRunIngestsAndToleratesMissing(t *testing.T) {
	dir := t.TempDir()
	graphPath := filepath.Join(dir, "graph.json")
	os.WriteFile(graphPath, []byte(`{"nodes":[{"id":"a","label":"A","file_type":"code"}],"edges":[]}`), 0o644)

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "org/has-graph", Graph: graphPath, Commit: "c1", Branch: "main"},
		{Repo: "org/missing", Graph: filepath.Join(dir, "nope.json"), Commit: "c2"},
	}}

	s, _ := store.Open(":memory:")
	defer s.Close()

	report, err := Run(s, cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Indexed != 1 || report.Skipped != 1 {
		t.Fatalf("expected 1 indexed / 1 skipped, got %+v", report)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 node ingested, got %d", n)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ingest/`
Expected: FAIL — `undefined: Run`

- [ ] **Step 3: Implement**

Create `internal/ingest/ingest.go`:

```go
package ingest

import (
	"fmt"
	"os"

	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/graph"
	"github.com/vend-ai/intel-mcp/internal/store"
)

// Report summarizes an ingestion run.
type Report struct {
	Indexed  int
	Skipped  int
	Warnings []string
}

// Run ingests every repo in cfg into the store. Missing graph files are
// skipped with a warning rather than aborting the whole run.
func Run(s *store.Store, cfg *config.Config) (Report, error) {
	var rep Report
	for _, r := range cfg.Repos {
		f, err := os.Open(r.Graph)
		if err != nil {
			rep.Skipped++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: %v", r.Repo, err))
			continue
		}
		g, err := graph.Parse(f)
		f.Close()
		if err != nil {
			rep.Skipped++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s: parse: %v", r.Repo, err))
			continue
		}
		indexID, err := s.UpsertIndex(r.Org(), r.Name(), r.Commit, r.Branch, r.Graph)
		if err != nil {
			return rep, err
		}
		if _, err := graph.Load(s, indexID, g); err != nil {
			return rep, err
		}
		rep.Indexed++
	}
	return rep, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ingest/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/ingest
git commit -m "feat(ingest): config-driven ingestion with missing-graph tolerance"
```

---

### Task 11: MCP server adapter (official SDK) + cobra CLI

> **SDK spike note:** Verify the exact `github.com/modelcontextprotocol/go-sdk/mcp` API (server construction, tool registration, stdio transport) at the start of this task with a tiny throwaway `main` before wiring. If the official SDK API differs from the calls below, adjust ONLY this file; the pure `Tools` and resources stay unchanged. If the SDK proves unworkable, switch this adapter to `github.com/mark3labs/mcp-go` behind the same wiring.

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/resources.go`
- Create: `internal/mcp/resources_test.go`
- Modify: `cmd/intel-mcp/main.go`

- [ ] **Step 1: Write the failing test (resources are pure → testable)**

Create `internal/mcp/resources_test.go`:

```go
package mcp

import (
	"strings"
	"testing"
)

func TestRepoResource(t *testing.T) {
	tl := seedTools(t)
	body, err := tl.RepoResource("org/svc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "org/svc") {
		t.Fatalf("expected repo identity in body, got %q", body)
	}
}

func TestGraphNodeResource(t *testing.T) {
	tl := seedTools(t)
	body, err := tl.GraphNodeResource("org/svc", "n1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "ReserveProduct") {
		t.Fatalf("expected node label, got %q", body)
	}
}

func TestGraphNodeResourceUnknown(t *testing.T) {
	tl := seedTools(t)
	if _, err := tl.GraphNodeResource("org/svc", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run Resource`
Expected: FAIL — `undefined: (*Tools).RepoResource`

- [ ] **Step 3: Implement resources (pure)**

Create `internal/mcp/resources.go`:

```go
package mcp

import "encoding/json"

// RepoResource returns the JSON snapshot summary for repo://org/name.
func (t *Tools) RepoResource(repo string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(ri, "", "  ")
	return string(b), err
}

// GraphNodeResource returns the JSON for a node behind
// graph://org/name/commit/<sha>/node/<node_id>.
func (t *Tools) GraphNodeResource(repo, nodeID string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	node, found, err := t.s.NodeByID(ri.IndexID, nodeID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(node, "", "  ")
	return string(b), err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/`
Expected: PASS

- [ ] **Step 5: Wire the SDK adapter and CLI (verified against the real SDK API)**

Create `internal/mcp/server.go` — register each tool/resource by delegating to the pure methods above. Pseudocode against the official SDK shape (adjust to the verified API):

```go
package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vend-ai/intel-mcp/internal/store"
)

// Serve runs the MCP stdio server backed by the store until ctx is cancelled.
func Serve(ctx context.Context, s *store.Store) error {
	tools := NewTools(s)
	srv := mcpsdk.NewServer("intel-mcp", "0.1.0-dev", nil)

	// Register each base tool; each handler unmarshals args, calls the pure
	// method on `tools`, and marshals the result to the SDK's content type.
	registerListRepos(srv, tools)
	registerResolveRepo(srv, tools)
	registerQueryRepo(srv, tools)
	registerExplainSymbol(srv, tools)
	registerGetFileContext(srv, tools)
	registerResources(srv, tools)

	return srv.Run(ctx, mcpsdk.NewStdioTransport())
}
```

Implement the `registerX` helpers in the same file, each thin (parse args → call `tools.X` → marshal). Keep all SDK types confined to this file.

Replace `cmd/intel-mcp/main.go` with a cobra root plus `serve` and `index` commands:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vend-ai/intel-mcp/internal/config"
	"github.com/vend-ai/intel-mcp/internal/ingest"
	"github.com/vend-ai/intel-mcp/internal/mcp"
	"github.com/vend-ai/intel-mcp/internal/store"
)

func main() {
	var dbPath, manifest string
	root := &cobra.Command{Use: "intel-mcp"}
	root.PersistentFlags().StringVar(&dbPath, "db", "intel.db", "SQLite database path")
	root.PersistentFlags().StringVar(&manifest, "config", "manifest.yaml", "repo manifest path")

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Ingest repo graphs from the manifest into the store",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(manifest)
			if err != nil {
				return err
			}
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			rep, err := ingest.Run(s, cfg)
			if err != nil {
				return err
			}
			fmt.Printf("indexed=%d skipped=%d\n", rep.Indexed, rep.Skipped)
			for _, w := range rep.Warnings {
				fmt.Fprintln(os.Stderr, "warning:", w)
			}
			return nil
		},
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the MCP stdio server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := store.Open(dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			return mcp.Serve(context.Background(), s)
		},
	}

	root.AddCommand(indexCmd, serveCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Verify build**

Run: `go build ./... && go test ./...`
Expected: build succeeds; all unit tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/mcp cmd
git commit -m "feat(mcp): stdio server adapter + cobra serve/index CLI"
```

---

### Task 12: End-to-end stdio test

**Files:**
- Create: `internal/mcp/e2e_test.go`

- [ ] **Step 1: Write the E2E test**

Build the binary, ingest a fixture, launch `serve` over stdio, and exchange MCP `initialize` + `tools/list` + one `tools/call`. Implement as a Go test that compiles the binary with `go build`, writes a fixture `graph.json` and a manifest in a temp dir, runs `index`, then runs `serve` as a subprocess wired to stdin/stdout pipes, sending newline-delimited JSON-RPC and asserting:
- `tools/list` response contains all five tool names.
- A `tools/call` for `list_repos` returns the ingested repo.

```go
package mcp

// TestEndToEndStdio: see steps. Skip with t.Skip if the SDK transport framing
// differs; in that case assert via an in-process server handle instead.
```

- [ ] **Step 2: Run it to confirm it fails (no assertions yet / not implemented)**

Run: `go test ./internal/mcp/ -run TestEndToEndStdio -v`
Expected: FAIL until implemented.

- [ ] **Step 3: Implement against the verified SDK transport framing**

Fill in the subprocess JSON-RPC exchange using the framing the official SDK's stdio transport expects (confirmed in Task 11). Assert the five tool names and the `list_repos` result.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp
git commit -m "test(mcp): end-to-end stdio initialize + tools/list + tools/call"
```

---

### Task 13: tasks.md sync + degradation sweep

**Files:**
- Modify: `openspec/changes/mcp-core-foundation/tasks.md`

- [ ] **Step 1: Confirm degradation behaviors are covered by tests**

Verify these tests exist and pass (add any missing as failing-first): empty graph (`TestParseEmpty`, ingest of empty nodes), missing graph file (`TestRunIngestsAndToleratesMissing`), unknown repo/symbol (`TestExplainSymbolUnknownReturnsEmpty`, `TestGraphNodeResourceUnknown`), malformed entries skipped (`TestLoadIsIdempotentAndSkipsMalformed`).

Run: `go test ./...`
Expected: PASS

- [ ] **Step 2: Check off OpenSpec tasks.md**

Mark items 1.1–7.3 in `openspec/changes/mcp-core-foundation/tasks.md` as complete (`- [ ]` → `- [x]`), and update task 3.3 to reflect the design decision: cross-repo is a query-time join (`NodesByLabelAllIndexes`); merged-graph input is deferred.

- [ ] **Step 3: Commit**

```bash
git add openspec/changes/mcp-core-foundation/tasks.md
git commit -m "docs(openspec): check off mcp-core-foundation tasks; note cross-repo join decision"
```

---

## Self-Review

**Spec coverage:**
- `graph-index` / Repo manifest registration → Task 6 (config), Task 8 (registry).
- `graph-index` / Per-repo index into index_id (idempotent) → Task 3 (upsert), Task 5 (loader), Task 10 (ingest).
- `graph-index` / Graphify schema tolerance (missing/empty/malformed) → Task 4, Task 5, Task 10, Task 13.
- `graph-index` / Cross-index query helper → Task 7 (`NodesByLabelAllIndexes`).
- `mcp-core` / MCP stdio server advertises tools → Task 11, Task 12.
- `mcp-core` / list_repos + resolve_repo → Task 8, Task 9.
- `mcp-core` / structural query_repo/explain_symbol/get_file_context + empty-not-error → Task 9.
- `mcp-core` / repo:// + graph:// resources (commit-pinned, degrade) → Task 11.

**Placeholder scan:** The only deliberately deferred specifics are the exact official-SDK API calls in Task 11 and the stdio framing in Task 12 — both gated behind an explicit spike note because the SDK is young, and both confined to one file. All store/graph/config/registry/tools code is complete and compilable.

**Type consistency:** `store.NodeRow`/`store.EdgeRow` are used consistently across Tasks 7, 9, 11. `registry.RepoInfo` consistent across Tasks 8, 9, 11. `Tools` methods (`ListRepos`, `ResolveRepo`, `QueryRepo`, `ExplainSymbol`, `GetFileContext`, `RepoResource`, `GraphNodeResource`) consistent between Tasks 9 and 11. `graph.Load` signature consistent between Tasks 5 and 10.

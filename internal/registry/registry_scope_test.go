package registry

import (
	"testing"

	"github.com/noviopenworks/candlegraph/internal/config"
	"github.com/noviopenworks/candlegraph/internal/store"
)

func seedTwoSnapshots(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
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
	if !reg.InScope(c2) {
		t.Fatal("c2 should be in scope")
	}
	if reg.InScope(c2 + 100) {
		t.Fatal("unknown id should be out of scope")
	}
}

func TestUnscopedRegistryUnchanged(t *testing.T) {
	s := seedTwoSnapshots(t)
	reg := New(s)

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

func TestBuildScopePinAndLatestAndMissing(t *testing.T) {
	s := seedTwoSnapshots(t)
	if _, err := s.UpsertIndex("org", "other", "x2", "release", "/g/other2.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE indexes SET ingested_at='2026-01-01T00:00:00Z' WHERE commit_sha='x1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(`UPDATE indexes SET ingested_at='2026-01-02T00:00:00Z' WHERE commit_sha='x2'`); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Repos: []config.RepoConfig{
		{Repo: "org/web", Commit: "c1", Graph: "/g/web1.json"},
		{Repo: "org/other", Graph: "/g/other.json"},
		{Repo: "org/ghost", Commit: "zz", Graph: "/g/ghost.json"},
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

	var c1, c2, x1, x2 int64
	if err := s.DB.QueryRow(`SELECT i.id FROM indexes i JOIN repos r ON r.id=i.repo_id WHERE r.name='web' AND i.commit_sha='c1'`).Scan(&c1); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(`SELECT i.id FROM indexes i JOIN repos r ON r.id=i.repo_id WHERE r.name='web' AND i.commit_sha='c2'`).Scan(&c2); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(`SELECT i.id FROM indexes i JOIN repos r ON r.id=i.repo_id WHERE r.name='other' AND i.commit_sha='x1'`).Scan(&x1); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.QueryRow(`SELECT i.id FROM indexes i JOIN repos r ON r.id=i.repo_id WHERE r.name='other' AND i.commit_sha='x2'`).Scan(&x2); err != nil {
		t.Fatal(err)
	}
	if !allowed[c1] || allowed[c2] {
		t.Fatalf("org/web must be pinned to c1 only: allowed=%v", allowed)
	}
	if !allowed[x2] || allowed[x1] {
		t.Fatalf("org/other must resolve to latest x2 only: allowed=%v", allowed)
	}
}

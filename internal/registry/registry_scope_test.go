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

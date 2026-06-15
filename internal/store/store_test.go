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

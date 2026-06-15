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

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

func TestNodeRowsIncludeStoredProvenance(t *testing.T) {
	s, idA, _ := seed(t)
	defer s.Close()
	mustExec(t, s, `UPDATE nodes SET source_url=?, captured_at=?, author=?, contributor=? WHERE index_id=? AND node_id=?`,
		"https://github.com/org/svc-a/blob/a1/h.go", "2026-07-09T12:00:00Z", "Ada", "Grace", idA, "n1")

	ns, err := s.NodesByLabel(idA, "ReserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if len(ns) != 1 {
		t.Fatalf("expected one node, got %+v", ns)
	}
	got := ns[0]
	if got.SourceURL != "https://github.com/org/svc-a/blob/a1/h.go" || got.CapturedAt != "2026-07-09T12:00:00Z" || got.Author != "Ada" || got.Contributor != "Grace" {
		t.Fatalf("provenance not scanned: %+v", got)
	}

	byID, ok, err := s.NodeByID(idA, "n1")
	if err != nil || !ok {
		t.Fatalf("NodeByID: ok=%v err=%v", ok, err)
	}
	if byID.SourceURL != got.SourceURL {
		t.Fatalf("NodeByID SourceURL=%q, want %q", byID.SourceURL, got.SourceURL)
	}

	byFile, err := s.NodesByFile(idA, "h.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(byFile) != 1 || byFile[0].Contributor != "Grace" {
		t.Fatalf("NodesByFile provenance mismatch: %+v", byFile)
	}
}

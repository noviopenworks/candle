package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

func TestMatchRPCsConfidence(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "server.go")
	code := "package svc\n" +
		"func (s *Server) ReserveProduct(ctx context.Context, req *pb.ReserveProductRequest) (*pb.ReserveProductResponse, error) { return nil, nil }\n" +
		"func (s *Server) Sync(stream pb.InventoryService_SyncServer) error { return nil }\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "n1", "ReserveProduct", src)
	mustNode(t, s, id, "n2", "Sync", src)
	mustNode(t, s, id, "n3", "RegisterInventoryServiceServer", src)

	rpcs := []RPC{
		{FullName: "acme.inventory.InventoryService.ReserveProduct", Service: "InventoryService", Name: "ReserveProduct", StreamKind: "unary"},
		{FullName: "acme.inventory.InventoryService.Sync", Service: "InventoryService", Name: "Sync", StreamKind: "bidi"},
		{FullName: "acme.inventory.InventoryService.Ghost", Service: "InventoryService", Name: "Ghost", StreamKind: "unary"},
	}
	links, err := MatchRPCs(s, id, rpcs)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	byRPC := map[string][]store.RPCImplLink{}
	for _, l := range links {
		byRPC[l.RPCFullName] = append(byRPC[l.RPCFullName], l)
	}
	rp := byRPC["acme.inventory.InventoryService.ReserveProduct"]
	if len(rp) != 1 || rp[0].NodeID != "n1" || rp[0].Confidence < 0.85 {
		t.Fatalf("ReserveProduct link: %+v", rp)
	}
	sy := byRPC["acme.inventory.InventoryService.Sync"]
	if len(sy) != 1 || sy[0].NodeID != "n2" || sy[0].Confidence < 0.85 {
		t.Fatalf("Sync link: %+v", sy)
	}
	if len(byRPC["acme.inventory.InventoryService.Ghost"]) != 0 {
		t.Fatalf("Ghost should have no impl")
	}
}

func TestMatchRPCsAmbiguousLowConfidence(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "other.go")
	// Source content does NOT contain a matching method signature for "Handle".
	code := "package svc\n" +
		"// Handle is referenced in a comment but no func signature matches.\n" +
		"var x = 1\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "other", "def", "main", "/g")
	// Two nodes with the same label, pointing at a non-matching source file.
	mustNode(t, s, id, "n4", "Handle", src)
	mustNode(t, s, id, "n5", "Handle", src)
	// No "RegisterOtherServiceServer" / "OtherServiceServer" node => no registration.

	rpcs := []RPC{
		{FullName: "acme.other.OtherService.Handle", Service: "OtherService", Name: "Handle", StreamKind: "unary"},
	}
	links, err := MatchRPCs(s, id, rpcs)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 ambiguous candidates, got %d: %+v", len(links), links)
	}
	for _, l := range links {
		if l.Confidence != 0.3 {
			t.Fatalf("expected LOW confidence 0.3, got %v: %+v", l.Confidence, l)
		}
	}
}

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

func mustNode(t *testing.T, s *store.Store, indexID int64, id, label, file string) {
	t.Helper()
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id, node_id, label, file_type, source_file) VALUES(?,?,?,?,?)`,
		indexID, id, label, "go", file); err != nil {
		t.Fatal(err)
	}
}

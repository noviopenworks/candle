package link

import (
	"path/filepath"
	"testing"

	"github.com/noviopenworks/candlegraph/internal/store"
)

// fixtureRoot is the testdata repo root containing parseable Go fixtures.
func fixtureRoot() string { return filepath.Join("testdata", "repo") }

func TestAstSignatureMatch(t *testing.T) {
	root := fixtureRoot()
	const serverGo = "internal/grpc/server.go"

	tests := []struct {
		name        string
		root        string
		sourceFile  string
		rpcName     string
		streamKind  string
		wantMatched bool
		wantOK      bool
	}{
		{"unary matches", root, serverGo, "ReserveProduct", "unary", true, true},
		{"unary wrong streamKind", root, serverGo, "ReserveProduct", "server_stream", false, true},
		{"server stream matches", root, serverGo, "Sync", "server_stream", true, true},
		{"server stream as bidi matches", root, serverGo, "Sync", "bidi", true, true},
		{"client stream matches", root, serverGo, "Upload", "client_stream", true, true},
		{"stream method wrong as unary", root, serverGo, "Sync", "unary", false, true},
		{"multi-line unary matches", root, serverGo, "MultiLine", "unary", true, true},
		{"free function no receiver no match", root, serverGo, "FreeFunction", "unary", false, true},
		{"unknown method not found", root, serverGo, "Ghost", "unary", false, true},
		{"empty root => not ok", "", serverGo, "ReserveProduct", "unary", false, false},
		{"missing file => not ok", root, "internal/grpc/missing.go", "ReserveProduct", "unary", false, false},
		{"unparseable file => not ok", root, "internal/other/broken.go", "Broken", "unary", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, ok := astSignatureMatch(tt.root, tt.sourceFile, tt.rpcName, tt.streamKind)
			if matched != tt.wantMatched || ok != tt.wantOK {
				t.Fatalf("astSignatureMatch(%q,%q,%q,%q) = (%v,%v), want (%v,%v)",
					tt.root, tt.sourceFile, tt.rpcName, tt.streamKind, matched, ok, tt.wantMatched, tt.wantOK)
			}
		})
	}
}

// TestMatchRPCsAST exercises the AST-confirmed HIGH tier and the AST-authoritative
// negative (matched=false, ok=true) which must NOT grant HIGH.
func TestMatchRPCsAST(t *testing.T) {
	root := fixtureRoot()
	const serverGo = "internal/grpc/server.go"

	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "n1", "ReserveProduct", serverGo)
	mustNode(t, s, id, "n2", "Sync", serverGo)
	mustNode(t, s, id, "n3", "RegisterInventoryServiceServer", serverGo)
	// MultiLine has a service registration => AST-negative should keep MEDIUM (here HIGH expected since AST matches).
	mustNode(t, s, id, "n4", "MultiLine", serverGo)

	rpcs := []RPC{
		{FullName: "acme.inventory.InventoryService.ReserveProduct", Service: "InventoryService", Name: "ReserveProduct", StreamKind: "unary"},
		{FullName: "acme.inventory.InventoryService.Sync", Service: "InventoryService", Name: "Sync", StreamKind: "server_stream"},
		{FullName: "acme.inventory.InventoryService.MultiLine", Service: "InventoryService", Name: "MultiLine", StreamKind: "unary"},
	}
	links, err := MatchRPCs(s, id, rpcs, root)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	byRPC := map[string]store.RPCImplLink{}
	for _, l := range links {
		byRPC[l.RPCFullName] = l
	}
	for _, full := range []string{
		"acme.inventory.InventoryService.ReserveProduct",
		"acme.inventory.InventoryService.Sync",
		"acme.inventory.InventoryService.MultiLine",
	} {
		l := byRPC[full]
		if l.Confidence != confHigh {
			t.Fatalf("%s: want HIGH %.2f, got %.2f (%s)", full, confHigh, l.Confidence, l.MatchReason)
		}
		if !containsAST(l.MatchReason) {
			t.Fatalf("%s: reason should mention ast, got %q", full, l.MatchReason)
		}
	}
}

// TestMatchRPCsASTNegativeNoHigh: AST is authoritative; when AST says no-match
// (ok=true,matched=false) the linker must NOT grant HIGH even if a string-scan
// would have. It keeps name+service (MEDIUM) and the reason must not say ast/signature.
func TestMatchRPCsASTNegativeNoHigh(t *testing.T) {
	root := fixtureRoot()
	const serverGo = "internal/grpc/server.go"

	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	// ReserveProduct is unary in source; declare the RPC as server_stream so AST
	// rejects the shape. Service is registered => expect MEDIUM, reason "name+service".
	mustNode(t, s, id, "n1", "ReserveProduct", serverGo)
	mustNode(t, s, id, "n3", "RegisterInventoryServiceServer", serverGo)

	rpcs := []RPC{
		{FullName: "acme.inventory.InventoryService.ReserveProduct", Service: "InventoryService", Name: "ReserveProduct", StreamKind: "server_stream"},
	}
	links, err := MatchRPCs(s, id, rpcs, root)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("want 1 link, got %d", len(links))
	}
	l := links[0]
	if l.Confidence != confMedium {
		t.Fatalf("want MEDIUM %.2f, got %.2f (%s)", confMedium, l.Confidence, l.MatchReason)
	}
	if containsAST(l.MatchReason) || containsSignature(l.MatchReason) {
		t.Fatalf("reason must not claim ast/signature, got %q", l.MatchReason)
	}
}

// TestMatchExportsASTDisambiguation: two same-named symbols in different packages;
// the export's package path must select the AST-declared one.
func TestMatchExportsASTDisambiguation(t *testing.T) {
	root := fixtureRoot()

	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "platform", "abc", "main", "/g")
	// Two ValidateToken nodes pointing at different package files.
	mustNode(t, s, id, "legacy", "ValidateToken", "internal/legacy/legacy.go")
	mustNode(t, s, id, "auth", "ValidateToken", "internal/auth/auth.go")

	exports := []Export{
		{PackagePath: "example.com/repo/internal/auth", Symbol: "ValidateToken", SourceHint: "auth"},
	}
	linked := MatchExports(s, id, exports, root)
	if linked[0].NodeID != "auth" {
		t.Fatalf("ValidateToken should resolve to the auth-package node, got %q", linked[0].NodeID)
	}
}

func containsAST(s string) bool       { return contains(s, "ast") }
func containsSignature(s string) bool { return contains(s, "signature") }
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

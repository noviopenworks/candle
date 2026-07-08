package link

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

// TestMatchOpenAPIHighViaAST: a handler whose go/ast declaration is a real HTTP
// handler under root earns HIGH with an "ast" reason.
func TestMatchOpenAPIHighViaAST(t *testing.T) {
	root := filepath.Join("testdata", "repo")
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", "internal/http/handler.go")

	ops := []Op{{Method: "POST", Path: "/products/{id}/reservations", OperationID: "reserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, root)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	// operationId "reserveProduct" → candidates {reserveProduct, ReserveProduct};
	// the PascalCase candidate matches node h1.
	var hit *store.HTTPOpImplLink
	for i := range links {
		if links[i].NodeID == "h1" {
			hit = &links[i]
		}
	}
	if hit == nil || hit.Confidence < 0.85 {
		t.Fatalf("expected HIGH link to h1, got: %+v", links)
	}
	if hit.Method != "POST" || hit.Path != "/products/{id}/reservations" {
		t.Fatalf("identity not carried: %+v", hit)
	}
}

// TestMatchOpenAPIHighViaStringScan: no root, but the node's source_file is
// directly readable and matches the handler shape → HIGH via "+signature".
func TestMatchOpenAPIHighViaStringScan(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "handler.go")
	code := "package h\n" +
		"import \"net/http\"\n" +
		"func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request) {}\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", src)

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, "") // root="" disables AST
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 || links[0].NodeID != "h1" || links[0].Confidence < 0.85 {
		t.Fatalf("expected HIGH via string-scan, got: %+v", links)
	}
}

// TestMatchOpenAPIMediumViaRoute: route-registration presence but no AST
// confirmation (no root, unreadable source) → MEDIUM "name+route".
func TestMatchOpenAPIMediumViaRoute(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", "/nonexistent/handler.go")
	mustNode(t, s, id, "r1", "HandleFunc", "/nonexistent/router.go") // route presence

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, "")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 || links[0].Confidence != 0.6 || links[0].MatchReason != "name+route" {
		t.Fatalf("expected MEDIUM name+route, got: %+v", links)
	}
}

// TestMatchOpenAPIBareHandleDoesNotInflate: a lone symbol named "Handle" (an
// extremely common method name) must NOT be treated as route-registration
// presence; the route signal requires specific router constructors. A name-only
// candidate therefore stays LOW, not MEDIUM.
func TestMatchOpenAPIBareHandleDoesNotInflate(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", "/nonexistent/handler.go")
	mustNode(t, s, id, "x1", "Handle", "/nonexistent/other.go") // common name, not a router

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, "")
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 || links[0].Confidence != 0.3 || links[0].MatchReason != "name" {
		t.Fatalf("expected LOW name (bare Handle must not inflate), got: %+v", links)
	}
}

// TestMatchOpenAPIMultiCandidateDisambiguation: when two same-named nodes exist —
// a real HTTP handler and a non-handler (e.g. a gRPC method) — both are linked,
// but the AST gate keeps them on distinct tiers (handler HIGH, non-handler LOW),
// so the tier disambiguates which is the real handler.
func TestMatchOpenAPIMultiCandidateDisambiguation(t *testing.T) {
	dir := t.TempDir()
	handlerSrc := filepath.Join(dir, "handler.go")
	if err := os.WriteFile(handlerSrc, []byte("package h\nimport \"net/http\"\n"+
		"func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	grpcSrc := filepath.Join(dir, "server.go")
	if err := os.WriteFile(grpcSrc, []byte("package g\nimport \"context\"\n"+
		"func (s *Server) ReserveProduct(ctx context.Context, req *Req) (*Resp, error) { return nil, nil }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "http1", "ReserveProduct", "handler.go")
	mustNode(t, s, id, "grpc1", "ReserveProduct", "server.go")

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, dir)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	byNode := map[string]store.HTTPOpImplLink{}
	for _, l := range links {
		byNode[l.NodeID] = l
	}
	if got := byNode["http1"]; got.Confidence < 0.85 {
		t.Fatalf("expected HTTP handler HIGH, got %+v", got)
	}
	if got := byNode["grpc1"]; got.Confidence != 0.3 || got.MatchReason != "name" {
		t.Fatalf("expected gRPC method LOW name (not promoted), got %+v", got)
	}
}

// TestMatchOpenAPILowForNonHandler: a same-named domain-service method (not an
// HTTP handler) with no route presence stays LOW and is never promoted to HIGH.
func TestMatchOpenAPILowForNonHandler(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "service.go")
	code := "package svc\n" +
		"import \"context\"\n" +
		"func (s *Service) ReserveProduct(ctx context.Context, req *Request) (*Reservation, error) { return nil, nil }\n"
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	root := dir
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "n1", "ReserveProduct", "service.go") // resolves under root=dir

	ops := []Op{{Method: "POST", Path: "/x", OperationID: "ReserveProduct"}}
	links, err := MatchOpenAPI(s, id, ops, root)
	if err != nil {
		t.Fatalf("match: %v", err)
	}
	if len(links) != 1 || links[0].Confidence != 0.3 || links[0].MatchReason != "name" {
		t.Fatalf("expected LOW name (no AST promotion), got: %+v", links)
	}
}

// TestMatchOpenAPINoLink: no operationId → no link; a candidate with no matching
// node → no link. Neither errors.
func TestMatchOpenAPINoLink(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	mustNode(t, s, id, "h1", "ReserveProduct", "/x/handler.go")

	// No operationId → no candidates → no link.
	ops := []Op{{Method: "GET", Path: "/health", OperationID: ""}}
	links, err := MatchOpenAPI(s, id, ops, "")
	if err != nil || len(links) != 0 {
		t.Fatalf("no-operationId: %+v err=%v", links, err)
	}

	// operationId with no matching node → no link.
	ops = []Op{{Method: "GET", Path: "/ghost", OperationID: "ghostHandler"}}
	links, err = MatchOpenAPI(s, id, ops, "")
	if err != nil || len(links) != 0 {
		t.Fatalf("no-candidate: %+v err=%v", links, err)
	}
}

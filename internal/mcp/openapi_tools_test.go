package mcp

import (
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func seedAPITools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("org", "svc", "abc", "main", "/g")
	s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "Inventory API", Version: "1.4.0", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "POST", Path: "/x", OperationID: "reserveProduct", ResponseSchema: "ReservationResponse"}},
		Schemas:    []store.APISchema{{Name: "ReservationResponse", Kind: "openapi_schema"}},
	}})
	return NewTools(s)
}

func TestListAPIs(t *testing.T) {
	tl := seedAPITools(t)
	apis, err := tl.ListAPIs("org/svc")
	if err != nil || len(apis) != 1 || apis[0].Kind != "openapi" || apis[0].Name != "Inventory API" {
		t.Fatalf("list_apis: %+v err=%v", apis, err)
	}
}

func TestExplainEndpoint(t *testing.T) {
	tl := seedAPITools(t)
	out, err := tl.ExplainEndpoint("org/svc", "POST", "/x")
	if err != nil {
		t.Fatal(err)
	}
	if out.Operation.OperationID != "reserveProduct" || out.Operation.ResponseSchema != "ReservationResponse" {
		t.Fatalf("explain: %+v", out)
	}
}

func TestExplainEndpointImplementedBy(t *testing.T) {
	s, _ := store.Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "I", Version: "1", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "POST", Path: "/x", OperationID: "reserveProduct"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkHTTPOpImpls(id, []store.HTTPOpImplLink{{
		Method: "POST", Path: "/x", NodeID: "h1", Confidence: 0.9, MatchReason: "name+route+ast"}}); err != nil {
		t.Fatal(err)
	}
	tools := NewTools(s)

	expl, err := tools.ExplainEndpoint("acme/inventory", "POST", "/x")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if expl.Operation.OperationID != "reserveProduct" {
		t.Fatalf("operation not returned: %+v", expl.Operation)
	}
	if len(expl.ImplementedBy) != 1 || expl.ImplementedBy[0].Symbol != "h1" || expl.ImplementedBy[0].Confidence != "HIGH" {
		t.Fatalf("implemented_by: %+v", expl.ImplementedBy)
	}

	// No link → empty (non-nil) slice, contract still returned.
	if err := s.ReplaceAPISpecs(id, []store.APISpecBundle{{
		Spec:       store.APISpec{Kind: "openapi", Name: "I", Version: "1", Path: "api/openapi.yaml"},
		Operations: []store.HTTPOperation{{Method: "GET", Path: "/y", OperationID: "noimpl"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkHTTPOpImpls(id, nil); err != nil {
		t.Fatal(err)
	}
	expl, err = tools.ExplainEndpoint("acme/inventory", "GET", "/y")
	if err != nil {
		t.Fatalf("explain noimpl: %v", err)
	}
	if expl.ImplementedBy == nil || len(expl.ImplementedBy) != 0 {
		t.Fatalf("expected empty non-nil implemented_by, got %#v", expl.ImplementedBy)
	}
}

func TestExplainEndpointUnknown(t *testing.T) {
	tl := seedAPITools(t)
	if _, err := tl.ExplainEndpoint("org/svc", "GET", "/nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFindEndpointAndSchema(t *testing.T) {
	tl := seedAPITools(t)
	ops, err := tl.FindEndpoint("org/svc", "reserve")
	if err != nil || len(ops) != 1 {
		t.Fatalf("find_endpoint: %+v err=%v", ops, err)
	}
	sc, err := tl.FindSchema("org/svc", "Reservation")
	if err != nil || len(sc) != 1 {
		t.Fatalf("find_schema: %+v err=%v", sc, err)
	}
}

// TestOpenAPIToolsUnknownRepo verifies every openapi tool degrades to ErrNotFound
// (not a protocol error) for an unresolvable repo.
func TestOpenAPIToolsUnknownRepo(t *testing.T) {
	tl := seedAPITools(t)
	if _, err := tl.ListAPIs("no/such"); err != ErrNotFound {
		t.Fatalf("list_apis unknown repo: expected ErrNotFound, got %v", err)
	}
	if _, err := tl.FindEndpoint("no/such", "x"); err != ErrNotFound {
		t.Fatalf("find_endpoint unknown repo: expected ErrNotFound, got %v", err)
	}
	if _, err := tl.ExplainEndpoint("no/such", "GET", "/x"); err != ErrNotFound {
		t.Fatalf("explain_endpoint unknown repo: expected ErrNotFound, got %v", err)
	}
	if _, err := tl.FindSchema("no/such", "x"); err != ErrNotFound {
		t.Fatalf("find_schema unknown repo: expected ErrNotFound, got %v", err)
	}
}

// TestFindUnknownReturnsEmpty verifies that find tools on a resolvable repo
// return an empty result (not an error) when nothing matches.
func TestFindUnknownReturnsEmpty(t *testing.T) {
	tl := seedAPITools(t)
	ops, err := tl.FindEndpoint("org/svc", "zzz-no-match")
	if err != nil || len(ops) != 0 {
		t.Fatalf("expected empty endpoints, got %+v err=%v", ops, err)
	}
	sc, err := tl.FindSchema("org/svc", "zzz-no-match")
	if err != nil || len(sc) != 0 {
		t.Fatalf("expected empty schemas, got %+v err=%v", sc, err)
	}
}

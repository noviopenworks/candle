package store

import "testing"

func seedAPI(t *testing.T) (*Store, int64) {
	t.Helper()
	s, _ := Open(":memory:")
	id, _ := s.UpsertIndex("org", "svc", "abc", "main", "/g")
	spec := APISpec{Kind: "openapi", Name: "Inventory API", Version: "1.4.0", Path: "api/openapi.yaml"}
	ops := []HTTPOperation{{Method: "POST", Path: "/x", OperationID: "reserveProduct", Summary: "Reserve",
		RequestSchema: "ReserveProductRequest", ResponseSchema: "ReservationResponse", Security: []string{"bearerAuth"}, Tags: []string{"reservations"}}}
	schemas := []APISchema{{Name: "ReserveProductRequest", Kind: "openapi_schema", RawRef: "#/components/schemas/ReserveProductRequest"}}
	if err := s.ReplaceAPISpecs(id, []APISpecBundle{{Spec: spec, Operations: ops, Schemas: schemas}}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	return s, id
}

func TestListAPISpecsAndIdempotent(t *testing.T) {
	s, id := seedAPI(t)
	defer s.Close()
	specs, err := s.ListAPISpecs(id)
	if err != nil || len(specs) != 1 || specs[0].Name != "Inventory API" {
		t.Fatalf("list: %+v err=%v", specs, err)
	}
	// Replace again → still 1 (idempotent)
	s.ReplaceAPISpecs(id, []APISpecBundle{{Spec: specs[0].APISpec, Operations: nil, Schemas: nil}})
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM api_specs WHERE index_id=?`, id).Scan(&n)
	if n != 1 {
		t.Fatalf("expected 1 spec after replace, got %d", n)
	}
}

func TestFindOperationAndSchema(t *testing.T) {
	s, id := seedAPI(t)
	defer s.Close()
	ops, err := s.FindOperations(id, "reserveProduct")
	if err != nil || len(ops) != 1 || ops[0].OperationID != "reserveProduct" {
		t.Fatalf("find op: %+v err=%v", ops, err)
	}
	op, found, err := s.OperationByMethodPath(id, "POST", "/x")
	if err != nil || !found || op.ResponseSchema != "ReservationResponse" {
		t.Fatalf("op by method/path: %+v found=%v err=%v", op, found, err)
	}
	sc, err := s.FindSchemas(id, "Reserve")
	if err != nil || len(sc) != 1 {
		t.Fatalf("find schema: %+v err=%v", sc, err)
	}
}

func TestLinkHTTPOpImplsRoundTrip(t *testing.T) {
	s, _ := Open(":memory:")
	defer s.Close()
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")

	links := []HTTPOpImplLink{{
		Method: "POST", Path: "/products/{id}/reservations",
		NodeID: "h1", Confidence: 0.9, MatchReason: "name+route+ast"}}
	if err := s.LinkHTTPOpImpls(id, links); err != nil {
		t.Fatalf("link: %v", err)
	}

	// Method match is case-insensitive; path is exact.
	got, err := s.HTTPOpImpls(id, "post", "/products/{id}/reservations")
	if err != nil || len(got) != 1 || got[0].NodeID != "h1" || got[0].Confidence < 0.85 {
		t.Fatalf("impls: %+v err=%v", got, err)
	}
	if got[0].MatchReason != "name+route+ast" {
		t.Fatalf("reason: %q", got[0].MatchReason)
	}

	// Re-linking replaces (idempotent, still 1).
	if err := s.LinkHTTPOpImpls(id, links); err != nil {
		t.Fatalf("relink: %v", err)
	}
	got, _ = s.HTTPOpImpls(id, "POST", "/products/{id}/reservations")
	if len(got) != 1 {
		t.Fatalf("expected 1 after relink, got %d: %+v", len(got), got)
	}

	// Unknown path yields no links and no error.
	none, err := s.HTTPOpImpls(id, "POST", "/missing")
	if err != nil || len(none) != 0 {
		t.Fatalf("unknown path: %+v err=%v", none, err)
	}
}

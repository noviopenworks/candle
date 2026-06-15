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

package mcp

import (
	"testing"

	"github.com/vend-ai/intel-mcp/internal/store"
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
	if out.OperationID != "reserveProduct" || out.ResponseSchema != "ReservationResponse" {
		t.Fatalf("explain: %+v", out)
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

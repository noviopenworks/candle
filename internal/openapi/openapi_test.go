package openapi

import "testing"

func TestParseSpec(t *testing.T) {
	spec, err := ParseFile("testdata/inventory.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if spec.Name != "Inventory API" || spec.Version != "1.4.0" {
		t.Fatalf("bad meta: %+v", spec)
	}
	if len(spec.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(spec.Operations))
	}
	op := spec.Operations[0]
	if op.Method != "POST" || op.Path != "/products/{productId}/reservations" || op.OperationID != "reserveProduct" {
		t.Fatalf("bad op: %+v", op)
	}
	if op.RequestSchema != "ReserveProductRequest" || op.ResponseSchema != "ReservationResponse" {
		t.Fatalf("bad schemas: %+v", op)
	}
	if len(op.Security) != 1 || op.Security[0] != "bearerAuth" {
		t.Fatalf("bad security: %+v", op.Security)
	}
	if len(spec.Schemas) != 2 {
		t.Fatalf("expected 2 schemas, got %d", len(spec.Schemas))
	}
}

func TestParseSwagger2IsRejected(t *testing.T) {
	_, err := ParseFile("testdata/swagger2.yaml")
	if err != ErrUnsupportedVersion {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
	}
}

package mcp

import (
	"strings"
	"testing"
)

func TestRepoResource(t *testing.T) {
	tl := seedTools(t)
	body, err := tl.RepoResource("org/svc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "org/svc") {
		t.Fatalf("expected repo identity in body, got %q", body)
	}
}

func TestGraphNodeResource(t *testing.T) {
	tl := seedTools(t)
	body, err := tl.GraphNodeResource("org/svc", "n1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "ReserveProduct") {
		t.Fatalf("expected node label, got %q", body)
	}
}

func TestGraphNodeResourceUnknown(t *testing.T) {
	tl := seedTools(t)
	if _, err := tl.GraphNodeResource("org/svc", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestOperationResource(t *testing.T) {
	tl := seedAPITools(t)
	body, err := tl.OperationResource("org/svc", "reserveProduct")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "reserveProduct") || !strings.Contains(body, "ReservationResponse") {
		t.Fatalf("expected operation contract, got %q", body)
	}
	if _, err := tl.OperationResource("org/svc", "nope"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSchemaResource(t *testing.T) {
	tl := seedAPITools(t)
	body, err := tl.SchemaResource("org/svc", "ReservationResponse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "ReservationResponse") {
		t.Fatalf("expected schema, got %q", body)
	}
	// Substring (non-exact) must not match — resource requires exact name.
	if _, err := tl.SchemaResource("org/svc", "Reservation"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for non-exact name, got %v", err)
	}
}

func TestSpecResource(t *testing.T) {
	tl := seedAPITools(t)
	body, err := tl.SpecResource("org/svc", "api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Inventory API") || !strings.Contains(body, "reserveProduct") {
		t.Fatalf("expected spec + its operations, got %q", body)
	}
	if _, err := tl.SpecResource("org/svc", "missing.yaml"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProtoResources(t *testing.T) {
	tools := seedProtoTools(t)

	rpcBody, err := tools.ProtoRPCResource("acme/inventory", "acme.inventory", "InventoryService", "ReserveProduct")
	if err != nil || !strings.Contains(rpcBody, "ReserveProduct") {
		t.Fatalf("rpc resource: %q err=%v", rpcBody, err)
	}
	msgBody, err := tools.ProtoMessageResource("acme/inventory", "acme.inventory", "ReserveProductRequest")
	if err != nil || !strings.Contains(msgBody, "sku") {
		t.Fatalf("message resource: %q err=%v", msgBody, err)
	}
	if _, err := tools.ProtoMessageResource("acme/inventory", "acme.inventory", "Nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestParseProtoURI(t *testing.T) {
	repo, kind, ref, err := parseProtoURI("proto://acme/inventory/commit/abc/rpc/acme.inventory/InventoryService/ReserveProduct")
	if err != nil || repo != "acme/inventory" || kind != "rpc" || ref != "acme.inventory/InventoryService/ReserveProduct" {
		t.Fatalf("parse: repo=%q kind=%q ref=%q err=%v", repo, kind, ref, err)
	}
}

func TestParseOpenAPIURI(t *testing.T) {
	repo, kind, ref, err := parseOpenAPIURI("openapi://org/svc/commit/abc/spec/api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if repo != "org/svc" || kind != "spec" || ref != "api/openapi.yaml" {
		t.Fatalf("bad parse: repo=%q kind=%q ref=%q", repo, kind, ref)
	}
	if _, _, _, err := parseOpenAPIURI("openapi://org/svc/bad"); err == nil {
		t.Fatalf("expected error for malformed uri")
	}
}

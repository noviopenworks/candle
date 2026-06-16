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

func TestLibResources(t *testing.T) {
	tools := seedGoDepTools(t)
	body, err := tools.LibraryResource("git.acme.local/platform/auth")
	if err != nil || !strings.Contains(body, "NewClient") {
		t.Fatalf("lib resource: %q err=%v", body, err)
	}
	symBody, err := tools.LibrarySymbolResource("git.acme.local/platform/auth", "NewClient")
	if err != nil || !strings.Contains(symBody, "constructor") {
		t.Fatalf("symbol resource: %q err=%v", symBody, err)
	}
	if _, err := tools.LibrarySymbolResource("git.acme.local/platform/auth", "Nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	pkgBody, err := tools.LibraryPackageResource("git.acme.local/platform/auth", "git.acme.local/platform/auth")
	if err != nil || !strings.Contains(pkgBody, "NewClient") {
		t.Fatalf("package resource: %q err=%v", pkgBody, err)
	}
	if _, err := tools.LibraryPackageResource("git.acme.local/platform/auth", "git.acme.local/platform/auth/unknown"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound for unknown package, got %v", err)
	}
}

func TestParseLibURI(t *testing.T) {
	mod, kind, ref, err := parseLibURI("lib://git.acme.local/platform/auth/symbol/NewClient")
	if err != nil || mod != "git.acme.local/platform/auth" || kind != "symbol" || ref != "NewClient" {
		t.Fatalf("parse: mod=%q kind=%q ref=%q err=%v", mod, kind, ref, err)
	}
	mod2, kind2, _, err := parseLibURI("lib://git.acme.local/platform/auth")
	if err != nil || mod2 != "git.acme.local/platform/auth" || kind2 != "" {
		t.Fatalf("parse bare: mod=%q kind=%q err=%v", mod2, kind2, err)
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

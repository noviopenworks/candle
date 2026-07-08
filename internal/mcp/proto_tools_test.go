package mcp

import (
	"testing"

	"github.com/noviopenworks/candle/internal/store"
)

func seedProtoTools(t *testing.T) *Tools {
	t.Helper()
	s, _ := store.Open(":memory:")
	id, _ := s.UpsertIndex("acme", "inventory", "abc", "main", "/g")
	bundle := store.ProtoFileBundle{
		File: store.ProtoFile{Path: "proto/inventory.proto", Package: "acme.inventory"},
		Services: []store.ProtoServiceBundle{{
			Service: store.ProtoService{Name: "InventoryService", FullName: "acme.inventory.InventoryService"},
			RPCs: []store.ProtoRPC{{Name: "ReserveProduct",
				FullName:        "acme.inventory.InventoryService.ReserveProduct",
				RequestMessage:  "acme.inventory.ReserveProductRequest",
				ResponseMessage: "acme.inventory.ReserveProductResponse", StreamKind: "unary"}}}},
		Messages: []store.ProtoMessage{{Name: "ReserveProductRequest", FullName: "acme.inventory.ReserveProductRequest",
			Fields: []store.ProtoField{{Name: "sku", Type: "string", Number: 1, Label: "optional"}}}},
	}
	if err := s.ReplaceProtoFiles(id, []store.ProtoFileBundle{bundle}); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRPCImpls(id, []store.RPCImplLink{{
		RPCFullName: "acme.inventory.InventoryService.ReserveProduct",
		NodeID:      "n1", Confidence: 0.9, MatchReason: "name+service+signature"}}); err != nil {
		t.Fatal(err)
	}
	return NewTools(s)
}

func TestFindRPCAndFilter(t *testing.T) {
	tools := seedProtoTools(t)
	got, err := tools.FindRPC("acme/inventory", "reserve", "")
	if err != nil || len(got) != 1 || got[0].StreamKind != "unary" {
		t.Fatalf("find: %+v err=%v", got, err)
	}
	if filtered, err := tools.FindRPC("acme/inventory", "reserve", "bidi"); err != nil || len(filtered) != 0 {
		t.Fatalf("filter: %+v err=%v", filtered, err)
	}
}

func TestExplainRPC(t *testing.T) {
	tools := seedProtoTools(t)
	out, err := tools.ExplainRPC("acme/inventory", "InventoryService", "ReserveProduct")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if out.RPC.StreamKind != "unary" || len(out.ImplementedBy) != 1 || out.ImplementedBy[0].NodeID != "n1" {
		t.Fatalf("explain shape: %+v", out)
	}
	if out.ConsumedBy == "" {
		t.Fatalf("consumed_by should be a deferred marker, got empty")
	}
	if len(out.RequestMessageFields) != 1 || out.RequestMessageFields[0].Name != "sku" {
		t.Fatalf("request fields: %+v", out.RequestMessageFields)
	}
	if _, err := tools.ExplainRPC("acme/inventory", "InventoryService", "Nope"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestListAPIsIncludesProto(t *testing.T) {
	tools := seedProtoTools(t)
	apis, err := tools.ListAPIs("acme/inventory")
	if err != nil {
		t.Fatal(err)
	}
	var sawProto bool
	for _, a := range apis {
		if a.Kind == "protobuf" && a.Path == "proto/inventory.proto" {
			sawProto = true
		}
	}
	if !sawProto {
		t.Fatalf("list_apis missing protobuf entry: %+v", apis)
	}
}

func TestFindSchemaIncludesProtoMessage(t *testing.T) {
	tools := seedProtoTools(t)
	out, err := tools.FindSchema("acme/inventory", "Reserve")
	if err != nil {
		t.Fatal(err)
	}
	var sawMsg bool
	for _, sc := range out {
		if sc.Kind == "proto_message" && sc.Name == "ReserveProductRequest" {
			sawMsg = true
		}
	}
	if !sawMsg {
		t.Fatalf("find_schema missing proto_message: %+v", out)
	}
}

package mcp

import (
	"errors"
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
	if len(out.ConsumedBy) != 0 {
		t.Fatalf("consumed_by should be empty for a single-repo seed, got %v", out.ConsumedBy)
	}
	if len(out.RequestMessageFields) != 1 || out.RequestMessageFields[0].Name != "sku" {
		t.Fatalf("request fields: %+v", out.RequestMessageFields)
	}
	if _, err := tools.ExplainRPC("acme/inventory", "InventoryService", "Nope"); !errors.Is(err, ErrNotFound) {
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

// TestExplainRPCConsumedByCrossRepo verifies the heuristic consumer aggregation:
// a second repo whose graph has a node labelled like the RPC (a gRPC client-call
// signal) is reported in consumed_by, while the provider repo and any repo that
// defines the RPC are excluded.
func TestExplainRPCConsumedByCrossRepo(t *testing.T) {
	tools := seedProtoTools(t)
	s := tools.s

	// A consumer repo that calls ReserveProduct but does not define the proto.
	consID, _ := s.UpsertIndex("acme", "warehouse", "w1", "main", "/w")
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		consID, "client_reserve", "ReserveProduct", "code", "client.go"); err != nil {
		t.Fatal(err)
	}

	// A second provider that also defines the RPC: its ReserveProduct node must
	// NOT be counted as a consumer of the first provider.
	otherID, _ := s.UpsertIndex("acme", "inventory2", "i2", "main", "/i2")
	if _, err := s.DB.Exec(`INSERT INTO nodes(index_id,node_id,label,file_type,source_file) VALUES(?,?,?,?,?)`,
		otherID, "server_reserve", "ReserveProduct", "code", "server.go"); err != nil {
		t.Fatal(err)
	}
	other := store.ProtoFileBundle{
		File: store.ProtoFile{Path: "proto/inventory.proto", Package: "acme.inventory"},
		Services: []store.ProtoServiceBundle{{
			Service: store.ProtoService{Name: "InventoryService", FullName: "acme.inventory.InventoryService"},
			RPCs: []store.ProtoRPC{{Name: "ReserveProduct",
				FullName:        "acme.inventory.InventoryService.ReserveProduct",
				RequestMessage:  "acme.inventory.ReserveProductRequest",
				ResponseMessage: "acme.inventory.ReserveProductResponse", StreamKind: "unary"}}}},
	}
	if err := s.ReplaceProtoFiles(otherID, []store.ProtoFileBundle{other}); err != nil {
		t.Fatal(err)
	}

	out, err := tools.ExplainRPC("acme/inventory", "InventoryService", "ReserveProduct")
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	if len(out.ConsumedBy) != 1 || out.ConsumedBy[0] != "acme/warehouse" {
		t.Fatalf("consumed_by should be [acme/warehouse], got %v", out.ConsumedBy)
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

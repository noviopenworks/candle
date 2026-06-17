package grpc

import (
	"context"

	pb "example.com/repo/gen/inventorypb"
)

// Server implements the gRPC InventoryService.
type Server struct{}

// ReserveProduct is a unary RPC: (context.Context, *Req) (*Resp, error).
func (s *Server) ReserveProduct(ctx context.Context, req *pb.ReserveProductRequest) (*pb.ReserveProductResponse, error) {
	return nil, nil
}

// Sync is a server-streaming RPC: (*Req, <Svc>_<Rpc>Server) error.
func (s *Server) Sync(req *pb.SyncRequest, stream pb.InventoryService_SyncServer) error {
	return nil
}

// Upload is a client-streaming RPC: (<Svc>_<Rpc>Server) error.
func (s *Server) Upload(stream pb.InventoryService_UploadServer) error {
	return nil
}

// MultiLine has a signature spanning several source lines (unary shape).
func (s *Server) MultiLine(
	ctx context.Context,
	req *pb.MultiLineRequest,
) (
	*pb.MultiLineResponse,
	error,
) {
	return nil, nil
}

// FreeFunction is NOT a method (no receiver), so it must not match an RPC impl.
func FreeFunction(ctx context.Context, req *pb.ReserveProductRequest) (*pb.ReserveProductResponse, error) {
	return nil, nil
}

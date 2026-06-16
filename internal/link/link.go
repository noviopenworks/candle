// Package link matches contract operations (currently proto RPCs) to their
// implementation symbols in a repo's code graph. It is intentionally generic so
// the OpenAPI handler linker can adopt it later.
package link

import (
	"os"
	"strings"

	"github.com/vend-ai/intel-mcp/internal/store"
)

// RPC is the subset of an RPC the linker needs.
type RPC struct {
	FullName   string
	Service    string
	Name       string
	StreamKind string
}

const (
	confHigh   = 0.9
	confMedium = 0.6
	confLow    = 0.3
)

// MatchRPCs returns impl links for rpcs within a single index. Each candidate is
// recorded; ambiguous matches keep their tier rather than being dropped or
// collapsed.
func MatchRPCs(s *store.Store, indexID int64, rpcs []RPC) ([]store.RPCImplLink, error) {
	var out []store.RPCImplLink
	for _, r := range rpcs {
		nodes, err := s.NodesByLabel(indexID, r.Name)
		if err != nil {
			return nil, err
		}
		serviceRegistered, err := hasServiceRegistration(s, indexID, r.Service)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			conf, reason := score(n, r, serviceRegistered)
			out = append(out, store.RPCImplLink{
				RPCFullName: r.FullName, NodeID: n.NodeID, Confidence: conf, MatchReason: reason,
			})
		}
	}
	return out, nil
}

func hasServiceRegistration(s *store.Store, indexID int64, service string) (bool, error) {
	for _, label := range []string{"Register" + service + "Server", service + "Server"} {
		nodes, err := s.NodesByLabel(indexID, label)
		if err != nil {
			return false, err
		}
		if len(nodes) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func score(n store.NodeRow, r RPC, serviceRegistered bool) (float64, string) {
	reason := "name"
	conf := confLow
	if serviceRegistered {
		reason = "name+service"
		conf = confMedium
	}
	if signatureMatches(n.SourceFile, r.Name, r.StreamKind) {
		reason += "+signature"
		conf = confHigh
	}
	return conf, reason
}

// signatureMatches best-effort reads the impl source and checks the method's
// parameter shape against the RPC stream_kind. Unreadable source returns false
// (the caller keeps the lower-confidence name/service match).
func signatureMatches(sourceFile, rpcName, streamKind string) bool {
	if sourceFile == "" {
		return false
	}
	data, err := os.ReadFile(sourceFile)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.Contains(line, ")") || !strings.Contains(line, rpcName+"(") {
			continue
		}
		if !strings.Contains(line, "func") {
			continue
		}
		// Examine only the method's own parameter list, after the method name,
		// so the receiver (e.g. "(s *Server)") cannot be mistaken for a gRPC
		// stream-server parameter.
		params := line
		if i := strings.Index(line, rpcName+"("); i >= 0 {
			params = line[i+len(rpcName):]
		}
		// gRPC streaming methods take a generated stream type such as
		// "InventoryService_SyncServer" and have no context.Context parameter.
		streaming := strings.Contains(params, "Server)") || strings.Contains(params, "Server ")
		unary := strings.Contains(params, "context.Context") && !streaming
		switch streamKind {
		case "unary":
			return unary
		default:
			return streaming
		}
	}
	return false
}

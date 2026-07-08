package mcp

import (
	"fmt"
	"sort"

	"github.com/noviopenworks/candle/internal/store"
)

// SchemaInfo is a kind-discriminated find_schema entry (openapi_schema|proto_message).
type SchemaInfo struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	SpecPath string `json:"spec_path"`
}

// RPCExplanation is the explain_rpc result.
type RPCExplanation struct {
	RPC                   store.ProtoRPCResult `json:"rpc"`
	RequestMessageFields  []store.ProtoField   `json:"request_message_fields"`
	ResponseMessageFields []store.ProtoField   `json:"response_message_fields"`
	ImplementedBy         []store.ProtoRPCImpl `json:"implemented_by"`
	Calls                 []store.EdgeRow      `json:"calls"`
	ConsumedBy            []string             `json:"consumed_by"`
}

// FindRPC implements find_rpc (lexical match + optional stream_kind filter).
func (t *Tools) FindRPC(repo, query, streamKind string) ([]store.ProtoRPCResult, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, repoNotFound(repo)
	}
	return t.s.FindRPCs(ri.IndexID, query, streamKind)
}

// ExplainRPC implements explain_rpc: proto facts + resolved messages + same-repo
// impls + best-effort one-hop calls + cross-repo consumed_by. consumed_by is a
// heuristic: repos whose graph contains a node labelled like the RPC (a gRPC
// client-call signal), excluding the provider and any repo that defines the RPC.
// candle does not index gRPC client calls, so a label match is the available
// cross-repo signal.
func (t *Tools) ExplainRPC(repo, service, rpc string) (RPCExplanation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return RPCExplanation{}, err
	}
	if !ok {
		return RPCExplanation{}, repoNotFound(repo)
	}
	r, found, err := t.s.RPCByServiceName(ri.IndexID, service, rpc)
	if err != nil {
		return RPCExplanation{}, err
	}
	if !found {
		return RPCExplanation{}, notFound(fmt.Sprintf("rpc %s.%s not found in %s", service, rpc, repo))
	}
	out := RPCExplanation{RPC: r}
	out.RequestMessageFields = t.messageFields(ri.IndexID, r.RequestMessage)
	out.ResponseMessageFields = t.messageFields(ri.IndexID, r.ResponseMessage)
	impls, err := t.s.ProtoRPCImpls(ri.IndexID, r.FullName)
	if err != nil {
		return RPCExplanation{}, err
	}
	out.ImplementedBy = impls
	if best := bestImpl(impls); best != "" {
		calls, err := t.s.Callees(ri.IndexID, best)
		if err != nil {
			return RPCExplanation{}, err
		}
		out.Calls = calls
	}
	out.ConsumedBy = t.rpcConsumers(ri.IndexID, r)
	return out, nil
}

// rpcConsumers returns the distinct in-scope repos whose code graph contains a
// node labelled with the RPC name, excluding the provider and any repo that
// defines the RPC. Best-effort: candle does not index gRPC client calls, so a
// label match is the available cross-repo consumer signal.
func (t *Tools) rpcConsumers(providerIndexID int64, r store.ProtoRPCResult) []string {
	nodes, err := t.s.NodesByLabelAllIndexes(r.Name)
	if err != nil || len(nodes) == 0 {
		return nil
	}
	exclude := map[int64]bool{providerIndexID: true}
	if definers, derr := t.s.ProtoRPCDefiningIndexes(r.FullName); derr == nil {
		for _, id := range definers {
			exclude[id] = true
		}
	}
	all, err := t.reg.List()
	if err != nil {
		return nil
	}
	idToRepo := make(map[int64]string, len(all))
	for _, ri := range all {
		idToRepo[ri.IndexID] = ri.Repo
	}
	seen := map[string]bool{}
	var out []string
	for _, n := range nodes {
		if exclude[n.IndexID] || !t.reg.InScope(n.IndexID) {
			continue
		}
		repo := idToRepo[n.IndexID]
		if repo == "" || seen[repo] {
			continue
		}
		seen[repo] = true
		out = append(out, repo)
	}
	sort.Strings(out)
	return out
}

func (t *Tools) messageFields(indexID int64, fullName string) []store.ProtoField {
	if fullName == "" {
		return nil
	}
	msgs, err := t.s.FindMessages(indexID, fullName)
	if err != nil {
		return nil
	}
	for _, m := range msgs {
		if m.FullName == fullName {
			return m.Fields
		}
	}
	return nil
}

func bestImpl(impls []store.ProtoRPCImpl) string {
	var best string
	var top float64 = -1
	for _, im := range impls {
		if im.Confidence > top {
			top, best = im.Confidence, im.NodeID
		}
	}
	return best
}

package mcp

import "github.com/noviopenworks/candlegraph/internal/store"

// consumedByDeferred is the explicit marker returned until cross-repo consumer
// linking ships in a later change.
const consumedByDeferred = "deferred: cross-repo consumed_by not available in this change"

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
	ConsumedBy            string               `json:"consumed_by"`
}

// FindRPC implements find_rpc (lexical match + optional stream_kind filter).
func (t *Tools) FindRPC(repo, query, streamKind string) ([]store.ProtoRPCResult, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.FindRPCs(ri.IndexID, query, streamKind)
}

// ExplainRPC implements explain_rpc: proto facts + resolved messages + same-repo
// impls + best-effort one-hop calls + deferred consumed_by marker.
func (t *Tools) ExplainRPC(repo, service, rpc string) (RPCExplanation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return RPCExplanation{}, err
	}
	if !ok {
		return RPCExplanation{}, ErrNotFound
	}
	r, found, err := t.s.RPCByServiceName(ri.IndexID, service, rpc)
	if err != nil {
		return RPCExplanation{}, err
	}
	if !found {
		return RPCExplanation{}, ErrNotFound
	}
	out := RPCExplanation{RPC: r, ConsumedBy: consumedByDeferred}
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
	return out, nil
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

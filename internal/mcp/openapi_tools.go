package mcp

import "github.com/noviopenworks/candlegraph/internal/store"

// APIInfo is one entry in list_apis output (kind-discriminated for future contract kinds).
type APIInfo struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ListAPIs implements list_apis for a repo.
func (t *Tools) ListAPIs(repo string) ([]APIInfo, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	specs, err := t.s.ListAPISpecs(ri.IndexID)
	if err != nil {
		return nil, err
	}
	out := make([]APIInfo, 0, len(specs))
	for _, sp := range specs {
		out = append(out, APIInfo{Kind: sp.Kind, Name: sp.Name, Version: sp.Version, Path: sp.Path})
	}
	pfiles, err := t.s.ListProtoFiles(ri.IndexID)
	if err != nil {
		return nil, err
	}
	for _, pf := range pfiles {
		out = append(out, APIInfo{Kind: "protobuf", Name: pf.Package, Version: "", Path: pf.Path})
	}
	return out, nil
}

// FindEndpoint implements find_endpoint (lexical match).
func (t *Tools) FindEndpoint(repo, query string) ([]store.HTTPOperation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.FindOperations(ri.IndexID, query)
}

// HTTPOpImpl is one handler implementation link for explain_endpoint. Note the
// deliberate divergence from explain_rpc's ProtoRPCImpl, which exposes the raw
// confidence float: explain_endpoint renders the tier as an agent-facing label.
type HTTPOpImpl struct {
	Symbol     string `json:"symbol"`
	Confidence string `json:"confidence"` // HIGH | MEDIUM | LOW
	Reason     string `json:"reason"`
}

// EndpointExplanation is the explain_endpoint result: contract data plus the
// AST-linked handler symbol(s).
type EndpointExplanation struct {
	Operation     store.HTTPOperation `json:"operation"`
	ImplementedBy []HTTPOpImpl        `json:"implemented_by"`
}

// ExplainEndpoint implements explain_endpoint: contract data plus same-repo
// handler impl links (empty list when none).
func (t *Tools) ExplainEndpoint(repo, method, path string) (EndpointExplanation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return EndpointExplanation{}, err
	}
	if !ok {
		return EndpointExplanation{}, ErrNotFound
	}
	op, found, err := t.s.OperationByMethodPath(ri.IndexID, method, path)
	if err != nil {
		return EndpointExplanation{}, err
	}
	if !found {
		return EndpointExplanation{}, ErrNotFound
	}
	out := EndpointExplanation{Operation: op, ImplementedBy: []HTTPOpImpl{}}
	links, err := t.s.HTTPOpImpls(ri.IndexID, method, path)
	if err != nil {
		return EndpointExplanation{}, err
	}
	for _, l := range links {
		out.ImplementedBy = append(out.ImplementedBy, HTTPOpImpl{
			Symbol: l.NodeID, Confidence: tierLabel(l.Confidence), Reason: l.MatchReason})
	}
	return out, nil
}

// tierLabel maps a confidence float to its agent-facing tier name. Thresholds
// sit between the linker's tier constants (link.confHigh=0.9, confMedium=0.6,
// confLow=0.3) so float rounding cannot cross a boundary; if those constants are
// retuned, revisit these midpoints.
func tierLabel(conf float64) string {
	switch {
	case conf >= 0.85:
		return "HIGH"
	case conf >= 0.5:
		return "MEDIUM"
	default:
		return "LOW"
	}
}

// FindSchema implements find_schema (OpenAPI schemas + proto messages).
func (t *Tools) FindSchema(repo, query string) ([]SchemaInfo, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	out := []SchemaInfo{}
	schemas, err := t.s.FindSchemas(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	for _, sc := range schemas {
		out = append(out, SchemaInfo{Kind: "openapi_schema", Name: sc.Name, SpecPath: sc.SpecPath})
	}
	msgs, err := t.s.FindMessages(ri.IndexID, query)
	if err != nil {
		return nil, err
	}
	for _, m := range msgs {
		out = append(out, SchemaInfo{Kind: "proto_message", Name: m.Name, SpecPath: m.ProtoPath})
	}
	return out, nil
}

package mcp

import "github.com/vend-ai/intel-mcp/internal/store"

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

// ExplainEndpoint implements explain_endpoint (contract data only — no handler/service_flow).
func (t *Tools) ExplainEndpoint(repo, method, path string) (store.HTTPOperation, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return store.HTTPOperation{}, err
	}
	if !ok {
		return store.HTTPOperation{}, ErrNotFound
	}
	op, found, err := t.s.OperationByMethodPath(ri.IndexID, method, path)
	if err != nil {
		return store.HTTPOperation{}, err
	}
	if !found {
		return store.HTTPOperation{}, ErrNotFound
	}
	return op, nil
}

// FindSchema implements find_schema (OpenAPI schemas).
func (t *Tools) FindSchema(repo, query string) ([]store.APISchema, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotFound
	}
	return t.s.FindSchemas(ri.IndexID, query)
}

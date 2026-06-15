package mcp

import (
	"encoding/json"

	"github.com/vend-ai/intel-mcp/internal/store"
)

// mustJSON marshals v to indented JSON, returning an error message string on
// failure (never panics) so it is safe to embed in tool results.
func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return `{"error":"marshal failed"}`
	}
	return string(b)
}

// RepoResource returns the JSON snapshot summary for repo://org/name.
func (t *Tools) RepoResource(repo string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(ri, "", "  ")
	return string(b), err
}

// GraphNodeResource returns the JSON for a node behind
// graph://org/name/commit/<sha>/node/<node_id>.
func (t *Tools) GraphNodeResource(repo, nodeID string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	node, found, err := t.s.NodeByID(ri.IndexID, nodeID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(node, "", "  ")
	return string(b), err
}

// OperationResource returns the JSON for an operation behind
// openapi://org/name/commit/<sha>/operation/<operationId>.
func (t *Tools) OperationResource(repo, operationID string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	op, found, err := t.s.OperationByID(ri.IndexID, operationID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNotFound
	}
	b, err := json.MarshalIndent(op, "", "  ")
	return string(b), err
}

// SchemaResource returns the JSON for a schema behind
// openapi://org/name/commit/<sha>/schema/<name>, filtered to an exact name match.
func (t *Tools) SchemaResource(repo, name string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	matches, err := t.s.FindSchemas(ri.IndexID, name)
	if err != nil {
		return "", err
	}
	for _, sc := range matches {
		if sc.Name == name {
			b, err := json.MarshalIndent(sc, "", "  ")
			return string(b), err
		}
	}
	return "", ErrNotFound
}

// SpecResource returns the JSON for a spec behind
// openapi://org/name/commit/<sha>/spec/<path>: the spec metadata plus its operations.
func (t *Tools) SpecResource(repo, path string) (string, error) {
	ri, ok, err := t.reg.Resolve(repo)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrNotFound
	}
	specs, err := t.s.ListAPISpecs(ri.IndexID)
	if err != nil {
		return "", err
	}
	for _, sp := range specs {
		if sp.Path != path {
			continue
		}
		ops, err := t.s.FindOperations(ri.IndexID, "")
		if err != nil {
			return "", err
		}
		var specOps []store.HTTPOperation
		for _, op := range ops {
			if op.SpecPath == sp.Path {
				specOps = append(specOps, op)
			}
		}
		out := struct {
			Spec       store.APISpecRow      `json:"spec"`
			Operations []store.HTTPOperation `json:"operations"`
		}{Spec: sp, Operations: specOps}
		b, err := json.MarshalIndent(out, "", "  ")
		return string(b), err
	}
	return "", ErrNotFound
}

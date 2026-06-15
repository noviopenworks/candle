package mcp

import "encoding/json"

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
